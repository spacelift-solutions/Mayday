package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacelift-solutions/Mayday/internal/model"
)

func TestNavigationTargetsTheCurrentAttemptAndCorrectStack(t *testing.T) {
	s := newTestServer(t)
	s.cfg.SpaceliftURL = "https://example.app.spacelift.io/"
	s.cfg.FlowURL = "https://useflows.eu/flow/test-flow"
	s.cfg.StackID, s.cfg.DiagnosticsStackID = "service", "diagnostics"
	for _, kind := range []string{"outage", "dependency", "capacity_up", "capacity_down", "reset"} {
		s.state.Incident = &model.Incident{Kind: kind, FlowResult: &model.FlowResult{Status: "started", RunID: "run-123"}}
		stack := "service"
		if kind == "dependency" {
			stack = "diagnostics"
		}
		links := s.navigationLocked()
		if links.RunURL != "https://example.app.spacelift.io/stack/"+stack+"/run/run-123" || links.FlowURL != s.cfg.FlowURL {
			t.Fatalf("incorrect links for %s: %+v", kind, links)
		}
		s.state.Incident.FlowResult = nil
		if s.navigationLocked().RunURL != "" {
			t.Fatal("new attempt retained old run link")
		}
	}
	h := s.Handler(http.NotFoundHandler())
	r := httptest.NewRequest("GET", "/api/state", nil)
	r.AddCookie(loginCookie(t, h))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var response struct {
		Navigation Navigation `json:"navigation"`
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Navigation.FlowURL != s.cfg.FlowURL {
		t.Fatal("authenticated state omitted navigation")
	}
}

func TestNavigationConfigurationRejectsUnsafeURLs(t *testing.T) {
	for _, bad := range []string{"javascript:alert(1)", "http://example.com", "https://user:secret@example.com", "https://example.com/?secret=value", "https://example.com/#fragment"} {
		for _, field := range []string{"flow", "spacelift"} {
			cfg := newTestServer(t).cfg
			if field == "flow" {
				cfg.FlowURL = bad
			} else {
				cfg.SpaceliftURL = bad
			}
			if _, err := New(cfg, &fakeStore{}, nil); err == nil {
				t.Fatalf("accepted unsafe %s URL", field)
			}
		}
	}
	if links := newTestServer(t).navigationLocked(); links.FlowURL != "" || links.RunURL != "" || links.StackURL != "" {
		t.Fatal("local mode invented cloud links")
	}
}
