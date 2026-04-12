// Package views provides view models for the TUI screens.
package views

import (
	"strings"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// PairsViewModel is the view model for the pair overview screen.
// It holds a snapshot of pairs from the store and provides filtering,
// selection, and grouping logic for the UI layer.
type PairsViewModel struct {
	ListViewModel
	store    *state.Store
	pairs    []client.PairStatus
	filtered []client.PairStatus
	filter   string
}

// NewPairsViewModel creates a PairsViewModel backed by the given store.
func NewPairsViewModel(store *state.Store) *PairsViewModel {
	vm := &PairsViewModel{store: store}
	vm.loadPairs()
	return vm
}

// Pairs returns all pairs (unfiltered).
func (vm *PairsViewModel) Pairs() []client.PairStatus {
	if vm == nil {
		return nil
	}
	return vm.pairs
}

// StatusColor maps a pair status string to a color name for rendering.
func (vm *PairsViewModel) StatusColor(status string) string {
	switch status {
	case "debating":
		return "cyan"
	case "escalated":
		return "red"
	case "consensus":
		return "green"
	case "idle":
		return "dim"
	default:
		return "white"
	}
}

// SetFilter applies a case-insensitive substring filter across pair name,
// component, phase, and status. Recomputes the filtered list and clamps selection.
func (vm *PairsViewModel) SetFilter(query string) {
	if vm == nil {
		return
	}
	vm.filter = query
	vm.applyFilter()
	vm.clampSelection()
}

// Filter returns the current filter value.
func (vm *PairsViewModel) Filter() string {
	if vm == nil {
		return ""
	}
	return vm.filter
}

// FilteredPairs returns pairs matching the current filter.
func (vm *PairsViewModel) FilteredPairs() []client.PairStatus {
	if vm == nil {
		return nil
	}
	return vm.filtered
}

// SelectedIndex returns the current selection index within the filtered list.
func (vm *PairsViewModel) SelectedIndex() int {
	if vm == nil {
		return 0
	}
	return vm.ListViewModel.Selected()
}

// SelectNext moves the selection forward, wrapping to 0 at the end.
func (vm *PairsViewModel) SelectNext() {
	if vm == nil {
		return
	}
	vm.ListViewModel.SelectNext(len(vm.filtered))
}

// SelectPrev moves the selection backward, wrapping to the last item at 0.
func (vm *PairsViewModel) SelectPrev() {
	if vm == nil {
		return
	}
	vm.ListViewModel.SelectPrevious(len(vm.filtered))
}

// SelectFirst moves the selection to the first item.
func (vm *PairsViewModel) SelectFirst() {
	if vm == nil {
		return
	}
	vm.ListViewModel.SelectFirst(len(vm.filtered))
}

// SelectLast moves the selection to the last item.
func (vm *PairsViewModel) SelectLast() {
	if vm == nil {
		return
	}
	vm.ListViewModel.SelectLast(len(vm.filtered))
}

// SelectedPair returns the currently selected pair, or nil if the list is empty.
func (vm *PairsViewModel) SelectedPair() *client.PairStatus {
	if vm == nil || len(vm.filtered) == 0 {
		return nil
	}
	p := vm.filtered[vm.ListViewModel.Selected()]
	return &p
}

// PairsByComponent groups all pairs by their Component field.
func (vm *PairsViewModel) PairsByComponent() map[string][]client.PairStatus {
	if vm == nil {
		return nil
	}
	grouped := make(map[string][]client.PairStatus)
	for _, p := range vm.pairs {
		grouped[p.Component] = append(grouped[p.Component], p)
	}
	return grouped
}

// ActiveCount returns the number of pairs with Active=true.
func (vm *PairsViewModel) ActiveCount() int {
	if vm == nil {
		return 0
	}
	count := 0
	for _, p := range vm.pairs {
		if p.Active {
			count++
		}
	}
	return count
}

// Refresh re-reads pairs from the store and reapplies the filter.
func (vm *PairsViewModel) Refresh() {
	if vm == nil {
		return
	}
	vm.loadPairs()
	vm.clampSelection()
}

func (vm *PairsViewModel) loadPairs() {
	if vm.store == nil {
		vm.pairs = nil
		vm.applyFilter()
		return
	}
	vm.pairs = vm.store.Pairs()
	vm.applyFilter()
}

func (vm *PairsViewModel) applyFilter() {
	var ws string
	if vm.store != nil {
		ws = vm.store.CurrentWorkspace()
	}
	q := strings.ToLower(vm.filter)
	vm.filtered = nil
	for _, p := range vm.pairs {
		if ws != "" && p.Workspace != ws {
			continue
		}
		if q != "" && !(strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Component), q) ||
			strings.Contains(strings.ToLower(p.Phase), q) ||
			strings.Contains(strings.ToLower(p.Status), q)) {
			continue
		}
		vm.filtered = append(vm.filtered, p)
	}
	if vm.filtered == nil {
		vm.filtered = []client.PairStatus{}
	}
}

// SetViewportHeight sets the viewport height and recalculates scroll offset.
func (vm *PairsViewModel) SetViewportHeight(h int) {
	if vm == nil {
		return
	}
	vm.ListViewModel.SetViewportHeight(h, len(vm.filtered))
}

// ScrollOffset returns the current scroll offset.
func (vm *PairsViewModel) ScrollOffset() int {
	if vm == nil {
		return 0
	}
	return vm.ListViewModel.ScrollOffset()
}

func (vm *PairsViewModel) clampSelection() {
	vm.ListViewModel.ClampSelection(len(vm.filtered))
}
