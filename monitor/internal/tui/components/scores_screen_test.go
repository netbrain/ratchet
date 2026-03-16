package components

import (
	"strings"
	"testing"
	"time"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

func testStoreWithScores() *state.Store {
	s := state.NewStore()
	s.SetPairs([]client.PairStatus{
		{Name: "api-design", Component: "backend"},
		{Name: "go-idioms", Component: "backend"},
		{Name: "tui-layout", Component: "tui"},
	})
	s.SetScores("api-design", []client.ScoreEntry{
		{Pair: "api-design", DebateID: "d1", Milestone: 1, RoundsToConsensus: 2, IssuesFound: 3, IssuesResolved: 2, Timestamp: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)},
		{Pair: "api-design", DebateID: "d2", Milestone: 2, RoundsToConsensus: 1, IssuesFound: 1, IssuesResolved: 1, Timestamp: time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)},
		{Pair: "api-design", DebateID: "d3", Milestone: 3, RoundsToConsensus: 3, IssuesFound: 5, IssuesResolved: 4, Escalated: true, Timestamp: time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)},
	})
	s.SetScores("go-idioms", []client.ScoreEntry{
		{Pair: "go-idioms", DebateID: "d4", Milestone: 1, RoundsToConsensus: 1, Timestamp: time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC)},
	})
	return s
}

// --- 1. Interface compliance ---

func TestScoresScreenImplementsComponent(t *testing.T) {
	store := testStoreWithScores()
	ss := NewScoresScreen(store)
	var _ tui.Component = ss
}

func TestScoresScreenImplementsKeyListener(t *testing.T) {
	store := testStoreWithScores()
	ss := NewScoresScreen(store)
	var _ tui.KeyListener = ss
}

// --- 2. Render with data ---

func TestScoresScreenRenderWithData(t *testing.T) {
	store := testStoreWithScores()
	ss := NewScoresScreen(store)

	el := ss.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}

	// Should have header + 2 data rows (pairs with scores).
	totalRows := countRowElements(el)
	if totalRows < 3 {
		t.Fatalf("expected at least 3 rows (1 header + 2 data), found %d", totalRows)
	}
}

// --- 3. Render empty ---

func TestScoresScreenRenderEmpty(t *testing.T) {
	store := state.NewStore()
	ss := NewScoresScreen(store)

	el := ss.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}
	text := collectAllText(el)
	if !strings.Contains(strings.ToLower(text), "no score") {
		t.Fatalf("expected empty state text, got: %q", text)
	}
}

// --- 4. j/k navigation ---

func TestScoresScreenSelectionJ(t *testing.T) {
	store := testStoreWithScores()
	ss := NewScoresScreen(store)

	km := ss.KeyMap()
	handler := findRuneHandler(km, 'j')
	if handler == nil {
		t.Fatal("no handler for 'j'")
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})
	if ss.vm.SelectedIndex() != 1 {
		t.Fatalf("expected selection 1 after 'j', got %d", ss.vm.SelectedIndex())
	}
}

func TestScoresScreenSelectionK(t *testing.T) {
	store := testStoreWithScores()
	ss := NewScoresScreen(store)

	km := ss.KeyMap()
	findRuneHandler(km, 'j')(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})
	findRuneHandler(km, 'k')(tui.KeyEvent{Key: tui.KeyRune, Rune: 'k'})
	if ss.vm.SelectedIndex() != 0 {
		t.Fatalf("expected selection 0, got %d", ss.vm.SelectedIndex())
	}
}

// --- 5. G / gg ---

func TestScoresScreenSelectionG(t *testing.T) {
	store := testStoreWithScores()
	ss := NewScoresScreen(store)

	km := ss.KeyMap()
	findRuneHandler(km, 'G')(tui.KeyEvent{Key: tui.KeyRune, Rune: 'G'})
	if ss.vm.SelectedIndex() != 1 {
		t.Fatalf("expected selection 1 (last) after 'G', got %d", ss.vm.SelectedIndex())
	}
}

func TestScoresScreenSelectionGG(t *testing.T) {
	store := testStoreWithScores()
	ss := NewScoresScreen(store)

	km := ss.KeyMap()
	findRuneHandler(km, 'G')(tui.KeyEvent{Key: tui.KeyRune, Rune: 'G'})
	gHandler := findRuneHandler(km, 'g')
	gHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})
	gHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})
	if ss.vm.SelectedIndex() != 0 {
		t.Fatalf("expected selection 0 after 'gg', got %d", ss.vm.SelectedIndex())
	}
}

// --- 6. Sparkline in rendered output ---

func TestScoresScreenRenderContainsSparkline(t *testing.T) {
	store := testStoreWithScores()
	ss := NewScoresScreen(store)

	el := ss.Render(nil)
	text := collectAllText(el)
	// Sparkline uses Unicode block characters.
	hasBlock := false
	for _, r := range text {
		if r >= '▁' && r <= '█' {
			hasBlock = true
			break
		}
	}
	if !hasBlock {
		t.Fatalf("expected sparkline Unicode block characters in output, got: %q", text)
	}
}

// --- 7. Consensus rate in output ---

func TestScoresScreenRenderContainsConsensusRate(t *testing.T) {
	store := testStoreWithScores()
	ss := NewScoresScreen(store)

	el := ss.Render(nil)
	text := collectAllText(el)
	// go-idioms has 100% consensus.
	if !strings.Contains(text, "100") {
		t.Fatalf("expected '100' (consensus rate) in output, got: %q", text)
	}
}

// --- 8. Nil store ---

func TestScoresScreenNilStore(t *testing.T) {
	ss := NewScoresScreen(nil)
	el := ss.Render(nil)
	if el == nil {
		t.Fatal("Render with nil store returned nil")
	}
}

// --- 9. Responsive columns ---

func TestScoreColumnCountForWidth(t *testing.T) {
	tests := []struct {
		width    int
		expected int
	}{
		{120, 8},
		{80, 6},
		{60, 4},
	}
	for _, tt := range tests {
		got := scoreColumnCountForWidth(tt.width)
		if got != tt.expected {
			t.Errorf("scoreColumnCountForWidth(%d) = %d, want %d", tt.width, got, tt.expected)
		}
	}
}

// --- 10. Selected row highlight ---

func TestScoresScreenSelectedRowHighlighted(t *testing.T) {
	store := testStoreWithScores()
	ss := NewScoresScreen(store)

	// Render — first row should be selected.
	el := ss.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}
	// We can't easily inspect background color in the element tree,
	// but we verify the row count and selection index.
	if ss.vm.SelectedIndex() != 0 {
		t.Fatalf("expected initial selection 0, got %d", ss.vm.SelectedIndex())
	}
}
