package control

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"regexp"

	"github.com/spacelift-solutions/Mayday/internal/model"
)

var eventID = regexp.MustCompile(`^[a-f0-9]{24}$`)

func (s *Server) flowResult(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	expected := sha256.Sum256([]byte("Bearer " + s.cfg.WebhookToken))
	actual := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	if len(s.cfg.WebhookToken) < 16 || subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}
	var result model.FlowResult
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF ||
		!eventID.MatchString(result.IncidentID) || !eventID.MatchString(result.AttemptID) ||
		len(result.RunID) > 128 || len(result.RunState) > 64 {
		respond(w, http.StatusBadRequest, map[string]string{"error": "Invalid Flow result"})
		return
	}
	switch result.Status {
	case "started":
		if result.RunID == "" || result.RunState == "" {
			respond(w, http.StatusBadRequest, map[string]string{"error": "Started result requires a run ID and state"})
			return
		}
	case "succeeded", "evidence_ready":
		if result.RunID == "" || result.RunState != "FINISHED" {
			respond(w, http.StatusBadRequest, map[string]string{"error": "Successful result requires a finished run"})
			return
		}
	case "failed":
	default:
		respond(w, http.StatusBadRequest, map[string]string{"error": "Invalid Flow status"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.state.Incident
	if i == nil || i.ID != result.IncidentID || i.AttemptID != result.AttemptID {
		// Acknowledge delayed callbacks without changing another attempt.
		respond(w, http.StatusOK, map[string]bool{"accepted": true, "stale": true})
		return
	}
	if (result.Status == "evidence_ready" && i.Kind != "dependency") ||
		(result.Status == "succeeded" && i.Kind == "dependency") {
		respond(w, http.StatusBadRequest, map[string]string{"error": "Result does not match incident kind"})
		return
	}
	if i.FlowResult != nil && (i.FlowResult.Status != "started" || result.Status == "started") {
		respond(w, http.StatusOK, map[string]bool{"accepted": true, "duplicate": true})
		return
	}
	before := copyIncident(i)
	if i.FlowResult != nil && result.RunID == "" {
		// Deadline/error callbacks may omit run details. Keep the known run link.
		result.RunID = i.FlowResult.RunID
		result.RunState = i.FlowResult.RunState
	}
	i.FlowResult = &result
	if i.Status != "recovered" && i.Status != "failed" {
		switch result.Status {
		case "started":
			i.Message = "Spacelift run started; follow its progress while monitoring continues"
		case "failed":
			i.Status = "failed"
			i.Message = "Flow recovery failed; inspect the Spacelift run before retrying"
		case "evidence_ready":
			i.Message = "Diagnostic evidence ready; investigate with Intelligence and restore through Intent"
		case "succeeded":
			i.Message = "Spacelift run finished; awaiting independent recovery verification"
		}
	}
	// Acknowledge only durable results. A lost response can safely be retried.
	if s.persistLocked() != nil {
		s.state.Incident = before
		respond(w, http.StatusServiceUnavailable, map[string]string{"error": "Unable to save Flow result; retry later"})
		return
	}
	respond(w, http.StatusOK, map[string]bool{"accepted": true})
}
