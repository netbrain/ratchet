package components

import (
	"strings"
	"testing"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// --- test helpers ---

func testStoreWithPairs(pairs []client.PairStatus) *state.Store {
	s := state.NewStore()
	s.SetPairs(pairs)
	return s
}

func samplePairs() []client.PairStatus {
	return []client.PairStatus{
		{Name: "tui-layout", Component: "monitor", Phase: "test", Status: "debating", Enabled: true, Scope: "internal/tui"},
		{Name: "api-design", Component: "server", Phase: "plan", Status: "consensus", Enabled: true, Scope: "internal/api"},
		{Name: "db-schema", Component: "storage", Phase: "implement", Status: "idle", Enabled: false, Scope: "internal/db"},
	}
}

// findAnyRuneBinding searches a KeyMap for a binding that matches any rune (AnyRune pattern).
func findAnyRuneBinding(km tui.KeyMap) *tui.KeyBinding {
	for i := range km {
		if km[i].Pattern.AnyRune {
			return &km[i]
		}
	}
	return nil
}

// hasFocusRequired checks if a binding has FocusRequired set.
func hasFocusRequired(b *tui.KeyBinding) bool {
	return b.Pattern.FocusRequired
}

// --- 1. Interface compliance ---

func TestPairsScreenImplementsComponent(t *testing.T) {
	store := testStoreWithPairs(nil)
	ps := NewPairsScreen(store)

	var _ tui.Component = ps
}

func TestPairsScreenImplementsKeyListener(t *testing.T) {
	store := testStoreWithPairs(nil)
	ps := NewPairsScreen(store)

	var _ tui.KeyListener = ps
}

// --- 2. Render with data ---

func TestPairsScreenRenderWithData(t *testing.T) {
	store := testStoreWithPairs(samplePairs())
	ps := NewPairsScreen(store)

	el := ps.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}

	// The table area should have a header row + 3 data rows = at least 4 children.
	// We walk the tree to find the table container. The exact structure depends on
	// implementation, but we expect at least 4 row-like children somewhere in the tree.
	totalRows := countRowElements(el)
	if totalRows < 4 {
		t.Fatalf("expected at least 4 rows (1 header + 3 data), found %d", totalRows)
	}
}

// countRowElements counts direct children of the element tree recursively,
// returning the max child count at any level (the table container).
func countRowElements(el *tui.Element) int {
	children := el.Children()
	max := len(children)
	for _, child := range children {
		n := countRowElements(child)
		if n > max {
			max = n
		}
	}
	return max
}

// --- 3. Render empty ---

func TestPairsScreenRenderEmpty(t *testing.T) {
	store := testStoreWithPairs(nil)
	ps := NewPairsScreen(store)

	el := ps.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}

	// The element tree should contain an empty-state message like "No pairs found".
	text := collectAllText(el)
	if !strings.Contains(strings.ToLower(text), "no pairs") {
		t.Fatalf("expected empty state text containing 'no pairs', got: %q", text)
	}
}

// collectAllText recursively collects all Text() from the element tree.
func collectAllText(el *tui.Element) string {
	var sb strings.Builder
	sb.WriteString(el.Text())
	for _, child := range el.Children() {
		sb.WriteString(" ")
		sb.WriteString(collectAllText(child))
	}
	return sb.String()
}

// --- 4. Selection j/k ---

func TestPairsScreenSelectionJ(t *testing.T) {
	store := testStoreWithPairs(samplePairs())
	ps := NewPairsScreen(store)

	km := ps.KeyMap()
	handler := findRuneHandler(km, 'j')
	if handler == nil {
		t.Fatal("no handler found for key 'j'")
	}

	// Initially selected index should be 0.
	if ps.vm.SelectedIndex() != 0 {
		t.Fatalf("expected initial selection 0, got %d", ps.vm.SelectedIndex())
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})

	if ps.vm.SelectedIndex() != 1 {
		t.Fatalf("expected selection 1 after 'j', got %d", ps.vm.SelectedIndex())
	}
}

func TestPairsScreenSelectionK(t *testing.T) {
	store := testStoreWithPairs(samplePairs())
	ps := NewPairsScreen(store)

	// Move to index 1 first.
	km := ps.KeyMap()
	jHandler := findRuneHandler(km, 'j')
	if jHandler == nil {
		t.Fatal("no handler found for key 'j'")
	}
	jHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})

	kHandler := findRuneHandler(km, 'k')
	if kHandler == nil {
		t.Fatal("no handler found for key 'k'")
	}
	kHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'k'})

	if ps.vm.SelectedIndex() != 0 {
		t.Fatalf("expected selection 0 after j then k, got %d", ps.vm.SelectedIndex())
	}
}

// --- 5. Filter mode toggle ---

func TestPairsScreenFilterModeToggle(t *testing.T) {
	store := testStoreWithPairs(samplePairs())
	ps := NewPairsScreen(store)

	// Press '/' to enter filter mode.
	km := ps.KeyMap()
	slashHandler := findRuneHandler(km, '/')
	if slashHandler == nil {
		t.Fatal("no handler found for key '/'")
	}
	slashHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: '/'})

	if !ps.filterMode {
		t.Fatal("expected filterMode=true after pressing '/'")
	}

	// Press Esc to exit filter mode.
	km = ps.KeyMap() // Re-fetch since KeyMap is dynamic.
	escHandler := findKeyHandler(km, tui.KeyEscape, tui.ModNone)
	if escHandler == nil {
		t.Fatal("no handler found for Escape in filter mode")
	}
	escHandler(tui.KeyEvent{Key: tui.KeyEscape})

	if ps.filterMode {
		t.Fatal("expected filterMode=false after pressing Esc")
	}
}

// --- 6. Filter mode KeyMap has OnFocused AnyRune ---

func TestPairsScreenFilterModeKeyMapHasAnyRune(t *testing.T) {
	store := testStoreWithPairs(samplePairs())
	ps := NewPairsScreen(store)
	ps.filterMode = true

	km := ps.KeyMap()
	binding := findAnyRuneBinding(km)
	if binding == nil {
		t.Fatal("expected AnyRune binding in filter mode KeyMap")
	}
	if !hasFocusRequired(binding) {
		t.Fatal("expected AnyRune binding to have FocusRequired (OnFocused)")
	}
}

// --- 7. Normal mode KeyMap has j, k, / but no AnyRune ---

func TestPairsScreenNormalModeKeyMap(t *testing.T) {
	store := testStoreWithPairs(samplePairs())
	ps := NewPairsScreen(store)
	ps.filterMode = false

	km := ps.KeyMap()

	if findRuneHandler(km, 'j') == nil {
		t.Fatal("expected 'j' binding in normal mode")
	}
	if findRuneHandler(km, 'k') == nil {
		t.Fatal("expected 'k' binding in normal mode")
	}
	if findRuneHandler(km, '/') == nil {
		t.Fatal("expected '/' binding in normal mode")
	}
	if findAnyRuneBinding(km) != nil {
		t.Fatal("did not expect AnyRune binding in normal mode")
	}
}

// --- 8. Filter clears on second Esc ---

func TestPairsScreenFilterClearsOnSecondEsc(t *testing.T) {
	store := testStoreWithPairs(samplePairs())
	ps := NewPairsScreen(store)

	// Enter filter mode and type something.
	ps.filterMode = true
	ps.vm.SetFilter("tui")

	// Exit filter mode (first Esc).
	ps.filterMode = false

	// Verify filter is still active.
	if ps.vm.Filter() != "tui" {
		t.Fatalf("expected filter 'tui' preserved after first Esc, got %q", ps.vm.Filter())
	}

	// Second Esc in normal mode should clear the filter.
	km := ps.KeyMap()
	escHandler := findKeyHandler(km, tui.KeyEscape, tui.ModNone)
	if escHandler == nil {
		t.Fatal("no handler found for Escape in normal mode")
	}
	escHandler(tui.KeyEvent{Key: tui.KeyEscape})

	if ps.vm.Filter() != "" {
		t.Fatalf("expected filter cleared after second Esc, got %q", ps.vm.Filter())
	}
}

// --- 9. Responsive columns ---

func TestColumnCountForWidth(t *testing.T) {
	tests := []struct {
		width    int
		expected int
	}{
		{width: 120, expected: 5},
		{width: 150, expected: 5},
		{width: 80, expected: 4},
		{width: 100, expected: 4},
		{width: 119, expected: 4},
		{width: 79, expected: 3},
		{width: 60, expected: 3},
		{width: 40, expected: 3},
	}

	for _, tt := range tests {
		got := columnCountForWidth(tt.width)
		if got != tt.expected {
			t.Errorf("columnCountForWidth(%d) = %d, want %d", tt.width, got, tt.expected)
		}
	}
}

// --- 10. Status color mapping ---

func TestStatusForeground(t *testing.T) {
	tests := []struct {
		colorName string
		expected  tui.Color
	}{
		{"cyan", tui.Cyan},
		{"red", tui.Red},
		{"green", tui.Green},
		{"white", tui.White},
	}

	for _, tt := range tests {
		got := statusForeground(tt.colorName)
		if !got.Equal(tt.expected) {
			t.Errorf("statusForeground(%q) = %v, want %v", tt.colorName, got, tt.expected)
		}
	}
}

// --- 11. G jumps to last item ---

func TestPairsScreenSelectionG(t *testing.T) {
	store := testStoreWithPairs(samplePairs())
	ps := NewPairsScreen(store)

	km := ps.KeyMap()
	handler := findRuneHandler(km, 'G')
	if handler == nil {
		t.Fatal("no handler found for key 'G'")
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'G'})

	if ps.vm.SelectedIndex() != 2 {
		t.Fatalf("expected selection 2 (last) after 'G', got %d", ps.vm.SelectedIndex())
	}
}

// --- 12. gg jumps to first item ---

func TestPairsScreenSelectionGG(t *testing.T) {
	store := testStoreWithPairs(samplePairs())
	ps := NewPairsScreen(store)

	// Move to last item first.
	km := ps.KeyMap()
	gHandler := findRuneHandler(km, 'G')
	gHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'G'})
	if ps.vm.SelectedIndex() != 2 {
		t.Fatalf("expected selection 2, got %d", ps.vm.SelectedIndex())
	}

	// Press 'g' twice.
	km = ps.KeyMap()
	ggHandler := findRuneHandler(km, 'g')
	if ggHandler == nil {
		t.Fatal("no handler found for key 'g'")
	}
	ggHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})
	// First g: no movement yet.
	if ps.vm.SelectedIndex() != 2 {
		t.Fatalf("expected selection 2 after first 'g', got %d", ps.vm.SelectedIndex())
	}
	// Second g: jump to top.
	ggHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})
	if ps.vm.SelectedIndex() != 0 {
		t.Fatalf("expected selection 0 after 'gg', got %d", ps.vm.SelectedIndex())
	}
}

// --- 13. Empty state with active filter shows filter text ---

func TestPairsScreenRenderEmptyWithFilter(t *testing.T) {
	store := testStoreWithPairs(samplePairs())
	ps := NewPairsScreen(store)
	ps.vm.SetFilter("nonexistent")

	el := ps.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}
	text := collectAllText(el)
	if !strings.Contains(text, "nonexistent") {
		t.Fatalf("expected empty state to mention filter text, got: %q", text)
	}
}

// --- 14. Nil store does not panic ---

func TestPairsScreenNilStore(t *testing.T) {
	ps := NewPairsScreen(nil)
	el := ps.Render(nil)
	if el == nil {
		t.Fatal("Render with nil store returned nil")
	}
}

// --- 15. Selection clamped after filter narrows list ---

func TestPairsScreenSelectionClampedAfterFilter(t *testing.T) {
	store := testStoreWithPairs(samplePairs())
	ps := NewPairsScreen(store)

	// Move to last item (index 2).
	km := ps.KeyMap()
	jHandler := findRuneHandler(km, 'j')
	jHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})
	jHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})
	if ps.vm.SelectedIndex() != 2 {
		t.Fatalf("expected selection 2, got %d", ps.vm.SelectedIndex())
	}

	// Apply a filter that leaves only 1 result.
	ps.vm.SetFilter("tui-layout")
	if ps.vm.SelectedIndex() >= len(ps.vm.FilteredPairs()) {
		t.Fatalf("selection %d out of range for %d filtered pairs",
			ps.vm.SelectedIndex(), len(ps.vm.FilteredPairs()))
	}
}

func TestStatusForegroundDim(t *testing.T) {
	// "dim" is a special case — the color itself should be a muted/gray tone
	// rather than a named color. We verify it returns a Color that is not
	// equal to any of the standard status colors.
	got := statusForeground("dim")
	if got.Equal(tui.Cyan) || got.Equal(tui.Red) || got.Equal(tui.Green) {
		t.Errorf("statusForeground(\"dim\") should not equal a primary status color, got %v", got)
	}
}
