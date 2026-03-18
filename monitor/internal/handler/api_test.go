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
	pairs           any
	debates         any
	debate          map[string]any
	debateErr       map[string]error // per-debate error overrides
	plan            any
	status          any
	scores          any
	workspaces      any
	err             error
	knownWorkspaces map[string]bool // non-nil means workspace validation is active
}

func (m *mockDataSource) Pairs(workspace string) (any, error) {
	if workspace != "" && m.knownWorkspaces != nil {
		if !m.knownWorkspaces[workspace] {
			return nil, &NotFoundError{Resource: "workspace", ID: workspace}
		}
	}
	return m.pairs, m.err
}

func (m *mockDataSource) Debates(workspace string) (any, error) {
	if workspace != "" && m.knownWorkspaces != nil {
		if !m.knownWorkspaces[workspace] {
			return nil, &NotFoundError{Resource: "workspace", ID: workspace}
		}
	}
	return m.debates, m.err
}

func (m *mockDataSource) Debate(id string) (any, error) {
	// Check per-debate error overrides first (for testing 500 vs 404)
	if m.debateErr != nil {
		if err, ok := m.debateErr[id]; ok {
			return nil, err
		}
	}
	if v, ok := m.debate[id]; ok {
		return v, nil
	}
	return nil, &NotFoundError{Resource: "debate", ID: id}
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

func (m *mockDataSource) Workspaces() (any, error) {
	return m.workspaces, m.err
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
		workspaces: []map[string]string{
			{"name": "frontend", "path": "/home/dev/frontend"},
			{"name": "backend", "path": "/home/dev/backend"},
		},
		knownWorkspaces: map[string]bool{
			"frontend": true,
			"backend":  true,
		},
	}
}

// newMockDS_V2 creates a mock datasource with v2 plan and status structures.
func newMockDS_V2() *mockDataSource {
	return &mockDataSource{
		pairs: []map[string]string{
			{"name": "api-contracts", "phase": "review"},
		},
		debates: []map[string]string{
			{"id": "debate-v2-1", "status": "consensus"},
		},
		debate: map[string]any{
			"debate-v2-1": map[string]string{"id": "debate-v2-1", "status": "consensus", "pair": "api-contracts"},
		},
		plan: map[string]any{
			"max_regressions": 5,
			"epic": map[string]any{
				"name":        "test-epic",
				"description": "Testing v2 plan",
				"milestones": []map[string]any{
					{
						"id":          1,
						"name":        "M1",
						"description": "First milestone",
						"status":      "done",
						"depends_on":  []int{},
						"regressions": 0,
						"issues": []map[string]any{
							{
								"ref":    "issue-1-1",
								"title":  "First issue",
								"pairs":  []string{"api-contracts"},
								"status": "done",
								"phase_status": map[string]string{
									"plan":   "done",
									"test":   "done",
									"build":  "done",
									"review": "done",
								},
								"depends_on": []string{},
								"files":      []string{"file1.go"},
								"debates":    []string{"debate-v2-1"},
								"branch":     "feature/issue-1-1",
							},
						},
					},
					{
						"id":          2,
						"name":        "M2",
						"description": "Second milestone",
						"status":      "in_progress",
						"depends_on":  []int{1},
						"regressions": 1,
						"issues": []map[string]any{
							{
								"ref":    "issue-2-1",
								"title":  "Second issue",
								"pairs":  []string{"api-contracts"},
								"status": "in_progress",
								"phase_status": map[string]string{
									"plan":  "done",
									"test":  "in_progress",
									"build": "pending",
								},
								"depends_on": []string{},
								"files":      []string{},
								"debates":    []string{},
								"branch":     "",
							},
						},
					},
				},
				"current_focus": map[string]any{
					"milestone_id": 2,
					"issue_ref":    "issue-2-1",
					"phase":        "test",
					"started":      "2026-03-16",
				},
			},
		},
		status: map[string]any{
			"milestone_id":   2,
			"milestone_name": "M2",
			"issue_ref":      "issue-2-1",
			"phase":          "test",
		},
		scores: []map[string]any{
			{"debate_id": "debate-v2-1", "pair": "api-contracts", "milestone": 1},
		},
		workspaces:      []map[string]string{},
		knownWorkspaces: map[string]bool{},
	}
}

// --- Method enforcement ---

func TestAPIHandlers_RejectNonGET(t *testing.T) {
	ds := newMockDS()
	handlers := map[string]http.Handler{
		"pairs":      PairsHandler(ds),
		"debates":    DebatesHandler(ds),
		"detail":     DebateDetailHandler(ds),
		"plan":       PlanHandler(ds),
		"status":     StatusHandler(ds),
		"scores":     ScoresHandler(ds),
		"workspaces": WorkspacesHandler(ds),
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
		"pairs":      {PairsHandler(ds), "/api/pairs"},
		"debates":    {DebatesHandler(ds), "/api/debates"},
		"detail":     {DebateDetailHandler(ds), "/api/debates/debate-1"},
		"plan":       {PlanHandler(ds), "/api/plan"},
		"status":     {StatusHandler(ds), "/api/status"},
		"scores":     {ScoresHandler(ds), "/api/scores"},
		"workspaces": {WorkspacesHandler(ds), "/api/workspaces"},
		"health":     {HealthHandler(), "/health"},
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

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	// Must say "debate not found", not the raw error from the data source.
	if body["error"] != "debate not found" {
		t.Errorf("error message: got %q, want %q", body["error"], "debate not found")
	}
}

func TestDebateDetailHandler_InternalError_Returns500(t *testing.T) {
	ds := newMockDS()
	ds.debateErr = map[string]error{
		"broken-debate": fmt.Errorf("disk I/O failure"),
	}
	h := DebateDetailHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/debates/broken-debate", nil)
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
		t.Errorf("error message: got %q, want %q", body["error"], "internal server error")
	}
	if strings.Contains(rec.Body.String(), "disk I/O failure") {
		t.Error("internal error details leaked to client")
	}
}

func TestDebateDetailHandler_NotFoundError_Returns404(t *testing.T) {
	ds := newMockDS()
	ds.debateErr = map[string]error{
		"missing-debate": &NotFoundError{Resource: "debate", ID: "missing-debate"},
	}
	h := DebateDetailHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/debates/missing-debate", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
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

// --- V2 API Tests ---

// TestPlanHandler_V2Plan verifies that /api/plan returns v2 milestone and issue fields.
func TestPlanHandler_V2Plan(t *testing.T) {
	ds := newMockDS_V2()
	h := PlanHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/plan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify epic structure
	epic, ok := response["epic"].(map[string]any)
	if !ok {
		t.Fatalf("epic field missing or wrong type: got %T", response["epic"])
	}

	milestones, ok := epic["milestones"].([]any)
	if !ok {
		t.Fatalf("milestones field missing or wrong type: got %T", epic["milestones"])
	}

	if len(milestones) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(milestones))
	}

	// Check milestone 1 v2 fields
	m1 := milestones[0].(map[string]any)
	if _, ok := m1["depends_on"]; !ok {
		t.Error("milestone 1 missing depends_on field")
	}
	if _, ok := m1["regressions"]; !ok {
		t.Error("milestone 1 missing regressions field")
	}
	issues1, ok := m1["issues"].([]any)
	if !ok {
		t.Fatalf("milestone 1 missing issues array: got %T", m1["issues"])
	}
	if len(issues1) != 1 {
		t.Errorf("milestone 1 expected 1 issue, got %d", len(issues1))
	}

	// Check issue 1 v2 fields
	issue1 := issues1[0].(map[string]any)
	requiredIssueFields := []string{"ref", "title", "pairs", "depends_on", "phase_status", "files", "debates", "branch", "status"}
	for _, field := range requiredIssueFields {
		if _, ok := issue1[field]; !ok {
			t.Errorf("issue missing required field: %s", field)
		}
	}

	// Check milestone 2 v2 fields
	m2 := milestones[1].(map[string]any)
	dependsOn, ok := m2["depends_on"].([]any)
	if !ok || len(dependsOn) == 0 {
		t.Errorf("milestone 2 depends_on: got %v, want non-empty array", m2["depends_on"])
	}
	regressions, ok := m2["regressions"].(float64)
	if !ok || regressions != 1 {
		t.Errorf("milestone 2 regressions: got %v, want 1", m2["regressions"])
	}

	// Check current_focus v2 fields
	focus, ok := epic["current_focus"].(map[string]any)
	if !ok {
		t.Fatalf("current_focus field missing: got %T", epic["current_focus"])
	}
	if _, ok := focus["issue_ref"]; !ok {
		t.Error("current_focus missing issue_ref field")
	}
	if focus["issue_ref"] != "issue-2-1" {
		t.Errorf("current_focus issue_ref: got %q, want %q", focus["issue_ref"], "issue-2-1")
	}

	// Check max_regressions is present and correct at the top level.
	maxReg, ok := response["max_regressions"].(float64)
	if !ok {
		t.Fatalf("max_regressions missing or wrong type: got %T (%v)", response["max_regressions"], response["max_regressions"])
	}
	if maxReg != 5 {
		t.Errorf("max_regressions: got %v, want 5", maxReg)
	}
}

// TestStatusHandler_V2Status verifies that /api/status includes issue_ref.
func TestStatusHandler_V2Status(t *testing.T) {
	ds := newMockDS_V2()
	h := StatusHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify v2 fields
	requiredFields := []string{"milestone_id", "milestone_name", "issue_ref", "phase"}
	for _, field := range requiredFields {
		if _, ok := response[field]; !ok {
			t.Errorf("status missing required v2 field: %s", field)
		}
	}

	if response["issue_ref"] != "issue-2-1" {
		t.Errorf("issue_ref: got %q, want %q", response["issue_ref"], "issue-2-1")
	}
	if response["phase"] != "test" {
		t.Errorf("phase: got %q, want %q", response["phase"], "test")
	}
}

// TestPlanHandler_V2IssuePhaseStatus verifies phase_status map serialization.
func TestPlanHandler_V2IssuePhaseStatus(t *testing.T) {
	ds := newMockDS_V2()
	h := PlanHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/plan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	epic := response["epic"].(map[string]any)
	milestones := epic["milestones"].([]any)
	m1 := milestones[0].(map[string]any)
	issues := m1["issues"].([]any)
	issue := issues[0].(map[string]any)

	phaseStatus, ok := issue["phase_status"].(map[string]any)
	if !ok {
		t.Fatalf("phase_status wrong type: got %T", issue["phase_status"])
	}

	expectedPhases := []string{"plan", "test", "build", "review"}
	for _, phase := range expectedPhases {
		if _, ok := phaseStatus[phase]; !ok {
			t.Errorf("phase_status missing phase: %s", phase)
		}
	}
}

// TestPlanHandler_V2IssueDependencies verifies issue depends_on serialization.
func TestPlanHandler_V2IssueDependencies(t *testing.T) {
	// Create a datasource with issue dependencies
	ds := &mockDataSource{
		plan: map[string]any{
			"epic": map[string]any{
				"name":        "test-epic",
				"description": "Testing issue dependencies",
				"milestones": []any{
					map[string]any{
						"id":          2,
						"name":        "M2",
						"description": "Second milestone",
						"status":      "in_progress",
						"depends_on":  []int{1},
						"regressions": 0,
						"issues": []any{
							map[string]any{
								"ref":        "issue-2-1",
								"title":      "First issue",
								"depends_on": []any{},
							},
							map[string]any{
								"ref":        "issue-2-2",
								"title":      "Second issue",
								"depends_on": []any{"issue-2-1"},
							},
						},
					},
				},
			},
		},
	}

	h := PlanHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/plan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	epic2 := response["epic"].(map[string]any)
	milestones2 := epic2["milestones"].([]any)
	if len(milestones2) < 1 {
		t.Fatalf("expected at least 1 milestone, got %d", len(milestones2))
	}
	m2Resp := milestones2[0].(map[string]any)
	issues := m2Resp["issues"].([]any)

	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}

	issue1 := issues[0].(map[string]any)
	issue1DependsOn, ok := issue1["depends_on"].([]any)
	if !ok {
		t.Fatalf("issue1 depends_on wrong type: got %T", issue1["depends_on"])
	}
	if len(issue1DependsOn) != 0 {
		t.Errorf("issue1 depends_on should be empty: got %v", issue1DependsOn)
	}

	issue2 := issues[1].(map[string]any)
	dependsOn, ok := issue2["depends_on"].([]any)
	if !ok {
		t.Fatalf("issue2 depends_on wrong type: got %T", issue2["depends_on"])
	}

	if len(dependsOn) != 1 {
		t.Errorf("issue2 depends_on length: got %d, want 1", len(dependsOn))
	}
	if dependsOn[0] != "issue-2-1" {
		t.Errorf("issue2 depends_on[0]: got %q, want %q", dependsOn[0], "issue-2-1")
	}
}

// --- GET /api/workspaces ---

func TestWorkspacesHandler_StatusCode(t *testing.T) {
	h := WorkspacesHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWorkspacesHandler_ContentType(t *testing.T) {
	h := WorkspacesHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

func TestWorkspacesHandler_ResponseShape(t *testing.T) {
	h := WorkspacesHandler(newMockDS())
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body []any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body) != 2 {
		t.Errorf("expected 2 workspaces, got %d", len(body))
	}
}

func TestWorkspacesHandler_DataSourceError(t *testing.T) {
	ds := newMockDS()
	ds.err = fmt.Errorf("disk failure")
	h := WorkspacesHandler(ds)
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces", nil)
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

// --- ?workspace= query parameter ---

func TestIsValidWorkspaceParam(t *testing.T) {
	tests := []struct {
		workspace string
		want      bool
	}{
		{"frontend", true},
		{"my-workspace", true},
		{"ws_1", true},
		{"ws.v2", true},
		{strings.Repeat("a", maxWorkspaceParamLength), true},
		{"", false},
		{"../etc", false},
		{"foo/bar", false},
		{"foo\\bar", false},
		{string([]byte{0x00}), false},
		{string([]byte{0x1F}), false},
		{strings.Repeat("x", maxWorkspaceParamLength+1), false},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%q", tc.workspace), func(t *testing.T) {
			got := isValidWorkspaceParam(tc.workspace)
			if got != tc.want {
				t.Errorf("isValidWorkspaceParam(%q) = %v, want %v", tc.workspace, got, tc.want)
			}
		})
	}
}

func TestPairsHandler_InvalidWorkspaceParam(t *testing.T) {
	ds := newMockDS()
	h := PairsHandler(ds)

	badWorkspaces := []string{
		"../etc/passwd",
		"foo/bar",
		"foo\\bar",
		strings.Repeat("x", maxWorkspaceParamLength+1),
	}

	for _, ws := range badWorkspaces {
		t.Run(ws, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/pairs?workspace="+ws, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status for workspace %q: got %d, want %d", ws, rec.Code, http.StatusBadRequest)
			}
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if body["error"] != "invalid workspace parameter" {
				t.Errorf("error: got %q, want %q", body["error"], "invalid workspace parameter")
			}
		})
	}
}

func TestDebatesHandler_InvalidWorkspaceParam(t *testing.T) {
	ds := newMockDS()
	h := DebatesHandler(ds)

	badWorkspaces := []string{
		"../etc/passwd",
		"foo/bar",
		strings.Repeat("x", maxWorkspaceParamLength+1),
	}

	for _, ws := range badWorkspaces {
		t.Run(ws, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/debates?workspace="+ws, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status for workspace %q: got %d, want %d", ws, rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestPairsHandler_UnknownWorkspace_Returns404(t *testing.T) {
	ds := newMockDS()
	h := PairsHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/pairs?workspace=nonexistent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "workspace not found" {
		t.Errorf("error: got %q, want %q", body["error"], "workspace not found")
	}
}

func TestDebatesHandler_UnknownWorkspace_Returns404(t *testing.T) {
	ds := newMockDS()
	h := DebatesHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/debates?workspace=nonexistent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "workspace not found" {
		t.Errorf("error: got %q, want %q", body["error"], "workspace not found")
	}
}

func TestPairsHandler_KnownWorkspace_Returns200(t *testing.T) {
	ds := newMockDS()
	h := PairsHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/pairs?workspace=frontend", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDebatesHandler_KnownWorkspace_Returns200(t *testing.T) {
	ds := newMockDS()
	h := DebatesHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/debates?workspace=backend", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPairsHandler_NoWorkspaceParam_Returns200(t *testing.T) {
	ds := newMockDS()
	h := PairsHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/pairs", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDebatesHandler_NoWorkspaceParam_Returns200(t *testing.T) {
	ds := newMockDS()
	h := DebatesHandler(ds)

	req := httptest.NewRequest(http.MethodGet, "/api/debates", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}
