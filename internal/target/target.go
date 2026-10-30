package target

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/spacelift-solutions/Mayday/internal/license"
	"github.com/spacelift-solutions/Mayday/internal/model"
)

type Server struct {
	mu                                  sync.Mutex
	store                               license.Store
	workers, queueLimit, active, queued int
	workTime                            time.Duration
	completed, rejected                 uint64
	licenseStatus, detail, bucket, key  string
	checkedAt                           time.Time
	permits                             chan struct{}
}

func New(store license.Store, workers, queue int, workTime time.Duration, bucket, key string) *Server {
	return &Server{store: store, workers: workers, queueLimit: queue, workTime: workTime, bucket: bucket, key: key, licenseStatus: "checking", permits: make(chan struct{}, workers)}
}

func (s *Server) Check(ctx context.Context) {
	c, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := s.store.Check(c)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.licenseStatus = license.Status(err)
	s.detail = ""
	if err != nil {
		s.detail = err.Error()
	}
	s.checkedAt = time.Now().UTC()
}

func (s *Server) Watch(ctx context.Context) {
	s.Check(ctx)
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Check(ctx)
		}
	}
}

func (s *Server) Health() model.Health {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := "healthy"
	if s.licenseStatus != "valid" || time.Since(s.checkedAt) > 10*time.Second {
		status = "degraded"
	}
	return model.Health{Service: "mayday", Status: status, License: s.licenseStatus, Detail: s.detail, Bucket: s.bucket, Key: s.key, Workers: s.workers, Active: s.active, Queued: s.queued, Completed: s.completed, Rejected: s.rejected, CheckedAt: s.checkedAt}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("alive\n"))
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.Health()) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		h := s.Health()
		status := 200
		if h.Status != "healthy" {
			status = 503
		}
		writeJSON(w, status, h)
	})
	mux.HandleFunc("POST /work", s.work)
	return mux
}

func (s *Server) work(w http.ResponseWriter, r *http.Request) {
	if s.Health().Status != "healthy" {
		writeJSON(w, 503, map[string]string{"error": "required license unavailable"})
		return
	}
	s.mu.Lock()
	if s.active+s.queued >= s.workers+s.queueLimit {
		s.rejected++
		s.mu.Unlock()
		writeJSON(w, 429, map[string]string{"error": "capacity exceeded"})
		return
	}
	s.queued++
	s.mu.Unlock()
	select {
	case s.permits <- struct{}{}:
	case <-r.Context().Done():
		s.mu.Lock()
		s.queued--
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	s.queued--
	s.active++
	s.mu.Unlock()
	defer func() { <-s.permits; s.mu.Lock(); s.active--; s.mu.Unlock() }()
	t := time.NewTimer(s.workTime)
	defer t.Stop()
	select {
	case <-r.Context().Done():
		return
	case <-t.C:
	}
	s.mu.Lock()
	s.completed++
	s.mu.Unlock()
	writeJSON(w, 200, map[string]string{"status": "completed", "workload": "synthetic I/O"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
