package control

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spacelift-solutions/Mayday/internal/model"
)

const callbackToken = "test-callback-token-only"

func callback(t *testing.T, s *Server, result model.FlowResult) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return callbackBody(s, string(body), "Bearer "+callbackToken)
}
func callbackBody(s *Server, body, auth string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/api/flows/result", strings.NewReader(body))
	r.Header.Set("Authorization", auth)
	w := httptest.NewRecorder()
	s.Handler(http.NotFoundHandler()).ServeHTTP(w, r)
	return w
}
func callbackServer(t *testing.T) (*Server, model.FlowResult) {
	t.Helper()
	s := newTestServer(t)
	s.cfg.WebhookToken = callbackToken
	s.openLocked("outage", time.Now().UTC())
	return s, model.FlowResult{IncidentID: s.state.Incident.ID, AttemptID: s.state.Incident.AttemptID, Status: "succeeded", RunID: "run-123", RunState: "FINISHED"}
}

func TestFlowCallbackAuthAndValidation(t *testing.T) {
	s, result := callbackServer(t)
	body, _ := json.Marshal(result)
	for _, auth := range []string{"", "Bearer wrong", "Basic " + callbackToken} {
		if w := callbackBody(s, string(body), auth); w.Code != 401 {
			t.Fatalf("auth: %d", w.Code)
		}
	}
	s.cfg.WebhookToken = ""
	if w := callbackBody(s, string(body), "Bearer "); w.Code != 401 {
		t.Fatal("empty token allowed")
	}
	s.cfg.WebhookToken = callbackToken
	for _, body := range []string{`{}`, string(body) + `{}`, `{"status":"recovered"}`, strings.Repeat("x", 4097)} {
		if w := callbackBody(s, body, "Bearer "+callbackToken); w.Code != 400 {
			t.Fatalf("invalid body: %d", w.Code)
		}
	}
	for _, mutate := range []func(*model.FlowResult){
		func(r *model.FlowResult) { r.Status = "recovered" },
		func(r *model.FlowResult) { r.RunState = "FAILED" },
		func(r *model.FlowResult) { r.RunID = "" },
		func(r *model.FlowResult) { r.IncidentID = "bad" },
		func(r *model.FlowResult) { r.Status = "evidence_ready" },
	} {
		bad := result
		mutate(&bad)
		if w := callback(t, s, bad); w.Code != 400 {
			t.Fatalf("invalid result accepted: %+v", bad)
		}
	}
	if s.state.Incident.FlowResult != nil {
		t.Fatal("invalid callback mutated incident")
	}
	// A dashboard session is not a callback credential, and the callback token
	// cannot access the operator API.
	handler := s.Handler(http.NotFoundHandler())
	cookie := loginCookie(t, handler)
	request := httptest.NewRequest("POST", "/api/flows/result", bytes.NewReader(body))
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 401 {
		t.Fatal("dashboard session accepted as callback credential")
	}
	r := httptest.NewRequest("GET", "/api/state", nil)
	r.Header.Set("Authorization", "Bearer "+callbackToken)
	w := httptest.NewRecorder()
	s.Handler(http.NotFoundHandler()).ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("callback token granted operator access")
	}
}

func TestFlowResultPersistsButDoesNotProveRecovery(t *testing.T) {
	s, result := callbackServer(t)
	s.cfg.StatePath = filepath.Join(t.TempDir(), "state.json")
	if w := callback(t, s, result); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if s.state.Incident.Status != "open" {
		t.Fatal("run success claimed health")
	}
	restarted, err := New(s.cfg, &fakeStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if *restarted.state.Incident.FlowResult != result {
		t.Fatal("result lost on restart")
	}
	s = restarted
	if w := callback(t, s, result); !bytes.Contains(w.Body.Bytes(), []byte(`"duplicate":true`)) {
		t.Fatal("duplicate not acknowledged")
	}
	result.Status = "failed"
	callback(t, s, result)
	if s.state.Incident.FlowResult.Status != "succeeded" {
		t.Fatal("conflicting terminal result replaced first outcome")
	}
	o := healthy(2)
	o.Samples = 0
	s.evaluateLocked(o)
	s.evaluateLocked(o)
	if s.state.Incident.Status == "recovered" {
		t.Fatal("recovery without work")
	}
	o.Samples = 4
	s.evaluateLocked(o)
	s.evaluateLocked(o)
	if s.state.Incident.Status != "recovered" {
		t.Fatal("independent evidence did not recover")
	}
}

func TestStartedCallbackCanFinishAndCannotOverwriteTerminalResult(t *testing.T) {
	for _, terminal := range []string{"succeeded", "failed"} {
		t.Run(terminal, func(t *testing.T) {
			s, result := callbackServer(t)
			s.cfg.StatePath = filepath.Join(t.TempDir(), "state.json")
			started := result
			started.Status, started.RunState = "started", "QUEUED"
			if w := callback(t, s, started); w.Code != 200 || s.state.Incident.Status != "open" {
				t.Fatal("started callback must preserve the open incident")
			}
			restarted, err := New(s.cfg, &fakeStore{}, nil)
			if err != nil || restarted.state.Incident.FlowResult.Status != "started" {
				t.Fatal("started run did not persist")
			}
			s = restarted
			if w := callback(t, s, started); !bytes.Contains(w.Body.Bytes(), []byte(`"duplicate":true`)) {
				t.Fatal("duplicate start was not acknowledged")
			}
			result.Status = terminal
			if terminal == "failed" {
				result.RunID, result.RunState = "", ""
			}
			if w := callback(t, s, result); w.Code != 200 {
				t.Fatal(w.Body.String())
			}
			if s.state.Incident.FlowResult.Status != terminal || s.state.Incident.FlowResult.RunID != started.RunID {
				t.Fatal("terminal result must replace start and retain the run link")
			}
			callback(t, s, started)
			if s.state.Incident.FlowResult.Status != terminal {
				t.Fatal("delayed start overwrote terminal result")
			}
		})
	}
}

func TestStartedCallbackValidationAndStaleAttempt(t *testing.T) {
	s, result := callbackServer(t)
	result.Status, result.RunState = "started", "QUEUED"
	for _, field := range []string{"id", "state"} {
		bad := result
		if field == "id" {
			bad.RunID = ""
		} else {
			bad.RunState = ""
		}
		if w := callback(t, s, bad); w.Code != 400 {
			t.Fatal("incomplete start accepted")
		}
	}
	result.AttemptID = "000000000000000000000000"
	if w := callback(t, s, result); !bytes.Contains(w.Body.Bytes(), []byte(`"stale":true`)) || s.state.Incident.FlowResult != nil {
		t.Fatal("stale start must not change the current attempt")
	}
}

func TestFlowFailureRetryAndDelayedCallbacks(t *testing.T) {
	s, result := callbackServer(t)
	result.Status = "failed"
	result.RunState = "FAILED"
	callback(t, s, result)
	if s.state.Incident.Status != "failed" {
		t.Fatal("flow failure not visible")
	}
	handler := s.Handler(http.NotFoundHandler())
	request := httptest.NewRequest("POST", "/api/retry", nil)
	request.AddCookie(loginCookie(t, handler))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 202 {
		t.Fatalf("retry failed: %d", response.Code)
	}
	if s.state.Incident.FlowResult != nil || s.state.Incident.AttemptID == result.AttemptID {
		t.Fatal("retry retained old result or attempt ID")
	}
	if w := callback(t, s, result); !bytes.Contains(w.Body.Bytes(), []byte(`"stale":true`)) {
		t.Fatal("old attempt not ignored")
	}
	if s.state.Incident.Status != "open" || s.state.Incident.FlowResult != nil {
		t.Fatal("stale callback overwrote retry")
	}
	result.AttemptID = s.state.Incident.AttemptID
	s.state.Incident.Status = "recovered"
	callback(t, s, result)
	if s.state.Incident.Status != "recovered" {
		t.Fatal("late failure reopened recovered incident")
	}
	s.openLocked("dependency", time.Now())
	if w := callback(t, s, result); !bytes.Contains(w.Body.Bytes(), []byte(`"stale":true`)) {
		t.Fatal("old incident not ignored")
	}
	result = model.FlowResult{IncidentID: s.state.Incident.ID, AttemptID: s.state.Incident.AttemptID, Status: "evidence_ready", RunID: "diagnostic-123", RunState: "FINISHED"}
	if w := callback(t, s, result); w.Code != 200 || s.state.Incident.Status != "open" {
		t.Fatal("diagnostic callback must await independent repair")
	}
}

func TestCallbackPersistenceFailureCanBeRetried(t *testing.T) {
	s, result := callbackServer(t)
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	s.cfg.StatePath = filepath.Join(blocker, "state.json")
	if w := callback(t, s, result); w.Code != 503 {
		t.Fatalf("persistence failure acknowledged: %d", w.Code)
	}
	if s.state.Incident.FlowResult != nil {
		t.Fatal("failed persistence prevented retry")
	}
	s.cfg.StatePath = filepath.Join(dir, "state.json")
	if w := callback(t, s, result); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if s.state.DeliveryError != "" {
		t.Fatal("successful retry did not clear persistence error")
	}
}

func TestIdleMonitorAndActivityNeverQueueFlowsInvocations(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, model.Health{Status: "healthy", Workers: 2})
	}))
	defer target.Close()
	s := newTestServer(t)
	s.cfg.TargetURL = target.URL
	s.cfg.WebhookURL = "https://example.test/events"
	for range 20 {
		s.Poll(context.Background())
	}
	for _, kind := range []string{"traffic.changed", "fault.outage", "fault.license", "incident.recovered", "incident.failed"} {
		s.eventLocked(kind)
	}
	if len(s.state.Outbox) != 0 {
		t.Fatal("idle observations or activity queued Flows requests")
	}
	if s.state.Observation.Health.Status != "healthy" || len(s.state.History) != 5 {
		t.Fatal("local telemetry or activity lost")
	}
	s.openLocked("outage", time.Now())
	if len(s.state.Outbox) != 1 || s.state.Outbox[0].Type != "incident.opened" {
		t.Fatal("incident not dispatched")
	}
}

func TestUpgradeDropsPersistedTelemetryAndPreservesIncidentOrder(t *testing.T) {
	s := newTestServer(t)
	s.cfg.StatePath = filepath.Join(t.TempDir(), "state.json")
	s.cfg.WebhookURL = "https://example.test/events"
	s.cfg.WebhookToken = callbackToken
	s.openLocked("outage", time.Now())
	first := s.state.Outbox[0]
	s.state.Outbox = append([]model.Event{{ID: id(), Type: "observation"}, {ID: id(), Type: "fault.outage"}}, first)
	s.eventLocked("incident.opened")
	last := s.state.Outbox[len(s.state.Outbox)-1]
	s.state.Outbox = append(s.state.Outbox, model.Event{ID: id(), Type: "incident.recovered"})
	s.state.Incident.AttemptID = ""
	if err := s.persistLocked(); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(s.cfg, &fakeStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(restarted.state.Outbox) != 2 || restarted.state.Outbox[0].ID != first.ID || restarted.state.Outbox[1].ID != last.ID {
		t.Fatal("upgrade lost incident ordering")
	}
	if restarted.state.Incident.AttemptID != last.ID {
		t.Fatal("upgrade did not correlate latest queued attempt")
	}
}

func TestOnlyIncidentIsDeliveredOverHTTP(t *testing.T) {
	deliveries := make(chan model.Event, 4)
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+callbackToken {
			t.Error("missing webhook authentication")
		}
		var event model.Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Error(err)
		}
		if r.Header.Get("Idempotency-Key") != event.ID {
			t.Error("missing stable delivery ID")
		}
		deliveries <- event
		w.WriteHeader(http.StatusAccepted)
	}))
	defer receiver.Close()
	s := newTestServer(t)
	s.cfg.WebhookURL = receiver.URL
	s.cfg.WebhookToken = callbackToken
	s.client = receiver.Client()
	s.eventLocked("traffic.changed")
	s.openLocked("outage", time.Now())
	s.eventLocked("incident.failed")
	expected := s.state.Incident.AttemptID
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	go func() { s.deliver(ctx); close(stopped) }()
	select {
	case event := <-deliveries:
		if event.Type != "incident.opened" || event.ID != expected {
			t.Fatalf("unexpected delivery: %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("incident not delivered")
	}
	// Wait through another delivery tick to ensure local events and an accepted
	// incident do not result in further HTTP invocations.
	select {
	case event := <-deliveries:
		t.Fatalf("unexpected repeated delivery: %+v", event)
	case <-time.After(1500 * time.Millisecond):
	}
	cancel()
	<-stopped
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.state.Outbox) != 0 {
		t.Fatal("accepted incident remains queued")
	}
}
