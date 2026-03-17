package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestServer returns an httptest.Server that serves canned JSON responses
// for all REST endpoints.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/pairs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]PairStatus{
			{Name: "lint-review", Component: "linter", Phase: "test", Scope: "module", Enabled: true, Active: true, Status: "debating"},
			{Name: "sec-audit", Component: "security", Phase: "review", Scope: "repo", Enabled: true, Active: false, Status: "idle"},
		})
	})

	mux.HandleFunc("GET /api/debates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		started := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
		json.NewEncoder(w).Encode([]DebateMeta{
			{ID: "d-001", Pair: "lint-review", Phase: "test", Milestone: 6, Files: []string{"main.go"}, Status: "active", RoundCount: 2, MaxRounds: 5, Started: started},
		})
	})

	mux.HandleFunc("GET /api/debates/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id != "d-001" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "debate not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		started := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
		json.NewEncoder(w).Encode(DebateWithRounds{
			DebateMeta: DebateMeta{ID: "d-001", Pair: "lint-review", Phase: "test", Milestone: 6, Files: []string{"main.go"}, Status: "active", RoundCount: 2, MaxRounds: 5, Started: started},
			Rounds: []Round{
				{Number: 1, Role: "challenger", Content: "Found issue in error handling"},
				{Number: 2, Role: "defender", Content: "Addressed with retry logic"},
			},
		})
	})

	mux.HandleFunc("GET /api/plan", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Plan{
			Epic: EpicConfig{
				Name:        "ratchet-monitor",
				Description: "Real-time dashboard",
				Milestones: []Milestone{
					{ID: 6, Name: "TUI App Shell", Description: "TUI scaffold", Pairs: []string{"lint-review"}, Status: "in_progress", PhaseStatus: map[string]string{"test": "active"}, DoneWhen: "binary connects"},
				},
				CurrentFocus: &CurrentFocus{MilestoneID: 6, Phase: "test", Started: "2026-03-15"},
			},
		})
	})

	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(StatusInfo{MilestoneID: 6, MilestoneName: "TUI App Shell", Phase: "test"})
	})

	mux.HandleFunc("GET /api/scores", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ts := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
		json.NewEncoder(w).Encode([]ScoreEntry{
			{Timestamp: ts, DebateID: "d-001", Pair: "lint-review", Milestone: 6, RoundsToConsensus: 3, Escalated: false, IssuesFound: 2, IssuesResolved: 2},
		})
	})

	mux.HandleFunc("GET /api/workspaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Workspace{
			{Name: "frontend", Path: "/home/dev/frontend"},
			{Name: "backend", Path: "/home/dev/backend"},
		})
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthStatus{Status: "ok"})
	})

	return httptest.NewServer(mux)
}

func TestNewClient(t *testing.T) {
	// NewClient should accept a base URL and return a usable Client.
	c := NewClient("http://localhost:8080")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
}

func TestNewClientWithHTTPClient(t *testing.T) {
	// NewClient should accept functional options, including a custom http.Client.
	custom := &http.Client{Timeout: 5 * time.Second}
	c := NewClient("http://localhost:8080", WithHTTPClient(custom))
	if c == nil {
		t.Fatal("NewClient with custom HTTP client returned nil")
	}
}

func TestFetchPairs(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := NewClient(srv.URL)
	pairs, err := c.Pairs(context.Background())
	if err != nil {
		t.Fatalf("Pairs() error: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	if pairs[0].Name != "lint-review" {
		t.Errorf("expected first pair name 'lint-review', got %q", pairs[0].Name)
	}
	if !pairs[0].Enabled {
		t.Error("expected first pair to be enabled")
	}
	if pairs[0].Status != "debating" {
		t.Errorf("expected status 'debating', got %q", pairs[0].Status)
	}
}

func TestFetchDebates(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := NewClient(srv.URL)
	debates, err := c.Debates(context.Background())
	if err != nil {
		t.Fatalf("Debates() error: %v", err)
	}
	if len(debates) != 1 {
		t.Fatalf("expected 1 debate, got %d", len(debates))
	}
	if debates[0].ID != "d-001" {
		t.Errorf("expected debate ID 'd-001', got %q", debates[0].ID)
	}
	if debates[0].RoundCount != 2 {
		t.Errorf("expected round_count 2, got %d", debates[0].RoundCount)
	}
}

func TestFetchDebateDetail(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := NewClient(srv.URL)
	detail, err := c.Debate(context.Background(), "d-001")
	if err != nil {
		t.Fatalf("Debate() error: %v", err)
	}
	if detail.ID != "d-001" {
		t.Errorf("expected ID 'd-001', got %q", detail.ID)
	}
	if len(detail.Rounds) != 2 {
		t.Fatalf("expected 2 rounds, got %d", len(detail.Rounds))
	}
	if detail.Rounds[0].Role != "challenger" {
		t.Errorf("expected round 1 role 'challenger', got %q", detail.Rounds[0].Role)
	}
}

func TestFetchDebateDetailNotFound(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Debate(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent debate, got nil")
	}
}

func TestFetchPlan(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := NewClient(srv.URL)
	plan, err := c.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}
	if plan.Epic.Name != "ratchet-monitor" {
		t.Errorf("expected epic name 'ratchet-monitor', got %q", plan.Epic.Name)
	}
	if len(plan.Epic.Milestones) != 1 {
		t.Fatalf("expected 1 milestone, got %d", len(plan.Epic.Milestones))
	}
	if plan.Epic.CurrentFocus == nil {
		t.Fatal("expected current_focus to be non-nil")
	}
	if plan.Epic.CurrentFocus.MilestoneID != 6 {
		t.Errorf("expected focus milestone 6, got %d", plan.Epic.CurrentFocus.MilestoneID)
	}
}

func TestFetchStatus(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := NewClient(srv.URL)
	status, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if status.MilestoneID != 6 {
		t.Errorf("expected milestone_id 6, got %d", status.MilestoneID)
	}
	if status.Phase != "test" {
		t.Errorf("expected phase 'test', got %q", status.Phase)
	}
}

func TestFetchScores(t *testing.T) {
	// Custom server that verifies the ?pair= query parameter is sent.
	var receivedPair string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPair = r.URL.Query().Get("pair")
		w.Header().Set("Content-Type", "application/json")
		ts := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
		json.NewEncoder(w).Encode([]ScoreEntry{
			{Timestamp: ts, DebateID: "d-001", Pair: "lint-review", Milestone: 6, RoundsToConsensus: 3, Escalated: false, IssuesFound: 2, IssuesResolved: 2},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	scores, err := c.Scores(context.Background(), "lint-review")
	if err != nil {
		t.Fatalf("Scores() error: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(scores))
	}
	if scores[0].RoundsToConsensus != 3 {
		t.Errorf("expected rounds_to_consensus 3, got %d", scores[0].RoundsToConsensus)
	}
	if receivedPair != "lint-review" {
		t.Errorf("expected server to receive ?pair=lint-review, got %q", receivedPair)
	}
}

func TestFetchScoresNoPairFilter(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := NewClient(srv.URL)
	scores, err := c.Scores(context.Background(), "")
	if err != nil {
		t.Fatalf("Scores('') error: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(scores))
	}
}

func TestHealthCheck(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := NewClient(srv.URL)
	health, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error: %v", err)
	}
	if health.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", health.Status)
	}
}

func TestContextCancellation(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := c.Pairs(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Pairs(context.Background())
	if err == nil {
		t.Fatal("expected error from 500 response, got nil")
	}
}

func TestUnreachableServer(t *testing.T) {
	c := NewClient("http://127.0.0.1:1") // unlikely to be listening
	_, err := c.Health(context.Background())
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
}

func TestFetchWorkspaces(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := NewClient(srv.URL)
	workspaces, err := c.Workspaces(context.Background())
	if err != nil {
		t.Fatalf("Workspaces() error: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
	if workspaces[0].Name != "frontend" {
		t.Errorf("expected first workspace name 'frontend', got %q", workspaces[0].Name)
	}
	if workspaces[1].Path != "/home/dev/backend" {
		t.Errorf("expected second workspace path '/home/dev/backend', got %q", workspaces[1].Path)
	}
}

func TestMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Pairs(context.Background())
	if err == nil {
		t.Fatal("expected JSON decode error, got nil")
	}
}
