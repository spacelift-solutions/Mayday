package control

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/spacelift-solutions/Mayday/internal/model"
)

var testPassword = rand.Text()

type fakeStore struct{ deletes int }

func (*fakeStore) Check(context.Context) error    { return nil }
func (f *fakeStore) Delete(context.Context) error { f.deletes++; return nil }

func newTestServer(t *testing.T) *Server {
	t.Helper()
	s, e := New(Config{TargetURL: "http://localhost:8081", Password: testPassword, Failures: 2, Recoveries: 2}, &fakeStore{}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func healthy(workers int) model.Observation {
	return model.Observation{At: time.Now().UTC(), Reachable: true, Samples: 4, RequestRate: 12, LatencyMS: 200, Health: model.Health{Status: "healthy", Workers: workers, License: "valid"}}
}

func TestIncidentsDebounceAndRequireRecoveryEvidence(t *testing.T) {
	s := newTestServer(t)
	o := healthy(2)
	o.Reachable = false
	s.evaluateLocked(o)
	if s.state.Incident != nil {
		t.Fatal("single failure opened incident")
	}
	s.evaluateLocked(o)
	if s.state.Incident == nil || s.state.Incident.Kind != "outage" {
		t.Fatal("sustained outage not detected")
	}
	incidentID := s.state.Incident.ID
	s.evaluateLocked(o)
	if s.state.Incident.ID != incidentID {
		t.Fatal("duplicate incident")
	}
	o = healthy(2)
	o.Samples = 0
	s.evaluateLocked(o)
	s.evaluateLocked(o)
	if s.state.Incident.Status == "recovered" {
		t.Fatal("recovery without successful work")
	}
	o.Samples = 4
	s.evaluateLocked(o)
	s.evaluateLocked(o)
	if s.state.Incident.Status != "recovered" {
		t.Fatal("successful independent probes did not close incident")
	}
}
func TestClassificationAndBoundedScaling(t *testing.T) {
	o := healthy(2)
	o.ErrorRate = .5
	if classify(o) != "capacity_up" {
		t.Fatal("overload not detected")
	}
	o.Health.Status = "degraded"
	o.Health.License = "missing"
	if classify(o) != "dependency" {
		t.Fatal("license outage classified as capacity")
	}
	o = healthy(8)
	o.RequestRate = 2
	if classify(o) != "capacity_down" {
		t.Fatal("low load not detected")
	}
	o = healthy(2)
	if recovered("capacity_up", o) {
		t.Fatal("capacity incident closed without increase")
	}
	o.Health.Workers = 8
	if !recovered("capacity_up", o) {
		t.Fatal("healthy increased capacity rejected")
	}
}
func TestPersistedOutboxAndRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	cfg := Config{TargetURL: "http://localhost:8081", Password: testPassword, StatePath: path, WebhookURL: "https://example.test/events", WebhookToken: "test-webhook-token-only"}
	s, err := New(cfg, &fakeStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.openLocked("dependency", time.Now().UTC())
	s.eventLocked("observation")
	s.eventLocked("observation")
	r, err := New(cfg, &fakeStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.state.Outbox) != 2 || r.state.Incident.ID != s.state.Incident.ID {
		t.Fatal("incident or delivery queue was lost on restart")
	}
	b, _ := json.Marshal(r.state)
	if bytes.Contains(b, []byte("outbox")) {
		t.Fatal("internal delivery queue exposed")
	}
}
func TestTimeoutDoesNotClaimRecovery(t *testing.T) {
	s := newTestServer(t)
	s.openLocked("outage", time.Now().Add(-10*time.Minute))
	s.evaluateLocked(model.Observation{At: time.Now()})
	if s.state.Incident.Status != "failed" {
		t.Fatal("timeout must remain visible")
	}
}
