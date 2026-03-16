package views

import (
	"maps"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// MilestoneStatus holds display data for a single milestone.
type MilestoneStatus struct {
	ID          int
	Name        string
	Status      string
	PhaseStatus map[string]string
	DoneWhen    string
	Regressions int
}

// EpicViewModel is the view model for the epic status tab.
type EpicViewModel struct {
	store          *state.Store
	plan           client.Plan
	milestones     []MilestoneStatus
	selected       int
	viewportHeight int
	scrollOffset   int
}

// NewEpicViewModel creates an EpicViewModel backed by the given store.
func NewEpicViewModel(store *state.Store) *EpicViewModel {
	vm := &EpicViewModel{store: store}
	vm.loadPlan()
	return vm
}

// EpicName returns the epic's name.
func (vm *EpicViewModel) EpicName() string {
	if vm == nil {
		return ""
	}
	return vm.plan.Epic.Name
}

// EpicDescription returns the epic's description.
func (vm *EpicViewModel) EpicDescription() string {
	if vm == nil {
		return ""
	}
	return vm.plan.Epic.Description
}

// Milestones returns all milestones with their status.
func (vm *EpicViewModel) Milestones() []MilestoneStatus {
	if vm == nil {
		return nil
	}
	return vm.milestones
}

// CompletedCount returns the number of done milestones.
func (vm *EpicViewModel) CompletedCount() int {
	if vm == nil {
		return 0
	}
	count := 0
	for _, m := range vm.milestones {
		if m.Status == "done" {
			count++
		}
	}
	return count
}

// TotalCount returns the total number of milestones.
func (vm *EpicViewModel) TotalCount() int {
	if vm == nil {
		return 0
	}
	return len(vm.milestones)
}

// ProgressPercent returns the completion fraction (0.0-1.0).
func (vm *EpicViewModel) ProgressPercent() float64 {
	if vm == nil || len(vm.milestones) == 0 {
		return 0.0
	}
	return float64(vm.CompletedCount()) / float64(len(vm.milestones))
}

// CurrentFocus returns the current focus, or nil if not set.
func (vm *EpicViewModel) CurrentFocus() *client.CurrentFocus {
	if vm == nil {
		return nil
	}
	return vm.plan.Epic.CurrentFocus
}

// MilestoneStatusColor maps a milestone status to a color name.
func (vm *EpicViewModel) MilestoneStatusColor(status string) string {
	switch status {
	case "pending":
		return "dim"
	case "in_progress":
		return "cyan"
	case "done":
		return "green"
	default:
		return "white"
	}
}

// SelectedIndex returns the current selection index.
func (vm *EpicViewModel) SelectedIndex() int {
	if vm == nil {
		return 0
	}
	return vm.selected
}

// SelectNext moves selection forward with wrap-around.
func (vm *EpicViewModel) SelectNext() {
	if vm == nil {
		return
	}
	n := len(vm.milestones)
	if n == 0 {
		return
	}
	vm.selected = (vm.selected + 1) % n
	vm.adjustScrollOffset()
}

// SelectPrev moves selection backward with wrap-around.
func (vm *EpicViewModel) SelectPrev() {
	if vm == nil {
		return
	}
	n := len(vm.milestones)
	if n == 0 {
		return
	}
	vm.selected = (vm.selected - 1 + n) % n
	vm.adjustScrollOffset()
}

// SelectFirst jumps to the first milestone.
func (vm *EpicViewModel) SelectFirst() {
	if vm == nil {
		return
	}
	vm.selected = 0
	vm.adjustScrollOffset()
}

// SelectLast jumps to the last milestone.
func (vm *EpicViewModel) SelectLast() {
	if vm == nil {
		return
	}
	n := len(vm.milestones)
	if n == 0 {
		return
	}
	vm.selected = n - 1
	vm.adjustScrollOffset()
}

// SelectedMilestone returns the currently selected milestone, or nil if empty.
func (vm *EpicViewModel) SelectedMilestone() *MilestoneStatus {
	if vm == nil || len(vm.milestones) == 0 {
		return nil
	}
	ms := vm.milestones[vm.selected]
	return &ms
}

// SetViewportHeight sets the viewport height and recalculates scroll offset.
func (vm *EpicViewModel) SetViewportHeight(h int) {
	if vm == nil {
		return
	}
	vm.viewportHeight = h
	vm.adjustScrollOffset()
}

// ScrollOffset returns the current scroll offset.
func (vm *EpicViewModel) ScrollOffset() int {
	if vm == nil {
		return 0
	}
	return vm.scrollOffset
}

// Refresh re-reads the plan from the store.
func (vm *EpicViewModel) Refresh() {
	if vm == nil {
		return
	}
	vm.loadPlan()
	vm.clampSelection()
	vm.adjustScrollOffset()
}

func (vm *EpicViewModel) clampSelection() {
	n := len(vm.milestones)
	if n == 0 {
		vm.selected = 0
	} else if vm.selected >= n {
		vm.selected = n - 1
	}
}

func (vm *EpicViewModel) adjustScrollOffset() {
	n := len(vm.milestones)
	if vm.viewportHeight <= 0 || n <= vm.viewportHeight {
		vm.scrollOffset = 0
		return
	}
	if vm.selected < vm.scrollOffset {
		vm.scrollOffset = vm.selected
	}
	if vm.selected >= vm.scrollOffset+vm.viewportHeight {
		vm.scrollOffset = vm.selected - vm.viewportHeight + 1
	}
}

func (vm *EpicViewModel) loadPlan() {
	if vm.store == nil {
		vm.plan = client.Plan{}
		vm.milestones = nil
		return
	}
	vm.plan = vm.store.Plan()
	vm.milestones = nil
	for _, m := range vm.plan.Epic.Milestones {
		// Deep copy PhaseStatus to avoid sharing the map with the store.
		ps := make(map[string]string, len(m.PhaseStatus))
		maps.Copy(ps, m.PhaseStatus)
		vm.milestones = append(vm.milestones, MilestoneStatus{
			ID:          m.ID,
			Name:        m.Name,
			Status:      m.Status,
			PhaseStatus: ps,
			DoneWhen:    m.DoneWhen,
			Regressions: m.Regressions,
		})
	}
}
