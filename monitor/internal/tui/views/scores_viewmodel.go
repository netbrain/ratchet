package views

import (
	"sort"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// PairScoreSummary holds computed metrics for a single pair.
type PairScoreSummary struct {
	Pair           string
	DebateCount    int
	ConsensusRate  float64 // fraction of non-escalated debates
	AvgRounds      float64
	IssuesFound    int
	IssuesResolved int
	Escalated      int
	Trend          []int // rounds-to-consensus values ordered by time
}

// ScoresViewModel is the view model for the scores tab.
type ScoresViewModel struct {
	store          *state.Store
	summaries      []PairScoreSummary
	selected       int
	viewportHeight int
	scrollOffset   int
}

// NewScoresViewModel creates a ScoresViewModel backed by the given store.
func NewScoresViewModel(store *state.Store) *ScoresViewModel {
	vm := &ScoresViewModel{store: store}
	vm.loadSummaries()
	return vm
}

// PairSummaries returns computed score summaries for all pairs that have data.
func (vm *ScoresViewModel) PairSummaries() []PairScoreSummary {
	if vm == nil {
		return nil
	}
	return vm.summaries
}

// SelectedIndex returns the current selection index.
func (vm *ScoresViewModel) SelectedIndex() int {
	if vm == nil {
		return 0
	}
	return vm.selected
}

// SelectNext moves selection forward with wrap-around.
func (vm *ScoresViewModel) SelectNext() {
	if vm == nil {
		return
	}
	n := len(vm.summaries)
	if n == 0 {
		return
	}
	vm.selected = (vm.selected + 1) % n
	vm.adjustScrollOffset()
}

// SelectPrev moves selection backward with wrap-around.
func (vm *ScoresViewModel) SelectPrev() {
	if vm == nil {
		return
	}
	n := len(vm.summaries)
	if n == 0 {
		return
	}
	vm.selected = (vm.selected - 1 + n) % n
	vm.adjustScrollOffset()
}

// SelectFirst jumps to the first item.
func (vm *ScoresViewModel) SelectFirst() {
	if vm == nil {
		return
	}
	vm.selected = 0
	vm.adjustScrollOffset()
}

// SelectLast jumps to the last item.
func (vm *ScoresViewModel) SelectLast() {
	if vm == nil {
		return
	}
	n := len(vm.summaries)
	if n == 0 {
		return
	}
	vm.selected = n - 1
	vm.adjustScrollOffset()
}

// SelectedSummary returns the currently selected summary, or nil if empty.
func (vm *ScoresViewModel) SelectedSummary() *PairScoreSummary {
	if vm == nil || len(vm.summaries) == 0 {
		return nil
	}
	s := vm.summaries[vm.selected]
	return &s
}

// SetViewportHeight sets the viewport height and recalculates scroll offset.
func (vm *ScoresViewModel) SetViewportHeight(h int) {
	if vm == nil {
		return
	}
	vm.viewportHeight = h
	vm.adjustScrollOffset()
}

// ScrollOffset returns the current scroll offset.
func (vm *ScoresViewModel) ScrollOffset() int {
	if vm == nil {
		return 0
	}
	return vm.scrollOffset
}

// Refresh re-reads scores from the store.
func (vm *ScoresViewModel) Refresh() {
	if vm == nil {
		return
	}
	vm.loadSummaries()
	vm.clampSelection()
}

func (vm *ScoresViewModel) clampSelection() {
	n := len(vm.summaries)
	if n == 0 {
		vm.selected = 0
	} else if vm.selected >= n {
		vm.selected = n - 1
	}
	vm.adjustScrollOffset()
}

func (vm *ScoresViewModel) adjustScrollOffset() {
	n := len(vm.summaries)
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

func (vm *ScoresViewModel) loadSummaries() {
	if vm.store == nil {
		vm.summaries = nil
		return
	}
	ws := vm.store.CurrentWorkspace()
	pairs := vm.store.Pairs()
	vm.summaries = nil

	for _, p := range pairs {
		if ws != "" && p.Workspace != ws {
			continue
		}
		scores := vm.store.Scores(p.Name)
		if ws != "" {
			filtered := scores[:0:0]
			for _, sc := range scores {
				if sc.Workspace == ws {
					filtered = append(filtered, sc)
				}
			}
			scores = filtered
		}
		if len(scores) == 0 {
			continue
		}
		vm.summaries = append(vm.summaries, computeSummary(p.Name, scores))
	}
}

func computeSummary(pair string, scores []client.ScoreEntry) PairScoreSummary {
	// Copy to avoid mutating the store's slice.
	sorted := make([]client.ScoreEntry, len(scores))
	copy(sorted, scores)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})
	scores = sorted

	s := PairScoreSummary{
		Pair:        pair,
		DebateCount: len(scores),
	}

	totalRounds := 0
	nonEscalated := 0
	for _, sc := range scores {
		totalRounds += sc.RoundsToConsensus
		s.IssuesFound += sc.IssuesFound
		s.IssuesResolved += sc.IssuesResolved
		if sc.Escalated {
			s.Escalated++
		} else {
			nonEscalated++
		}
		s.Trend = append(s.Trend, sc.RoundsToConsensus)
	}

	if s.DebateCount > 0 {
		s.ConsensusRate = float64(nonEscalated) / float64(s.DebateCount)
		s.AvgRounds = float64(totalRounds) / float64(s.DebateCount)
	}

	return s
}
