package views_test

import (
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

func seedDebateStore(debates []client.DebateMeta) *state.Store {
	s := state.NewStore()
	s.SetDebates(debates)
	return s
}

var sampleDebates = []client.DebateMeta{
	{ID: "api-design-m1-review-20260301", Pair: "api-design", Phase: "review", Milestone: 1, Status: "consensus", RoundCount: 2, MaxRounds: 3, Started: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)},
	{ID: "sse-correct-m2-test-20260305", Pair: "sse-correctness", Phase: "test", Milestone: 2, Status: "in_progress", RoundCount: 1, MaxRounds: 3, Started: time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)},
	{ID: "tui-layout-m7-build-20260315", Pair: "tui-layout", Phase: "build", Milestone: 7, Status: "consensus", RoundCount: 1, MaxRounds: 3, Started: time.Date(2026, 3, 15, 12, 30, 0, 0, time.UTC)},
	{ID: "tui-client-m7-test-20260315", Pair: "tui-client", Phase: "test", Milestone: 7, Status: "escalated", RoundCount: 3, MaxRounds: 3, Started: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)},
	{ID: "go-idioms-m6-review-20260315", Pair: "go-idioms", Phase: "review", Milestone: 6, Status: "initiated", RoundCount: 0, MaxRounds: 3, Started: time.Date(2026, 3, 15, 11, 0, 0, 0, time.UTC)},
}

// ── Construction ────────────────────────────────────────────────────────

func TestNewDebatesViewModel(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)
	if vm == nil {
		t.Fatal("NewDebatesViewModel returned nil")
	}
}

// ── Debates list ────────────────────────────────────────────────────────

func TestDebatesReturnAll(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)
	debates := vm.Debates()
	if len(debates) != len(sampleDebates) {
		t.Fatalf("expected %d debates, got %d", len(sampleDebates), len(debates))
	}
}

// ── Status color mapping ────────────────────────────────────────────────

func TestDebateStatusColor(t *testing.T) {
	store := seedDebateStore(nil)
	vm := views.NewDebatesViewModel(store)

	tests := []struct {
		status string
		want   string
	}{
		{"initiated", "yellow"},
		{"in_progress", "cyan"},
		{"consensus", "green"},
		{"escalated", "red"},
		{"resolved", "dim"},
		{"unknown", "white"},
		{"", "white"},
	}
	for _, tt := range tests {
		got := vm.DebateStatusColor(tt.status)
		if got != tt.want {
			t.Errorf("DebateStatusColor(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

// ── Text filtering ──────────────────────────────────────────────────────

func TestDebateFilterByPairName(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	vm.SetFilter("tui")
	filtered := vm.FilteredDebates()
	if len(filtered) != 2 {
		t.Fatalf("expected 2 debates matching 'tui', got %d", len(filtered))
	}
	for _, d := range filtered {
		if d.Pair != "tui-layout" && d.Pair != "tui-client" {
			t.Errorf("unexpected pair %q in filtered results", d.Pair)
		}
	}
}

func TestDebateFilterByID(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	vm.SetFilter("m7-build")
	filtered := vm.FilteredDebates()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 debate matching 'm7-build', got %d", len(filtered))
	}
	if filtered[0].ID != "tui-layout-m7-build-20260315" {
		t.Errorf("filtered debate ID = %q, want tui-layout-m7-build-20260315", filtered[0].ID)
	}
}

func TestDebateFilterCaseInsensitive(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	vm.SetFilter("SSE")
	filtered := vm.FilteredDebates()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 debate matching 'SSE', got %d", len(filtered))
	}
}

func TestDebateFilterEmptyReturnsAll(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	vm.SetFilter("")
	if len(vm.FilteredDebates()) != len(sampleDebates) {
		t.Fatalf("empty filter should return all debates")
	}
}

// ── Status filter ───────────────────────────────────────────────────────

func TestDebateStatusFilter(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	vm.SetStatusFilter("consensus")
	filtered := vm.FilteredDebates()
	if len(filtered) != 2 {
		t.Fatalf("expected 2 consensus debates, got %d", len(filtered))
	}
	for _, d := range filtered {
		if d.Status != "consensus" {
			t.Errorf("expected status consensus, got %q", d.Status)
		}
	}
}

func TestDebateStatusFilterAll(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	vm.SetStatusFilter("") // empty = all
	if len(vm.FilteredDebates()) != len(sampleDebates) {
		t.Fatal("empty status filter should return all debates")
	}
}

func TestDebateCombinedFilters(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	vm.SetFilter("tui")
	vm.SetStatusFilter("escalated")
	filtered := vm.FilteredDebates()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 debate matching tui+escalated, got %d", len(filtered))
	}
	if filtered[0].Pair != "tui-client" {
		t.Errorf("expected tui-client, got %q", filtered[0].Pair)
	}
}

// ── Selection ───────────────────────────────────────────────────────────

func TestDebateSelectionNavigation(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

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

func TestDebateSelectionWraps(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	vm.SelectPrev()
	if vm.SelectedIndex() != len(sampleDebates)-1 {
		t.Errorf("SelectPrev at 0 should wrap, got %d", vm.SelectedIndex())
	}
}

func TestDebateSelectedDebate(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	d := vm.SelectedDebate()
	if d == nil {
		t.Fatal("SelectedDebate returned nil")
	}
	// Debates are sorted newest-first, so the first selected is the newest
	if d.ID != "tui-layout-m7-build-20260315" {
		t.Errorf("SelectedDebate ID = %q, want newest debate (tui-layout-m7-build-20260315)", d.ID)
	}
}

func TestDebateSelectedDebateEmpty(t *testing.T) {
	store := seedDebateStore(nil)
	vm := views.NewDebatesViewModel(store)

	if vm.SelectedDebate() != nil {
		t.Error("SelectedDebate with empty list should be nil")
	}
}

// ── Selection clamp after filter ────────────────────────────────────────

func TestDebateSelectionClampsAfterFilter(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	// Move to last item (index 4)
	for range 4 {
		vm.SelectNext()
	}
	if vm.SelectedIndex() != 4 {
		t.Fatalf("precondition: index = %d, want 4", vm.SelectedIndex())
	}

	// Filter to 1 result — selection must clamp
	vm.SetStatusFilter("initiated")
	filtered := vm.FilteredDebates()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 initiated debate, got %d", len(filtered))
	}
	if vm.SelectedIndex() != 0 {
		t.Errorf("SelectedIndex should clamp to 0, got %d", vm.SelectedIndex())
	}
}

// ── Ordering ────────────────────────────────────────────────────────────

func TestDebatesOrderedByStartTimeDesc(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	debates := vm.FilteredDebates()
	for i := 1; i < len(debates); i++ {
		if debates[i].Started.After(debates[i-1].Started) {
			t.Errorf("debates not ordered by start time desc: [%d]=%v > [%d]=%v",
				i, debates[i].Started, i-1, debates[i-1].Started)
		}
	}
}

// ── Refresh ─────────────────────────────────────────────────────────────

func TestDebateRefresh(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	store.SetDebates([]client.DebateMeta{
		{ID: "new-debate", Pair: "new", Status: "initiated"},
	})
	vm.Refresh()

	if len(vm.Debates()) != 1 {
		t.Fatalf("after Refresh expected 1 debate, got %d", len(vm.Debates()))
	}
	if vm.Debates()[0].ID != "new-debate" {
		t.Errorf("debate ID = %q, want new-debate", vm.Debates()[0].ID)
	}
}

// ── Status cycle ────────────────────────────────────────────────────────

func TestDebateCycleStatusFilter(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	// Initial state: no filter
	if vm.StatusFilter() != "" {
		t.Errorf("initial StatusFilter = %q, want empty", vm.StatusFilter())
	}

	// Cycle through all statuses
	expected := []string{"initiated", "in_progress", "consensus", "escalated", "resolved", ""}
	for _, want := range expected {
		vm.CycleStatusFilter()
		if vm.StatusFilter() != want {
			t.Errorf("CycleStatusFilter: got %q, want %q", vm.StatusFilter(), want)
		}
	}
}

// ── Viewport scroll offset (M10) ────────────────────────────────────────

func TestDebatesSetViewportHeight(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)
	vm.SetViewportHeight(3)

	if vm.ScrollOffset() != 0 {
		t.Errorf("initial ScrollOffset = %d, want 0", vm.ScrollOffset())
	}
}

func TestDebatesScrollOffsetFollowsSelection(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)
	vm.SetViewportHeight(2)

	vm.SelectNext() // selected=1, visible in viewport [0,1]
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset = %d, want 0", vm.ScrollOffset())
	}

	vm.SelectNext() // selected=2, needs scroll
	if vm.ScrollOffset() != 1 {
		t.Errorf("ScrollOffset = %d, want 1", vm.ScrollOffset())
	}

	vm.SelectNext() // selected=3
	if vm.ScrollOffset() != 2 {
		t.Errorf("ScrollOffset = %d, want 2", vm.ScrollOffset())
	}

	vm.SelectPrev() // selected=2
	vm.SelectPrev() // selected=1
	if vm.ScrollOffset() != 1 {
		t.Errorf("ScrollOffset = %d, want 1 after scrolling back", vm.ScrollOffset())
	}
}

func TestDebatesScrollOffsetViewportLargerThanList(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)
	vm.SetViewportHeight(20)

	for i := 0; i < len(sampleDebates); i++ {
		vm.SelectNext()
		if vm.ScrollOffset() != 0 {
			t.Errorf("ScrollOffset = %d, want 0 (viewport larger than list)", vm.ScrollOffset())
		}
	}
}

// ── Filter getter ───────────────────────────────────────────────────────

// ── HARDEN: Nil receiver safety ──────────────────────────────────────────

func TestDebatesNilReceiver(t *testing.T) {
	var vm *views.DebatesViewModel

	// None of these should panic.
	if vm.Debates() != nil {
		t.Error("nil Debates should return nil")
	}
	if vm.FilteredDebates() != nil {
		t.Error("nil FilteredDebates should return nil")
	}
	if vm.Filter() != "" {
		t.Error("nil Filter should return empty")
	}
	if vm.StatusFilter() != "" {
		t.Error("nil StatusFilter should return empty")
	}
	if vm.SelectedIndex() != 0 {
		t.Error("nil SelectedIndex should return 0")
	}
	if vm.SelectedDebate() != nil {
		t.Error("nil SelectedDebate should return nil")
	}
	if vm.ScrollOffset() != 0 {
		t.Error("nil ScrollOffset should return 0")
	}
	vm.SelectNext()
	vm.SelectPrev()
	vm.SetFilter("test")
	vm.SetStatusFilter("consensus")
	vm.CycleStatusFilter()
	vm.SetViewportHeight(5)
	vm.Refresh()
}

// ── HARDEN: Viewport scroll resets on wrap-around ────────────────────────

func TestDebatesScrollResetsOnWrapForward(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)
	vm.SetViewportHeight(2)

	// Navigate to last item
	n := len(sampleDebates)
	for i := 0; i < n-1; i++ {
		vm.SelectNext()
	}
	if vm.SelectedIndex() != n-1 {
		t.Fatalf("precondition: selected = %d, want %d", vm.SelectedIndex(), n-1)
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

func TestDebatesNegativeViewportHeight(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)
	vm.SetViewportHeight(-1)

	vm.SelectNext()
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset with negative viewport = %d, want 0", vm.ScrollOffset())
	}
}

// ── Filter getter ───────────────────────────────────────────────────────

func TestDebateFilterGetter(t *testing.T) {
	store := seedDebateStore(sampleDebates)
	vm := views.NewDebatesViewModel(store)

	if vm.Filter() != "" {
		t.Errorf("initial Filter = %q, want empty", vm.Filter())
	}

	vm.SetFilter("tui")
	if vm.Filter() != "tui" {
		t.Errorf("Filter after set = %q, want tui", vm.Filter())
	}
}
