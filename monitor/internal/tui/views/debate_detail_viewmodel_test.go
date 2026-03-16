package views_test

import (
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

var sampleRounds = []client.Round{
	{Number: 1, Role: "generative", Content: "# Round 1\n\nProposed implementation."},
	{Number: 1, Role: "adversarial", Content: "# Round 1 Review\n\nACCEPT with notes."},
	{Number: 2, Role: "generative", Content: "# Round 2\n\nAddressed feedback."},
	{Number: 2, Role: "adversarial", Content: "# Round 2 Review\n\nACCEPT."},
}

func makeDebate(rounds []client.Round) *client.DebateWithRounds {
	return &client.DebateWithRounds{
		DebateMeta: client.DebateMeta{
			ID:         "test-debate",
			Pair:       "test-pair",
			Status:     "consensus",
			RoundCount: len(rounds),
		},
		Rounds: rounds,
	}
}

// ── Construction ────────────────────────────────────────────────────────

func TestNewDebateDetailViewModel(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	if vm == nil {
		t.Fatal("NewDebateDetailViewModel returned nil")
	}
}

// ── Round access ────────────────────────────────────────────────────────

func TestDebateDetailRounds(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	if vm.RoundCount() != 4 {
		t.Errorf("RoundCount = %d, want 4", vm.RoundCount())
	}
	rounds := vm.Rounds()
	if len(rounds) != 4 {
		t.Fatalf("Rounds() len = %d, want 4", len(rounds))
	}
}

// ── Round navigation ────────────────────────────────────────────────────

func TestDebateDetailCurrentRound(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	if vm.CurrentRound() != 0 {
		t.Errorf("initial CurrentRound = %d, want 0", vm.CurrentRound())
	}
}

func TestDebateDetailNextPrevRound(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))

	vm.NextRound()
	if vm.CurrentRound() != 1 {
		t.Errorf("after NextRound CurrentRound = %d, want 1", vm.CurrentRound())
	}

	vm.NextRound()
	vm.NextRound()
	if vm.CurrentRound() != 3 {
		t.Errorf("after 3x NextRound CurrentRound = %d, want 3", vm.CurrentRound())
	}

	// Should clamp at end, not wrap
	vm.NextRound()
	if vm.CurrentRound() != 3 {
		t.Errorf("NextRound past end should clamp, got %d", vm.CurrentRound())
	}

	vm.PrevRound()
	if vm.CurrentRound() != 2 {
		t.Errorf("after PrevRound CurrentRound = %d, want 2", vm.CurrentRound())
	}
}

func TestDebateDetailPrevRoundClampsAtZero(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.PrevRound()
	if vm.CurrentRound() != 0 {
		t.Errorf("PrevRound at 0 should clamp, got %d", vm.CurrentRound())
	}
}

// ── Round content ───────────────────────────────────────────────────────

func TestDebateDetailCurrentRoundContent(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	content := vm.CurrentRoundContent()
	if content != sampleRounds[0].Content {
		t.Errorf("CurrentRoundContent = %q, want %q", content, sampleRounds[0].Content)
	}

	vm.NextRound()
	content = vm.CurrentRoundContent()
	if content != sampleRounds[1].Content {
		t.Errorf("after NextRound content = %q, want %q", content, sampleRounds[1].Content)
	}
}

func TestDebateDetailContentEmptyRounds(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(nil))
	content := vm.CurrentRoundContent()
	if content != "" {
		t.Errorf("CurrentRoundContent with no rounds = %q, want empty", content)
	}
}

// ── Follow mode ─────────────────────────────────────────────────────────

func TestDebateDetailFollowMode(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))

	if vm.IsFollowing() {
		t.Error("IsFollowing should default to false")
	}

	vm.ToggleFollow()
	if !vm.IsFollowing() {
		t.Error("IsFollowing should be true after toggle")
	}

	vm.ToggleFollow()
	if vm.IsFollowing() {
		t.Error("IsFollowing should be false after second toggle")
	}
}

// ── Live update ─────────────────────────────────────────────────────────

func TestDebateDetailUpdate(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds[:2]))
	if vm.RoundCount() != 2 {
		t.Fatalf("precondition: RoundCount = %d, want 2", vm.RoundCount())
	}

	// Simulate new rounds arriving
	updated := makeDebate(sampleRounds)
	vm.Update(updated)
	if vm.RoundCount() != 4 {
		t.Errorf("after Update RoundCount = %d, want 4", vm.RoundCount())
	}
}

// ── Content scroll (M10) ────────────────────────────────────────────────

func TestDebateDetailContentScrollOffset(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))

	if vm.ContentScrollOffset() != 0 {
		t.Errorf("initial ContentScrollOffset = %d, want 0", vm.ContentScrollOffset())
	}
}

func TestDebateDetailScrollDown(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(5)

	vm.ScrollDown()
	if vm.ContentScrollOffset() != 1 {
		t.Errorf("after ScrollDown ContentScrollOffset = %d, want 1", vm.ContentScrollOffset())
	}

	vm.ScrollDown()
	if vm.ContentScrollOffset() != 2 {
		t.Errorf("after 2x ScrollDown ContentScrollOffset = %d, want 2", vm.ContentScrollOffset())
	}
}

func TestDebateDetailScrollUp(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(5)

	vm.ScrollDown()
	vm.ScrollDown()
	vm.ScrollUp()
	if vm.ContentScrollOffset() != 1 {
		t.Errorf("after ScrollUp ContentScrollOffset = %d, want 1", vm.ContentScrollOffset())
	}
}

func TestDebateDetailScrollUpClampsAtZero(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(5)

	vm.ScrollUp()
	if vm.ContentScrollOffset() != 0 {
		t.Errorf("ScrollUp at 0 should clamp, got %d", vm.ContentScrollOffset())
	}
}

func TestDebateDetailPageDown(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(3)

	vm.PageDown()
	if vm.ContentScrollOffset() != 3 {
		t.Errorf("after PageDown(viewport=3) ContentScrollOffset = %d, want 3", vm.ContentScrollOffset())
	}

	vm.PageDown()
	if vm.ContentScrollOffset() != 6 {
		t.Errorf("after 2x PageDown ContentScrollOffset = %d, want 6", vm.ContentScrollOffset())
	}
}

func TestDebateDetailContentScrollResetsOnRoundChange(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(5)

	// Scroll down in current round
	vm.ScrollDown()
	vm.ScrollDown()
	vm.ScrollDown()
	if vm.ContentScrollOffset() == 0 {
		t.Fatal("precondition: ContentScrollOffset should be > 0")
	}

	// Change round — scroll should reset to 0
	vm.NextRound()
	if vm.ContentScrollOffset() != 0 {
		t.Errorf("ContentScrollOffset should reset to 0 on round change, got %d", vm.ContentScrollOffset())
	}
}

func TestDebateDetailContentScrollResetsOnPrevRound(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(5)

	vm.NextRound() // go to round 1
	vm.ScrollDown()
	vm.ScrollDown()
	if vm.ContentScrollOffset() == 0 {
		t.Fatal("precondition: ContentScrollOffset should be > 0")
	}

	vm.PrevRound()
	if vm.ContentScrollOffset() != 0 {
		t.Errorf("ContentScrollOffset should reset to 0 on PrevRound, got %d", vm.ContentScrollOffset())
	}
}

func TestDebateDetailSetViewportHeight(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(10)

	// PageDown should jump by viewport height
	vm.PageDown()
	if vm.ContentScrollOffset() != 10 {
		t.Errorf("PageDown with viewport=10 should advance by 10, got %d", vm.ContentScrollOffset())
	}
}

func TestDebateDetailContentScrollEmptyRounds(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(nil))
	vm.SetViewportHeight(5)

	// Should not panic
	vm.ScrollDown()
	vm.ScrollUp()
	vm.PageDown()
	if vm.ContentScrollOffset() != 0 {
		t.Errorf("content scroll on empty rounds should stay at 0, got %d", vm.ContentScrollOffset())
	}
}

func TestDebateDetailSetViewportHeightZero(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(0) // should not panic

	vm.PageDown()
	// With viewport 0, PageDown should be a no-op (advance by 0)
	if vm.ContentScrollOffset() != 0 {
		t.Errorf("PageDown with viewport=0 should not advance, got %d", vm.ContentScrollOffset())
	}
}

func TestDebateDetailScrollAfterViewportResize(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(3)

	// Scroll down
	vm.PageDown() // offset=3
	if vm.ContentScrollOffset() != 3 {
		t.Fatalf("precondition: offset = %d, want 3", vm.ContentScrollOffset())
	}

	// Resize viewport — offset stays (no content length known)
	vm.SetViewportHeight(10)
	if vm.ContentScrollOffset() != 3 {
		t.Errorf("offset should be preserved on viewport resize, got %d", vm.ContentScrollOffset())
	}
}

// ── Live update ─────────────────────────────────────────────────────────

// ── HARDEN: PageUp clamping at 0 ─────────────────────────────────────────

func TestDebateDetailPageUpClampsAtZero(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(5)

	// PageUp at offset 0 should stay at 0
	vm.PageUp()
	if vm.ContentScrollOffset() != 0 {
		t.Errorf("PageUp at 0 should clamp, got %d", vm.ContentScrollOffset())
	}
}

func TestDebateDetailPageUpClampsPartialPage(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(10)

	// Scroll down 3 lines (less than a full page)
	vm.ScrollDown()
	vm.ScrollDown()
	vm.ScrollDown()
	if vm.ContentScrollOffset() != 3 {
		t.Fatalf("precondition: offset = %d, want 3", vm.ContentScrollOffset())
	}

	// PageUp(viewport=10) from offset 3 should clamp to 0, not go negative
	vm.PageUp()
	if vm.ContentScrollOffset() != 0 {
		t.Errorf("PageUp should clamp to 0, got %d", vm.ContentScrollOffset())
	}
}

func TestDebateDetailPageUpExactPage(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(5)

	// Scroll down exactly one page
	vm.PageDown() // offset=5
	if vm.ContentScrollOffset() != 5 {
		t.Fatalf("precondition: offset = %d, want 5", vm.ContentScrollOffset())
	}

	// PageUp should bring us back to 0
	vm.PageUp()
	if vm.ContentScrollOffset() != 0 {
		t.Errorf("PageUp after exact PageDown should return to 0, got %d", vm.ContentScrollOffset())
	}
}

func TestDebateDetailPageUpEmptyRounds(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(nil))
	vm.SetViewportHeight(5)

	// Should not panic
	vm.PageUp()
	if vm.ContentScrollOffset() != 0 {
		t.Errorf("PageUp on empty rounds should stay at 0, got %d", vm.ContentScrollOffset())
	}
}

func TestDebateDetailPageUpZeroViewport(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(0)

	// Should be no-op
	vm.ScrollDown()
	vm.ScrollDown()
	vm.PageUp()
	// With viewport 0, PageUp is a no-op so offset should stay at 2
	if vm.ContentScrollOffset() != 2 {
		t.Errorf("PageUp with viewport=0 should be no-op, got %d", vm.ContentScrollOffset())
	}
}

// ── HARDEN: Nil receiver safety ──────────────────────────────────────────

func TestDebateDetailNilReceiver(t *testing.T) {
	var vm *views.DebateDetailViewModel

	// None of these should panic.
	if vm.Rounds() != nil {
		t.Error("nil Rounds should return nil")
	}
	if vm.RoundCount() != 0 {
		t.Error("nil RoundCount should return 0")
	}
	if vm.CurrentRound() != 0 {
		t.Error("nil CurrentRound should return 0")
	}
	if vm.CurrentRoundContent() != "" {
		t.Error("nil CurrentRoundContent should return empty")
	}
	if vm.ContentScrollOffset() != 0 {
		t.Error("nil ContentScrollOffset should return 0")
	}
	if vm.IsFollowing() {
		t.Error("nil IsFollowing should return false")
	}
	vm.NextRound()
	vm.PrevRound()
	vm.ScrollDown()
	vm.ScrollUp()
	vm.PageDown()
	vm.PageUp()
	vm.SetViewportHeight(5)
	vm.ToggleFollow()
	vm.Update(nil)
}

// ── HARDEN: Nil debate in constructor ────────────────────────────────────

func TestDebateDetailNilDebate(t *testing.T) {
	vm := views.NewDebateDetailViewModel(nil)

	// All methods should be safe with nil debate
	if vm.Rounds() != nil {
		t.Error("Rounds with nil debate should return nil")
	}
	if vm.RoundCount() != 0 {
		t.Error("RoundCount with nil debate should return 0")
	}
	if vm.CurrentRoundContent() != "" {
		t.Error("CurrentRoundContent with nil debate should return empty")
	}
	vm.NextRound()
	vm.PrevRound()
	vm.ScrollDown()
	vm.ScrollUp()
	vm.PageDown()
	vm.PageUp()
}

// ── HARDEN: Negative viewport height ────────────────────────────────────

func TestDebateDetailNegativeViewportHeight(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds))
	vm.SetViewportHeight(-1)

	// PageDown/PageUp should be no-ops with non-positive viewport
	vm.PageDown()
	if vm.ContentScrollOffset() != 0 {
		t.Errorf("PageDown with negative viewport should be no-op, got %d", vm.ContentScrollOffset())
	}
	vm.PageUp()
	if vm.ContentScrollOffset() != 0 {
		t.Errorf("PageUp with negative viewport should be no-op, got %d", vm.ContentScrollOffset())
	}
}

// ── Live update ─────────────────────────────────────────────────────────

func TestDebateDetailUpdateWithFollowJumpsToEnd(t *testing.T) {
	vm := views.NewDebateDetailViewModel(makeDebate(sampleRounds[:2]))
	vm.ToggleFollow() // enable follow mode

	updated := makeDebate(sampleRounds) // 4 rounds now
	vm.Update(updated)

	if vm.CurrentRound() != 3 {
		t.Errorf("with follow mode, Update should jump to last round, got %d", vm.CurrentRound())
	}
}
