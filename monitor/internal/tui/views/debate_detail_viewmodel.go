package views

import "github.com/netbrain/ratchet-monitor/internal/tui/client"

// DebateDetailViewModel is the view model for viewing a single debate's rounds.
type DebateDetailViewModel struct {
	debate         *client.DebateWithRounds
	current        int
	following      bool
	contentOffset  int
	viewportHeight int
}

// NewDebateDetailViewModel creates a detail view model for the given debate.
func NewDebateDetailViewModel(debate *client.DebateWithRounds) *DebateDetailViewModel {
	return &DebateDetailViewModel{debate: debate}
}

// Rounds returns all rounds in the debate.
func (vm *DebateDetailViewModel) Rounds() []client.Round {
	if vm == nil || vm.debate == nil {
		return nil
	}
	return vm.debate.Rounds
}

// RoundCount returns the number of rounds.
func (vm *DebateDetailViewModel) RoundCount() int {
	if vm == nil || vm.debate == nil {
		return 0
	}
	return len(vm.debate.Rounds)
}

// CurrentRound returns the index of the currently viewed round.
func (vm *DebateDetailViewModel) CurrentRound() int {
	if vm == nil {
		return 0
	}
	return vm.current
}

// NextRound advances to the next round, clamping at the end.
func (vm *DebateDetailViewModel) NextRound() {
	if vm == nil || vm.debate == nil {
		return
	}
	if vm.current < len(vm.debate.Rounds)-1 {
		vm.current++
		vm.contentOffset = 0
	}
}

// PrevRound goes back one round, clamping at 0.
func (vm *DebateDetailViewModel) PrevRound() {
	if vm == nil || vm.debate == nil {
		return
	}
	if vm.current > 0 {
		vm.current--
		vm.contentOffset = 0
	}
}

// ScrollDown advances the content scroll offset by 1.
func (vm *DebateDetailViewModel) ScrollDown() {
	if vm == nil || vm.debate == nil || len(vm.debate.Rounds) == 0 {
		return
	}
	vm.contentOffset++
}

// ScrollUp decreases the content scroll offset by 1, clamping at 0.
func (vm *DebateDetailViewModel) ScrollUp() {
	if vm == nil || vm.debate == nil || len(vm.debate.Rounds) == 0 {
		return
	}
	if vm.contentOffset > 0 {
		vm.contentOffset--
	}
}

// PageDown advances the content scroll offset by viewportHeight.
func (vm *DebateDetailViewModel) PageDown() {
	if vm == nil || vm.debate == nil || len(vm.debate.Rounds) == 0 || vm.viewportHeight <= 0 {
		return
	}
	vm.contentOffset += vm.viewportHeight
}

// PageUp decreases the content scroll offset by viewportHeight, clamping at 0.
func (vm *DebateDetailViewModel) PageUp() {
	if vm == nil || vm.debate == nil || len(vm.debate.Rounds) == 0 || vm.viewportHeight <= 0 {
		return
	}
	vm.contentOffset -= vm.viewportHeight
	if vm.contentOffset < 0 {
		vm.contentOffset = 0
	}
}

// SetViewportHeight sets the viewport height for content scrolling.
func (vm *DebateDetailViewModel) SetViewportHeight(h int) {
	if vm == nil {
		return
	}
	vm.viewportHeight = h
}

// ContentScrollOffset returns the current content scroll offset.
func (vm *DebateDetailViewModel) ContentScrollOffset() int {
	if vm == nil {
		return 0
	}
	return vm.contentOffset
}

// ScrollToTop resets the content scroll offset to 0.
func (vm *DebateDetailViewModel) ScrollToTop() {
	if vm == nil {
		return
	}
	vm.contentOffset = 0
}

// ScrollToBottom sets the content scroll offset to a large value.
// The caller is expected to clamp it against the actual line count.
func (vm *DebateDetailViewModel) ScrollToBottom() {
	if vm == nil {
		return
	}
	vm.contentOffset = 1 << 30 // effectively infinite; render will clamp
}

// CurrentRoundContent returns the content of the current round, or "" if no rounds.
func (vm *DebateDetailViewModel) CurrentRoundContent() string {
	if vm == nil || vm.debate == nil || len(vm.debate.Rounds) == 0 {
		return ""
	}
	return vm.debate.Rounds[vm.current].Content
}

// IsFollowing returns whether follow mode is enabled.
func (vm *DebateDetailViewModel) IsFollowing() bool {
	if vm == nil {
		return false
	}
	return vm.following
}

// ToggleFollow toggles follow mode on/off.
func (vm *DebateDetailViewModel) ToggleFollow() {
	if vm == nil {
		return
	}
	vm.following = !vm.following
}

// Update replaces the debate data with a newer version (e.g., new rounds arrived).
// If follow mode is on, jumps to the last round.
func (vm *DebateDetailViewModel) Update(debate *client.DebateWithRounds) {
	if vm == nil {
		return
	}
	vm.debate = debate
	if vm.following && len(vm.debate.Rounds) > 0 {
		vm.current = len(vm.debate.Rounds) - 1
		vm.contentOffset = 0
	} else if n := len(vm.debate.Rounds); n > 0 && vm.current >= n {
		vm.current = n - 1
	} else if n == 0 {
		vm.current = 0
	}
}
