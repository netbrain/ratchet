package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// NotFoundError signals that a requested resource does not exist.
// Handlers use errors.As to distinguish 404 from 500.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
}

// maxDebateIDLength caps the debate ID parameter to prevent abuse.
const maxDebateIDLength = 256

// maxPairParamLength caps the pair query parameter to prevent abuse.
const maxPairParamLength = 128

// maxWorkspaceParamLength caps the workspace query parameter to prevent abuse.
const maxWorkspaceParamLength = 128

// maxStatusParamLength caps the status query parameter to prevent abuse.
const maxStatusParamLength = 64

// maxPhaseParamLength caps the phase query parameter to prevent abuse.
const maxPhaseParamLength = 64

// isCleanParam rejects strings that contain path traversal sequences,
// path separators, null bytes, or control characters.
// This is the single validation gate for all user-supplied path segments;
// every handler parameter must pass through it before reaching the filesystem.
func isCleanParam(s string, maxLen int) bool {
	if len(s) == 0 || len(s) > maxLen {
		return false
	}
	if strings.Contains(s, "..") ||
		strings.ContainsAny(s, "/\\") {
		return false
	}
	// Reject control characters (0x00-0x1F, 0x7F), which also covers null bytes.
	for _, r := range s {
		if r < 0x20 || r == 0x7F {
			return false
		}
	}
	return true
}

// isValidDebateID rejects IDs that contain path traversal sequences,
// path separators, null bytes, or control characters.
func isValidDebateID(id string) bool {
	return isCleanParam(id, maxDebateIDLength)
}

// isValidPairParam validates the optional ?pair= query parameter.
func isValidPairParam(pair string) bool {
	return isCleanParam(pair, maxPairParamLength)
}

// isValidWorkspaceParam validates the optional ?workspace= query parameter.
func isValidWorkspaceParam(workspace string) bool {
	return isCleanParam(workspace, maxWorkspaceParamLength)
}

// isValidStatusParam validates the optional ?status= query parameter.
func isValidStatusParam(status string) bool {
	return isCleanParam(status, maxStatusParamLength)
}

// isValidPhaseParam validates the optional ?phase= query parameter.
func isValidPhaseParam(phase string) bool {
	return isCleanParam(phase, maxPhaseParamLength)
}

// extractWorkspace reads and validates the ?workspace= query parameter.
// Returns ("", false) when absent (caller proceeds without filtering).
// Returns ("", true) when present but syntactically invalid (caller returns 400).
// Returns (workspace, false) when valid.
func extractWorkspace(r *http.Request) (string, bool) {
	workspace := r.URL.Query().Get("workspace")
	if workspace == "" {
		return "", false
	}
	if !isValidWorkspaceParam(workspace) {
		return "", true
	}
	return workspace, false
}

// setSecurityHeaders writes common security headers to every API response.
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

// DataSource provides read access to parsed .ratchet/ data for API handlers.
type DataSource interface {
	Pairs(workspace string) (any, error)
	Debates(workspace string) (any, error)
	Debate(id string) (any, error)
	Plan() (any, error)
	Status() (any, error)
	Scores(pair string) (any, error)
	Workspaces() (any, error)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	setSecurityHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Status header already written; log the failure.
		slog.Error("failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	setSecurityHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

// methodNotAllowed writes a 405 response with an Allow header.
func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// PairsHandler returns a handler that serves GET /api/pairs.
// An optional ?workspace= query parameter filters results by workspace name.
func PairsHandler(ds DataSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		workspace, invalid := extractWorkspace(r)
		if invalid {
			writeError(w, http.StatusBadRequest, "invalid workspace parameter")
			return
		}
		data, err := ds.Pairs(workspace)
		if err != nil {
			var nfe *NotFoundError
			if errors.As(err, &nfe) {
				writeError(w, http.StatusNotFound, "workspace not found")
				return
			}
			slog.Error("pairs data source failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, data)
	})
}

// DebatesHandler returns a handler that serves GET /api/debates.
// Optional query parameters:
//   - ?workspace= — filter by workspace name
//   - ?status= — filter by debate status (exact match)
//   - ?pair= — filter by pair name (exact match)
//   - ?phase= — filter by phase (exact match)
//
// Multiple filters AND together. Empty/missing params return all debates.
func DebatesHandler(ds DataSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		workspace, invalid := extractWorkspace(r)
		if invalid {
			writeError(w, http.StatusBadRequest, "invalid workspace parameter")
			return
		}

		// Extract and validate optional filter params.
		status := r.URL.Query().Get("status")
		if status != "" && !isValidStatusParam(status) {
			writeError(w, http.StatusBadRequest, "invalid status parameter")
			return
		}
		pair := r.URL.Query().Get("pair")
		if pair != "" && !isValidPairParam(pair) {
			writeError(w, http.StatusBadRequest, "invalid pair parameter")
			return
		}
		phase := r.URL.Query().Get("phase")
		if phase != "" && !isValidPhaseParam(phase) {
			writeError(w, http.StatusBadRequest, "invalid phase parameter")
			return
		}

		data, err := ds.Debates(workspace)
		if err != nil {
			var nfe *NotFoundError
			if errors.As(err, &nfe) {
				writeError(w, http.StatusNotFound, "workspace not found")
				return
			}
			slog.Error("debates data source failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Apply filters if any are present.
		if status != "" || pair != "" || phase != "" {
			data = filterDebates(data, status, pair, phase)
		}

		writeJSON(w, http.StatusOK, data)
	})
}

// filterDebates applies exact-match filters to a debate list.
// The data parameter is expected to be a slice; each element is
// re-encoded via JSON to extract filterable fields without coupling
// the handler to the concrete debate type.
func filterDebates(data any, status, pair, phase string) any {
	// Marshal to JSON then unmarshal to a slice of maps.
	raw, err := json.Marshal(data)
	if err != nil {
		slog.Error("filterDebates: marshal failed", "error", err)
		return data
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		slog.Error("filterDebates: unmarshal failed", "error", err)
		return data
	}

	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if status != "" {
			v, _ := item["status"].(string)
			if v != status {
				continue
			}
		}
		if pair != "" {
			v, _ := item["pair"].(string)
			if v != pair {
				continue
			}
		}
		if phase != "" {
			v, _ := item["phase"].(string)
			if v != phase {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// DebateDetailHandler returns a handler that serves GET /api/debates/{id}.
func DebateDetailHandler(ds DataSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		id := r.PathValue("id")
		if id == "" {
			// Fallback for muxes that don't inject path values.
			path := strings.TrimPrefix(r.URL.Path, "/api/debates/")
			id = strings.TrimRight(path, "/")
		}

		if id == "" {
			writeError(w, http.StatusBadRequest, "missing debate id")
			return
		}

		if !isValidDebateID(id) {
			writeError(w, http.StatusBadRequest, "invalid debate id")
			return
		}

		data, err := ds.Debate(id)
		if err != nil {
			var nfe *NotFoundError
			if errors.As(err, &nfe) {
				writeError(w, http.StatusNotFound, "debate not found")
			} else {
				slog.Error("debate detail data source failed", "id", id, "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
			return
		}
		writeJSON(w, http.StatusOK, data)
	})
}

// PlanHandler returns a handler that serves GET /api/plan.
func PlanHandler(ds DataSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		data, err := ds.Plan()
		if err != nil {
			slog.Error("plan data source failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, data)
	})
}

// StatusHandler returns a handler that serves GET /api/status.
func StatusHandler(ds DataSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		data, err := ds.Status()
		if err != nil {
			slog.Error("status data source failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, data)
	})
}

// WorkspacesHandler returns a handler that serves GET /api/workspaces.
func WorkspacesHandler(ds DataSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		data, err := ds.Workspaces()
		if err != nil {
			slog.Error("workspaces data source failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, data)
	})
}

// ScoresHandler returns a handler that serves GET /api/scores.
// An optional ?pair= query parameter filters results by pair name.
func ScoresHandler(ds DataSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		pair := r.URL.Query().Get("pair")
		if pair != "" && !isValidPairParam(pair) {
			writeError(w, http.StatusBadRequest, "invalid pair parameter")
			return
		}
		data, err := ds.Scores(pair)
		if err != nil {
			slog.Error("scores data source failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, data)
	})
}
