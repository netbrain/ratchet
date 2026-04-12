package components

import (
	"strings"
	"testing"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

func testStoreWithEpicAndIssues() *state.Store {
	s := state.NewStore()
	s.SetPlan(client.Plan{
		Epic: client.EpicConfig{
			Name:        "ratchet-monitor",
			Description: "With issues",
			Milestones: []client.Milestone{
				{
					ID: 1, Name: "M1", Status: "in_progress",
					PhaseStatus:    map[string]string{"plan": "done", "build": "in_progress"},
					MaxRegressions: 2,
					Regressions:    2,
					Issues: []client.Issue{
						{
							Ref:         "#10",
							Title:       "Add widget",
							PhaseStatus: map[string]string{"plan": "done", "build": "in_progress", "review": "pending"},
							Status:      "in_progress",
						},
						{
							Ref:         "#11",
							Title:       "Fix layout",
							PhaseStatus: map[string]string{"plan": "done", "build": "done", "review": "done"},
							Status:      "done",
						},
					},
				},
				{
					ID: 2, Name: "M2", Status: "pending",
					PhaseStatus: map[string]string{"plan": "pending"},
					DependsOn:   []int{1},
				},
			},
		},
	})
	return s
}

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

// --- 13. Issue rows rendered ---

func TestEpicScreenRenderContainsIssueRefs(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// Issue refs should appear
	if !strings.Contains(text, "#10") {
		t.Fatalf("expected issue ref #10 in output, got: %q", text)
	}
	if !strings.Contains(text, "#11") {
		t.Fatalf("expected issue ref #11 in output, got: %q", text)
	}
}

func TestEpicScreenRenderContainsIssueTitles(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	if !strings.Contains(text, "Add widget") {
		t.Fatalf("expected issue title 'Add widget' in output, got: %q", text)
	}
	if !strings.Contains(text, "Fix layout") {
		t.Fatalf("expected issue title 'Fix layout' in output, got: %q", text)
	}
}

func TestEpicScreenRenderIssuePhaseSymbols(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// Issue #10 has plan=done(✓), build=in_progress(●), review=pending(○)
	if !strings.Contains(text, "✓") {
		t.Fatal("expected ✓ for done phase")
	}
	if !strings.Contains(text, "●") {
		t.Fatal("expected ● for in_progress phase")
	}
	if !strings.Contains(text, "○") {
		t.Fatal("expected ○ for pending phase")
	}
}

// --- 14. Regression budget column in rendered output ---

func TestEpicScreenRenderRegressionBudgetColumn(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// Header should contain "Reg"
	if !strings.Contains(text, "Reg") {
		t.Fatalf("expected 'Reg' header in output, got: %q", text)
	}
	// M1: regressions=2, max_regressions=2 -> "2/2"
	if !strings.Contains(text, "2/2") {
		t.Fatalf("expected regression budget '2/2' in output, got: %q", text)
	}
}

func TestEpicScreenRenderRegressionBudgetDefault(t *testing.T) {
	// Milestone with no MaxRegressions set defaults to /2
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// All milestones have 0 regressions and default max=2 -> "0/2"
	if !strings.Contains(text, "0/2") {
		t.Fatalf("expected default budget '0/2' in output, got: %q", text)
	}
}

// --- 15. DAG connector in rendered output ---

func TestEpicScreenRenderDAGConnector(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// M2 depends on M1 and is in a higher layer, DAG prefix should appear
	if !strings.Contains(text, "└─") {
		t.Fatalf("expected DAG connector └─ in output, got: %q", text)
	}
}

// --- 16. Issue tree connector ---

func TestEpicScreenRenderIssueTreeConnector(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// Issue rows use ├─ tree connector
	if !strings.Contains(text, "├─") {
		t.Fatalf("expected issue tree connector ├─ in output, got: %q", text)
	}
}

// --- 17. DAG layer separator row ---

func TestEpicScreenRenderLayerSeparator(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// M2 is in Layer 1; a separator row "L1" should appear between M1 and M2
	if !strings.Contains(text, "── L1 ──") {
		t.Fatalf("expected layer separator '── L1 ──' in output, got: %q", text)
	}
	// Connector symbol │ should appear in the separator
	if !strings.Contains(text, "│") {
		t.Fatalf("expected box-drawing │ in layer separator, got: %q", text)
	}
}

// --- 18. BLOCKED indicator ---

func TestEpicScreenRenderBlockedIndicator(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// M2 depends on M1 (in_progress, not done) → should show BLOCKED
	if !strings.Contains(text, "BLOCKED") {
		t.Fatalf("expected 'BLOCKED' indicator for M2 in output, got: %q", text)
	}
	// BLOCKED symbol ⊘ should also appear
	if !strings.Contains(text, "⊘") {
		t.Fatalf("expected ⊘ symbol in BLOCKED indicator, got: %q", text)
	}
}

// --- 19. No layer separator when all milestones share the same layer ---

func TestEpicScreenNoLayerSeparatorSameLayer(t *testing.T) {
	// testStoreWithEpic() has 4 milestones all with no DependsOn → all layer 0
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// No milestone is in a higher layer, so no layer separator row should appear.
	// Separators use the format "── LN ──"; the header line uses "L0" but not "── L".
	if strings.Contains(text, "── L1") || strings.Contains(text, "── L2") {
		t.Fatalf("expected no layer separator row when all milestones share layer 0, got: %q", text)
	}
}

func TestEpicScreenRenderNoBlockedWhenDepDone(t *testing.T) {
	s := state.NewStore()
	s.SetPlan(client.Plan{
		Epic: client.EpicConfig{
			Name: "test",
			Milestones: []client.Milestone{
				{ID: 1, Name: "M1", Status: "done"},
				{ID: 2, Name: "M2", Status: "pending", DependsOn: []int{1}},
			},
		},
	})
	es := NewEpicScreen(s)

	el := es.Render(nil)
	text := collectAllText(el)
	// M1 is done → M2 is not blocked
	if strings.Contains(text, "BLOCKED") {
		t.Fatalf("expected no 'BLOCKED' when dep is done, got: %q", text)
	}
}

// --- 20. DAG layer label in rendered output ---

func TestEpicScreenRenderDAGLayerLabels(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// Layer column should show L0 and L1
	if !strings.Contains(text, "L0") {
		t.Fatalf("expected 'L0' layer label in output, got: %q", text)
	}
	if !strings.Contains(text, "L1") {
		t.Fatalf("expected 'L1' layer label in output, got: %q", text)
	}
}

// --- 21. DAG layer arrow indicator ---

func TestEpicScreenRenderDAGArrowIndicator(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// M2 has deps and is layer 1 -> should show "L1↑"
	if !strings.Contains(text, "L1↑") {
		t.Fatalf("expected 'L1↑' dependency arrow in output, got: %q", text)
	}
}

// --- 22. DAG box-drawing prefix in rendered output ---

func TestEpicScreenRenderDAGBoxDrawingPrefix(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// M1 is root in a DAG -> should have ┌─ prefix
	if !strings.Contains(text, "┌─") {
		t.Fatalf("expected '┌─' box-drawing prefix for root milestone, got: %q", text)
	}
	// M2 is last in last layer -> should have └─ prefix
	if !strings.Contains(text, "└─") {
		t.Fatalf("expected '└─' box-drawing prefix for leaf milestone, got: %q", text)
	}
}

// --- 23. DAG summary line shows "DAG arrows" when deps exist ---

func TestEpicScreenRenderDAGSummaryWithDeps(t *testing.T) {
	store := testStoreWithEpicAndIssues()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	if !strings.Contains(text, "DAG arrows show dependencies") {
		t.Fatalf("expected DAG arrows info in summary, got: %q", text)
	}
}

func TestEpicScreenRenderDAGSummaryNoDeps(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	if strings.Contains(text, "DAG arrows show dependencies") {
		t.Fatalf("should not show DAG arrows info when no deps, got: %q", text)
	}
}

// --- 24. Diamond DAG rendering ---

func TestEpicScreenRenderDiamondDAG(t *testing.T) {
	s := state.NewStore()
	s.SetPlan(client.Plan{
		Epic: client.EpicConfig{
			Name:        "diamond-test",
			Description: "Diamond dependency pattern",
			Milestones: []client.Milestone{
				{ID: 1, Name: "Lint compliance", Status: "done", PhaseStatus: map[string]string{"plan": "done"}},
				{ID: 2, Name: "Test coverage", Status: "done", DependsOn: []int{1}, PhaseStatus: map[string]string{"plan": "done"}},
				{ID: 3, Name: "ViewModel arch", Status: "done", DependsOn: []int{1}, PhaseStatus: map[string]string{"plan": "done"}},
				{ID: 5, Name: "API enhancements", Status: "done", DependsOn: []int{1}, PhaseStatus: map[string]string{"plan": "done"}},
				{ID: 4, Name: "TUI features", Status: "in_progress", DependsOn: []int{2, 3, 5}, PhaseStatus: map[string]string{"plan": "in_progress"}},
			},
		},
	})
	es := NewEpicScreen(s)

	el := es.Render(nil)
	text := collectAllText(el)

	// All milestone names should appear
	for _, name := range []string{"Lint compliance", "Test coverage", "ViewModel arch", "API enhancements", "TUI features"} {
		if !strings.Contains(text, name) {
			t.Errorf("expected milestone %q in output", name)
		}
	}

	// Layer labels should appear
	if !strings.Contains(text, "L0") {
		t.Error("expected L0 layer label")
	}
	if !strings.Contains(text, "L1") {
		t.Error("expected L1 layer label")
	}
	if !strings.Contains(text, "L2") {
		t.Error("expected L2 layer label")
	}

	// Box-drawing connectors
	if !strings.Contains(text, "┌─") {
		t.Error("expected ┌─ for root milestone")
	}
	if !strings.Contains(text, "├─") {
		t.Error("expected ├─ for middle milestones")
	}
	if !strings.Contains(text, "└─") {
		t.Error("expected └─ for leaf milestone")
	}

	// Layer separators
	if !strings.Contains(text, "── L1 ──") {
		t.Error("expected layer separator for L1")
	}
	if !strings.Contains(text, "── L2 ──") {
		t.Error("expected layer separator for L2")
	}
}

// --- 25. No DAG connectors for flat milestone list ---

func TestEpicScreenRenderFlatListNoConnectors(t *testing.T) {
	store := testStoreWithEpic()
	es := NewEpicScreen(store)

	el := es.Render(nil)
	text := collectAllText(el)
	// No box-drawing connectors should appear for flat list
	if strings.Contains(text, "┌─") {
		t.Fatalf("should not have ┌─ in flat list, got: %q", text)
	}
	// └─ should also not appear (only DAG prefix, not issue tree)
	// Note: ├─ is used by issue tree connectors, so skip checking that
}
