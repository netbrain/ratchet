package views_test

import (
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

func seedScoreStore() *state.Store {
	s := state.NewStore()
	s.SetScores("api-design", []client.ScoreEntry{
		{Pair: "api-design", DebateID: "d1", Milestone: 1, RoundsToConsensus: 2, IssuesFound: 3, IssuesResolved: 2, Escalated: false, Timestamp: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)},
		{Pair: "api-design", DebateID: "d2", Milestone: 2, RoundsToConsensus: 1, IssuesFound: 1, IssuesResolved: 1, Escalated: false, Timestamp: time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)},
		{Pair: "api-design", DebateID: "d3", Milestone: 3, RoundsToConsensus: 3, IssuesFound: 5, IssuesResolved: 4, Escalated: true, Timestamp: time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)},
	})
	s.SetScores("go-idioms", []client.ScoreEntry{
		{Pair: "go-idioms", DebateID: "d4", Milestone: 1, RoundsToConsensus: 1, IssuesFound: 0, IssuesResolved: 0, Escalated: false, Timestamp: time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC)},
	})
	// Set pairs so the viewmodel knows which pairs exist
	s.SetPairs([]client.PairStatus{
		{Name: "api-design", Component: "backend"},
		{Name: "go-idioms", Component: "backend"},
		{Name: "tui-layout", Component: "tui"},
	})
	return s
}

// ── Construction ────────────────────────────────────────────────────────

func TestNewScoresViewModel(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)
	if vm == nil {
		t.Fatal("NewScoresViewModel returned nil")
	}
}

// ── Pair summaries ──────────────────────────────────────────────────────

func TestPairSummaries(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)
	summaries := vm.PairSummaries()

	// Should have entries for pairs that have scores
	if len(summaries) < 2 {
		t.Fatalf("expected at least 2 summaries, got %d", len(summaries))
	}
}

func TestPairSummaryMetrics(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)
	summaries := vm.PairSummaries()

	// Find api-design summary
	var apiDesign *views.PairScoreSummary
	for i := range summaries {
		if summaries[i].Pair == "api-design" {
			apiDesign = &summaries[i]
			break
		}
	}
	if apiDesign == nil {
		t.Fatal("api-design summary not found")
	}

	if apiDesign.DebateCount != 3 {
		t.Errorf("DebateCount = %d, want 3", apiDesign.DebateCount)
	}

	// 2 out of 3 reached consensus without escalation
	wantRate := 2.0 / 3.0
	if diff := apiDesign.ConsensusRate - wantRate; diff > 0.01 || diff < -0.01 {
		t.Errorf("ConsensusRate = %f, want ~%f", apiDesign.ConsensusRate, wantRate)
	}

	// Average rounds: (2+1+3)/3 = 2.0
	if diff := apiDesign.AvgRounds - 2.0; diff > 0.01 || diff < -0.01 {
		t.Errorf("AvgRounds = %f, want 2.0", apiDesign.AvgRounds)
	}

	if apiDesign.IssuesFound != 9 {
		t.Errorf("IssuesFound = %d, want 9", apiDesign.IssuesFound)
	}
	if apiDesign.IssuesResolved != 7 {
		t.Errorf("IssuesResolved = %d, want 7", apiDesign.IssuesResolved)
	}
	if apiDesign.Escalated != 1 {
		t.Errorf("Escalated = %d, want 1", apiDesign.Escalated)
	}
}

func TestPairSummaryTrend(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)
	summaries := vm.PairSummaries()

	var apiDesign *views.PairScoreSummary
	for i := range summaries {
		if summaries[i].Pair == "api-design" {
			apiDesign = &summaries[i]
			break
		}
	}
	if apiDesign == nil {
		t.Fatal("api-design summary not found")
	}

	// Trend should contain rounds-to-consensus values
	if len(apiDesign.Trend) != 3 {
		t.Fatalf("Trend length = %d, want 3", len(apiDesign.Trend))
	}
	// Values: 2, 1, 3 (ordered by timestamp)
	want := []int{2, 1, 3}
	for i, v := range apiDesign.Trend {
		if v != want[i] {
			t.Errorf("Trend[%d] = %d, want %d", i, v, want[i])
		}
	}
}

// ── Empty scores ────────────────────────────────────────────────────────

func TestPairSummariesEmpty(t *testing.T) {
	store := state.NewStore()
	vm := views.NewScoresViewModel(store)
	summaries := vm.PairSummaries()
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries with no data, got %d", len(summaries))
	}
}

// ── Selection ───────────────────────────────────────────────────────────

func TestScoresSelection(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)

	if vm.SelectedIndex() != 0 {
		t.Errorf("initial index = %d, want 0", vm.SelectedIndex())
	}

	vm.SelectNext()
	if vm.SelectedIndex() != 1 {
		t.Errorf("after SelectNext index = %d, want 1", vm.SelectedIndex())
	}

	vm.SelectPrev()
	if vm.SelectedIndex() != 0 {
		t.Errorf("after SelectPrev index = %d, want 0", vm.SelectedIndex())
	}
}

func TestScoresSelectionWraps(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)

	// SelectPrev at 0 should wrap to last
	vm.SelectPrev()
	summaries := vm.PairSummaries()
	if vm.SelectedIndex() != len(summaries)-1 {
		t.Errorf("SelectPrev at 0 should wrap to %d, got %d", len(summaries)-1, vm.SelectedIndex())
	}

	// SelectNext at last should wrap to 0
	vm.SelectNext()
	if vm.SelectedIndex() != 0 {
		t.Errorf("SelectNext at last should wrap to 0, got %d", vm.SelectedIndex())
	}
}

func TestScoresSelectedSummary(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)

	s := vm.SelectedSummary()
	if s == nil {
		t.Fatal("SelectedSummary returned nil")
	}
}

func TestScoresSelectedSummaryEmpty(t *testing.T) {
	store := state.NewStore()
	vm := views.NewScoresViewModel(store)

	if vm.SelectedSummary() != nil {
		t.Error("SelectedSummary with no data should be nil")
	}
}

// ── Refresh ─────────────────────────────────────────────────────────────

func TestScoresRefresh(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)

	initial := len(vm.PairSummaries())

	// Add new score data
	store.SetScores("tui-layout", []client.ScoreEntry{
		{Pair: "tui-layout", DebateID: "d5", RoundsToConsensus: 1},
	})
	vm.Refresh()

	if len(vm.PairSummaries()) <= initial {
		t.Error("Refresh should pick up new score data")
	}
}

// ── Viewport scroll offset (M10) ────────────────────────────────────────

func TestScoresSetViewportHeight(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)
	vm.SetViewportHeight(1)

	if vm.ScrollOffset() != 0 {
		t.Errorf("initial ScrollOffset = %d, want 0", vm.ScrollOffset())
	}
}

func TestScoresScrollOffsetFollowsSelection(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)
	vm.SetViewportHeight(1) // viewport shows 1 item

	// selected=0 visible
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset = %d, want 0", vm.ScrollOffset())
	}

	vm.SelectNext() // selected=1, needs scroll
	if vm.ScrollOffset() != 1 {
		t.Errorf("ScrollOffset = %d, want 1", vm.ScrollOffset())
	}

	vm.SelectPrev() // selected=0, needs scroll back
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset = %d, want 0 after scroll back", vm.ScrollOffset())
	}
}

func TestScoresScrollOffsetViewportLargerThanList(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)
	vm.SetViewportHeight(50) // much larger than 2 summaries

	vm.SelectNext()
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset = %d, want 0 (viewport larger than list)", vm.ScrollOffset())
	}
}

// ── Nil receiver safety ─────────────────────────────────────────────────

func TestScoresNilReceiver(t *testing.T) {
	var vm *views.ScoresViewModel

	// None of these should panic.
	if vm.PairSummaries() != nil {
		t.Error("nil receiver PairSummaries should return nil")
	}
	if vm.SelectedIndex() != 0 {
		t.Error("nil receiver SelectedIndex should return 0")
	}
	if vm.SelectedSummary() != nil {
		t.Error("nil receiver SelectedSummary should return nil")
	}
	vm.SelectNext()
	vm.SelectPrev()
	vm.Refresh()
}

// ── Selection on empty then populated ───────────────────────────────────

func TestScoresSelectionEmptyOps(t *testing.T) {
	store := state.NewStore()
	vm := views.NewScoresViewModel(store)

	// Operations on empty list should not panic.
	vm.SelectNext()
	vm.SelectPrev()
	if vm.SelectedIndex() != 0 {
		t.Errorf("empty SelectNext/Prev should keep index at 0, got %d", vm.SelectedIndex())
	}
}

func TestScoresRefreshClampsSelection(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)

	// Move selection to index 1
	vm.SelectNext()
	if vm.SelectedIndex() != 1 {
		t.Fatalf("precondition: selected = %d, want 1", vm.SelectedIndex())
	}

	// Remove all scores except one pair
	store.SetPairs([]client.PairStatus{
		{Name: "api-design", Component: "backend"},
	})
	store.SetScores("go-idioms", nil)
	vm.Refresh()

	// Selection should be clamped to valid range
	if vm.SelectedIndex() >= len(vm.PairSummaries()) && len(vm.PairSummaries()) > 0 {
		t.Errorf("selected index %d out of range for %d summaries", vm.SelectedIndex(), len(vm.PairSummaries()))
	}
}

// ── All escalated debates ───────────────────────────────────────────────

func TestPairSummaryAllEscalated(t *testing.T) {
	store := state.NewStore()
	store.SetPairs([]client.PairStatus{{Name: "test-pair"}})
	store.SetScores("test-pair", []client.ScoreEntry{
		{Pair: "test-pair", DebateID: "d1", RoundsToConsensus: 3, Escalated: true, Timestamp: time.Now()},
		{Pair: "test-pair", DebateID: "d2", RoundsToConsensus: 3, Escalated: true, Timestamp: time.Now()},
	})
	vm := views.NewScoresViewModel(store)
	summaries := vm.PairSummaries()
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].ConsensusRate != 0.0 {
		t.Errorf("all-escalated ConsensusRate = %f, want 0.0", summaries[0].ConsensusRate)
	}
	if summaries[0].Escalated != 2 {
		t.Errorf("Escalated = %d, want 2", summaries[0].Escalated)
	}
}

// ── Pair with empty scores is excluded ──────────────────────────────────

// ── HARDEN: Viewport scroll resets on wrap-around ────────────────────────

func TestScoresScrollResetsOnWrapForward(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)
	vm.SetViewportHeight(1) // 2 summaries, viewport of 1

	// Navigate to last item (index 1)
	vm.SelectNext()
	if vm.SelectedIndex() != 1 {
		t.Fatalf("precondition: selected = %d, want 1", vm.SelectedIndex())
	}

	// Wrap forward to 0
	vm.SelectNext()
	if vm.SelectedIndex() != 0 {
		t.Errorf("selected after wrap = %d, want 0", vm.SelectedIndex())
	}
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset should reset to 0 on wrap-around forward, got %d", vm.ScrollOffset())
	}
}

// ── HARDEN: Negative viewport height ────────────────────────────────────

func TestScoresNegativeViewportHeight(t *testing.T) {
	store := seedScoreStore()
	vm := views.NewScoresViewModel(store)
	vm.SetViewportHeight(-3)

	vm.SelectNext()
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset with negative viewport = %d, want 0", vm.ScrollOffset())
	}
}

// ── HARDEN: Nil receiver for M10 viewport methods ───────────────────────

func TestScoresNilReceiverViewport(t *testing.T) {
	var vm *views.ScoresViewModel
	vm.SetViewportHeight(5)
	if vm.ScrollOffset() != 0 {
		t.Errorf("nil ScrollOffset = %d, want 0", vm.ScrollOffset())
	}
}

// ── Pair with empty scores is excluded ──────────────────────────────────

func TestPairWithEmptyScoresExcluded(t *testing.T) {
	store := state.NewStore()
	store.SetPairs([]client.PairStatus{
		{Name: "has-scores"},
		{Name: "no-scores"},
	})
	store.SetScores("has-scores", []client.ScoreEntry{
		{Pair: "has-scores", DebateID: "d1", RoundsToConsensus: 1, Timestamp: time.Now()},
	})
	vm := views.NewScoresViewModel(store)
	summaries := vm.PairSummaries()
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary (empty-scores pair excluded), got %d", len(summaries))
	}
	if summaries[0].Pair != "has-scores" {
		t.Errorf("expected has-scores, got %s", summaries[0].Pair)
	}
}
