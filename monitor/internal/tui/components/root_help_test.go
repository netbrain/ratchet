package components

import (
	"strings"
	"testing"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/app"
	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// --- Help overlay ---

func TestRootHelpToggle(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	if root.showHelp {
		t.Fatal("help should be hidden initially")
	}

	km := root.KeyMap()
	handler := findRuneHandler(km, '?')
	if handler == nil {
		t.Fatal("no handler found for '?'")
	}

	// Toggle on
	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: '?'})
	if !root.showHelp {
		t.Fatal("help should be visible after pressing '?'")
	}

	// Toggle off
	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: '?'})
	if root.showHelp {
		t.Fatal("help should be hidden after pressing '?' again")
	}
}

func TestRootHelpOverlayRendered(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)
	root.showHelp = true

	el := root.Render(nil)
	text := collectAllText(el)

	// Help overlay should contain key binding descriptions
	for _, expect := range []string{"Tab", "quit", "help"} {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(expect)) {
			t.Errorf("help overlay should mention %q, got: %q", expect, text)
		}
	}
}

func TestRootHelpEscDismisses(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)
	root.showHelp = true

	km := root.KeyMap()
	handler := findKeyHandler(km, tui.KeyEscape, tui.ModNone)
	if handler == nil {
		t.Fatal("no handler found for Esc")
	}

	handler(tui.KeyEvent{Key: tui.KeyEscape})
	if root.showHelp {
		t.Fatal("Esc should dismiss help overlay")
	}
}

// --- Stale data banner ---

func TestRootStaleBannerWhenDisconnected(t *testing.T) {
	store := state.NewStore()
	store.SetConnectionState(client.Disconnected)
	a := &app.App{ActiveTab: app.TabPairs, Store: store}
	root := NewRoot(a)

	el := root.Render(nil)
	text := collectAllText(el)
	if !strings.Contains(strings.ToLower(text), "stale") && !strings.Contains(strings.ToLower(text), "disconnected") {
		t.Fatalf("expected stale/disconnected banner when disconnected, got: %q", text)
	}
}

func TestRootStaleBannerWhenReconnecting(t *testing.T) {
	store := state.NewStore()
	store.SetConnectionState(client.Reconnecting)
	a := &app.App{ActiveTab: app.TabPairs, Store: store}
	root := NewRoot(a)

	el := root.Render(nil)
	text := collectAllText(el)
	if !strings.Contains(strings.ToLower(text), "reconnect") {
		t.Fatalf("expected reconnecting banner, got: %q", text)
	}
}

func TestRootNoBannerWhenConnected(t *testing.T) {
	store := state.NewStore()
	store.SetConnectionState(client.Connected)
	a := &app.App{ActiveTab: app.TabPairs, Store: store}
	root := NewRoot(a)

	el := root.Render(nil)
	text := collectAllText(el)
	if strings.Contains(strings.ToLower(text), "stale") || strings.Contains(strings.ToLower(text), "reconnect") {
		t.Fatalf("should not show stale/reconnect banner when connected, got: %q", text)
	}
}
