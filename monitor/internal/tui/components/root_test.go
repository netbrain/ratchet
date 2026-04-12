package components

import (
	"strings"
	"testing"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/app"
	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// TestRootImplementsComponent verifies Root satisfies tui.Component.
func TestRootImplementsComponent(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	var _ tui.Component = root
}

// TestRootImplementsKeyListener verifies Root satisfies tui.KeyListener.
func TestRootImplementsKeyListener(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	var _ tui.KeyListener = root
}

// TestRootRenderReturnsNonNil verifies that Render produces an element tree.
func TestRootRenderReturnsNonNil(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	el := root.Render(nil)
	if el == nil {
		t.Fatal("Root.Render returned nil")
	}
}

// TestRootRenderHasFourZones verifies the element tree has children
// representing header, tab bar, content area, and status bar (AC-1).
func TestRootRenderHasFourZones(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	el := root.Render(nil)
	children := el.Children()
	if len(children) < 4 {
		t.Fatalf("expected at least 4 child zones, got %d", len(children))
	}
}

// TestRootKeyMapHasNumberKeys verifies keys 1-4 are bound (AC-4).
func TestRootKeyMapHasExpectedBindings(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	km := root.KeyMap()
	if km == nil {
		t.Fatal("KeyMap returned nil")
	}

	// We expect at least 7 bindings: 1,2,3,4, Tab, Shift+Tab, q
	if len(km) < 7 {
		t.Fatalf("expected at least 7 key bindings, got %d", len(km))
	}
}

// TestRootKeyHandlerSetsTab1 verifies pressing '1' switches to TabPairs.
func TestRootKeyHandlerSetsTab1(t *testing.T) {
	a := &app.App{ActiveTab: app.TabDebates}
	root := NewRoot(a)

	km := root.KeyMap()
	handler := findRuneHandler(km, '1')
	if handler == nil {
		t.Fatal("no handler found for key '1'")
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: '1'})

	if a.ActiveTab != app.TabPairs {
		t.Fatalf("expected TabPairs after pressing '1', got %v", a.ActiveTab)
	}
}

// TestRootKeyHandlerSetsTab2 verifies pressing '2' switches to TabDebates.
func TestRootKeyHandlerSetsTab2(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	km := root.KeyMap()
	handler := findRuneHandler(km, '2')
	if handler == nil {
		t.Fatal("no handler found for key '2'")
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: '2'})

	if a.ActiveTab != app.TabDebates {
		t.Fatalf("expected TabDebates after pressing '2', got %v", a.ActiveTab)
	}
}

// TestRootKeyHandlerSetsTab3 verifies pressing '3' switches to TabScores.
func TestRootKeyHandlerSetsTab3(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	km := root.KeyMap()
	handler := findRuneHandler(km, '3')
	if handler == nil {
		t.Fatal("no handler found for key '3'")
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: '3'})

	if a.ActiveTab != app.TabScores {
		t.Fatalf("expected TabScores after pressing '3', got %v", a.ActiveTab)
	}
}

// TestRootKeyHandlerSetsTab4 verifies pressing '4' switches to TabEpic.
func TestRootKeyHandlerSetsTab4(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	km := root.KeyMap()
	handler := findRuneHandler(km, '4')
	if handler == nil {
		t.Fatal("no handler found for key '4'")
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: '4'})

	if a.ActiveTab != app.TabEpic {
		t.Fatalf("expected TabEpic after pressing '4', got %v", a.ActiveTab)
	}
}

// TestRootTabKeyNextTab verifies Tab key cycles to next tab (AC-5).
func TestRootTabKeyNextTab(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	km := root.KeyMap()
	handler := findKeyHandler(km, tui.KeyTab, tui.ModNone)
	if handler == nil {
		t.Fatal("no handler found for Tab key")
	}

	handler(tui.KeyEvent{Key: tui.KeyTab})

	if a.ActiveTab != app.TabDebates {
		t.Fatalf("expected TabDebates after Tab from TabPairs, got %v", a.ActiveTab)
	}
}

// TestRootTabKeyWrapsAround verifies Tab wraps from last to first tab (AC-5).
func TestRootTabKeyWrapsAround(t *testing.T) {
	a := &app.App{ActiveTab: app.TabEpic}
	root := NewRoot(a)

	km := root.KeyMap()
	handler := findKeyHandler(km, tui.KeyTab, tui.ModNone)
	if handler == nil {
		t.Fatal("no handler found for Tab key")
	}

	handler(tui.KeyEvent{Key: tui.KeyTab})

	if a.ActiveTab != app.TabPairs {
		t.Fatalf("expected TabPairs after Tab from TabEpic (wrap), got %v", a.ActiveTab)
	}
}

// TestRootShiftTabPrevTab verifies Shift+Tab cycles to previous tab (AC-5).
func TestRootShiftTabPrevTab(t *testing.T) {
	a := &app.App{ActiveTab: app.TabDebates}
	root := NewRoot(a)

	km := root.KeyMap()
	handler := findKeyHandler(km, tui.KeyTab, tui.ModShift)
	if handler == nil {
		t.Fatal("no handler found for Shift+Tab")
	}

	handler(tui.KeyEvent{Key: tui.KeyTab, Mod: tui.ModShift})

	if a.ActiveTab != app.TabPairs {
		t.Fatalf("expected TabPairs after Shift+Tab from TabDebates, got %v", a.ActiveTab)
	}
}

// TestRootShiftTabWrapsAround verifies Shift+Tab wraps from first to last tab.
func TestRootShiftTabWrapsAround(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	km := root.KeyMap()
	handler := findKeyHandler(km, tui.KeyTab, tui.ModShift)
	if handler == nil {
		t.Fatal("no handler found for Shift+Tab")
	}

	handler(tui.KeyEvent{Key: tui.KeyTab, Mod: tui.ModShift})

	if a.ActiveTab != app.TabEpic {
		t.Fatalf("expected TabEpic after Shift+Tab from TabPairs (wrap), got %v", a.ActiveTab)
	}
}

// TestRootQuitHandlerExists verifies 'q' has a handler (AC-8).
func TestRootQuitHandlerExists(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	km := root.KeyMap()
	handler := findRuneHandler(km, 'q')
	if handler == nil {
		t.Fatal("no handler found for key 'q'")
	}

	// Call the handler — it should invoke Shutdown on the app.
	// We can't test ke.App().Stop() without a real tui.App,
	// but we verify the handler exists and doesn't panic with a zero KeyEvent.
	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'q'})
}

// TestRootCtrlCHandlerExists verifies Ctrl+C has a handler (AC-8).
// go-tui disables ISIG in raw mode, so Ctrl+C is a key event, not SIGINT.
func TestRootCtrlCHandlerExists(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	km := root.KeyMap()
	// Ctrl+C is Rune('c') with ModCtrl
	handler := findRuneWithModHandler(km, 'c', tui.ModCtrl)
	if handler == nil {
		t.Fatal("no handler found for Ctrl+C")
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'c', Mod: tui.ModCtrl})
}

// TestNewRootNilApp verifies NewRoot with nil app doesn't panic.
func TestNewRootNilApp(t *testing.T) {
	root := NewRoot(nil)
	if root == nil {
		t.Fatal("NewRoot(nil) should not return nil")
	}

	// Render should not panic with the fallback empty App.
	el := root.Render(nil)
	if el == nil {
		t.Fatal("Render with fallback App returned nil")
	}
}

// TestNewRootNilAppKeyMap verifies KeyMap works with nil-constructed root.
func TestNewRootNilAppKeyMap(t *testing.T) {
	root := NewRoot(nil)
	km := root.KeyMap()
	if km == nil {
		t.Fatal("KeyMap with fallback App returned nil")
	}
}

// TestRootRenderNilStore verifies Render works when App.Store is nil (graceful degradation).
func TestRootRenderNilStore(t *testing.T) {
	a := &app.App{ActiveTab: app.TabPairs}
	root := NewRoot(a)

	el := root.Render(nil)
	if el == nil {
		t.Fatal("Root.Render returned nil with nil Store")
	}
	children := el.Children()
	if len(children) < 4 {
		t.Fatalf("expected at least 4 child zones with nil Store, got %d", len(children))
	}
}

// TestRootWorkspaceKeyBinding verifies 'w' key handler exists and cycles workspaces.
func TestRootWorkspaceKeyBinding(t *testing.T) {
	store := state.NewStore()
	store.SetWorkspaces([]string{"ws-a", "ws-b"})
	store.SetCurrentWorkspace("ws-a")
	a := &app.App{ActiveTab: app.TabPairs, Store: store}
	root := NewRoot(a)

	km := root.KeyMap()
	handler := findRuneHandler(km, 'w')
	if handler == nil {
		t.Fatal("no handler found for 'w' key")
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'w'})

	if got := store.CurrentWorkspace(); got != "ws-b" {
		t.Fatalf("expected workspace 'ws-b' after pressing 'w', got %q", got)
	}
}

// TestRootRenderShowsWorkspaceInHeader verifies the header includes the workspace name.
func TestRootRenderShowsWorkspaceInHeader(t *testing.T) {
	store := state.NewStore()
	store.SetWorkspaces([]string{"engine"})
	store.SetCurrentWorkspace("engine")
	store.SetConnectionState(client.Connected)
	a := &app.App{ActiveTab: app.TabPairs, Store: store}
	root := NewRoot(a)

	el := root.Render(nil)
	text := collectAllText(el)
	if !strings.Contains(text, "[engine]") {
		t.Fatalf("expected header to contain '[engine]', got: %q", text)
	}
}

// --- helpers ---

// findRuneHandler searches a KeyMap for a binding matching a specific rune.
// We match on Pattern.Rune field which is set by tui.Rune('x').
func findRuneHandler(km tui.KeyMap, r rune) func(tui.KeyEvent) {
	for _, b := range km {
		if b.Pattern.Rune == r {
			return b.Handler
		}
	}
	return nil
}

// findKeyHandler searches a KeyMap for a binding matching a specific key+modifier.
func findKeyHandler(km tui.KeyMap, key tui.Key, mod tui.Modifier) func(tui.KeyEvent) {
	for _, b := range km {
		if b.Pattern.Key == key && b.Pattern.Mod == mod {
			return b.Handler
		}
	}
	return nil
}

// findRuneWithModHandler searches a KeyMap for a rune binding with a specific modifier.
func findRuneWithModHandler(km tui.KeyMap, r rune, mod tui.Modifier) func(tui.KeyEvent) {
	for _, b := range km {
		if b.Pattern.Rune == r && b.Pattern.Mod == mod {
			return b.Handler
		}
	}
	return nil
}
