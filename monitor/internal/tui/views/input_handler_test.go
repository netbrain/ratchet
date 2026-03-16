package views_test

import (
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

// ── KeyEvent struct ─────────────────────────────────────────────────────

func TestKeyEventRune(t *testing.T) {
	ke := views.KeyEvent{Rune: 'j'}
	if ke.Rune != 'j' {
		t.Errorf("Rune = %c, want j", ke.Rune)
	}
	if ke.Special != "" {
		t.Errorf("Special = %q, want empty", ke.Special)
	}
	if ke.Shift {
		t.Error("Shift should default to false")
	}
}

func TestKeyEventSpecial(t *testing.T) {
	ke := views.KeyEvent{Special: "Enter"}
	if ke.Rune != 0 {
		t.Errorf("Rune = %c, want zero value", ke.Rune)
	}
	if ke.Special != "Enter" {
		t.Errorf("Special = %q, want Enter", ke.Special)
	}
}

func TestKeyEventShift(t *testing.T) {
	ke := views.KeyEvent{Rune: 'J', Shift: true}
	if !ke.Shift {
		t.Error("Shift should be true")
	}
}

func TestKeyEventSpecialKeys(t *testing.T) {
	specials := []string{"Up", "Down", "Left", "Right", "Enter", "Esc", "Tab", "Backspace", "PgUp", "PgDn"}
	for _, s := range specials {
		ke := views.KeyEvent{Special: s}
		if ke.Special != s {
			t.Errorf("Special = %q, want %q", ke.Special, s)
		}
	}
}

// ── InputHandler interface ──────────────────────────────────────────────

// testHandler is a mock that records calls and returns a configured value.
type testHandler struct {
	lastKey  views.KeyEvent
	consumed bool
	calls    int
}

func (h *testHandler) HandleKey(ke views.KeyEvent) bool {
	h.lastKey = ke
	h.calls++
	return h.consumed
}

// Verify testHandler implements InputHandler at compile time.
var _ views.InputHandler = (*testHandler)(nil)

func TestInputHandlerInterfaceConsumed(t *testing.T) {
	h := &testHandler{consumed: true}
	ke := views.KeyEvent{Rune: 'q'}
	result := h.HandleKey(ke)
	if !result {
		t.Error("HandleKey should return true when consumed")
	}
	if h.lastKey.Rune != 'q' {
		t.Errorf("lastKey.Rune = %c, want q", h.lastKey.Rune)
	}
	if h.calls != 1 {
		t.Errorf("calls = %d, want 1", h.calls)
	}
}

func TestInputHandlerInterfaceNotConsumed(t *testing.T) {
	h := &testHandler{consumed: false}
	ke := views.KeyEvent{Special: "Esc"}
	result := h.HandleKey(ke)
	if result {
		t.Error("HandleKey should return false when not consumed")
	}
}

func TestInputHandlerMultipleKeys(t *testing.T) {
	h := &testHandler{consumed: true}

	keys := []views.KeyEvent{
		{Rune: 'j'},
		{Rune: 'k'},
		{Special: "Enter"},
		{Rune: 'J', Shift: true},
	}

	for _, ke := range keys {
		h.HandleKey(ke)
	}

	if h.calls != 4 {
		t.Errorf("calls = %d, want 4", h.calls)
	}
	// Last key should be the shift-J
	if h.lastKey.Rune != 'J' || !h.lastKey.Shift {
		t.Errorf("lastKey = %+v, want Rune='J' Shift=true", h.lastKey)
	}
}

// ── HARDEN: Ctrl modifier on KeyEvent ───────────────────────────────────

func TestKeyEventCtrl(t *testing.T) {
	ke := views.KeyEvent{Rune: 'c', Ctrl: true}
	if !ke.Ctrl {
		t.Error("Ctrl should be true")
	}
	if ke.Alt {
		t.Error("Alt should default to false")
	}
	if ke.Shift {
		t.Error("Shift should default to false")
	}
}

func TestKeyEventAlt(t *testing.T) {
	ke := views.KeyEvent{Rune: 'x', Alt: true}
	if !ke.Alt {
		t.Error("Alt should be true")
	}
	if ke.Ctrl {
		t.Error("Ctrl should default to false")
	}
}

func TestKeyEventAllModifiers(t *testing.T) {
	ke := views.KeyEvent{Rune: 'a', Ctrl: true, Alt: true, Shift: true}
	if !ke.Ctrl || !ke.Alt || !ke.Shift {
		t.Errorf("all modifiers should be true: Ctrl=%v Alt=%v Shift=%v", ke.Ctrl, ke.Alt, ke.Shift)
	}
}

func TestKeyEventCtrlSpecialKey(t *testing.T) {
	ke := views.KeyEvent{Special: views.KeyUp, Ctrl: true}
	if ke.Special != views.KeyUp {
		t.Errorf("Special = %q, want %q", ke.Special, views.KeyUp)
	}
	if !ke.Ctrl {
		t.Error("Ctrl should be true")
	}
}

func TestKeyEventAltSpecialKey(t *testing.T) {
	ke := views.KeyEvent{Special: views.KeyTab, Alt: true}
	if ke.Special != views.KeyTab {
		t.Errorf("Special = %q, want %q", ke.Special, views.KeyTab)
	}
	if !ke.Alt {
		t.Error("Alt should be true")
	}
}

// ── HARDEN: InputHandler receives Ctrl/Alt keys correctly ───────────────

func TestInputHandlerCtrlKey(t *testing.T) {
	h := &testHandler{consumed: true}
	ke := views.KeyEvent{Rune: 'c', Ctrl: true}
	h.HandleKey(ke)

	if !h.lastKey.Ctrl {
		t.Error("handler should receive Ctrl=true")
	}
	if h.lastKey.Rune != 'c' {
		t.Errorf("handler Rune = %c, want c", h.lastKey.Rune)
	}
}

func TestInputHandlerAltKey(t *testing.T) {
	h := &testHandler{consumed: true}
	ke := views.KeyEvent{Rune: 'n', Alt: true}
	h.HandleKey(ke)

	if !h.lastKey.Alt {
		t.Error("handler should receive Alt=true")
	}
}

func TestInputHandlerCtrlAltSpecialKey(t *testing.T) {
	h := &testHandler{consumed: true}
	ke := views.KeyEvent{Special: views.KeyPgUp, Ctrl: true, Alt: true}
	h.HandleKey(ke)

	if h.lastKey.Special != views.KeyPgUp {
		t.Errorf("handler Special = %q, want PgUp", h.lastKey.Special)
	}
	if !h.lastKey.Ctrl || !h.lastKey.Alt {
		t.Error("handler should receive both Ctrl and Alt")
	}
}

// ── HARDEN: Special key constants match expected values ──────────────────

func TestSpecialKeyConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"KeyUp", views.KeyUp, "Up"},
		{"KeyDown", views.KeyDown, "Down"},
		{"KeyLeft", views.KeyLeft, "Left"},
		{"KeyRight", views.KeyRight, "Right"},
		{"KeyEnter", views.KeyEnter, "Enter"},
		{"KeyEsc", views.KeyEsc, "Esc"},
		{"KeyTab", views.KeyTab, "Tab"},
		{"KeyBackspace", views.KeyBackspace, "Backspace"},
		{"KeySpace", views.KeySpace, "Space"},
		{"KeyPgUp", views.KeyPgUp, "PgUp"},
		{"KeyPgDn", views.KeyPgDn, "PgDn"},
	}
	for _, tt := range tests {
		if tt.value != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.value, tt.want)
		}
	}
}

// ── HARDEN: KeyEvent zero value ─────────────────────────────────────────

func TestKeyEventZeroValue(t *testing.T) {
	var ke views.KeyEvent
	if ke.Rune != 0 {
		t.Errorf("zero Rune = %c, want 0", ke.Rune)
	}
	if ke.Special != "" {
		t.Errorf("zero Special = %q, want empty", ke.Special)
	}
	if ke.Ctrl || ke.Alt || ke.Shift {
		t.Error("zero value modifiers should all be false")
	}
}

// ── HARDEN: Handler receives zero-value KeyEvent ────────────────────────

func TestInputHandlerZeroKeyEvent(t *testing.T) {
	h := &testHandler{consumed: false}
	ke := views.KeyEvent{}
	result := h.HandleKey(ke)
	if result {
		t.Error("should return false for not-consumed zero event")
	}
	if h.calls != 1 {
		t.Errorf("calls = %d, want 1", h.calls)
	}
}
