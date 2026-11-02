package control

import (
	"net/url"
	"strings"
)

type Navigation struct {
	FlowURL        string `json:"flow_url,omitempty"`
	ServiceURL     string `json:"service_url,omitempty"`
	DiagnosticsURL string `json:"diagnostics_url,omitempty"`
	StackURL       string `json:"stack_url,omitempty"`
	RunURL         string `json:"run_url,omitempty"`
}

func (s *Server) navigationLocked() Navigation {
	links := Navigation{FlowURL: s.cfg.FlowURL}
	if s.cfg.SpaceliftURL == "" {
		return links
	}
	stackURL := func(stack string) string {
		if stack == "" {
			return ""
		}
		return strings.TrimRight(s.cfg.SpaceliftURL, "/") + "/stack/" + url.PathEscape(stack)
	}
	links.ServiceURL = stackURL(s.cfg.StackID)
	links.DiagnosticsURL = stackURL(s.cfg.DiagnosticsStackID)
	links.StackURL = links.ServiceURL
	i := s.state.Incident
	if i != nil {
		if i.Kind == "dependency" {
			links.StackURL = links.DiagnosticsURL
		}
		if links.StackURL != "" && i.FlowResult != nil && i.FlowResult.RunID != "" {
			links.RunURL = links.StackURL + "/run/" + url.PathEscape(i.FlowResult.RunID)
		}
	}
	return links
}
