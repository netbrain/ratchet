package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockDataSource implements DataSource for testing.
type mockDataSource struct {
	pairs   any
	debates any
	debate  map[string]any
	plan    any
	status  any
	scores  any
	err     error
}

func (m *mockDataSource) Pairs() (any, error) {
	return m.pairs, m.err
}

func (m *mockDataSource) Debates() (any, error) {
	return m.debates, m.err
}

func (m *mockDataSource) Debate(id string) (any, error) {
	if v, ok := m.debate[id]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("debate %q not found", id)
}

func (m *mockDataSource) Plan() (any, error) {
	return m.plan, m.err
}

func (m *mockDataSource) Status() (any, error) {
	return m.status, m.err
}

func (m *mockDataSource) Scores(pair string) (any, error) {
	return m.scores, m.err
}

func newMockDS() *mockDataSource {
	return &mockDataSource{
		pairs: []map[string]string{
			{"name": "api-design", "phase": "review"},
			{"name": "sse-correctness", "phase": "test"},
		},
		debates: []map[string]string{
			{"id": "debate-1", "status": "consensus"},
			{"id": "debate-2", "status": "in_progress"},
		},
		debate: map[string]any{
			"debate-1": map[string]string{"id": "debate-1", "status": "consensus", "pair": "api-design"},
		},
		plan: map[string]any{
			"name":       "ratchet-monitor",
			"milestones": []string{"Spike", "Solid Backend"},
		},
		status: map[string]string{
			"milestone": "Solid Backend",
			"phase":     "test",
		},
		scores: []map[string]any{
			{"debate_id": "d1", "pair": "api-design", "milestone": 1},
			{"debate_id": "d2", "pair": "sse-correctness", "milestone": 2},
		},
	}
}

// --- Method enforcement ---

func TestAPIHandlers_RejectNonGET(t *testing.T) {
	ds := newMockDS()
	handlers := map[string]http.Handler{
		"pairs":   PairsHandler(ds),
		"debates": DebatesHandler(ds),
		"detail":  DebateDetailHandler(ds),
		"plan":    PlanHandler(ds),
		"status":  StatusHandler(ds),
		"scores":  ScoresHandler(ds),
	}

	for name, h := range handlers {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
			if allow := rec.Header().Get("Allow"); allow == "" {
				t.Error("missing Allow header on 405 response")
			}

			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			if body["error"] != "method not allowed" {
				t.Errorf("error message: got %q", body["error"])
			}
		})
	}
}

// --- Error handling ---

func TestPairsHandler_DataSourceError(t *testing.T) {
	ds := newMockDS()
	ds.err = fmt.Errorf("database unavailable")
	h := PairsHandler(ds)
	req := httptest.NewRequest(http.MethodGet, "/api/pairs", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	// Internal error details must not leak to the client.
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Errorf("error message: got %q, want %q", body["error"], "internal server error")
	}
	if strings.Contains(rec.Body.String(), "database unavailable") {
		t.Error("internal error details leaked to client")
	}
}

// --- Security headers ---

func TestAPIHandlers_XContentTypeOptions(t *testing.T) {
	ds := newMockDS()
	handlers := map[string]struct {
		handler http.Handler
		path    string
	}{
		"pairs":   {PairsHandler(ds), "/api/pairs"},
		"debates": {DebatesHandler(ds), "/api/debates"},
		"detail":  {DebateDetailHandler(ds), "/api/debates/debate-1"},
		"plan":    {PlanHandler(ds), "/api/plan"},
		"status":  {StatusHandler(ds), "/api/status"},
		"scores":  {ScoresHandler(ds), "/api/scores"},
		"health":  {HealthHandler(), "/health"},
	}

	for name, tc := range handlers {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			tc.handler.ServeHTTP(rec, req)

			xcto := rec.Header().Get("X-Content-Type-Options")
			if xcto != "nosniff" {
				t.Errorf("X-Content-Type-Options: got %q, want %q", xcto, "nosniff")
			}
		})
	}
}

// --- Debate ID validation ---

func TestDebateDetailHandler_PathTraversal(t *testing.T) {
	ds := newMockDS()
	h := DebateDetailHandler(ds)

	maliciousIDs := []string{
		"../etc/passwd",
		"..%2fetc%2fpasswd",
		"debate-1/../secret",
		"foo\\bar",
	}

	for _, id := range maliciousIDs {
		t.Run(id, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/debates/"+id, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status for id %q: got %d, want %d", id, rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestDebateDetailHandler_OversizedID(t *testing.T) {
	ds := newMockDS()
	h := DebateDetailHandler(ds)

	longID := strings.Repeat("a", maxDebateIDLength+1)
	req := httptest.NewRequest(http.MethodGet, "/api/debates/"+longID, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDebateDetailHandler_ValidIDAtMaxLength(t *testing.T) {
	maxID := strings.Repeat("a", maxDebateIDLength)
	ds := newMockDS()
	ds.debate[maxID] = map[string]string{"id": maxID}
	h := DebateDetailHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/debates/"+maxID, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDebateDetailHandler_NotFoundHidesInternalError(t *testing.T) {
	ds := newMockDS()
	h := DebateDetailHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/debates/nonexistent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	// Must say "debate not found", not the raw error from the data source.
	if body["error"] != "debate not found" {
		t.Errorf("error message: got %q, want %q", body["error"], "debate not found")
	}
}

func TestIsValidDebateID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"debate-1", true},
		{"abc_123", true},
		{"debate.final", true},
		{"", false},
		{"..", false},
		{"../foo", false},
		{"foo/bar", false},
		{"foo\\bar", false},
		{string([]byte{0x00}), false},
		{string([]byte{0x1F}), false},
		{string([]byte{0x7F}), false},
		{strings.Repeat("x", maxDebateIDLength), true},
		{strings.Repeat("x", maxDebateIDLength+1), false},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%q", tc.id), func(t *testing.T) {
			got := isValidDebateID(tc.id)
			if got != tc.want {
				t.Errorf("isValidDebateID(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

// --- GET /api/pairs ---

func TestPairsHandler_StatusCode(t *testing.T) {
	h := PairsHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/pairs", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPairsHandler_ContentType(t *testing.T) {
	h := PairsHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/pairs", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

func TestPairsHandler_ResponseShape(t *testing.T) {
	h := PairsHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/pairs", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body []any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body) != 2 {
		t.Errorf("expected 2 pairs, got %d", len(body))
	}
}

// --- GET /api/debates ---

func TestDebatesHandler_StatusCode(t *testing.T) {
	h := DebatesHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/debates", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDebatesHandler_ContentType(t *testing.T) {
	h := DebatesHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/debates", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

func TestDebatesHandler_ResponseShape(t *testing.T) {
	h := DebatesHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/debates", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body []any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body) != 2 {
		t.Errorf("expected 2 debates, got %d", len(body))
	}
}

// --- GET /api/debates/{id} ---

func TestDebateDetailHandler_Found(t *testing.T) {
	h := DebateDetailHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/debates/debate-1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["id"] != "debate-1" {
		t.Errorf("id: got %v, want debate-1", body["id"])
	}
}

func TestDebateDetailHandler_NotFound(t *testing.T) {
	h := DebateDetailHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/debates/nonexistent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// --- GET /api/plan ---

func TestPlanHandler_StatusCode(t *testing.T) {
	h := PlanHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/plan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPlanHandler_ContentType(t *testing.T) {
	h := PlanHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/plan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

func TestPlanHandler_ResponseShape(t *testing.T) {
	h := PlanHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/plan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["name"] != "ratchet-monitor" {
		t.Errorf("name: got %v", body["name"])
	}
}

// --- GET /api/status ---

func TestStatusHandler_StatusCode(t *testing.T) {
	h := StatusHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestStatusHandler_ContentType(t *testing.T) {
	h := StatusHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

func TestStatusHandler_ResponseShape(t *testing.T) {
	h := StatusHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["phase"] != "test" {
		t.Errorf("phase: got %v", body["phase"])
	}
}

// --- GET /api/scores ---

func TestScoresHandler_StatusCode(t *testing.T) {
	h := ScoresHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/scores", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestScoresHandler_ContentType(t *testing.T) {
	h := ScoresHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/scores", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

func TestScoresHandler_ResponseShape(t *testing.T) {
	h := ScoresHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/scores", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body []any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body) != 2 {
		t.Errorf("expected 2 scores, got %d", len(body))
	}
}

func TestScoresHandler_InvalidPairParam(t *testing.T) {
	ds := newMockDS()
	h := ScoresHandler(ds)

	badPairs := []string{
		"../etc/passwd",
		"pair/with/slashes",
		"pair\\backslash",
		strings.Repeat("x", maxPairParamLength+1),
	}

	for _, pair := range badPairs {
		t.Run(pair, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/scores?pair="+pair, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status for pair %q: got %d, want %d", pair, rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestScoresHandler_ValidPairParam(t *testing.T) {
	ds := newMockDS()
	h := ScoresHandler(ds)

	goodPairs := []string{
		"api-design",
		"sse-correctness",
		"my_pair.v2",
		strings.Repeat("a", maxPairParamLength),
	}

	for _, pair := range goodPairs {
		t.Run(pair, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/scores?pair="+pair, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status for pair %q: got %d, want %d", pair, rec.Code, http.StatusOK)
			}
		})
	}
}

func TestScoresHandler_EmptyPairIsValid(t *testing.T) {
	ds := newMockDS()
	h := ScoresHandler(ds)

	// No pair param at all — should return all scores.
	req := httptest.NewRequest(http.MethodGet, "/api/scores", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestScoresHandler_DataSourceError(t *testing.T) {
	ds := newMockDS()
	ds.err = fmt.Errorf("disk failure")
	h := ScoresHandler(ds)
	req := httptest.NewRequest(http.MethodGet, "/api/scores", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Errorf("error message: got %q", body["error"])
	}
}
