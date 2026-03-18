package views_test

import (
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

func seedEpicStore() *state.Store {
	s := state.NewStore()
	s.SetPlan(client.Plan{
		Epic: client.EpicConfig{
			Name:        "ratchet-monitor",
			Description: "Real-time observability dashboard",
			Milestones: []client.Milestone{
				{ID: 1, Name: "Spike & Contract", Status: "done", PhaseStatus: map[string]string{"plan": "done", "test": "done", "build": "done", "review": "done", "harden": "done"}, DoneWhen: "working proof"},
				{ID: 2, Name: "Solid Backend", Status: "done", PhaseStatus: map[string]string{"plan": "done", "test": "done", "build": "done", "review": "done", "harden": "done"}, DoneWhen: "robust backend"},
				{ID: 3, Name: "Dashboard", Status: "in_progress", PhaseStatus: map[string]string{"plan": "done", "test": "done", "build": "in_progress", "review": "pending", "harden": "pending"}, DoneWhen: "live dashboard"},
				{ID: 4, Name: "Scores", Status: "pending", PhaseStatus: map[string]string{"plan": "pending", "test": "pending", "build": "pending", "review": "pending", "harden": "pending"}, DoneWhen: "score charts"},
			},
			CurrentFocus: &client.CurrentFocus{
				MilestoneID: 3,
				Phase:       "build",
				Started:     "2026-03-15T10:00:00Z",
			},
		},
	})
	return s
}

func seedEpicStoreWithIssues() *state.Store {
	s := state.NewStore()
	s.SetPlan(client.Plan{
		Epic: client.EpicConfig{
			Name:        "ratchet-monitor",
			Description: "Real-time observability dashboard",
			Milestones: []client.Milestone{
				{
					ID: 1, Name: "M1", Status: "in_progress",
					PhaseStatus:    map[string]string{"plan": "done", "build": "in_progress"},
					MaxRegressions: 3,
					Regressions:    1,
					Issues: []client.Issue{
						{
							Ref:         "#10",
							Title:       "Add widget",
							Pairs:       []string{"tui-layout"},
							DependsOn:   []string{},
							PhaseStatus: map[string]string{"plan": "done", "build": "in_progress", "review": "pending"},
							Status:      "in_progress",
							Files:       []string{"widget.go"},
							Debates:     []string{"debate-1"},
						},
						{
							Ref:         "#11",
							Title:       "Fix layout",
							Pairs:       []string{"tui-layout"},
							PhaseStatus: map[string]string{"plan": "done", "build": "done", "review": "done"},
							Status:      "done",
						},
					},
				},
				{
					ID: 2, Name: "M2", Status: "pending",
					PhaseStatus:    map[string]string{"plan": "pending"},
					DependsOn:      []int{1},
					MaxRegressions: 2,
					Regressions:    2,
					Issues: []client.Issue{
						{
							Ref:         "#20",
							Title:       "Backend API",
							PhaseStatus: map[string]string{"plan": "pending"},
							Status:      "pending",
						},
					},
				},
			},
		},
	})
	return s
}

// ── Construction ────────────────────────────────────────────────────────

func TestNewEpicViewModel(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	if vm == nil {
		t.Fatal("NewEpicViewModel returned nil")
	}
}

// ── Epic info ───────────────────────────────────────────────────────────

func TestEpicName(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	if vm.EpicName() != "ratchet-monitor" {
		t.Errorf("EpicName = %q, want ratchet-monitor", vm.EpicName())
	}
}

func TestEpicDescription(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	if vm.EpicDescription() != "Real-time observability dashboard" {
		t.Errorf("EpicDescription = %q", vm.EpicDescription())
	}
}

// ── Milestones ──────────────────────────────────────────────────────────

func TestMilestones(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	if len(ms) != 4 {
		t.Fatalf("expected 4 milestones, got %d", len(ms))
	}
	if ms[0].Name != "Spike & Contract" {
		t.Errorf("ms[0].Name = %q", ms[0].Name)
	}
	if ms[0].Status != "done" {
		t.Errorf("ms[0].Status = %q, want done", ms[0].Status)
	}
}

func TestMilestonePhaseStatus(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()

	// Milestone 3 (Dashboard) is in_progress
	if ms[2].PhaseStatus["build"] != "in_progress" {
		t.Errorf("ms[2] build phase = %q, want in_progress", ms[2].PhaseStatus["build"])
	}
	if ms[2].PhaseStatus["review"] != "pending" {
		t.Errorf("ms[2] review phase = %q, want pending", ms[2].PhaseStatus["review"])
	}
}

// ── Progress ────────────────────────────────────────────────────────────

func TestCompletedCount(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	if vm.CompletedCount() != 2 {
		t.Errorf("CompletedCount = %d, want 2", vm.CompletedCount())
	}
}

func TestTotalCount(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	if vm.TotalCount() != 4 {
		t.Errorf("TotalCount = %d, want 4", vm.TotalCount())
	}
}

func TestProgressPercent(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	want := 0.5 // 2/4
	if diff := vm.ProgressPercent() - want; diff > 0.01 || diff < -0.01 {
		t.Errorf("ProgressPercent = %f, want %f", vm.ProgressPercent(), want)
	}
}

func TestProgressPercentEmpty(t *testing.T) {
	store := state.NewStore()
	vm := views.NewEpicViewModel(store)
	if vm.ProgressPercent() != 0.0 {
		t.Errorf("ProgressPercent with no milestones = %f, want 0", vm.ProgressPercent())
	}
}

// ── Current focus ───────────────────────────────────────────────────────

func TestCurrentFocus(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	focus := vm.CurrentFocus()
	if focus == nil {
		t.Fatal("CurrentFocus returned nil")
	}
	if focus.MilestoneID != 3 {
		t.Errorf("focus MilestoneID = %d, want 3", focus.MilestoneID)
	}
	if focus.Phase != "build" {
		t.Errorf("focus Phase = %q, want build", focus.Phase)
	}
}

func TestCurrentFocusNil(t *testing.T) {
	store := state.NewStore()
	store.SetPlan(client.Plan{Epic: client.EpicConfig{Name: "test"}})
	vm := views.NewEpicViewModel(store)
	if vm.CurrentFocus() != nil {
		t.Error("CurrentFocus should be nil when not set")
	}
}

// ── Milestone status color ──────────────────────────────────────────────

func TestMilestoneStatusColor(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	tests := []struct {
		status string
		want   string
	}{
		{"pending", "dim"},
		{"in_progress", "cyan"},
		{"done", "green"},
		{"unknown", "white"},
	}
	for _, tt := range tests {
		got := vm.MilestoneStatusColor(tt.status)
		if got != tt.want {
			t.Errorf("MilestoneStatusColor(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

// ── Refresh ─────────────────────────────────────────────────────────────

func TestEpicRefresh(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)

	if vm.CompletedCount() != 2 {
		t.Fatalf("precondition: completed = %d", vm.CompletedCount())
	}

	// Update plan — mark milestone 3 as done
	plan := store.Plan()
	plan.Epic.Milestones[2].Status = "done"
	store.SetPlan(plan)
	vm.Refresh()

	if vm.CompletedCount() != 3 {
		t.Errorf("after Refresh CompletedCount = %d, want 3", vm.CompletedCount())
	}
}

// ── Nil receiver safety ─────────────────────────────────────────────────

func TestEpicNilReceiver(t *testing.T) {
	var vm *views.EpicViewModel

	// None of these should panic.
	if vm.EpicName() != "" {
		t.Error("nil receiver EpicName should return empty string")
	}
	if vm.EpicDescription() != "" {
		t.Error("nil receiver EpicDescription should return empty string")
	}
	if vm.Milestones() != nil {
		t.Error("nil receiver Milestones should return nil")
	}
	if vm.CompletedCount() != 0 {
		t.Error("nil receiver CompletedCount should return 0")
	}
	if vm.TotalCount() != 0 {
		t.Error("nil receiver TotalCount should return 0")
	}
	if vm.ProgressPercent() != 0.0 {
		t.Error("nil receiver ProgressPercent should return 0.0")
	}
	if vm.CurrentFocus() != nil {
		t.Error("nil receiver CurrentFocus should return nil")
	}
	vm.Refresh()
}

// ── Empty plan ──────────────────────────────────────────────────────────

func TestEpicEmptyPlan(t *testing.T) {
	store := state.NewStore()
	vm := views.NewEpicViewModel(store)

	if vm.EpicName() != "" {
		t.Errorf("empty plan EpicName = %q, want empty", vm.EpicName())
	}
	if vm.TotalCount() != 0 {
		t.Errorf("empty plan TotalCount = %d, want 0", vm.TotalCount())
	}
	if vm.ProgressPercent() != 0.0 {
		t.Errorf("empty plan ProgressPercent = %f, want 0.0", vm.ProgressPercent())
	}
}

// ── All milestones done ─────────────────────────────────────────────────

func TestEpicAllDone(t *testing.T) {
	store := state.NewStore()
	store.SetPlan(client.Plan{
		Epic: client.EpicConfig{
			Name: "test",
			Milestones: []client.Milestone{
				{ID: 1, Name: "M1", Status: "done"},
				{ID: 2, Name: "M2", Status: "done"},
			},
		},
	})
	vm := views.NewEpicViewModel(store)
	if vm.ProgressPercent() != 1.0 {
		t.Errorf("all-done ProgressPercent = %f, want 1.0", vm.ProgressPercent())
	}
	if vm.CompletedCount() != 2 {
		t.Errorf("all-done CompletedCount = %d, want 2", vm.CompletedCount())
	}
}

// ── PhaseStatus map isolation ───────────────────────────────────────────

func TestEpicPhaseStatusMapIsolation(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)

	ms := vm.Milestones()
	if len(ms) == 0 {
		t.Fatal("no milestones")
	}

	// Mutate the viewmodel's PhaseStatus map
	ms[0].PhaseStatus["extra"] = "injected"

	// The store's data should not be affected
	plan := store.Plan()
	if _, exists := plan.Epic.Milestones[0].PhaseStatus["extra"]; exists {
		t.Error("mutating viewmodel PhaseStatus should not affect store data")
	}
}

// ── Nil PhaseStatus in milestone ────────────────────────────────────────

func TestEpicNilPhaseStatus(t *testing.T) {
	store := state.NewStore()
	store.SetPlan(client.Plan{
		Epic: client.EpicConfig{
			Name: "test",
			Milestones: []client.Milestone{
				{ID: 1, Name: "M1", Status: "pending", PhaseStatus: nil},
			},
		},
	})
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	if len(ms) != 1 {
		t.Fatalf("expected 1 milestone, got %d", len(ms))
	}
	// PhaseStatus should be an empty map, not nil
	if ms[0].PhaseStatus == nil {
		t.Error("PhaseStatus should be an empty map, not nil")
	}
}

// ── MilestoneStatusColor is a pure function ─────────────────────────────

func TestMilestoneStatusColorEmptyString(t *testing.T) {
	store := state.NewStore()
	vm := views.NewEpicViewModel(store)
	if got := vm.MilestoneStatusColor(""); got != "white" {
		t.Errorf("MilestoneStatusColor(\"\") = %q, want white", got)
	}
}

// ── Selection (M10) ─────────────────────────────────────────────────────

func TestEpicSelectionInitialIndex(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	if vm.SelectedIndex() != 0 {
		t.Errorf("initial SelectedIndex = %d, want 0", vm.SelectedIndex())
	}
}

func TestEpicSelectNext(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)

	vm.SelectNext()
	if vm.SelectedIndex() != 1 {
		t.Errorf("after SelectNext SelectedIndex = %d, want 1", vm.SelectedIndex())
	}

	vm.SelectNext()
	if vm.SelectedIndex() != 2 {
		t.Errorf("after 2x SelectNext SelectedIndex = %d, want 2", vm.SelectedIndex())
	}
}

func TestEpicSelectPrev(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)

	vm.SelectNext()
	vm.SelectNext()
	vm.SelectPrev()
	if vm.SelectedIndex() != 1 {
		t.Errorf("after SelectPrev SelectedIndex = %d, want 1", vm.SelectedIndex())
	}
}

func TestEpicSelectNextWrapsAround(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)

	// 4 milestones: move to end then one more
	for i := 0; i < 4; i++ {
		vm.SelectNext()
	}
	if vm.SelectedIndex() != 0 {
		t.Errorf("SelectNext at end should wrap to 0, got %d", vm.SelectedIndex())
	}
}

func TestEpicSelectPrevWrapsAround(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)

	vm.SelectPrev()
	want := 3 // last milestone index
	if vm.SelectedIndex() != want {
		t.Errorf("SelectPrev at 0 should wrap to %d, got %d", want, vm.SelectedIndex())
	}
}

func TestEpicSelectionOnEmptyList(t *testing.T) {
	store := state.NewStore()
	vm := views.NewEpicViewModel(store)

	vm.SelectNext()
	if vm.SelectedIndex() != 0 {
		t.Errorf("SelectNext on empty should stay at 0, got %d", vm.SelectedIndex())
	}
	vm.SelectPrev()
	if vm.SelectedIndex() != 0 {
		t.Errorf("SelectPrev on empty should stay at 0, got %d", vm.SelectedIndex())
	}
}

func TestEpicSelectedMilestone(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)

	ms := vm.SelectedMilestone()
	if ms == nil {
		t.Fatal("SelectedMilestone returned nil")
	}
	if ms.Name != "Spike & Contract" {
		t.Errorf("SelectedMilestone Name = %q, want Spike & Contract", ms.Name)
	}

	vm.SelectNext()
	vm.SelectNext()
	ms = vm.SelectedMilestone()
	if ms == nil {
		t.Fatal("SelectedMilestone after SelectNext returned nil")
	}
	if ms.Name != "Dashboard" {
		t.Errorf("SelectedMilestone Name = %q, want Dashboard", ms.Name)
	}
}

func TestEpicSelectedMilestoneEmpty(t *testing.T) {
	store := state.NewStore()
	vm := views.NewEpicViewModel(store)
	if vm.SelectedMilestone() != nil {
		t.Error("SelectedMilestone with no milestones should return nil")
	}
}

func TestEpicSelectionClampAfterRefresh(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)

	// Select last milestone (index 3)
	for i := 0; i < 3; i++ {
		vm.SelectNext()
	}
	if vm.SelectedIndex() != 3 {
		t.Fatalf("precondition: SelectedIndex = %d, want 3", vm.SelectedIndex())
	}

	// Shrink milestones to 2
	store.SetPlan(client.Plan{
		Epic: client.EpicConfig{
			Name: "test",
			Milestones: []client.Milestone{
				{ID: 1, Name: "M1", Status: "done"},
				{ID: 2, Name: "M2", Status: "pending"},
			},
		},
	})
	vm.Refresh()

	if vm.SelectedIndex() >= 2 {
		t.Errorf("SelectedIndex should be clamped after Refresh, got %d", vm.SelectedIndex())
	}
}

// ── Viewport scroll offset (M10) ────────────────────────────────────────

func TestEpicSetViewportHeight(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	vm.SetViewportHeight(2)

	// ScrollOffset starts at 0
	if vm.ScrollOffset() != 0 {
		t.Errorf("initial ScrollOffset = %d, want 0", vm.ScrollOffset())
	}
}

func TestEpicScrollOffsetFollowsSelection(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	vm.SetViewportHeight(2) // viewport shows 2 items at a time

	// Select items 0,1 — offset stays at 0
	vm.SelectNext() // selected=1
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset = %d, want 0 (item 1 visible in viewport 0..1)", vm.ScrollOffset())
	}

	// Select item 2 — offset should advance
	vm.SelectNext() // selected=2
	if vm.ScrollOffset() != 1 {
		t.Errorf("ScrollOffset = %d, want 1 (item 2 needs offset 1 with viewport 2)", vm.ScrollOffset())
	}

	// Select item 3 — offset should advance more
	vm.SelectNext() // selected=3
	if vm.ScrollOffset() != 2 {
		t.Errorf("ScrollOffset = %d, want 2", vm.ScrollOffset())
	}

	// Go back to item 2 — offset stays
	vm.SelectPrev() // selected=2
	if vm.ScrollOffset() != 2 {
		t.Errorf("ScrollOffset = %d, want 2 (item 2 still in viewport 2..3)", vm.ScrollOffset())
	}

	// Go back to item 1 — offset should decrease
	vm.SelectPrev() // selected=1
	if vm.ScrollOffset() != 1 {
		t.Errorf("ScrollOffset = %d, want 1 (item 1 needs offset adjustment)", vm.ScrollOffset())
	}
}

func TestEpicSetViewportHeightZero(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	vm.SetViewportHeight(0) // should not panic

	vm.SelectNext()
	// With viewport 0, ScrollOffset behavior is implementation-defined
	// but must not panic
	_ = vm.ScrollOffset()
}

func TestEpicNilReceiverSelection(t *testing.T) {
	var vm *views.EpicViewModel
	// New M10 methods should be nil-safe
	vm.SelectNext()
	vm.SelectPrev()
	vm.SetViewportHeight(5)
	if vm.SelectedIndex() != 0 {
		t.Errorf("nil SelectedIndex = %d, want 0", vm.SelectedIndex())
	}
	if vm.SelectedMilestone() != nil {
		t.Error("nil SelectedMilestone should return nil")
	}
	if vm.ScrollOffset() != 0 {
		t.Errorf("nil ScrollOffset = %d, want 0", vm.ScrollOffset())
	}
}

func TestEpicScrollOffsetAdjustsOnViewportResize(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	vm.SetViewportHeight(2)

	// Scroll to item 3
	vm.SelectNext()
	vm.SelectNext()
	vm.SelectNext() // selected=3, offset=2

	// Enlarge viewport to fit everything
	vm.SetViewportHeight(10)
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset = %d, want 0 (viewport now fits all items)", vm.ScrollOffset())
	}
}

func TestEpicScrollOffsetViewportLargerThanList(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	vm.SetViewportHeight(10) // viewport larger than 4 milestones

	// Navigate through all items — offset should always be 0
	for i := 0; i < 4; i++ {
		vm.SelectNext()
		if vm.ScrollOffset() != 0 {
			t.Errorf("ScrollOffset = %d after SelectNext(%d), want 0 (viewport >= list)", vm.ScrollOffset(), i)
		}
	}
}

// ── HARDEN: Viewport scroll resets on wrap-around ────────────────────────

func TestEpicScrollResetsOnWrapForward(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	vm.SetViewportHeight(2) // 4 milestones, viewport of 2

	// Navigate to last item (index 3)
	for i := 0; i < 3; i++ {
		vm.SelectNext()
	}
	if vm.SelectedIndex() != 3 {
		t.Fatalf("precondition: selected = %d, want 3", vm.SelectedIndex())
	}
	if vm.ScrollOffset() != 2 {
		t.Fatalf("precondition: scroll = %d, want 2", vm.ScrollOffset())
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

func TestEpicScrollResetsOnWrapBackward(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	vm.SetViewportHeight(2)

	// At index 0, wrap backward to last
	vm.SelectPrev()
	if vm.SelectedIndex() != 3 {
		t.Fatalf("selected = %d, want 3", vm.SelectedIndex())
	}
	// Scroll should follow to make index 3 visible
	if vm.ScrollOffset() != 2 {
		t.Errorf("ScrollOffset after wrap backward = %d, want 2", vm.ScrollOffset())
	}
}

// ── HARDEN: Negative viewport height ────────────────────────────────────

func TestEpicNegativeViewportHeight(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	vm.SetViewportHeight(-1) // should not panic

	vm.SelectNext()
	vm.SelectNext()
	// Must not panic, scroll offset should be 0
	if vm.ScrollOffset() != 0 {
		t.Errorf("ScrollOffset with negative viewport = %d, want 0", vm.ScrollOffset())
	}
}

// ── HARDEN: Rapid wrap cycling ──────────────────────────────────────────

func TestEpicRapidWrapCycling(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	vm.SetViewportHeight(2)

	// Cycle forward 100 times — should never panic or go out of range
	for i := 0; i < 100; i++ {
		vm.SelectNext()
		idx := vm.SelectedIndex()
		if idx < 0 || idx >= vm.TotalCount() {
			t.Fatalf("SelectedIndex %d out of range after %d SelectNext", idx, i+1)
		}
		off := vm.ScrollOffset()
		if off < 0 {
			t.Fatalf("ScrollOffset %d negative after %d SelectNext", off, i+1)
		}
	}

	// Cycle backward 100 times
	for i := 0; i < 100; i++ {
		vm.SelectPrev()
		idx := vm.SelectedIndex()
		if idx < 0 || idx >= vm.TotalCount() {
			t.Fatalf("SelectedIndex %d out of range after %d SelectPrev", idx, i+1)
		}
	}
}

// ── Issue-level data (Issue 32) ──────────────────────────────────────────

func TestEpicMilestoneIssuesPopulated(t *testing.T) {
	store := seedEpicStoreWithIssues()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()

	if len(ms) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(ms))
	}

	if len(ms[0].Issues) != 2 {
		t.Fatalf("expected 2 issues in M1, got %d", len(ms[0].Issues))
	}
	if ms[0].Issues[0].Ref != "#10" {
		t.Errorf("issue 0 Ref = %q, want #10", ms[0].Issues[0].Ref)
	}
	if ms[0].Issues[0].Title != "Add widget" {
		t.Errorf("issue 0 Title = %q, want Add widget", ms[0].Issues[0].Title)
	}
	if ms[0].Issues[0].Status != "in_progress" {
		t.Errorf("issue 0 Status = %q, want in_progress", ms[0].Issues[0].Status)
	}
	if ms[0].Issues[1].Ref != "#11" {
		t.Errorf("issue 1 Ref = %q, want #11", ms[0].Issues[1].Ref)
	}
}

func TestEpicIssuePhaseStatus(t *testing.T) {
	store := seedEpicStoreWithIssues()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()

	iss := ms[0].Issues[0]
	if iss.PhaseStatus["plan"] != "done" {
		t.Errorf("issue plan = %q, want done", iss.PhaseStatus["plan"])
	}
	if iss.PhaseStatus["build"] != "in_progress" {
		t.Errorf("issue build = %q, want in_progress", iss.PhaseStatus["build"])
	}
	if iss.PhaseStatus["review"] != "pending" {
		t.Errorf("issue review = %q, want pending", iss.PhaseStatus["review"])
	}
}

func TestEpicIssueMapIsolation(t *testing.T) {
	store := seedEpicStoreWithIssues()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()

	ms[0].Issues[0].PhaseStatus["extra"] = "injected"

	plan := store.Plan()
	if _, exists := plan.Epic.Milestones[0].Issues[0].PhaseStatus["extra"]; exists {
		t.Error("mutating issue PhaseStatus should not affect store data")
	}
}

func TestEpicMilestoneNoIssues(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	for _, m := range ms {
		if m.Issues != nil {
			t.Errorf("milestone %q should have nil issues, got %d", m.Name, len(m.Issues))
		}
	}
}

// ── Regression budget text ───────────────────────────────────────────────

func TestRegressionBudgetText(t *testing.T) {
	store := seedEpicStoreWithIssues()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()

	// M1: regressions=1, max=3 -> "1/3"
	if got := vm.RegressionBudgetText(ms[0]); got != "1/3" {
		t.Errorf("M1 RegressionBudgetText = %q, want 1/3", got)
	}
	// M2: regressions=2, max=2 -> "2/2"
	if got := vm.RegressionBudgetText(ms[1]); got != "2/2" {
		t.Errorf("M2 RegressionBudgetText = %q, want 2/2", got)
	}
}

func TestRegressionBudgetTextDefaultMax(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	// MaxRegressions=0 -> defaults to 2; regressions=0 -> "0/2"
	if got := vm.RegressionBudgetText(ms[0]); got != "0/2" {
		t.Errorf("default max RegressionBudgetText = %q, want 0/2", got)
	}
}

func TestRegressionBudgetTextNilReceiver(t *testing.T) {
	var vm *views.EpicViewModel
	// nil receiver uses default max=2
	if got := vm.RegressionBudgetText(views.MilestoneStatus{}); got != "0/2" {
		t.Errorf("nil receiver RegressionBudgetText = %q, want 0/2", got)
	}
}

// ── Regression budget warning (Issue 32) ─────────────────────────────────

func TestRegressionWarningLevelNone(t *testing.T) {
	store := seedEpicStoreWithIssues()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	// M1: regressions=1, max=3 -> none
	level := vm.RegressionWarningLevel(ms[0])
	if level != "none" {
		t.Errorf("M1 warning level = %q, want none", level)
	}
}

func TestRegressionWarningLevelWarn(t *testing.T) {
	store := seedEpicStoreWithIssues()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	// M1: regressions=1, max=3 -> none; change to 2 to get warn
	m := ms[0]
	m.Regressions = 2 // at max-1
	level := vm.RegressionWarningLevel(m)
	if level != "warn" {
		t.Errorf("warning level = %q, want warn", level)
	}
}

func TestRegressionWarningLevelDanger(t *testing.T) {
	store := seedEpicStoreWithIssues()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	// M2: regressions=2, max=2 -> danger
	level := vm.RegressionWarningLevel(ms[1])
	if level != "danger" {
		t.Errorf("M2 warning level = %q, want danger", level)
	}
}

func TestRegressionWarningLevelDefaultMax(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	// MaxRegressions=0 -> defaults to 2
	level := vm.RegressionWarningLevel(ms[0])
	if level != "none" {
		t.Errorf("no regressions should be 'none', got %q", level)
	}
}

func TestRegressionWarningNilReceiver(t *testing.T) {
	var vm *views.EpicViewModel
	level := vm.RegressionWarningLevel(views.MilestoneStatus{})
	if level != "none" {
		t.Errorf("nil receiver should return 'none', got %q", level)
	}
}

// ── DAG connectors (Issue 32) ────────────────────────────────────────────

func TestDAGConnectors(t *testing.T) {
	store := seedEpicStoreWithIssues()
	vm := views.NewEpicViewModel(store)
	connectors := vm.DAGConnectors()
	// M2 depends on M1, so there should be 1 connector
	if len(connectors) != 1 {
		t.Fatalf("expected 1 DAG connector, got %d", len(connectors))
	}
	if connectors[0].FromID != 1 || connectors[0].ToID != 2 {
		t.Errorf("connector: from=%d to=%d, want from=1 to=2", connectors[0].FromID, connectors[0].ToID)
	}
}

func TestDAGConnectorsEmpty(t *testing.T) {
	store := seedEpicStore()
	vm := views.NewEpicViewModel(store)
	connectors := vm.DAGConnectors()
	// No dependencies in seedEpicStore
	if len(connectors) != 0 {
		t.Errorf("expected 0 DAG connectors with no deps, got %d", len(connectors))
	}
}

func TestDAGConnectorsNilReceiver(t *testing.T) {
	var vm *views.EpicViewModel
	if vm.DAGConnectors() != nil {
		t.Error("nil receiver DAGConnectors should return nil")
	}
}

func TestDAGPrefix(t *testing.T) {
	store := seedEpicStoreWithIssues()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()

	// M1 has no deps -> "  "
	prefix0 := vm.DAGPrefix(ms[0])
	if prefix0 != "  " {
		t.Errorf("DAGPrefix for root = %q, want \"  \"", prefix0)
	}

	// M2 depends on M1 -> "└─"
	prefix1 := vm.DAGPrefix(ms[1])
	if prefix1 != "└─" {
		t.Errorf("DAGPrefix for dependent = %q, want \"└─\"", prefix1)
	}
}

func TestDAGPrefixNilReceiver(t *testing.T) {
	var vm *views.EpicViewModel
	if vm.DAGPrefix(views.MilestoneStatus{}) != "" {
		t.Error("nil receiver DAGPrefix should return empty string")
	}
}

// ── IsBlocked ─────────────────────────────────────────────────────────────

func TestIsBlockedNoDeps(t *testing.T) {
	store := seedEpicStoreWithIssues()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	// M1 has no deps — never blocked
	if vm.IsBlocked(ms[0]) {
		t.Error("M1 has no deps, should not be blocked")
	}
}

func TestIsBlockedTrue(t *testing.T) {
	store := seedEpicStoreWithIssues()
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	// M2 depends on M1 which is "in_progress" (not done) → blocked
	if !vm.IsBlocked(ms[1]) {
		t.Error("M2 should be blocked: dep M1 is in_progress")
	}
}

func TestIsBlockedFalseWhenDepDone(t *testing.T) {
	store := state.NewStore()
	store.SetPlan(client.Plan{
		Epic: client.EpicConfig{
			Name: "test",
			Milestones: []client.Milestone{
				{ID: 1, Name: "M1", Status: "done"},
				{ID: 2, Name: "M2", Status: "pending", DependsOn: []int{1}},
			},
		},
	})
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	// M2 depends on M1 which is "done" → not blocked
	if vm.IsBlocked(ms[1]) {
		t.Error("M2 should not be blocked: dep M1 is done")
	}
}

func TestIsBlockedNilReceiver(t *testing.T) {
	var vm *views.EpicViewModel
	if vm.IsBlocked(views.MilestoneStatus{DependsOn: []int{1}}) {
		t.Error("nil receiver IsBlocked should return false")
	}
}

func TestIsBlockedDoneMilestoneNotBlocked(t *testing.T) {
	store := state.NewStore()
	store.SetPlan(client.Plan{
		Epic: client.EpicConfig{
			Name: "test",
			Milestones: []client.Milestone{
				{ID: 1, Name: "M1", Status: "in_progress"},
				{ID: 2, Name: "M2", Status: "done", DependsOn: []int{1}},
			},
		},
	})
	vm := views.NewEpicViewModel(store)
	ms := vm.Milestones()
	// M2 is "done" — should never show as blocked even though dep M1 is not done
	if vm.IsBlocked(ms[1]) {
		t.Error("done milestone should not be blocked even if dep is not done")
	}
}
