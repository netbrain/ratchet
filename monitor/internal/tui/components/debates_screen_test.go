package components

import (
	"strings"
	"testing"
	"time"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// --- test helpers ---

func testStoreWithDebates(debates []client.DebateMeta) *state.Store {
	s := state.NewStore()
	s.SetDebates(debates)
	return s
}

func sampleDebates() []client.DebateMeta {
	return []client.DebateMeta{
		{ID: "tui-layout-m12-build-20260315", Pair: "tui-layout", Phase: "build", Milestone: 12, Status: "consensus", RoundCount: 1, MaxRounds: 3, Started: time.Date(2026, 3, 15, 22, 25, 0, 0, time.UTC)},
		{ID: "tui-ux-m12-review-20260315", Pair: "tui-ux", Phase: "review", Milestone: 12, Status: "consensus", RoundCount: 1, MaxRounds: 3, Started: time.Date(2026, 3, 15, 22, 50, 0, 0, time.UTC)},
		{ID: "sse-correct-m2-test-20260305", Pair: "sse-correctness", Phase: "test", Milestone: 2, Status: "in_progress", RoundCount: 2, MaxRounds: 3, Started: time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)},
		{ID: "tui-client-m7-test-20260315", Pair: "tui-client", Phase: "test", Milestone: 7, Status: "escalated", RoundCount: 3, MaxRounds: 3, Started: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)},
	}
}

func sampleDebateWithRounds() *client.DebateWithRounds {
	// Must match the debate that sorts first (newest) in sampleDebates().
	return &client.DebateWithRounds{
		DebateMeta: client.DebateMeta{
			ID:         "tui-ux-m12-review-20260315",
			Pair:       "tui-ux",
			Phase:      "review",
			Milestone:  12,
			Status:     "consensus",
			RoundCount: 2,
			MaxRounds:  3,
		},
		Rounds: []client.Round{
			{Number: 1, Role: "generative", Content: "# Round 1\n\nProposed **implementation** with `code`."},
			{Number: 1, Role: "adversarial", Content: "# Round 1 Review\n\nACCEPT with notes."},
			{Number: 2, Role: "generative", Content: "# Round 2\n\nAddressed feedback."},
			{Number: 2, Role: "adversarial", Content: "# Round 2 Review\n\nACCEPT."},
		},
	}
}

// --- 1. Interface compliance ---

func TestDebatesScreenImplementsComponent(t *testing.T) {
	store := testStoreWithDebates(nil)
	ds := NewDebatesScreen(store, nil)
	var _ tui.Component = ds
}

func TestDebatesScreenImplementsKeyListener(t *testing.T) {
	store := testStoreWithDebates(nil)
	ds := NewDebatesScreen(store, nil)
	var _ tui.KeyListener = ds
}

// --- 2. List mode: render with data ---

func TestDebatesScreenRenderListWithData(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)

	el := ds.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}

	// Should have a header row + 4 data rows = at least 5 rows in the table.
	totalRows := countRowElements(el)
	if totalRows < 5 {
		t.Fatalf("expected at least 5 rows (1 header + 4 data), found %d", totalRows)
	}
}

// --- 3. List mode: render empty ---

func TestDebatesScreenRenderEmpty(t *testing.T) {
	store := testStoreWithDebates(nil)
	ds := NewDebatesScreen(store, nil)

	el := ds.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}
	text := collectAllText(el)
	if !strings.Contains(strings.ToLower(text), "no debates") {
		t.Fatalf("expected empty state text containing 'no debates', got: %q", text)
	}
}

// --- 4. List mode: empty state with filter shows filter text ---

func TestDebatesScreenRenderEmptyWithFilter(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)
	ds.listVM.SetFilter("nonexistent")

	el := ds.Render(nil)
	text := collectAllText(el)
	if !strings.Contains(text, "nonexistent") {
		t.Fatalf("expected empty state to mention filter text, got: %q", text)
	}
}

// --- 5. List mode: j/k navigation ---

func TestDebatesScreenListSelectionJ(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)

	km := ds.KeyMap()
	handler := findRuneHandler(km, 'j')
	if handler == nil {
		t.Fatal("no handler found for key 'j'")
	}

	if ds.listVM.SelectedIndex() != 0 {
		t.Fatalf("expected initial selection 0, got %d", ds.listVM.SelectedIndex())
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})
	if ds.listVM.SelectedIndex() != 1 {
		t.Fatalf("expected selection 1 after 'j', got %d", ds.listVM.SelectedIndex())
	}
}

func TestDebatesScreenListSelectionK(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)

	km := ds.KeyMap()
	jHandler := findRuneHandler(km, 'j')
	jHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})

	kHandler := findRuneHandler(km, 'k')
	if kHandler == nil {
		t.Fatal("no handler found for key 'k'")
	}
	kHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'k'})

	if ds.listVM.SelectedIndex() != 0 {
		t.Fatalf("expected selection 0 after j then k, got %d", ds.listVM.SelectedIndex())
	}
}

// --- 6. List mode: G / gg navigation ---

func TestDebatesScreenListSelectionG(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)

	km := ds.KeyMap()
	handler := findRuneHandler(km, 'G')
	if handler == nil {
		t.Fatal("no handler found for key 'G'")
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'G'})
	expected := len(sampleDebates()) - 1
	if ds.listVM.SelectedIndex() != expected {
		t.Fatalf("expected selection %d after 'G', got %d", expected, ds.listVM.SelectedIndex())
	}
}

func TestDebatesScreenListSelectionGG(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)

	// Move to last.
	km := ds.KeyMap()
	gBigHandler := findRuneHandler(km, 'G')
	gBigHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'G'})

	// Press 'g' twice.
	ggHandler := findRuneHandler(km, 'g')
	if ggHandler == nil {
		t.Fatal("no handler found for key 'g'")
	}
	ggHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})
	ggHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})

	if ds.listVM.SelectedIndex() != 0 {
		t.Fatalf("expected selection 0 after 'gg', got %d", ds.listVM.SelectedIndex())
	}
}

// --- 7. List mode: filter mode toggle ---

func TestDebatesScreenFilterModeToggle(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)

	km := ds.KeyMap()
	slashHandler := findRuneHandler(km, '/')
	if slashHandler == nil {
		t.Fatal("no handler found for key '/'")
	}
	slashHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: '/'})

	if !ds.filterMode {
		t.Fatal("expected filterMode=true after pressing '/'")
	}

	// KeyMap should now have AnyRune with OnFocused.
	km = ds.KeyMap()
	binding := findAnyRuneBinding(km)
	if binding == nil {
		t.Fatal("expected AnyRune binding in filter mode")
	}
	if !hasFocusRequired(binding) {
		t.Fatal("expected AnyRune binding to have FocusRequired (OnFocused)")
	}

	// Exit with Esc.
	escHandler := findKeyHandler(km, tui.KeyEscape, tui.ModNone)
	if escHandler == nil {
		t.Fatal("no handler found for Escape in filter mode")
	}
	escHandler(tui.KeyEvent{Key: tui.KeyEscape})

	if ds.filterMode {
		t.Fatal("expected filterMode=false after pressing Esc")
	}
}

// --- 8. List mode: status filter cycling ---

func TestDebatesScreenStatusFilterCycle(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)

	km := ds.KeyMap()
	sHandler := findRuneHandler(km, 's')
	if sHandler == nil {
		t.Fatal("no handler found for key 's'")
	}

	// Initial: no status filter.
	if ds.listVM.StatusFilter() != "" {
		t.Fatalf("expected empty initial status filter, got %q", ds.listVM.StatusFilter())
	}

	// First press: should cycle to first status.
	sHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 's'})
	if ds.listVM.StatusFilter() == "" {
		t.Fatal("expected non-empty status filter after pressing 's'")
	}
}

// --- 9. Enter opens detail mode ---

func TestDebatesScreenEnterOpensDetail(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	if enterHandler == nil {
		t.Fatal("no handler found for Enter key")
	}

	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	if ds.mode != modeDetail {
		t.Fatalf("expected mode=modeDetail after Enter, got %d", ds.mode)
	}
	if ds.detailVM == nil {
		t.Fatal("expected detailVM to be non-nil in detail mode")
	}
}

// --- 10. Esc returns to list from detail ---

func TestDebatesScreenEscReturnsToList(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Enter detail mode.
	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	if ds.mode != modeDetail {
		t.Fatalf("precondition: expected detail mode, got %d", ds.mode)
	}

	// Press Esc.
	km = ds.KeyMap()
	escHandler := findKeyHandler(km, tui.KeyEscape, tui.ModNone)
	if escHandler == nil {
		t.Fatal("no Esc handler in detail mode")
	}
	escHandler(tui.KeyEvent{Key: tui.KeyEscape})

	if ds.mode != modeList {
		t.Fatalf("expected mode=modeList after Esc, got %d", ds.mode)
	}
}

// --- 11. Detail mode: scroll to top/bottom ---

func TestDebatesScreenDetailScrollTopBottom(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Enter detail mode.
	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	// Scroll down first.
	km = ds.KeyMap()
	jHandler := findRuneHandler(km, 'j')
	if jHandler == nil {
		t.Fatal("no 'j' handler in detail mode")
	}
	jHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})
	jHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})

	if ds.detailVM.ContentScrollOffset() == 0 {
		t.Fatal("expected non-zero scroll after j presses")
	}

	// Press 'G' for scroll to bottom — offset should be very large.
	gHandler := findRuneHandler(km, 'G')
	if gHandler == nil {
		t.Fatal("no 'G' handler in detail mode")
	}
	gHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'G'})
	if ds.detailVM.ContentScrollOffset() < 100 {
		t.Fatal("expected large scroll offset after 'G'")
	}

	// Press 'gg' for scroll to top.
	ggHandler := findRuneHandler(km, 'g')
	if ggHandler == nil {
		t.Fatal("no 'g' handler in detail mode")
	}
	ggHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})
	ggHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})
	if ds.detailVM.ContentScrollOffset() != 0 {
		t.Fatalf("expected scroll 0 after 'gg', got %d", ds.detailVM.ContentScrollOffset())
	}
}

// --- 12. Detail mode: back with backspace ---

func TestDebatesScreenDetailBackspace(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Enter detail.
	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	if ds.mode != modeDetail {
		t.Fatal("expected detail mode after enter")
	}

	// Press Backspace to go back.
	km = ds.KeyMap()
	bsHandler := findKeyHandler(km, tui.KeyBackspace, tui.ModNone)
	if bsHandler == nil {
		t.Fatal("no Backspace handler in detail mode")
	}
	bsHandler(tui.KeyEvent{Key: tui.KeyBackspace})

	if ds.mode != modeList {
		t.Fatal("expected list mode after Backspace")
	}
}

// --- 13. Detail mode: content scrolling ---

func TestDebatesScreenDetailContentScroll(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Enter detail.
	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	if ds.detailVM.ContentScrollOffset() != 0 {
		t.Fatalf("expected initial scroll 0, got %d", ds.detailVM.ContentScrollOffset())
	}

	// Press 'j' for scroll down in detail mode.
	km = ds.KeyMap()
	jHandler := findRuneHandler(km, 'j')
	if jHandler == nil {
		t.Fatal("no 'j' handler in detail mode")
	}
	jHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})

	if ds.detailVM.ContentScrollOffset() != 1 {
		t.Fatalf("expected scroll offset 1 after 'j', got %d", ds.detailVM.ContentScrollOffset())
	}
}

// --- 14. Detail mode: page navigation ---

func TestDebatesScreenDetailPageNavigation(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Enter detail.
	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	ds.detailVM.SetViewportHeight(5)

	km = ds.KeyMap()
	dHandler := findRuneHandler(km, 'd')
	if dHandler == nil {
		t.Fatal("no 'd' handler in detail mode")
	}
	dHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'd'})

	if ds.detailVM.ContentScrollOffset() != 5 {
		t.Fatalf("expected scroll offset 5 after page down, got %d", ds.detailVM.ContentScrollOffset())
	}

	uHandler := findRuneHandler(km, 'u')
	if uHandler == nil {
		t.Fatal("no 'u' handler in detail mode")
	}
	uHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'u'})

	if ds.detailVM.ContentScrollOffset() != 0 {
		t.Fatalf("expected scroll offset 0 after page up, got %d", ds.detailVM.ContentScrollOffset())
	}
}

// --- 15. Detail mode render shows thread with all rounds ---

func TestDebatesScreenDetailRender(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Enter detail.
	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	el := ds.Render(nil)
	if el == nil {
		t.Fatal("Render in detail mode returned nil")
	}

	text := collectAllText(el)
	// Should contain the debate ID.
	if !strings.Contains(text, "tui-ux-m12-review") {
		t.Fatalf("expected debate ID in detail view, got: %q", text)
	}
	// Should contain rounds count.
	if !strings.Contains(text, "4 rounds") {
		t.Fatalf("expected rounds count in detail view, got: %q", text)
	}
	// Should contain content from multiple rounds in the thread.
	if !strings.Contains(text, "Round 1") {
		t.Fatalf("expected round 1 content in thread view, got: %q", text)
	}
	if !strings.Contains(text, "Round 2") {
		t.Fatalf("expected round 2 content in thread view, got: %q", text)
	}
}

// --- 16. Detail mode render with no detail data shows loading ---

func TestDebatesScreenDetailRenderLoading(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	// Don't set debate detail — it should show loading state.
	ds := NewDebatesScreen(store, nil)

	// Force into detail mode.
	ds.mode = modeDetail

	el := ds.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}
	text := collectAllText(el)
	if !strings.Contains(strings.ToLower(text), "loading") {
		t.Fatalf("expected loading state text, got: %q", text)
	}
}

// --- 17. Nil store does not panic ---

func TestDebatesScreenNilStore(t *testing.T) {
	ds := NewDebatesScreen(nil, nil)
	el := ds.Render(nil)
	if el == nil {
		t.Fatal("Render with nil store returned nil")
	}
}

// --- 18. Responsive columns for debate list ---

func TestDebateColumnCountForWidth(t *testing.T) {
	tests := []struct {
		width    int
		expected int
	}{
		{120, 5},
		{80, 4},
		{60, 3},
	}

	for _, tt := range tests {
		got := debateColumnCountForWidth(tt.width)
		if got != tt.expected {
			t.Errorf("debateColumnCountForWidth(%d) = %d, want %d", tt.width, got, tt.expected)
		}
	}
}

// --- 19. List mode keybindings don't include detail keys ---

func TestDebatesScreenListModeNoDetailKeys(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)

	km := ds.KeyMap()
	// 'n' and 'N' should NOT be in list mode.
	if findRuneHandler(km, 'n') != nil {
		t.Fatal("'n' should not be bound in list mode")
	}
	if findRuneHandler(km, 'N') != nil {
		t.Fatal("'N' should not be bound in list mode")
	}
	if findRuneHandler(km, 'F') != nil {
		t.Fatal("'F' should not be bound in list mode")
	}
}

// --- 20. Detail mode keybindings don't include list keys ---

func TestDebatesScreenDetailModeNoListKeys(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Enter detail.
	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	km = ds.KeyMap()
	// 's' (status cycle) should NOT be in detail mode.
	if findRuneHandler(km, 's') != nil {
		t.Fatal("'s' should not be bound in detail mode")
	}
}

// --- 21. List preserves selection after returning from detail ---

func TestDebatesScreenListSelectionPreservedAfterDetail(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Move to index 2.
	km := ds.KeyMap()
	jHandler := findRuneHandler(km, 'j')
	jHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})
	jHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})
	if ds.listVM.SelectedIndex() != 2 {
		t.Fatalf("precondition: expected selection 2, got %d", ds.listVM.SelectedIndex())
	}

	// Enter detail.
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	// Return to list.
	km = ds.KeyMap()
	escHandler := findKeyHandler(km, tui.KeyEscape, tui.ModNone)
	escHandler(tui.KeyEvent{Key: tui.KeyEscape})

	// Selection should be preserved.
	if ds.listVM.SelectedIndex() != 2 {
		t.Fatalf("expected selection preserved at 2, got %d", ds.listVM.SelectedIndex())
	}
}

// --- 22. Status filter badge shown in render ---

func TestDebatesScreenStatusFilterBadgeInRender(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)
	ds.listVM.SetStatusFilter("consensus")

	el := ds.Render(nil)
	text := collectAllText(el)
	if !strings.Contains(strings.ToLower(text), "consensus") {
		t.Fatalf("expected status filter badge containing 'consensus', got: %q", text)
	}
}

// ── HARDEN: Edge cases ──────────────────────────────────────────────────

// --- 23. Detail G with empty rounds doesn't panic ---

func TestDebatesScreenDetailGEmptyRounds(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	emptyDetail := &client.DebateWithRounds{
		DebateMeta: client.DebateMeta{
			ID:     "tui-ux-m12-review-20260315",
			Pair:   "tui-ux",
			Status: "consensus",
		},
		Rounds: nil, // no rounds
	}
	store.SetDebateDetail(emptyDetail)
	ds := NewDebatesScreen(store, nil)

	// Enter detail.
	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	// Press G — should not panic or infinite loop.
	km = ds.KeyMap()
	gHandler := findRuneHandler(km, 'G')
	if gHandler != nil {
		gHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'G'})
	}
}

// --- 24. lastKey resets on mode transitions ---

func TestDebatesScreenLastKeyResetsOnModeSwitch(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Press 'g' once in list mode (starts gg sequence).
	km := ds.KeyMap()
	gHandler := findRuneHandler(km, 'g')
	gHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})
	if ds.lastKey != 'g' {
		t.Fatalf("expected lastKey='g', got %d", ds.lastKey)
	}

	// Enter detail — lastKey should reset.
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})
	if ds.lastKey != 0 {
		t.Fatalf("expected lastKey=0 after entering detail, got %d", ds.lastKey)
	}

	// Press 'g' in detail, then Esc — lastKey should reset again.
	km = ds.KeyMap()
	gHandler = findRuneHandler(km, 'g')
	gHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})
	if ds.lastKey != 'g' {
		t.Fatalf("expected lastKey='g' in detail, got %d", ds.lastKey)
	}

	escHandler := findKeyHandler(km, tui.KeyEscape, tui.ModNone)
	escHandler(tui.KeyEvent{Key: tui.KeyEscape})
	if ds.lastKey != 0 {
		t.Fatalf("expected lastKey=0 after Esc to list, got %d", ds.lastKey)
	}
}

// --- 25. Enter on empty list doesn't switch to detail ---

func TestDebatesScreenEnterOnEmptyList(t *testing.T) {
	store := testStoreWithDebates(nil)
	ds := NewDebatesScreen(store, nil)

	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	if enterHandler == nil {
		t.Fatal("no Enter handler")
	}
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	if ds.mode != modeList {
		t.Fatalf("expected to stay in list mode on empty list, got %d", ds.mode)
	}
}

// --- 26. Store.DebateDetail with wrong ID returns nil ---

func TestStoreDebateDetailWrongID(t *testing.T) {
	store := state.NewStore()
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)

	got := store.DebateDetail("wrong-id")
	if got != nil {
		t.Fatal("expected nil for wrong ID")
	}

	got = store.DebateDetail(detail.ID)
	if got == nil {
		t.Fatal("expected non-nil for correct ID")
	}
}

// --- 27. Store.DebateDetail nil set ---

func TestStoreDebateDetailNilSet(t *testing.T) {
	store := state.NewStore()
	store.SetDebateDetail(nil)
	got := store.DebateDetail("")
	if got != nil {
		t.Fatal("expected nil after setting nil")
	}
}

// --- 28. Detail render with empty content ---

func TestDebatesScreenDetailRenderEmptyContent(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	emptyRounds := &client.DebateWithRounds{
		DebateMeta: client.DebateMeta{
			ID:     "tui-ux-m12-review-20260315",
			Pair:   "tui-ux",
			Status: "consensus",
		},
		Rounds: []client.Round{
			{Number: 1, Role: "generative", Content: ""},
		},
	}
	store.SetDebateDetail(emptyRounds)
	ds := NewDebatesScreen(store, nil)

	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	el := ds.Render(nil)
	if el == nil {
		t.Fatal("Render with empty content returned nil")
	}
}

// --- 29. Detail mode: n key advances to next round ---

func TestDebatesScreenDetailNNextRound(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Enter detail mode.
	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	if ds.detailVM == nil {
		t.Fatal("expected detailVM to be non-nil")
	}
	if ds.detailVM.CurrentRound() != 0 {
		t.Fatalf("expected initial round 0, got %d", ds.detailVM.CurrentRound())
	}

	km = ds.KeyMap()
	nHandler := findRuneHandler(km, 'n')
	if nHandler == nil {
		t.Fatal("no handler found for key 'n' in detail mode")
	}
	nHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'n'})

	if ds.detailVM.CurrentRound() != 1 {
		t.Fatalf("expected round 1 after 'n', got %d", ds.detailVM.CurrentRound())
	}
}

// --- 30. Detail mode: N key goes to previous round ---

func TestDebatesScreenDetailNPrevRound(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Enter detail.
	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	// Advance to round 1.
	km = ds.KeyMap()
	nHandler := findRuneHandler(km, 'n')
	nHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'n'})
	if ds.detailVM.CurrentRound() != 1 {
		t.Fatalf("precondition: expected round 1, got %d", ds.detailVM.CurrentRound())
	}

	// Go back with N.
	bigNHandler := findRuneHandler(km, 'N')
	if bigNHandler == nil {
		t.Fatal("no handler found for key 'N' in detail mode")
	}
	bigNHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'N'})

	if ds.detailVM.CurrentRound() != 0 {
		t.Fatalf("expected round 0 after 'N', got %d", ds.detailVM.CurrentRound())
	}
}

// --- 31. Detail mode: F key toggles follow mode ---

func TestDebatesScreenDetailFToggleFollow(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	detail := sampleDebateWithRounds()
	store.SetDebateDetail(detail)
	ds := NewDebatesScreen(store, nil)

	// Enter detail.
	km := ds.KeyMap()
	enterHandler := findKeyHandler(km, tui.KeyEnter, tui.ModNone)
	enterHandler(tui.KeyEvent{Key: tui.KeyEnter})

	if ds.detailVM.IsFollowing() {
		t.Fatal("expected follow mode off initially")
	}

	km = ds.KeyMap()
	fHandler := findRuneHandler(km, 'F')
	if fHandler == nil {
		t.Fatal("no handler found for key 'F' in detail mode")
	}
	fHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'F'})

	if !ds.detailVM.IsFollowing() {
		t.Fatal("expected follow mode on after 'F'")
	}

	// Toggle again.
	fHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'F'})
	if ds.detailVM.IsFollowing() {
		t.Fatal("expected follow mode off after second 'F'")
	}
}

// --- 32. Detail mode: n/N/F with nil detailVM doesn't panic ---

func TestDebatesScreenDetailKeysNilVM(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)

	// Force detail mode without a detailVM.
	ds.mode = modeDetail

	km := ds.KeyMap()
	nHandler := findRuneHandler(km, 'n')
	if nHandler != nil {
		nHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'n'}) // must not panic
	}
	bigNHandler := findRuneHandler(km, 'N')
	if bigNHandler != nil {
		bigNHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'N'}) // must not panic
	}
	fHandler := findRuneHandler(km, 'F')
	if fHandler != nil {
		fHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'F'}) // must not panic
	}
}

// --- 33. SelectFirst/SelectLast on DebatesViewModel ---

func TestDebatesViewModelSelectFirstLast(t *testing.T) {
	store := testStoreWithDebates(sampleDebates())
	ds := NewDebatesScreen(store, nil)

	// Move to index 2.
	ds.listVM.SelectNext()
	ds.listVM.SelectNext()
	if ds.listVM.SelectedIndex() != 2 {
		t.Fatalf("expected 2, got %d", ds.listVM.SelectedIndex())
	}

	ds.listVM.SelectFirst()
	if ds.listVM.SelectedIndex() != 0 {
		t.Fatalf("expected 0 after SelectFirst, got %d", ds.listVM.SelectedIndex())
	}

	ds.listVM.SelectLast()
	expected := len(sampleDebates()) - 1
	if ds.listVM.SelectedIndex() != expected {
		t.Fatalf("expected %d after SelectLast, got %d", expected, ds.listVM.SelectedIndex())
	}
}
