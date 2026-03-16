package components

import (
	"strings"
	"testing"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

func testStoreWithEpic() *state.Store {
	s := state.NewStore()
	s.SetPlan(client.Plan{
		Epic: client.EpicConfig{
			Name:        "ratchet-monitor",
			Description: "Real-time observability dashboard",
			Milestones: []client.Milestone{
				{ID: 1, Name: "Spike & Contract", Status: "done", PhaseStatus: map[string]string{"plan": "done", "test": "done", "build": "done", "review": "done", "harden": "done"}, DoneWhen: "working proof"},
				{ID: 2, Name: "Solid Backend", Status: "done", PhaseStatus: map[string]string{"plan": "done", "test": "done", "build": "done", "review": "done", "harden": "done"}, DoneWhen: "robust backend"},
				{ID: 3, Name: "Dashboard", Status: "in_progress", PhaseStatus: map[string]string{"plan": "done", "test": "done", "build": "in_progress", "review": "pending", "harden": "pending"}, DoneWhen: "live dashboard"},
				{ID: 4, Name: "Scores", Status: "pending", PhaseStatus: map[string]string{"plan": "pending", "test": "pending", "build": "pending", "review": "pending", "harden": "pending"}, DoneWhen: "score charts"},
			},
			CurrentFocus: &client.CurrentFocus{
				MilestoneID: 3,
				Phase:       "build",
				Started:     "2026-03-15T10:00:00Z",
			},
		},
	})
	return s
}

// --- 1. Interface compliance ---

func TestEpicScreenImplementsComponent(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)
	var _ tui.Component = es
}

func TestEpicScreenImplementsKeyListener(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)
	var _ tui.KeyListener = es
}

// --- 2. Render with data ---

func TestEpicScreenRenderWithData(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}

	text := collectAllText(el)
	// Should contain epic name
	if !strings.Contains(text, "ratchet-monitor") {
		t.Fatalf("expected epic name in output, got: %q", text)
	}
}

// --- 3. Render empty ---

func TestEpicScreenRenderEmpty(t *testing.T) {
	store := state.NewStore()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}
	text := collectAllText(el)
	if !strings.Contains(strings.ToLower(text), "no epic") {
		t.Fatalf("expected empty state text, got: %q", text)
	}
}

// --- 4. Progress bar in output ---

func TestEpicScreenRenderContainsProgressBar(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// Should contain progress fraction (2/4 = 50%)
	if !strings.Contains(text, "2/4") {
		t.Fatalf("expected progress '2/4' in output, got: %q", text)
	}
	if !strings.Contains(text, "50%") {
		t.Fatalf("expected '50%%' in output, got: %q", text)
	}
}

// --- 5. Phase status checkmarks ---

func TestEpicScreenRenderContainsPhaseCheckmarks(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// Done phases should show ✓
	if !strings.Contains(text, "✓") {
		t.Fatalf("expected checkmark ✓ for done phases, got: %q", text)
	}
	// In-progress phases should show ●
	if !strings.Contains(text, "●") {
		t.Fatalf("expected ● for in_progress phases, got: %q", text)
	}
	// Pending phases should show ○
	if !strings.Contains(text, "○") {
		t.Fatalf("expected ○ for pending phases, got: %q", text)
	}
}

// --- 6. Milestone names in output ---

func TestEpicScreenRenderContainsMilestoneNames(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	for _, name := range []string{"Spike & Contract", "Solid Backend", "Dashboard", "Scores"} {
		if !strings.Contains(text, name) {
			t.Errorf("expected milestone %q in output", name)
		}
	}
}

// --- 7. Current focus indicator ---

func TestEpicScreenRenderContainsCurrentFocus(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// Should mention the current focus phase
	if !strings.Contains(text, "build") {
		t.Fatalf("expected current focus 'build' in output, got: %q", text)
	}
}

// --- 8. j/k navigation ---

func TestEpicScreenSelectionJ(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	km := es.KeyMap()
	handler := findRuneHandler(km, 'j')
	if handler == nil {
		t.Fatal("no handler for 'j'")
	}

	handler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})
	if es.vm.SelectedIndex() != 1 {
		t.Fatalf("expected selection 1 after 'j', got %d", es.vm.SelectedIndex())
	}
}

func TestEpicScreenSelectionK(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	km := es.KeyMap()
	findRuneHandler(km, 'j')(tui.KeyEvent{Key: tui.KeyRune, Rune: 'j'})
	findRuneHandler(km, 'k')(tui.KeyEvent{Key: tui.KeyRune, Rune: 'k'})
	if es.vm.SelectedIndex() != 0 {
		t.Fatalf("expected selection 0, got %d", es.vm.SelectedIndex())
	}
}

// --- 9. G / gg ---

func TestEpicScreenSelectionG(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	km := es.KeyMap()
	findRuneHandler(km, 'G')(tui.KeyEvent{Key: tui.KeyRune, Rune: 'G'})
	if es.vm.SelectedIndex() != 3 {
		t.Fatalf("expected selection 3 (last) after 'G', got %d", es.vm.SelectedIndex())
	}
}

func TestEpicScreenSelectionGG(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	km := es.KeyMap()
	findRuneHandler(km, 'G')(tui.KeyEvent{Key: tui.KeyRune, Rune: 'G'})
	gHandler := findRuneHandler(km, 'g')
	gHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})
	gHandler(tui.KeyEvent{Key: tui.KeyRune, Rune: 'g'})
	if es.vm.SelectedIndex() != 0 {
		t.Fatalf("expected selection 0 after 'gg', got %d", es.vm.SelectedIndex())
	}
}

// --- 10. Nil store ---

func TestEpicScreenNilStore(t *testing.T) {
	es := NewEpicScreen(nil)
	el := es.Render(nil)
	if el == nil {
		t.Fatal("Render with nil store returned nil")
	}
}

// --- 11. Responsive columns ---

func TestEpicColumnCountForWidth(t *testing.T) {
	tests := []struct {
		width    int
		expected int
	}{
		{120, 8},
		{80, 6},
		{60, 4},
	}
	for _, tt := range tests {
		got := epicColumnCountForWidth(tt.width)
		if got != tt.expected {
			t.Errorf("epicColumnCountForWidth(%d) = %d, want %d", tt.width, got, tt.expected)
		}
	}
}

// --- 12. Selected milestone highlighted ---

func TestEpicScreenSelectedMilestoneHighlighted(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	if el == nil {
		t.Fatal("Render returned nil")
	}
	if es.vm.SelectedIndex() != 0 {
		t.Fatalf("expected initial selection 0, got %d", es.vm.SelectedIndex())
	}
}
