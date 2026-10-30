package target

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spacelift-solutions/Mayday/internal/license"
)

func TestLicenseLossAndRestoration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "license.json")
	s := New(license.File{Path: path}, 2, 2, time.Millisecond, "bucket", "key")
	s.Check(context.Background())
	for _, route := range []string{"/readyz", "/work"} {
		r := httptest.NewRequest(http.MethodGet, route, nil)
		if route == "/work" {
			r.Method = http.MethodPost
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 503 {
			t.Fatalf("%s: got %d", route, w.Code)
		}
	}
	if err := os.WriteFile(path, []byte(`{"service":"mayday","license_key":"DEMO-ONLY","enabled":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	s.Check(context.Background())
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/work", nil))
	if w.Code != 200 {
		t.Fatalf("restored license should permit work: %d", w.Code)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s.Check(context.Background())
	if s.Health().Status != "degraded" {
		t.Fatal("deletion did not degrade service")
	}
}

type validStore struct{}

func (validStore) Check(context.Context) error  { return nil }
func (validStore) Delete(context.Context) error { return nil }

func TestCapacityAndCancellation(t *testing.T) {
	s := New(validStore{}, 1, 0, time.Hour, "", "")
	s.Check(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/work", nil).WithContext(ctx))
	}()
	deadline := time.Now().Add(time.Second)
	for s.Health().Active != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if s.Health().Active != 1 {
		t.Fatal("request never started")
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/work", nil))
	if w.Code != 429 {
		t.Fatalf("expected bounded overload, got %d", w.Code)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled work leaked")
	}
	if s.Health().Active != 0 || s.Health().Queued != 0 {
		t.Fatal("cancelled work retained capacity")
	}
}
