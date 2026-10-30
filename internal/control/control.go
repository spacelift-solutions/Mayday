package control

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spacelift-solutions/Mayday/internal/license"
	"github.com/spacelift-solutions/Mayday/internal/model"
)

type Config struct {
	TargetURL, Password, WebhookURL, WebhookToken, StatePath, StackID string
	SecureCookies                                                     bool
	PollInterval                                                      time.Duration
	Failures, Recoveries                                              int
	IncidentTimeout, Cooldown                                         time.Duration
}
type Faults interface{ Stop(context.Context) error }
type State struct {
	Rate          int               `json:"rate"`
	Observation   model.Observation `json:"observation"`
	Incident      *model.Incident   `json:"incident,omitempty"`
	History       []model.Event     `json:"history"`
	Outbox        []model.Event     `json:"-"`
	DeliveryError string            `json:"delivery_error,omitempty"`
}
type diskState struct {
	State  State         `json:"state"`
	Outbox []model.Event `json:"outbox"`
}
type Server struct {
	mu                sync.Mutex
	cfg               Config
	state             State
	store             license.Store
	faults            Faults
	client            *http.Client
	samples, failures uint64
	totalLatency      time.Duration
	bad, good         int
	candidate         string
	lastClosed        time.Time
	activeRequests    chan struct{}
	auth              *sessionAuth
}

func New(cfg Config, store license.Store, faults Faults) (*Server, error) {
	u, err := url.Parse(cfg.TargetURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("TARGET_URL must be an HTTP(S) URL")
	}
	if len(cfg.Password) < 12 {
		return nil, errors.New("DEMO_PASSWORD must contain at least 12 characters")
	}
	if cfg.WebhookURL != "" {
		w, e := url.Parse(cfg.WebhookURL)
		if e != nil || w.Scheme != "https" || w.Host == "" {
			return nil, errors.New("FLOWS_WEBHOOK_URL must use HTTPS")
		}
		if len(cfg.WebhookToken) < 16 {
			return nil, errors.New("FLOWS_WEBHOOK_TOKEN must contain at least 16 characters")
		}
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.Failures <= 0 {
		cfg.Failures = 3
	}
	if cfg.Recoveries <= 0 {
		cfg.Recoveries = 3
	}
	if cfg.IncidentTimeout <= 0 {
		cfg.IncidentTimeout = 5 * time.Minute
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = 30 * time.Second
	}
	s := &Server{cfg: cfg, store: store, faults: faults, client: &http.Client{Timeout: 4 * time.Second}, state: State{Rate: 2, History: []model.Event{}}, activeRequests: make(chan struct{}, 128)}
	s.auth = newSessionAuth(cfg.Password, cfg.SecureCookies)
	if cfg.StatePath != "" {
		b, e := os.ReadFile(cfg.StatePath)
		if e == nil {
			var d diskState
			if e = json.Unmarshal(b, &d); e != nil {
				return nil, fmt.Errorf("read saved monitor state: %w", e)
			}
			s.state = d.State
			s.state.Outbox = d.Outbox
		} else if !errors.Is(e, os.ErrNotExist) {
			return nil, e
		}
	}
	if s.state.Rate < 0 || s.state.Rate > 80 {
		return nil, errors.New("invalid saved request rate")
	}
	return s, nil
}

func (s *Server) persistLocked() {
	if s.cfg.StatePath == "" {
		return
	}
	b, err := json.Marshal(diskState{State: s.state, Outbox: s.state.Outbox})
	if err == nil {
		err = os.MkdirAll(filepath.Dir(s.cfg.StatePath), 0700)
	}
	if err == nil {
		err = os.WriteFile(s.cfg.StatePath+".tmp", b, 0600)
	}
	if err == nil {
		err = os.Rename(s.cfg.StatePath+".tmp", s.cfg.StatePath)
	}
	if err != nil {
		s.state.DeliveryError = "Monitor persistence failed"
		slog.Error("persist monitor state", "error", err)
	}
}

func id() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func copyIncident(i *model.Incident) *model.Incident {
	if i == nil {
		return nil
	}
	v := *i
	return &v
}
func (s *Server) eventLocked(kind string) {
	e := model.Event{ID: id(), Type: kind, Service: "mayday", At: time.Now().UTC(), Incident: copyIncident(s.state.Incident), Observation: s.state.Observation, StackID: s.cfg.StackID}
	if kind != "observation" {
		s.state.History = append(s.state.History, e)
		if len(s.state.History) > 100 {
			s.state.History = s.state.History[len(s.state.History)-100:]
		}
	}
	if s.cfg.WebhookURL != "" {
		// Only the newest pending observation matters. Incident events retain order.
		if kind == "observation" {
			out := s.state.Outbox[:0]
			for _, v := range s.state.Outbox {
				if v.Type != "observation" {
					out = append(out, v)
				}
			}
			s.state.Outbox = out
		}
		s.state.Outbox = append(s.state.Outbox, e)
	}
	s.persistLocked()
}

func (s *Server) Run(ctx context.Context) {
	go s.generate(ctx)
	go s.deliver(ctx)
	s.Poll(ctx)
	t := time.NewTicker(s.cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Poll(ctx)
		}
	}
}

func (s *Server) Poll(ctx context.Context) {
	o := model.Observation{At: time.Now().UTC()}
	r, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(s.cfg.TargetURL, "/")+"/health", nil)
	if err == nil {
		resp, e := s.client.Do(r)
		if e == nil {
			o.Reachable = resp.StatusCode == 200 && json.NewDecoder(io.LimitReader(resp.Body, 16384)).Decode(&o.Health) == nil
			_ = resp.Body.Close()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	o.RequestRate = s.state.Rate
	o.Samples = s.samples
	o.Errors = s.failures
	if s.samples > 0 {
		o.ErrorRate = float64(s.failures) / float64(s.samples)
		o.LatencyMS = float64(s.totalLatency.Microseconds()) / 1000 / float64(s.samples)
	}
	s.samples = 0
	s.failures = 0
	s.totalLatency = 0
	s.state.Observation = o
	s.evaluateLocked(o)
	s.eventLocked("observation")
}

func classify(o model.Observation) string {
	if !o.Reachable {
		return "outage"
	}
	if o.Health.Status != "healthy" {
		return "dependency"
	}
	if o.Samples >= 2 && (o.ErrorRate > 0.1 || o.LatencyMS > 600) {
		return "capacity_up"
	}
	if o.RequestRate <= 4 && o.Health.Workers > 2 {
		return "capacity_down"
	}
	return ""
}

func recovered(kind string, o model.Observation) bool {
	if !o.Reachable || o.Health.Status != "healthy" || o.Samples == 0 || o.Errors > 0 || o.LatencyMS > 600 {
		return false
	}
	switch kind {
	case "capacity_up":
		return o.Health.Workers >= 8
	case "capacity_down", "reset":
		return o.Health.Workers == 2
	default:
		return true
	}
}

func (s *Server) evaluateLocked(o model.Observation) {
	if i := s.state.Incident; i != nil && i.Status != "recovered" {
		if recovered(i.Kind, o) {
			s.good++
		} else {
			s.good = 0
		}
		if s.good >= s.cfg.Recoveries {
			now := o.At
			i.Status = "recovered"
			i.ClosedAt = &now
			i.Message = "Independent probes and work requests confirm recovery"
			s.lastClosed = now
			s.good = 0
			s.bad = 0
			s.eventLocked("incident.recovered")
		} else if i.Status != "failed" && o.At.Sub(i.OpenedAt) > s.cfg.IncidentTimeout {
			i.Status = "failed"
			i.Message = "Recovery timed out; inspect the Spacelift run and retry after resolving the cause"
			s.eventLocked("incident.failed")
		}
		return
	}
	kind := classify(o)
	if kind == "" {
		s.bad = 0
		s.candidate = ""
		return
	}
	if kind == "capacity_down" && o.At.Sub(s.lastClosed) < s.cfg.Cooldown {
		return
	}
	if kind != s.candidate {
		s.bad = 0
		s.candidate = kind
	}
	s.bad++
	if s.bad < s.cfg.Failures {
		return
	}
	s.openLocked(kind, o.At)
}

func (s *Server) openLocked(kind string, now time.Time) {
	s.state.Incident = &model.Incident{ID: id(), Kind: kind, Status: "open", OpenedAt: now, Message: "Detected by independent monitoring"}
	s.bad = 0
	s.good = 0
	s.eventLocked("incident.opened")
}

func (s *Server) generate(ctx context.Context) {
	// A fixed-rate generator retains load even when the target slows down.
	t := time.NewTicker(125 * time.Millisecond)
	defer t.Stop()
	credit := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.mu.Lock()
			credit += s.state.Rate
			s.mu.Unlock()
			for credit >= 8 {
				credit -= 8
				select {
				case s.activeRequests <- struct{}{}:
					go s.request(ctx)
				default:
					s.mu.Lock()
					s.samples++
					s.failures++
					s.mu.Unlock()
				}
			}
		}
	}
}

func (s *Server) request(ctx context.Context) {
	defer func() { <-s.activeRequests }()
	start := time.Now()
	failed := true
	r, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(s.cfg.TargetURL, "/")+"/work", nil)
	if err == nil {
		resp, e := s.client.Do(r)
		if e == nil {
			failed = resp.StatusCode != 200
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 16384))
			_ = resp.Body.Close()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples++
	if failed {
		s.failures++
	}
	s.totalLatency += time.Since(start)
}

func (s *Server) deliver(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for n := 0; n < 5; n++ {
				s.mu.Lock()
				if len(s.state.Outbox) == 0 {
					s.mu.Unlock()
					break
				}
				e := s.state.Outbox[0]
				s.mu.Unlock()
				b, _ := json.Marshal(e)
				r, err := http.NewRequestWithContext(ctx, "POST", s.cfg.WebhookURL, bytes.NewReader(b))
				if err == nil {
					r.Header.Set("Content-Type", "application/json")
					r.Header.Set("Authorization", "Bearer "+s.cfg.WebhookToken)
					r.Header.Set("Idempotency-Key", e.ID)
					resp, x := s.client.Do(r)
					err = x
					if x == nil {
						_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
						_ = resp.Body.Close()
						if resp.StatusCode < 200 || resp.StatusCode >= 300 {
							err = fmt.Errorf("webhook returned %d", resp.StatusCode)
						}
					}
				}
				s.mu.Lock()
				if err != nil {
					s.state.DeliveryError = "Flows delivery failed; retrying"
					s.persistLocked()
					s.mu.Unlock()
					break
				}
				s.state.DeliveryError = ""
				for i, v := range s.state.Outbox {
					if v.ID == e.ID {
						s.state.Outbox = append(s.state.Outbox[:i], s.state.Outbox[i+1:]...)
						break
					}
				}
				s.persistLocked()
				s.mu.Unlock()
			}
		}
	}
}

func (s *Server) Handler(ui http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", ui)
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("POST /api/login", s.auth.login)
	api := http.NewServeMux()
	api.HandleFunc("POST /api/logout", s.auth.logout)
	api.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		respond(w, 200, s.state)
	})
	api.HandleFunc("POST /api/traffic", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Rate int `json:"rate"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&v) != nil || v.Rate < 1 || v.Rate > 80 {
			respond(w, 400, map[string]string{"error": "rate must be between 1 and 80"})
			return
		}
		s.mu.Lock()
		s.state.Rate = v.Rate
		s.eventLocked("traffic.changed")
		s.mu.Unlock()
		respond(w, 200, v)
	})
	api.HandleFunc("POST /api/fault/license", func(w http.ResponseWriter, r *http.Request) {
		if !s.canFault() {
			respond(w, 409, map[string]string{"error": "Resolve the current incident first"})
			return
		}
		s.action(w, r, func(ctx context.Context) error { return s.store.Delete(ctx) }, "fault.license")
	})
	api.HandleFunc("POST /api/fault/outage", func(w http.ResponseWriter, r *http.Request) {
		if !s.canFault() {
			respond(w, 409, map[string]string{"error": "Resolve the current incident first"})
			return
		}
		if s.faults == nil {
			respond(w, 409, map[string]string{"error": "SSM outage injection is not configured; stop the local target process manually"})
			return
		}
		s.action(w, r, s.faults.Stop, "fault.outage")
	})
	api.HandleFunc("POST /api/retry", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.state.Incident == nil || s.state.Incident.Status != "failed" {
			respond(w, 409, map[string]string{"error": "No failed incident to retry"})
			return
		}
		s.state.Incident.Status = "open"
		s.state.Incident.OpenedAt = time.Now().UTC()
		s.good = 0
		s.eventLocked("incident.opened")
		respond(w, 202, s.state.Incident)
	})
	api.HandleFunc("POST /api/reset", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.state.Incident != nil && s.state.Incident.Status != "recovered" {
			respond(w, 409, map[string]string{"error": "Resolve the active incident before resetting"})
			return
		}
		s.state.Rate = 2
		s.openLocked("reset", time.Now().UTC())
		respond(w, 202, s.state.Incident)
	})
	mux.Handle("/api/", s.auth.protect(api))
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond(w, 403, map[string]string{"error": "Cross-origin request rejected"})
	}))
	return protection.Handler(mux)
}

func (s *Server) canFault() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Incident == nil || s.state.Incident.Status == "recovered"
}
func (s *Server) action(w http.ResponseWriter, r *http.Request, f func(context.Context) error, kind string) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := f(ctx); err != nil {
		slog.Error("fault injection failed", "kind", kind, "error", err)
		respond(w, 502, map[string]string{"error": "Fault injection failed; inspect monitor logs"})
		return
	}
	s.mu.Lock()
	s.eventLocked(kind)
	s.mu.Unlock()
	respond(w, 202, map[string]string{"status": "fault injected; awaiting independent detection"})
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
