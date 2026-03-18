package views_test

import (
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

func seedStore(pairs []client.PairStatus) *state.Store {
	s := state.NewStore()
	s.SetPairs(pairs)
	return s
}

var samplePairs = []client.PairStatus{
	{Name: "arch-review", Component: "backend", Phase: "design", Status: "debating", Active: true},
	{Name: "code-quality", Component: "backend", Phase: "impl", Status: "idle", Active: false},
	{Name: "ux-audit", Component: "frontend", Phase: "design", Status: "consensus", Active: true},
	{Name: "api-contract", Component: "frontend", Phase: "test", Status: "escalated", Active: true},
	{Name: "perf-bench", Component: "infra", Phase: "impl", Status: "idle", Active: false},
}

// ── Construction ────────────────────────────────────────────────────────

func TestNewPairsViewModel(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)
	if vm == nil {
		t.Fatal("NewPairsViewModel returned nil")
	}
}

// ── Pairs list ──────────────────────────────────────────────────────────

func TestPairsReturnAll(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)
	pairs := vm.Pairs()
	if len(pairs) != len(samplePairs) {
		t.Fatalf("expected %d pairs, got %d", len(samplePairs), len(pairs))
	}
	for i, p := range pairs {
		if p.Name != samplePairs[i].Name {
			t.Errorf("pair[%d] name = %q, want %q", i, p.Name, samplePairs[i].Name)
		}
	}
}

// ── Status color mapping ────────────────────────────────────────────────

func TestStatusColor(t *testing.T) {
	store := seedStore(nil)
	vm := views.NewPairsViewModel(store)

	tests := []struct {
		status string
		want   string
	}{
		{"debating", "cyan"},
		{"escalated", "red"},
		{"consensus", "green"},
		{"idle", "dim"},
		{"unknown-thing", "white"},
		{"", "white"},
	}
	for _, tt := range tests {
		got := vm.StatusColor(tt.status)
		if got != tt.want {
			t.Errorf("StatusColor(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

// ── Filtering ───────────────────────────────────────────────────────────

func TestSetFilterAndFilteredPairs(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	vm.SetFilter("arch")
	filtered := vm.FilteredPairs()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered pair, got %d", len(filtered))
	}
	if filtered[0].Name != "arch-review" {
		t.Errorf("filtered pair name = %q, want %q", filtered[0].Name, "arch-review")
	}
}

func TestFilterCaseInsensitive(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	vm.SetFilter("UX")
	filtered := vm.FilteredPairs()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered pair, got %d", len(filtered))
	}
	if filtered[0].Name != "ux-audit" {
		t.Errorf("filtered pair name = %q, want %q", filtered[0].Name, "ux-audit")
	}
}

func TestFilterByPhase(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	vm.SetFilter("design")
	filtered := vm.FilteredPairs()
	if len(filtered) != 2 {
		t.Fatalf("expected 2 pairs matching phase 'design', got %d", len(filtered))
	}
	names := map[string]bool{}
	for _, p := range filtered {
		names[p.Name] = true
	}
	if !names["arch-review"] || !names["ux-audit"] {
		t.Errorf("expected arch-review and ux-audit, got %v", names)
	}
}

func TestFilterByStatus(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	vm.SetFilter("escalated")
	filtered := vm.FilteredPairs()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 pair matching status 'escalated', got %d", len(filtered))
	}
	if filtered[0].Name != "api-contract" {
		t.Errorf("filtered pair name = %q, want %q", filtered[0].Name, "api-contract")
	}
}

func TestFilterEmptyReturnsAll(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	vm.SetFilter("")
	filtered := vm.FilteredPairs()
	if len(filtered) != len(samplePairs) {
		t.Fatalf("expected %d pairs with empty filter, got %d", len(samplePairs), len(filtered))
	}
}

// ── Selection ───────────────────────────────────────────────────────────

func TestSelectionInitialIndex(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)
	if idx := vm.SelectedIndex(); idx != 0 {
		t.Errorf("initial SelectedIndex = %d, want 0", idx)
	}
}

func TestSelectNextAndPrev(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	vm.SelectNext()
	if idx := vm.SelectedIndex(); idx != 1 {
		t.Errorf("after SelectNext SelectedIndex = %d, want 1", idx)
	}

	vm.SelectNext()
	if idx := vm.SelectedIndex(); idx != 2 {
		t.Errorf("after 2x SelectNext SelectedIndex = %d, want 2", idx)
	}

	vm.SelectPrev()
	if idx := vm.SelectedIndex(); idx != 1 {
		t.Errorf("after SelectPrev SelectedIndex = %d, want 1", idx)
	}
}

func TestSelectNextWraps(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	// Move to end
	for i := 0; i < len(samplePairs); i++ {
		vm.SelectNext()
	}
	// Should have wrapped to 0
	if idx := vm.SelectedIndex(); idx != 0 {
		t.Errorf("SelectNext at end should wrap to 0, got %d", idx)
	}
}

func TestSelectFirst(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	// Move to index 3.
	for i := 0; i < 3; i++ {
		vm.SelectNext()
	}
	if idx := vm.SelectedIndex(); idx != 3 {
		t.Fatalf("precondition: SelectedIndex = %d, want 3", idx)
	}

	vm.SelectFirst()
	if idx := vm.SelectedIndex(); idx != 0 {
		t.Errorf("after SelectFirst SelectedIndex = %d, want 0", idx)
	}
}

func TestSelectLast(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	vm.SelectLast()
	want := len(samplePairs) - 1
	if idx := vm.SelectedIndex(); idx != want {
		t.Errorf("after SelectLast SelectedIndex = %d, want %d", idx, want)
	}
}

func TestSelectFirstEmptyList(t *testing.T) {
	store := seedStore(nil)
	vm := views.NewPairsViewModel(store)
	vm.SelectFirst() // should not panic
	if idx := vm.SelectedIndex(); idx != 0 {
		t.Errorf("SelectFirst on empty list: SelectedIndex = %d, want 0", idx)
	}
}

func TestSelectLastEmptyList(t *testing.T) {
	store := seedStore(nil)
	vm := views.NewPairsViewModel(store)
	vm.SelectLast() // should not panic
	if idx := vm.SelectedIndex(); idx != 0 {
		t.Errorf("SelectLast on empty list: SelectedIndex = %d, want 0", idx)
	}
}

func TestSelectPrevWraps(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	vm.SelectPrev()
	want := len(samplePairs) - 1
	if idx := vm.SelectedIndex(); idx != want {
		t.Errorf("SelectPrev at 0 should wrap to %d, got %d", want, idx)
	}
}

// ── Selection clamp after filter ────────────────────────────────────────

func TestSelectionClampsAfterFilter(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	// Move selection to index 4 (last item)
	for i := 0; i < 4; i++ {
		vm.SelectNext()
	}
	if idx := vm.SelectedIndex(); idx != 4 {
		t.Fatalf("precondition: SelectedIndex = %d, want 4", idx)
	}

	// Now filter to only 1 result — selection must clamp
	vm.SetFilter("arch")
	filtered := vm.FilteredPairs()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered pair, got %d", len(filtered))
	}
	if idx := vm.SelectedIndex(); idx != 0 {
		t.Errorf("SelectedIndex should clamp to 0 after filter, got %d", idx)
	}
}

// ── Navigation within filtered list ─────────────────────────────────────

func TestNavigateWithinFilteredList(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	// Filter to "backend" component pairs: arch-review, code-quality
	vm.SetFilter("backend")
	filtered := vm.FilteredPairs()
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered pairs, got %d", len(filtered))
	}

	// Initial selection should be at index 0 of filtered list
	if idx := vm.SelectedIndex(); idx != 0 {
		t.Errorf("initial filtered SelectedIndex = %d, want 0", idx)
	}

	p := vm.SelectedPair()
	if p == nil || p.Name != "arch-review" {
		t.Errorf("initial SelectedPair = %v, want arch-review", p)
	}

	// Navigate to next within filtered list
	vm.SelectNext()
	p = vm.SelectedPair()
	if p == nil || p.Name != "code-quality" {
		t.Errorf("after SelectNext SelectedPair = %v, want code-quality", p)
	}

	// Wrap around within filtered list
	vm.SelectNext()
	p = vm.SelectedPair()
	if p == nil || p.Name != "arch-review" {
		t.Errorf("after wrap SelectNext SelectedPair = %v, want arch-review (wrap)", p)
	}

	// Navigate prev to wrap back
	vm.SelectPrev()
	p = vm.SelectedPair()
	if p == nil || p.Name != "code-quality" {
		t.Errorf("after SelectPrev SelectedPair = %v, want code-quality (wrap back)", p)
	}
}

// ── Selection clamp after Refresh ───────────────────────────────────────

func TestSelectionClampsAfterRefresh(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	// Move selection to index 4 (last item)
	for i := 0; i < 4; i++ {
		vm.SelectNext()
	}
	if idx := vm.SelectedIndex(); idx != 4 {
		t.Fatalf("precondition: SelectedIndex = %d, want 4", idx)
	}

	// Shrink the store to 1 pair
	store.SetPairs([]client.PairStatus{
		{Name: "only-pair", Component: "solo", Phase: "design", Status: "idle", Active: false},
	})
	vm.Refresh()

	if idx := vm.SelectedIndex(); idx != 0 {
		t.Errorf("SelectedIndex should clamp to 0 after Refresh, got %d", idx)
	}
	p := vm.SelectedPair()
	if p == nil || p.Name != "only-pair" {
		t.Errorf("SelectedPair after clamp = %v, want only-pair", p)
	}
}

// ── Filter clear round-trip ─────────────────────────────────────────────

func TestFilterClearRoundTrip(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	// Apply filter
	vm.SetFilter("arch")
	if len(vm.FilteredPairs()) != 1 {
		t.Fatalf("expected 1 filtered pair, got %d", len(vm.FilteredPairs()))
	}

	// Clear filter
	vm.SetFilter("")
	filtered := vm.FilteredPairs()
	if len(filtered) != len(samplePairs) {
		t.Errorf("after clearing filter expected %d pairs, got %d", len(samplePairs), len(filtered))
	}
}

// ── Selected pair ───────────────────────────────────────────────────────

func TestSelectedPair(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	p := vm.SelectedPair()
	if p == nil {
		t.Fatal("SelectedPair returned nil")
	}
	if p.Name != samplePairs[0].Name {
		t.Errorf("SelectedPair name = %q, want %q", p.Name, samplePairs[0].Name)
	}

	vm.SelectNext()
	vm.SelectNext()
	p = vm.SelectedPair()
	if p == nil {
		t.Fatal("SelectedPair returned nil after SelectNext")
	}
	if p.Name != samplePairs[2].Name {
		t.Errorf("SelectedPair name = %q, want %q", p.Name, samplePairs[2].Name)
	}
}

func TestSelectedPairEmptyList(t *testing.T) {
	store := seedStore(nil)
	vm := views.NewPairsViewModel(store)

	p := vm.SelectedPair()
	if p != nil {
		t.Errorf("SelectedPair with empty list should be nil, got %+v", p)
	}
}

// ── Group by component ──────────────────────────────────────────────────

func TestPairsByComponent(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	grouped := vm.PairsByComponent()
	if len(grouped) != 3 {
		t.Fatalf("expected 3 component groups, got %d", len(grouped))
	}
	if len(grouped["backend"]) != 2 {
		t.Errorf("backend group: expected 2, got %d", len(grouped["backend"]))
	}
	if len(grouped["frontend"]) != 2 {
		t.Errorf("frontend group: expected 2, got %d", len(grouped["frontend"]))
	}
	if len(grouped["infra"]) != 1 {
		t.Errorf("infra group: expected 1, got %d", len(grouped["infra"]))
	}
}

// ── Active count ────────────────────────────────────────────────────────

func TestActiveCount(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	count := vm.ActiveCount()
	if count != 3 {
		t.Errorf("ActiveCount = %d, want 3", count)
	}
}

func TestActiveCountEmpty(t *testing.T) {
	store := seedStore(nil)
	vm := views.NewPairsViewModel(store)

	count := vm.ActiveCount()
	if count != 0 {
		t.Errorf("ActiveCount with no pairs = %d, want 0", count)
	}
}

// ── Refresh from store ──────────────────────────────────────────────────

// ── Filter getter (M10) ─────────────────────────────────────────────────

func TestPairsFilterGetter(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	if vm.Filter() != "" {
		t.Errorf("initial Filter = %q, want empty", vm.Filter())
	}

	vm.SetFilter("backend")
	if vm.Filter() != "backend" {
		t.Errorf("Filter after set = %q, want backend", vm.Filter())
	}

	vm.SetFilter("")
	if vm.Filter() != "" {
		t.Errorf("Filter after clear = %q, want empty", vm.Filter())
	}
}

// ── Viewport scroll offset (M10) ────────────────────────────────────────

func TestPairsSetViewportHeight(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)
	vm.SetViewportHeight(3)

	if vm.ScrollOffset() != 0 {
		t.Errorf("initial ScrollOffset = %d, want 0", vm.ScrollOffset())
	}
}

func TestPairsScrollOffsetFollowsSelection(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)
	vm.SetViewportHeight(2)

	// items 0,1 fit in viewport
	vm.SelectNext() // selected=1
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset = %d, want 0", vm.ScrollOffset())
	}

	// item 2 pushes scroll
	vm.SelectNext() // selected=2
	if vm.ScrollOffset() != 1 {
		t.Errorf("ScrollOffset = %d, want 1", vm.ScrollOffset())
	}

	// item 3
	vm.SelectNext() // selected=3
	if vm.ScrollOffset() != 2 {
		t.Errorf("ScrollOffset = %d, want 2", vm.ScrollOffset())
	}

	// scroll back up
	vm.SelectPrev() // selected=2
	vm.SelectPrev() // selected=1
	if vm.ScrollOffset() != 1 {
		t.Errorf("ScrollOffset = %d, want 1 after scrolling back up", vm.ScrollOffset())
	}
}

func TestPairsScrollOffsetViewportLargerThanList(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)
	vm.SetViewportHeight(20) // 5 pairs, viewport of 20

	for i := 0; i < 5; i++ {
		vm.SelectNext()
		if vm.ScrollOffset() != 0 {
			t.Errorf("ScrollOffset = %d, want 0 (viewport larger than list)", vm.ScrollOffset())
		}
	}
}

func TestPairsScrollOffsetResetsOnFilter(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)
	vm.SetViewportHeight(2)

	// Scroll down
	vm.SelectNext()
	vm.SelectNext()
	vm.SelectNext()
	if vm.ScrollOffset() == 0 {
		t.Fatal("precondition: scroll offset should be > 0")
	}

	// Applying filter should adjust scroll offset to keep selection visible
	vm.SetFilter("arch")
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset after filter = %d, want 0 (selection clamped to 0)", vm.ScrollOffset())
	}
}

// ── Refresh from store ──────────────────────────────────────────────────

// ── HARDEN: Nil receiver safety ──────────────────────────────────────────

func TestPairsNilReceiver(t *testing.T) {
	var vm *views.PairsViewModel

	// None of these should panic.
	if vm.Pairs() != nil {
		t.Error("nil Pairs should return nil")
	}
	if vm.FilteredPairs() != nil {
		t.Error("nil FilteredPairs should return nil")
	}
	if vm.Filter() != "" {
		t.Error("nil Filter should return empty")
	}
	if vm.SelectedIndex() != 0 {
		t.Error("nil SelectedIndex should return 0")
	}
	if vm.SelectedPair() != nil {
		t.Error("nil SelectedPair should return nil")
	}
	if vm.PairsByComponent() != nil {
		t.Error("nil PairsByComponent should return nil")
	}
	if vm.ActiveCount() != 0 {
		t.Error("nil ActiveCount should return 0")
	}
	if vm.ScrollOffset() != 0 {
		t.Error("nil ScrollOffset should return 0")
	}
	vm.SelectNext()
	vm.SelectPrev()
	vm.SelectFirst()
	vm.SelectLast()
	vm.SetFilter("test")
	vm.SetViewportHeight(5)
	vm.Refresh()
}

// ── HARDEN: Viewport scroll resets on wrap-around ────────────────────────

func TestPairsScrollResetsOnWrapForward(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)
	vm.SetViewportHeight(2)

	// Navigate to last item (index 4)
	for i := 0; i < 4; i++ {
		vm.SelectNext()
	}
	if vm.SelectedIndex() != 4 {
		t.Fatalf("precondition: selected = %d, want 4", vm.SelectedIndex())
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

func TestPairsScrollResetsOnWrapBackward(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)
	vm.SetViewportHeight(2)

	// At index 0, wrap backward to last
	vm.SelectPrev()
	if vm.SelectedIndex() != 4 {
		t.Fatalf("selected = %d, want 4", vm.SelectedIndex())
	}
	if vm.ScrollOffset() != 3 {
		t.Errorf("ScrollOffset after wrap backward = %d, want 3", vm.ScrollOffset())
	}
}

// ── HARDEN: Negative viewport height ────────────────────────────────────

func TestPairsNegativeViewportHeight(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)
	vm.SetViewportHeight(-5)

	vm.SelectNext()
	vm.SelectNext()
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset with negative viewport = %d, want 0", vm.ScrollOffset())
	}
}

// ── Refresh from store ──────────────────────────────────────────────────

func TestRefreshPicksUpNewData(t *testing.T) {
	store := seedStore(samplePairs)
	vm := views.NewPairsViewModel(store)

	if len(vm.Pairs()) != 5 {
		t.Fatalf("precondition: expected 5 pairs, got %d", len(vm.Pairs()))
	}

	// Update the store externally
	newPairs := []client.PairStatus{
		{Name: "new-pair", Component: "new", Phase: "design", Status: "idle", Active: false},
	}
	store.SetPairs(newPairs)

	// Before refresh, viewmodel may still have old data
	vm.Refresh()

	pairs := vm.Pairs()
	if len(pairs) != 1 {
		t.Fatalf("after Refresh expected 1 pair, got %d", len(pairs))
	}
	if pairs[0].Name != "new-pair" {
		t.Errorf("pair name = %q, want %q", pairs[0].Name, "new-pair")
	}
}

// ── Workspace filtering ──────────────────────────────────────────────────

func TestPairsWorkspaceFilterIncludesMatchingPairs(t *testing.T) {
	store := state.NewStore()
	store.SetPairs([]client.PairStatus{
		{Name: "pair-a", Workspace: "ws-a"},
		{Name: "pair-b", Workspace: "ws-b"},
	})
	store.SetCurrentWorkspace("ws-a")
	vm := views.NewPairsViewModel(store)

	filtered := vm.FilteredPairs()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 pair for ws-a, got %d", len(filtered))
	}
	if filtered[0].Name != "pair-a" {
		t.Errorf("expected pair-a, got %q", filtered[0].Name)
	}
}

func TestPairsWorkspaceFilterExcludesOtherWorkspace(t *testing.T) {
	store := state.NewStore()
	store.SetPairs([]client.PairStatus{
		{Name: "pair-a", Workspace: "ws-a"},
		{Name: "pair-b", Workspace: "ws-b"},
		{Name: "pair-c", Workspace: "ws-b"},
	})
	store.SetCurrentWorkspace("ws-a")
	vm := views.NewPairsViewModel(store)

	filtered := vm.FilteredPairs()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 pair for ws-a, got %d", len(filtered))
	}
	if filtered[0].Name != "pair-a" {
		t.Errorf("expected pair-a, got %q", filtered[0].Name)
	}
}

func TestPairsWorkspaceFilterEmptyWorkspaceShowsAll(t *testing.T) {
	store := state.NewStore()
	store.SetPairs([]client.PairStatus{
		{Name: "pair-a", Workspace: "ws-a"},
		{Name: "pair-b", Workspace: "ws-b"},
	})
	// No workspace set — empty string means show all
	vm := views.NewPairsViewModel(store)

	filtered := vm.FilteredPairs()
	if len(filtered) != 2 {
		t.Fatalf("empty workspace should show all pairs, got %d", len(filtered))
	}
}

func TestPairsWorkspaceFilterCombinedWithTextFilter(t *testing.T) {
	store := state.NewStore()
	store.SetPairs([]client.PairStatus{
		{Name: "api-design", Workspace: "ws-a"},
		{Name: "api-contracts", Workspace: "ws-a"},
		{Name: "api-other", Workspace: "ws-b"},
	})
	store.SetCurrentWorkspace("ws-a")
	vm := views.NewPairsViewModel(store)
	vm.SetFilter("contracts")

	filtered := vm.FilteredPairs()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 pair matching ws-a+contracts, got %d", len(filtered))
	}
	if filtered[0].Name != "api-contracts" {
		t.Errorf("expected api-contracts, got %q", filtered[0].Name)
	}
}

func TestPairsWorkspaceFilterRefreshUpdatesFilter(t *testing.T) {
	store := state.NewStore()
	store.SetPairs([]client.PairStatus{
		{Name: "pair-a", Workspace: "ws-a"},
		{Name: "pair-b", Workspace: "ws-b"},
	})
	vm := views.NewPairsViewModel(store)

	// Initially no workspace filter — all visible
	if len(vm.FilteredPairs()) != 2 {
		t.Fatalf("precondition: expected 2 pairs, got %d", len(vm.FilteredPairs()))
	}

	// Set workspace, then refresh
	store.SetCurrentWorkspace("ws-b")
	vm.Refresh()

	filtered := vm.FilteredPairs()
	if len(filtered) != 1 {
		t.Fatalf("after workspace change+Refresh expected 1 pair, got %d", len(filtered))
	}
	if filtered[0].Name != "pair-b" {
		t.Errorf("expected pair-b, got %q", filtered[0].Name)
	}
}
