package state

import (
	"sync"
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
)

func TestNewStore(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
}

func TestStoreSetAndGetPairs(t *testing.T) {
	s := NewStore()
	pairs := []client.PairStatus{
		{Name: "lint-review", Status: "debating", Active: true},
		{Name: "sec-audit", Status: "idle", Active: false},
	}
	s.SetPairs(pairs)

	got := s.Pairs()
	if len(got) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(got))
	}
	if got[0].Name != "lint-review" {
		t.Errorf("expected 'lint-review', got %q", got[0].Name)
	}
}

func TestStoreSetAndGetDebates(t *testing.T) {
	s := NewStore()
	debates := []client.DebateMeta{
		{ID: "d-001", Pair: "lint-review", Status: "active", RoundCount: 2, MaxRounds: 5, Started: time.Now()},
	}
	s.SetDebates(debates)

	got := s.Debates()
	if len(got) != 1 {
		t.Fatalf("expected 1 debate, got %d", len(got))
	}
	if got[0].ID != "d-001" {
		t.Errorf("expected 'd-001', got %q", got[0].ID)
	}
}

func TestStoreSetAndGetPlan(t *testing.T) {
	s := NewStore()
	plan := client.Plan{
		Epic: client.EpicConfig{
			Name:        "ratchet-monitor",
			Description: "Dashboard",
			Milestones: []client.Milestone{
				{ID: 6, Name: "TUI", Status: "in_progress"},
			},
		},
	}
	s.SetPlan(plan)

	got := s.Plan()
	if got.Epic.Name != "ratchet-monitor" {
		t.Errorf("expected 'ratchet-monitor', got %q", got.Epic.Name)
	}
	if len(got.Epic.Milestones) != 1 {
		t.Fatalf("expected 1 milestone, got %d", len(got.Epic.Milestones))
	}
}

func TestStoreSetAndGetStatus(t *testing.T) {
	s := NewStore()
	status := client.StatusInfo{MilestoneID: 6, MilestoneName: "TUI App Shell", Phase: "test"}
	s.SetStatus(status)

	got := s.Status()
	if got.MilestoneID != 6 {
		t.Errorf("expected milestone_id 6, got %d", got.MilestoneID)
	}
}

func TestStoreSetAndGetScores(t *testing.T) {
	s := NewStore()
	scores := []client.ScoreEntry{
		{DebateID: "d-001", Pair: "lint-review", Milestone: 6, RoundsToConsensus: 3},
	}
	s.SetScores("lint-review", scores)

	got := s.Scores("lint-review")
	if len(got) != 1 {
		t.Fatalf("expected 1 score, got %d", len(got))
	}
	if got[0].RoundsToConsensus != 3 {
		t.Errorf("expected rounds_to_consensus 3, got %d", got[0].RoundsToConsensus)
	}
}

func TestStoreScoresForUnknownPairReturnsEmpty(t *testing.T) {
	s := NewStore()
	got := s.Scores("nonexistent")
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 scores, got %d", len(got))
	}
}

func TestStoreConnectionState(t *testing.T) {
	s := NewStore()

	// Default should be Disconnected.
	if s.ConnectionState() != client.Disconnected {
		t.Errorf("expected initial state Disconnected, got %s", s.ConnectionState())
	}

	s.SetConnectionState(client.Connected)
	if s.ConnectionState() != client.Connected {
		t.Errorf("expected Connected, got %s", s.ConnectionState())
	}

	s.SetConnectionState(client.Reconnecting)
	if s.ConnectionState() != client.Reconnecting {
		t.Errorf("expected Reconnecting, got %s", s.ConnectionState())
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	s := NewStore()

	// Use a barrier to ensure all goroutines start simultaneously.
	var start sync.WaitGroup
	start.Add(1)

	var wg sync.WaitGroup

	// Writer goroutine 1 — pairs, debates, status, connection state.
	wg.Add(1)
	go func() {
		defer wg.Done()
		start.Wait()
		for i := 0; i < 100; i++ {
			s.SetPairs([]client.PairStatus{{Name: "pair", Status: "idle"}})
			s.SetDebates([]client.DebateMeta{{ID: "d-001"}})
			s.SetStatus(client.StatusInfo{MilestoneID: i})
			s.SetConnectionState(client.Connected)
		}
	}()

	// Writer goroutine 2 — plan, scores, apply events.
	wg.Add(1)
	go func() {
		defer wg.Done()
		start.Wait()
		for i := 0; i < 100; i++ {
			s.SetPlan(client.Plan{Epic: client.EpicConfig{Name: "test"}})
			s.SetScores("pair-a", []client.ScoreEntry{{DebateID: "d-001"}})
			_ = s.ApplyEvent(client.SSEEvent{ID: "1", Type: "debate:started", Data: []byte(`{}`)})
		}
	}()

	// Reader goroutine — reads all store methods concurrently with writers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		start.Wait()
		for i := 0; i < 100; i++ {
			_ = s.Pairs()
			_ = s.Debates()
			_ = s.Plan()
			_ = s.Status()
			_ = s.Scores("pair-a")
			_ = s.ConnectionState()
			_ = s.NeedsRefresh("debates")
			_ = s.LastEventID()
		}
	}()

	// Release all goroutines simultaneously.
	start.Done()
	wg.Wait()
}

func TestStoreApplySSEEvent(t *testing.T) {
	s := NewStore()

	// ApplyEvent should accept an SSE event and update state accordingly.
	// For debate:started, it should trigger a refresh of debates.
	ev := client.SSEEvent{
		ID:   "1",
		Type: "debate:started",
		Data: []byte(`{"id":1,"type":"debate:started","path":"debates/d-001.yaml","timestamp":"2026-03-15T10:00:00Z"}`),
	}

	err := s.ApplyEvent(ev)
	if err != nil {
		t.Fatalf("ApplyEvent() error: %v", err)
	}

	// The store should mark that it needs a refresh for the affected data.
	if !s.NeedsRefresh("debates") {
		t.Error("expected debates to need refresh after debate:started event")
	}
}

func TestStoreApplySSEEventTypes(t *testing.T) {
	tests := []struct {
		eventType    string
		needsRefresh string
	}{
		{"debate:started", "debates"},
		{"debate:updated", "debates"},
		{"debate:resolved", "debates"},
		{"score:updated", "scores"},
		{"pair:modified", "pairs"},
		{"plan:updated", "plan"},
		{"config:changed", "config"},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			s := NewStore()
			ev := client.SSEEvent{
				ID:   "1",
				Type: tt.eventType,
				Data: []byte(`{}`),
			}
			err := s.ApplyEvent(ev)
			if err != nil {
				t.Fatalf("ApplyEvent(%s) error: %v", tt.eventType, err)
			}
			if !s.NeedsRefresh(tt.needsRefresh) {
				t.Errorf("expected %s to need refresh after %s event", tt.needsRefresh, tt.eventType)
			}
		})
	}
}

func TestStoreLastEventID(t *testing.T) {
	s := NewStore()

	if s.LastEventID() != "" {
		t.Errorf("expected empty initial LastEventID, got %q", s.LastEventID())
	}

	ev := client.SSEEvent{ID: "42", Type: "debate:started", Data: []byte(`{}`)}
	_ = s.ApplyEvent(ev)

	if s.LastEventID() != "42" {
		t.Errorf("expected LastEventID '42', got %q", s.LastEventID())
	}
}
