package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// testServer creates an httptest.Server that returns canned responses for
// all the API endpoints that loadAll/refreshDirty call. The optional
// sseHandler is wired to /events; if nil, /events returns 200 and closes.
func testServer(t *testing.T, sseHandler http.HandlerFunc) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/pairs", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.PairStatus{
			{Name: "pair-a", Phase: "plan", Enabled: true, Active: true, Status: "idle"},
		})
	})
	mux.HandleFunc("/api/debates", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.DebateMeta{
			{ID: "d1", Pair: "pair-a", Phase: "plan", Status: "active", RoundCount: 1, MaxRounds: 3},
		})
	})
	mux.HandleFunc("/api/debates/d1", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(client.DebateWithRounds{
			DebateMeta: client.DebateMeta{ID: "d1", Pair: "pair-a", Status: "active"},
			Rounds:     []client.Round{{Number: 1, Role: "generative", Content: "round 1"}},
		})
	})
	mux.HandleFunc("/api/plan", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(client.Plan{
			Epic: client.EpicConfig{Name: "test-epic", Description: "Testing epic"},
		})
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(client.StatusInfo{
			MilestoneID:   1,
			MilestoneName: "M1-tests",
			Phase:         "test",
		})
	})
	mux.HandleFunc("/api/scores", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.ScoreEntry{
			{Pair: "pair-a", Milestone: 1, RoundsToConsensus: 2},
		})
	})
	mux.HandleFunc("/api/workspaces", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.Workspace{
			{Name: "monitor", Path: "/workspace/monitor"},
			{Name: "frontend", Path: "/workspace/frontend"},
		})
	})

	if sseHandler != nil {
		mux.HandleFunc("/events", sseHandler)
	} else {
		mux.HandleFunc("/events", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			// Close immediately - the SSE client will see EOF.
		})
	}

	return httptest.NewServer(mux)
}

// --- Tab.String() all known branches ---

func TestTabStringKnownValues(t *testing.T) {
	tests := []struct {
		tab  Tab
		want string
	}{
		{TabPairs, "Pairs"},
		{TabDebates, "Debates"},
		{TabScores, "Scores"},
		{TabEpic, "Epic"},
		{Tab(99), "?"},
	}
	for _, tt := range tests {
		if got := tt.tab.String(); got != tt.want {
			t.Errorf("Tab(%d).String() = %q, want %q", int(tt.tab), got, tt.want)
		}
	}
}

// --- AllTabs ---

func TestAllTabsOrder(t *testing.T) {
	tabs := AllTabs()
	if len(tabs) != 4 {
		t.Fatalf("AllTabs() returned %d tabs, want 4", len(tabs))
	}
	expected := []Tab{TabPairs, TabDebates, TabScores, TabEpic}
	for i, tab := range tabs {
		if tab != expected[i] {
			t.Errorf("AllTabs()[%d] = %v, want %v", i, tab, expected[i])
		}
	}
}

// --- NextTab normal cycling ---

func TestNextTabFullCycle(t *testing.T) {
	a := &App{ActiveTab: TabPairs}
	expected := []Tab{TabDebates, TabScores, TabEpic, TabPairs}
	for i, want := range expected {
		a.NextTab()
		if a.ActiveTab != want {
			t.Errorf("step %d: NextTab got %v, want %v", i, a.ActiveTab, want)
		}
	}
}

// --- PrevTab normal cycling ---

func TestPrevTabFullCycle(t *testing.T) {
	a := &App{ActiveTab: TabPairs}
	expected := []Tab{TabEpic, TabScores, TabDebates, TabPairs}
	for i, want := range expected {
		a.PrevTab()
		if a.ActiveTab != want {
			t.Errorf("step %d: PrevTab got %v, want %v", i, a.ActiveTab, want)
		}
	}
}

// --- PrevTab from negative ---

func TestPrevTabFromNegative(t *testing.T) {
	a := &App{ActiveTab: Tab(-5)}
	a.PrevTab()
	if a.ActiveTab != TabEpic {
		t.Fatalf("PrevTab from negative should reset to last tab, got %v", a.ActiveTab)
	}
}

// --- SetTab valid ---

func TestSetTabValid(t *testing.T) {
	a := &App{ActiveTab: TabPairs}
	a.SetTab(TabScores)
	if a.ActiveTab != TabScores {
		t.Fatalf("SetTab(TabScores) = %v, want TabScores", a.ActiveTab)
	}
	a.SetTab(TabEpic)
	if a.ActiveTab != TabEpic {
		t.Fatalf("SetTab(TabEpic) = %v, want TabEpic", a.ActiveTab)
	}
}

// --- New ---

func TestNewCreatesAppWithDefaults(t *testing.T) {
	srv := testServer(t, nil)
	defer srv.Close()

	a := New(srv.URL)
	if a == nil {
		t.Fatal("New returned nil")
	}
	if a.Store == nil {
		t.Fatal("New().Store is nil")
	}
	if a.Client == nil {
		t.Fatal("New().Client is nil")
	}
	if a.ActiveTab != TabPairs {
		t.Errorf("New().ActiveTab = %v, want TabPairs", a.ActiveTab)
	}
}

// --- Start + loadAll (via test server) ---

func TestStartAndLoadAll(t *testing.T) {
	srv := testServer(t, nil)
	defer srv.Close()

	a := New(srv.URL)

	var updateCalled atomic.Int64
	a.SetOnUpdate(func() {
		updateCalled.Add(1)
	})

	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cancel()

	// Wait for loadAll goroutine to populate store.
	deadline := time.After(3 * time.Second)
	for {
		pairs := a.Store.Pairs()
		if len(pairs) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for loadAll to populate pairs")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Verify all data was loaded.
	pairs := a.Store.Pairs()
	if len(pairs) != 1 || pairs[0].Name != "pair-a" {
		t.Errorf("pairs = %+v, want 1 pair named pair-a", pairs)
	}

	debates := a.Store.Debates()
	if len(debates) != 1 || debates[0].ID != "d1" {
		t.Errorf("debates = %+v, want 1 debate with ID d1", debates)
	}

	plan := a.Store.Plan()
	if plan.Epic.Name != "test-epic" {
		t.Errorf("plan.Epic.Name = %q, want %q", plan.Epic.Name, "test-epic")
	}

	status := a.Store.Status()
	if status.MilestoneName != "M1-tests" {
		t.Errorf("status.MilestoneName = %q, want %q", status.MilestoneName, "M1-tests")
	}

	// Workspaces should be loaded, and first one set as current.
	ws := a.Store.Workspaces()
	if len(ws) != 2 {
		t.Errorf("workspaces = %v, want 2", ws)
	}
	if cur := a.Store.CurrentWorkspace(); cur != "monitor" {
		t.Errorf("CurrentWorkspace = %q, want %q", cur, "monitor")
	}

	// onUpdate callback should have been called.
	if updateCalled.Load() == 0 {
		t.Error("onUpdate callback was never called")
	}
}

// --- Start with loadAll errors ---

func TestStartLoadAllWithErrors(t *testing.T) {
	// Server that returns errors for all endpoints.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/pairs", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/debates", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/plan", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/workspaces", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := New(srv.URL)

	ctx, ctxCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cancel()

	// Give loadAll time to complete (it will log warnings but not crash).
	time.Sleep(200 * time.Millisecond)

	// Store should still be in default state.
	if len(a.Store.Pairs()) != 0 {
		t.Error("expected no pairs after error")
	}
	if len(a.Store.Debates()) != 0 {
		t.Error("expected no debates after error")
	}
}

// --- processEvents ---

func TestProcessEvents(t *testing.T) {
	// SSE handler that sends a pair:updated event and then closes.
	sseHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "flusher not supported", http.StatusInternalServerError)
			return
		}

		// Send an SSE event that will trigger a pairs refresh.
		fmt.Fprintf(w, "id: evt-1\nevent: pair:updated\ndata: {}\n\n")
		flusher.Flush()

		// Wait for context cancellation (client disconnect).
		<-r.Context().Done()
	}

	srv := testServer(t, sseHandler)
	defer srv.Close()

	a := New(srv.URL)

	var notified atomic.Int64
	a.SetOnUpdate(func() {
		notified.Add(1)
	})

	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cancel()

	// Wait for the SSE event to be processed and pairs refreshed.
	deadline := time.After(3 * time.Second)
	for {
		pairs := a.Store.Pairs()
		if len(pairs) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for SSE-triggered pairs refresh")
		case <-time.After(10 * time.Millisecond):
		}
	}

	if notified.Load() == 0 {
		t.Error("expected onUpdate to be called after SSE event")
	}
}

// --- processEvents with multiple event types ---

func TestProcessEventsMultipleTypes(t *testing.T) {
	sseHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// Send debate, score, plan events.
		fmt.Fprintf(w, "event: debate:created\ndata: {}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: score:recorded\ndata: {}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: plan:updated\ndata: {}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: config:changed\ndata: {}\n\n")
		flusher.Flush()

		<-r.Context().Done()
	}

	srv := testServer(t, sseHandler)
	defer srv.Close()

	a := New(srv.URL)

	var notified atomic.Int64
	a.SetOnUpdate(func() {
		notified.Add(1)
	})

	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cancel()

	// Wait for events to be processed.
	deadline := time.After(3 * time.Second)
	for {
		if notified.Load() >= 4 { // loadAll + at least some events
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out; only got %d notifications", notified.Load())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// --- processEvents context cancellation ---

func TestProcessEventsStopsOnCancel(t *testing.T) {
	sseHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// Keep connection open.
		fmt.Fprintf(w, ": keepalive\n\n")
		flusher.Flush()
		<-r.Context().Done()
	}

	srv := testServer(t, sseHandler)
	defer srv.Close()

	a := New(srv.URL)

	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Cancel immediately.
	cancel()

	// Give a moment for goroutines to wind down.
	time.Sleep(100 * time.Millisecond)
}

// --- FetchDebateDetail ---

func TestFetchDebateDetail(t *testing.T) {
	srv := testServer(t, nil)
	defer srv.Close()

	a := New(srv.URL)

	a.FetchDebateDetail("d1")

	// Wait for the async goroutine to populate the store.
	deadline := time.After(3 * time.Second)
	for {
		if detail := a.Store.DebateDetail("d1"); detail != nil {
			if detail.ID != "d1" {
				t.Errorf("debate detail ID = %q, want %q", detail.ID, "d1")
			}
			if len(detail.Rounds) != 1 {
				t.Errorf("debate rounds = %d, want 1", len(detail.Rounds))
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for FetchDebateDetail")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestFetchDebateDetailNotFound(t *testing.T) {
	// Server that returns 404 for any debate detail.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/debates/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := &App{
		Store:  state.NewStore(),
		Client: client.NewClient(srv.URL),
	}

	var notified atomic.Bool
	a.SetOnUpdate(func() {
		notified.Store(true)
	})

	a.FetchDebateDetail("nonexistent")

	// Give time for the goroutine to complete.
	time.Sleep(200 * time.Millisecond)

	// Should not have updated store.
	if detail := a.Store.DebateDetail("nonexistent"); detail != nil {
		t.Error("expected no debate detail for nonexistent ID")
	}
	// onUpdate should NOT have been called on error.
	if notified.Load() {
		t.Error("onUpdate should not be called on error")
	}
}

func TestFetchDebateDetailTriggersOnUpdate(t *testing.T) {
	srv := testServer(t, nil)
	defer srv.Close()

	a := New(srv.URL)

	var notified atomic.Bool
	a.SetOnUpdate(func() {
		notified.Store(true)
	})

	a.FetchDebateDetail("d1")

	deadline := time.After(3 * time.Second)
	for {
		if notified.Load() {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for onUpdate after FetchDebateDetail")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// --- StatusLine with milestone ---

func TestStatusLineWithMilestone(t *testing.T) {
	a := &App{Store: state.NewStore()}
	a.Store.SetStatus(client.StatusInfo{
		MilestoneID:   2,
		MilestoneName: "M2-coverage",
		Phase:         "test",
	})

	line := a.StatusLine()
	if !strings.Contains(line, "M2") {
		t.Errorf("StatusLine should contain milestone ID, got %q", line)
	}
	if !strings.Contains(line, "M2-coverage") {
		t.Errorf("StatusLine should contain milestone name, got %q", line)
	}
	if !strings.Contains(line, "test") {
		t.Errorf("StatusLine should contain phase, got %q", line)
	}
}

func TestStatusLineWithMilestoneAndWorkspace(t *testing.T) {
	a := &App{Store: state.NewStore()}
	a.Store.SetCurrentWorkspace("monitor")
	a.Store.SetStatus(client.StatusInfo{
		MilestoneID:   1,
		MilestoneName: "M1-bootstrap",
		Phase:         "plan",
	})

	line := a.StatusLine()
	if !strings.Contains(line, "WS:monitor") {
		t.Errorf("StatusLine should contain workspace, got %q", line)
	}
	if !strings.Contains(line, "M1-bootstrap") {
		t.Errorf("StatusLine should contain milestone, got %q", line)
	}
}

func TestStatusLineNoMilestoneNoWorkspace(t *testing.T) {
	a := &App{Store: state.NewStore()}
	line := a.StatusLine()
	// Should just be [disconnected]
	if !strings.Contains(line, "disconnected") {
		t.Errorf("StatusLine should contain connection state, got %q", line)
	}
	if strings.Contains(line, "WS:") {
		t.Errorf("StatusLine should not contain WS: without workspace, got %q", line)
	}
}

// --- CycleWorkspace with nil store ---

func TestCycleWorkspaceNilStore(t *testing.T) {
	a := &App{}
	a.CycleWorkspace() // must not panic
}

// --- CycleWorkspace current not found ---

func TestCycleWorkspaceCurrentNotFound(t *testing.T) {
	a := &App{Store: state.NewStore()}
	a.Store.SetWorkspaces([]string{"alpha", "beta"})
	a.Store.SetCurrentWorkspace("nonexistent")

	a.CycleWorkspace()
	// When current is not found, idx = -1, next = (−1+1)%2 = 0 → "alpha"
	if got := a.Store.CurrentWorkspace(); got != "alpha" {
		t.Errorf("CycleWorkspace with missing current = %q, want %q", got, "alpha")
	}
}

// --- CycleWorkspace triggers onUpdate ---

func TestCycleWorkspaceNotifiesUpdate(t *testing.T) {
	a := &App{Store: state.NewStore()}
	a.Store.SetWorkspaces([]string{"ws1", "ws2"})
	a.Store.SetCurrentWorkspace("ws1")

	var notified atomic.Bool
	a.SetOnUpdate(func() {
		notified.Store(true)
	})

	a.CycleWorkspace()
	if !notified.Load() {
		t.Error("CycleWorkspace should trigger onUpdate")
	}
}

// --- refreshDirty with scores ---

func TestRefreshDirtyScores(t *testing.T) {
	sseHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// Send a score event to trigger scores refresh.
		fmt.Fprintf(w, "event: score:recorded\ndata: {}\n\n")
		flusher.Flush()

		<-r.Context().Done()
	}

	srv := testServer(t, sseHandler)
	defer srv.Close()

	a := New(srv.URL)

	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cancel()

	// Wait for scores to be populated by refreshDirty.
	deadline := time.After(3 * time.Second)
	for {
		scores := a.Store.Scores("")
		if len(scores) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for scores refresh via SSE")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// --- loadAll workspace auto-selection ---

func TestLoadAllSetsFirstWorkspaceAsCurrent(t *testing.T) {
	srv := testServer(t, nil)
	defer srv.Close()

	a := New(srv.URL)

	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cancel()

	// Wait for workspaces to load.
	deadline := time.After(3 * time.Second)
	for {
		ws := a.Store.Workspaces()
		if len(ws) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for workspaces to load")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// loadAll should auto-select first workspace.
	if got := a.Store.CurrentWorkspace(); got != "monitor" {
		t.Errorf("CurrentWorkspace = %q, want %q", got, "monitor")
	}
}

// --- loadAll does NOT override existing workspace ---

func TestLoadAllPreservesExistingWorkspace(t *testing.T) {
	srv := testServer(t, nil)
	defer srv.Close()

	a := New(srv.URL)
	// Pre-set a workspace before loadAll runs.
	a.Store.SetCurrentWorkspace("pre-existing")

	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cancel()

	// Wait for workspaces to load.
	deadline := time.After(3 * time.Second)
	for {
		ws := a.Store.Workspaces()
		if len(ws) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for workspaces to load")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Should keep pre-existing workspace since it's non-empty.
	if got := a.Store.CurrentWorkspace(); got != "pre-existing" {
		t.Errorf("CurrentWorkspace = %q, want %q (should preserve existing)", got, "pre-existing")
	}
}

// --- processEvents with unknown event type ---

func TestProcessEventsUnknownType(t *testing.T) {
	sseHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// Unknown event type - should not crash.
		fmt.Fprintf(w, "event: unknown:event\ndata: {}\n\n")
		flusher.Flush()

		<-r.Context().Done()
	}

	srv := testServer(t, sseHandler)
	defer srv.Close()

	a := New(srv.URL)

	ctx, ctxCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cancel()

	// Give time for event to be processed without crashing.
	time.Sleep(200 * time.Millisecond)
}

// --- loadAll with empty workspaces ---

func TestLoadAllEmptyWorkspaces(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/pairs", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.PairStatus{})
	})
	mux.HandleFunc("/api/debates", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.DebateMeta{})
	})
	mux.HandleFunc("/api/plan", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(client.Plan{})
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(client.StatusInfo{})
	})
	mux.HandleFunc("/api/workspaces", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.Workspace{})
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := New(srv.URL)

	ctx, ctxCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cancel()

	// Give loadAll time.
	time.Sleep(200 * time.Millisecond)

	// No workspaces, so current should remain empty.
	if got := a.Store.CurrentWorkspace(); got != "" {
		t.Errorf("CurrentWorkspace = %q, want empty", got)
	}
}

// --- loadAll with nil plan ---

func TestLoadAllNilPlan(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/pairs", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.PairStatus{})
	})
	mux.HandleFunc("/api/debates", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.DebateMeta{})
	})
	mux.HandleFunc("/api/plan", func(w http.ResponseWriter, _ *http.Request) {
		// Return JSON null - Plan will be nil.
		fmt.Fprintf(w, "null")
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "null")
	})
	mux.HandleFunc("/api/workspaces", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.Workspace{})
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := New(srv.URL)

	ctx, ctxCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cancel()

	time.Sleep(200 * time.Millisecond)
	// Should not crash.
}

// --- refreshDirty with errors ---

func TestRefreshDirtyWithErrors(t *testing.T) {
	// Server that returns valid SSE events but 500s on refresh endpoints.
	callCount := &atomic.Int64{}

	mux := http.NewServeMux()
	// Initially serve valid data for loadAll, then fail on refreshes.
	mux.HandleFunc("/api/pairs", func(w http.ResponseWriter, _ *http.Request) {
		c := callCount.Add(1)
		if c <= 1 {
			_ = json.NewEncoder(w).Encode([]client.PairStatus{{Name: "p1"}})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/api/debates", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.DebateMeta{})
	})
	mux.HandleFunc("/api/plan", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(client.Plan{})
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(client.StatusInfo{})
	})
	mux.HandleFunc("/api/scores", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/workspaces", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]client.Workspace{})
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// Trigger pairs and scores refresh.
		fmt.Fprintf(w, "event: pair:updated\ndata: {}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: score:recorded\ndata: {}\n\n")
		flusher.Flush()
		<-r.Context().Done()
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := New(srv.URL)

	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	cancel, err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cancel()

	// Wait for events to be processed (and refreshDirty to run with errors).
	time.Sleep(500 * time.Millisecond)

	// Should not crash. Original pairs from loadAll should remain.
	pairs := a.Store.Pairs()
	if len(pairs) != 1 {
		t.Errorf("pairs should still have 1 from initial load, got %d", len(pairs))
	}
}
