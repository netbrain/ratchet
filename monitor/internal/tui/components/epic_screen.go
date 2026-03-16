package components

import (
	"fmt"
	"strings"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

// EpicScreen renders the epic progress display with milestone table and progress bar.
type EpicScreen struct {
	vm      *views.EpicViewModel
	lastKey rune
}

// NewEpicScreen creates a new EpicScreen backed by the given store.
func NewEpicScreen(store *state.Store) *EpicScreen {
	return &EpicScreen{
		vm: views.NewEpicViewModel(store),
	}
}

// phaseSymbol returns the display symbol for a phase status.
func phaseSymbol(status string) string {
	switch status {
	case "done":
		return "✓"
	case "in_progress":
		return "●"
	default:
		return "○"
	}
}

// phaseColor returns the color for a phase status.
func phaseColor(status string) tui.Color {
	switch status {
	case "done":
		return tui.Green
	case "in_progress":
		return tui.Cyan
	default:
		return tui.ANSIColor(243) // dim
	}
}

// Render builds the element tree for the epic screen.
func (es *EpicScreen) Render(app *tui.App) *tui.Element {
	es.vm.Refresh()

	root := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Column),
		tui.WithFlexGrow(1),
	)

	milestones := es.vm.Milestones()

	if len(milestones) == 0 {
		empty := tui.New(
			tui.WithText("No epic data"),
			tui.WithFlexGrow(1),
		)
		root.AddChild(empty)
		return root
	}

	width := 120
	height := 24
	if app != nil {
		w, h := app.Size()
		if w > 0 {
			width = w
		}
		if h > 0 {
			height = h
		}
	}
	cols := epicColumnCountForWidth(width)

	// Epic header
	headerText := fmt.Sprintf("%s — %s", es.vm.EpicName(), es.vm.EpicDescription())
	header := tui.New(
		tui.WithText(headerText),
		tui.WithTextStyle(tui.NewStyle().Bold()),
		tui.WithHeight(1),
	)
	root.AddChild(header)

	// Progress bar
	completed := es.vm.CompletedCount()
	total := es.vm.TotalCount()
	pct := es.vm.ProgressPercent()

	barWidth := 30
	if width < 80 {
		barWidth = 20
	} else if width >= 120 {
		barWidth = 40
	}
	filled := min(int(pct*float64(barWidth)), barWidth)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	progressText := fmt.Sprintf("Progress: %s %d/%d (%.0f%%)", bar, completed, total, pct*100)
	progressEl := tui.New(
		tui.WithText(progressText),
		tui.WithHeight(1),
	)
	root.AddChild(progressEl)

	// Spacer
	spacer := tui.New(tui.WithHeight(1))
	root.AddChild(spacer)

	// Set viewport height: subtract root chrome (3) + epic header (1) + progress (1) + spacer (1) + table header (1) + focus (1).
	overhead := 8
	es.vm.SetViewportHeight(height - overhead)

	// Milestone table with fixed height to prevent overflow.
	tableHeight := height - overhead + 1 // +1 for header row
	table := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Column),
		tui.WithHeight(tableHeight),
	)

	// Table header
	tableHeader := es.buildHeaderRow(cols)
	table.AddChild(tableHeader)

	// Table rows (sliced by scroll offset for viewport scrolling).
	phases := []string{"plan", "test", "build", "review", "harden"}
	offset := es.vm.ScrollOffset()
	end := offset + (height - overhead)
	if end > len(milestones) {
		end = len(milestones)
	}
	for i := offset; i < end; i++ {
		m := milestones[i]
		selected := i == es.vm.SelectedIndex()
		row := es.buildMilestoneRow(cols, i+1, m, phases, selected)
		table.AddChild(row)
	}

	root.AddChild(table)

	// Current focus indicator
	if focus := es.vm.CurrentFocus(); focus != nil {
		focusText := fmt.Sprintf("Current focus: Milestone %d — %s phase", focus.MilestoneID, focus.Phase)
		focusEl := tui.New(
			tui.WithText(focusText),
			tui.WithTextStyle(tui.NewStyle().Foreground(tui.Cyan)),
			tui.WithHeight(1),
		)
		root.AddChild(focusEl)
	}

	return root
}

// buildHeaderRow creates the header row for the milestone table.
func (es *EpicScreen) buildHeaderRow(cols int) *tui.Element {
	row := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Row),
		tui.WithHeight(1),
	)

	style := tui.NewStyle().Bold()

	idEl := tui.New(tui.WithText("#"), tui.WithTextStyle(style), tui.WithWidth(4))
	nameEl := tui.New(tui.WithText("Milestone"), tui.WithTextStyle(style), tui.WithFlexGrow(1))
	statusEl := tui.New(tui.WithText("Status"), tui.WithTextStyle(style), tui.WithWidth(14))

	row.AddChild(idEl, nameEl, statusEl)

	if cols >= 6 {
		for _, phase := range []string{"plan", "test", "build"} {
			el := tui.New(tui.WithText(phase), tui.WithTextStyle(style), tui.WithWidth(8))
			row.AddChild(el)
		}
	}

	if cols >= 8 {
		for _, phase := range []string{"review", "harden"} {
			el := tui.New(tui.WithText(phase), tui.WithTextStyle(style), tui.WithWidth(8))
			row.AddChild(el)
		}
	}

	if cols < 6 {
		el := tui.New(tui.WithText("Phase"), tui.WithTextStyle(style), tui.WithWidth(8))
		row.AddChild(el)
	}

	return row
}

// buildMilestoneRow creates a row for a single milestone.
func (es *EpicScreen) buildMilestoneRow(cols, num int, m views.MilestoneStatus, phases []string, selected bool) *tui.Element {
	statusColor := milestoneStatusTuiColor(m.Status)
	baseStyle := tui.NewStyle().Foreground(statusColor)
	if selected {
		baseStyle = baseStyle.Reverse()
	}

	row := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Row),
		tui.WithHeight(1),
	)

	idEl := tui.New(tui.WithText(fmt.Sprintf("%d", num)), tui.WithTextStyle(baseStyle), tui.WithWidth(4))

	// Add regression indicator to milestone name if regressions > 0
	nameText := m.Name
	if m.Regressions > 0 {
		nameText = fmt.Sprintf("%s [↻%d]", m.Name, m.Regressions)
	}
	nameEl := tui.New(tui.WithText(nameText), tui.WithTextStyle(baseStyle), tui.WithFlexGrow(1))
	statusEl := tui.New(tui.WithText(m.Status), tui.WithTextStyle(baseStyle), tui.WithWidth(14))

	row.AddChild(idEl, nameEl, statusEl)

	if cols >= 6 {
		for _, phase := range phases[:3] {
			ps := m.PhaseStatus[phase]
			sym := phaseSymbol(ps)
			col := phaseColor(ps)
			style := tui.NewStyle().Foreground(col)
			if selected {
				style = style.Reverse()
			}
			el := tui.New(tui.WithText(sym), tui.WithTextStyle(style), tui.WithWidth(8))
			row.AddChild(el)
		}
	}

	if cols >= 8 {
		for _, phase := range phases[3:] {
			ps := m.PhaseStatus[phase]
			sym := phaseSymbol(ps)
			col := phaseColor(ps)
			style := tui.NewStyle().Foreground(col)
			if selected {
				style = style.Reverse()
			}
			el := tui.New(tui.WithText(sym), tui.WithTextStyle(style), tui.WithWidth(8))
			row.AddChild(el)
		}
	}

	if cols < 6 {
		// Show current phase indicator for narrow widths
		currentPhase := ""
		for _, phase := range phases {
			ps := m.PhaseStatus[phase]
			if ps == "in_progress" {
				currentPhase = phase
				break
			}
		}
		if currentPhase == "" && m.Status == "done" {
			currentPhase = "✓"
		}
		style := baseStyle
		el := tui.New(tui.WithText(currentPhase), tui.WithTextStyle(style), tui.WithWidth(8))
		row.AddChild(el)
	}

	return row
}

// milestoneStatusTuiColor maps a milestone status to a tui.Color.
func milestoneStatusTuiColor(status string) tui.Color {
	switch status {
	case "done":
		return tui.Green
	case "in_progress":
		return tui.Cyan
	default:
		return tui.ANSIColor(243) // dim
	}
}

// KeyMap returns key bindings for the epic screen.
func (es *EpicScreen) KeyMap() tui.KeyMap {
	return tui.KeyMap{
		tui.On(tui.Rune('j'), dirty(func(ke tui.KeyEvent) {
			es.lastKey = 0
			es.vm.SelectNext()
		})),
		tui.On(tui.Rune('k'), dirty(func(ke tui.KeyEvent) {
			es.lastKey = 0
			es.vm.SelectPrev()
		})),
		tui.On(tui.Rune('G'), dirty(func(ke tui.KeyEvent) {
			es.lastKey = 0
			es.vm.SelectLast()
		})),
		tui.On(tui.Rune('g'), dirty(func(ke tui.KeyEvent) {
			if es.lastKey == 'g' {
				es.vm.SelectFirst()
				es.lastKey = 0
			} else {
				es.lastKey = 'g'
			}
		})),
	}
}

// epicColumnCountForWidth returns the number of table columns for a given terminal width.
func epicColumnCountForWidth(width int) int {
	switch {
	case width >= 120:
		return 8
	case width >= 80:
		return 6
	default:
		return 4
	}
}

// Ensure EpicScreen satisfies the interfaces.
var (
	_ tui.Component   = (*EpicScreen)(nil)
	_ tui.KeyListener = (*EpicScreen)(nil)
)
