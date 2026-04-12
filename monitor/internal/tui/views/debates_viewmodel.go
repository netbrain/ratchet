package views

import (
	"sort"
	"strings"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// DebatesViewModel is the view model for the debate list screen.
type DebatesViewModel struct {
	ListViewModel
	store        *state.Store
	debates      []client.DebateMeta
	filtered     []client.DebateMeta
	filter       string
	statusFilter string
}

// NewDebatesViewModel creates a DebatesViewModel backed by the given store.
func NewDebatesViewModel(store *state.Store) *DebatesViewModel {
	vm := &DebatesViewModel{store: store}
	vm.loadDebates()
	return vm
}

// Debates returns all debates (unfiltered), sorted newest-first.
func (vm *DebatesViewModel) Debates() []client.DebateMeta {
	if vm == nil {
		return nil
	}
	return vm.debates
}

// DebateStatusColor maps a debate status to a color name.
func (vm *DebatesViewModel) DebateStatusColor(status string) string {
	switch status {
	case "initiated":
		return "yellow"
	case "in_progress":
		return "cyan"
	case "consensus":
		return "green"
	case "escalated":
		return "red"
	case "resolved":
		return "dim"
	default:
		return "white"
	}
}

// SetFilter applies a case-insensitive text filter across debate ID, pair, phase, and status.
func (vm *DebatesViewModel) SetFilter(query string) {
	if vm == nil {
		return
	}
	vm.filter = query
	vm.applyFilters()
	vm.clampSelection()
}

// StatusFilter returns the current status filter value.
func (vm *DebatesViewModel) StatusFilter() string {
	if vm == nil {
		return ""
	}
	return vm.statusFilter
}

// Filter returns the current text filter value.
func (vm *DebatesViewModel) Filter() string {
	if vm == nil {
		return ""
	}
	return vm.filter
}

// debateStatuses defines the cycle order for status filtering.
var debateStatuses = []string{"", "initiated", "in_progress", "consensus", "escalated", "resolved"}

// CycleStatusFilter advances the status filter to the next value in the cycle.
// The cycle is: all -> initiated -> in_progress -> consensus -> escalated -> resolved -> all.
func (vm *DebatesViewModel) CycleStatusFilter() {
	if vm == nil {
		return
	}
	for i, s := range debateStatuses {
		if s == vm.statusFilter {
			vm.SetStatusFilter(debateStatuses[(i+1)%len(debateStatuses)])
			return
		}
	}
	vm.SetStatusFilter("")
}

// SetStatusFilter filters debates by exact status match. Empty string means all.
func (vm *DebatesViewModel) SetStatusFilter(status string) {
	if vm == nil {
		return
	}
	vm.statusFilter = status
	vm.applyFilters()
	vm.clampSelection()
}

// FilteredDebates returns debates matching current filters, sorted newest-first.
func (vm *DebatesViewModel) FilteredDebates() []client.DebateMeta {
	if vm == nil {
		return nil
	}
	return vm.filtered
}

// SelectedIndex returns the current selection index within the filtered list.
func (vm *DebatesViewModel) SelectedIndex() int {
	if vm == nil {
		return 0
	}
	return vm.ListViewModel.Selected()
}

// SelectNext moves selection forward with wrap-around.
func (vm *DebatesViewModel) SelectNext() {
	if vm == nil {
		return
	}
	vm.ListViewModel.SelectNext(len(vm.filtered))
}

// SelectPrev moves selection backward with wrap-around.
func (vm *DebatesViewModel) SelectPrev() {
	if vm == nil {
		return
	}
	vm.ListViewModel.SelectPrevious(len(vm.filtered))
}

// SelectFirst jumps to the first item.
func (vm *DebatesViewModel) SelectFirst() {
	if vm == nil {
		return
	}
	vm.ListViewModel.SelectFirst(len(vm.filtered))
}

// SelectLast jumps to the last item.
func (vm *DebatesViewModel) SelectLast() {
	if vm == nil {
		return
	}
	vm.ListViewModel.SelectLast(len(vm.filtered))
}

// SelectedDebate returns the currently selected debate, or nil if empty.
func (vm *DebatesViewModel) SelectedDebate() *client.DebateMeta {
	if vm == nil || len(vm.filtered) == 0 {
		return nil
	}
	d := vm.filtered[vm.ListViewModel.Selected()]
	return &d
}

// Refresh re-reads debates from the store and reapplies filters.
func (vm *DebatesViewModel) Refresh() {
	if vm == nil {
		return
	}
	vm.loadDebates()
	vm.clampSelection()
}

func (vm *DebatesViewModel) loadDebates() {
	if vm.store == nil {
		vm.debates = nil
		vm.applyFilters()
		return
	}
	vm.debates = vm.store.Debates()
	sort.Slice(vm.debates, func(i, j int) bool {
		return vm.debates[i].Started.After(vm.debates[j].Started)
	})
	vm.applyFilters()
}

func (vm *DebatesViewModel) applyFilters() {
	var ws string
	if vm.store != nil {
		ws = vm.store.CurrentWorkspace()
	}
	vm.filtered = nil
	for _, d := range vm.debates {
		if ws != "" && d.Workspace != ws {
			continue
		}
		if !vm.matchesTextFilter(d) {
			continue
		}
		if vm.statusFilter != "" && d.Status != vm.statusFilter {
			continue
		}
		vm.filtered = append(vm.filtered, d)
	}
	if vm.filtered == nil {
		vm.filtered = []client.DebateMeta{}
	}
}

func (vm *DebatesViewModel) matchesTextFilter(d client.DebateMeta) bool {
	if vm.filter == "" {
		return true
	}
	q := strings.ToLower(vm.filter)
	return strings.Contains(strings.ToLower(d.ID), q) ||
		strings.Contains(strings.ToLower(d.Pair), q) ||
		strings.Contains(strings.ToLower(d.Phase), q) ||
		strings.Contains(strings.ToLower(d.Status), q)
}

// SetViewportHeight sets the viewport height and recalculates scroll offset.
func (vm *DebatesViewModel) SetViewportHeight(h int) {
	if vm == nil {
		return
	}
	vm.ListViewModel.SetViewportHeight(h, len(vm.filtered))
}

// ScrollOffset returns the current scroll offset.
func (vm *DebatesViewModel) ScrollOffset() int {
	if vm == nil {
		return 0
	}
	return vm.ListViewModel.ScrollOffset()
}

func (vm *DebatesViewModel) clampSelection() {
	vm.ListViewModel.ClampSelection(len(vm.filtered))
}
