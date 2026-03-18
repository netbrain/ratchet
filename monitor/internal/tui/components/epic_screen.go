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

	// DAG layer summary
	maxLayer := es.vm.MaxLayer()
	if maxLayer >= 0 {
		layerText := fmt.Sprintf("Milestone layers: %d (Layer 0 = no dependencies)", maxLayer+1)
		layerEl := tui.New(
			tui.WithText(layerText),
			tui.WithTextStyle(tui.NewStyle().Foreground(tui.ANSIColor(243))),
			tui.WithHeight(1),
		)
		root.AddChild(layerEl)
	}

	// Spacer
	spacer := tui.New(tui.WithHeight(1))
	root.AddChild(spacer)

	// Set viewport height: subtract root chrome (3) + epic header (1) + progress (1) + layer summary (1, if shown) + spacer (1) + table header (1) + focus (1).
	overhead := 8
	if maxLayer >= 0 {
		overhead = 9 // Add 1 for layer summary
	}
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
	// We track rendered rows to avoid overflowing the fixed-height table,
	// since each milestone may also emit issue sub-rows and DAG connectors.
	phases := []string{"plan", "test", "build", "review", "harden"}
	offset := es.vm.ScrollOffset()
	maxRows := height - overhead // available data rows (excluding header)
	rowsRendered := 0
	for i := offset; i < len(milestones) && rowsRendered < maxRows; i++ {
		m := milestones[i]
		selected := i == es.vm.SelectedIndex()

		// DAG connector row between layers
		if i > 0 && m.Layer > milestones[i-1].Layer && len(m.DependsOn) > 0 {
			if rowsRendered >= maxRows {
				break
			}
			connectorText := "  │"
			connectorEl := tui.New(
				tui.WithText(connectorText),
				tui.WithTextStyle(tui.NewStyle().Foreground(tui.ANSIColor(243))),
				tui.WithHeight(1),
			)
			table.AddChild(connectorEl)
			rowsRendered++
		}

		if rowsRendered >= maxRows {
			break
		}
		row := es.buildMilestoneRow(cols, i+1, m, phases, selected)
		table.AddChild(row)
		rowsRendered++

		// Render per-issue rows under each milestone
		for _, iss := range m.Issues {
			if rowsRendered >= maxRows {
				break
			}
			issueRow := es.buildIssueRow(cols, iss, phases, selected)
			table.AddChild(issueRow)
			rowsRendered++
		}
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
	layerEl := tui.New(tui.WithText("L"), tui.WithTextStyle(style), tui.WithWidth(3))
	nameEl := tui.New(tui.WithText("Milestone"), tui.WithTextStyle(style), tui.WithFlexGrow(1))
	statusEl := tui.New(tui.WithText("Status"), tui.WithTextStyle(style), tui.WithWidth(14))
	regEl := tui.New(tui.WithText("Reg"), tui.WithTextStyle(style), tui.WithWidth(8))

	row.AddChild(idEl, layerEl, nameEl, statusEl, regEl)

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

	// DAG prefix + ID
	dagPrefix := es.vm.DAGPrefix(m)
	idEl := tui.New(tui.WithText(fmt.Sprintf("%s%d", dagPrefix, num)), tui.WithTextStyle(baseStyle), tui.WithWidth(6))

	// Layer indicator with dependency arrow
	layerSymbol := fmt.Sprintf("%d", m.Layer)
	if len(m.DependsOn) > 0 && m.Layer > 0 {
		layerSymbol = fmt.Sprintf("%d↑", m.Layer) // Arrow indicates it depends on earlier layers
	}
	layerEl := tui.New(tui.WithText(layerSymbol), tui.WithTextStyle(baseStyle), tui.WithWidth(3))

	nameEl := tui.New(tui.WithText(m.Name), tui.WithTextStyle(baseStyle), tui.WithFlexGrow(1))
	statusEl := tui.New(tui.WithText(m.Status), tui.WithTextStyle(baseStyle), tui.WithWidth(14))

	// Regression budget cell: "X/Y" with color coding
	regText := es.vm.RegressionBudgetText(m)
	var regColor tui.Color
	switch es.vm.RegressionWarningLevel(m) {
	case "danger":
		regColor = tui.Red
	case "warn":
		regColor = tui.Yellow
	default:
		regColor = tui.Green
	}
	regStyle := tui.NewStyle().Foreground(regColor)
	if selected {
		regStyle = regStyle.Reverse()
	}
	regEl := tui.New(tui.WithText(regText), tui.WithTextStyle(regStyle), tui.WithWidth(8))

	row.AddChild(idEl, layerEl, nameEl, statusEl, regEl)

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

// buildIssueRow creates a sub-row for a single issue within a milestone.
func (es *EpicScreen) buildIssueRow(cols int, iss views.IssueStatus, phases []string, _ bool) *tui.Element {
	statusColor := milestoneStatusTuiColor(iss.Status)
	baseStyle := tui.NewStyle().Foreground(statusColor).Dim()

	row := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Row),
		tui.WithHeight(1),
	)

	// Indented ref with tree connector
	refText := fmt.Sprintf("    ├─ %s", iss.Ref)
	refEl := tui.New(tui.WithText(refText), tui.WithTextStyle(baseStyle), tui.WithWidth(14))

	// Skip layer column for issues
	spacerEl := tui.New(tui.WithText(""), tui.WithWidth(3))

	// Issue title
	titleEl := tui.New(tui.WithText(iss.Title), tui.WithTextStyle(baseStyle), tui.WithFlexGrow(1))

	// Issue status
	statusEl := tui.New(tui.WithText(iss.Status), tui.WithTextStyle(baseStyle), tui.WithWidth(14))

	// Empty spacer for Reg column alignment
	regSpacerEl := tui.New(tui.WithText(""), tui.WithWidth(8))

	row.AddChild(refEl, spacerEl, titleEl, statusEl, regSpacerEl)

	// Phase indicators
	if cols >= 6 {
		for _, phase := range phases[:3] {
			ps := iss.PhaseStatus[phase]
			sym := phaseSymbol(ps)
			col := phaseColor(ps)
			style := tui.NewStyle().Foreground(col)
			el := tui.New(tui.WithText(sym), tui.WithTextStyle(style), tui.WithWidth(8))
			row.AddChild(el)
		}
	}

	if cols >= 8 {
		for _, phase := range phases[3:] {
			ps := iss.PhaseStatus[phase]
			sym := phaseSymbol(ps)
			col := phaseColor(ps)
			style := tui.NewStyle().Foreground(col)
			el := tui.New(tui.WithText(sym), tui.WithTextStyle(style), tui.WithWidth(8))
			row.AddChild(el)
		}
	}

	if cols < 6 {
		currentPhase := ""
		for _, phase := range phases {
			if iss.PhaseStatus[phase] == "in_progress" {
				currentPhase = phase
				break
			}
		}
		if currentPhase == "" && iss.Status == "done" {
			currentPhase = "✓"
		}
		el := tui.New(tui.WithText(currentPhase), tui.WithTextStyle(baseStyle), tui.WithWidth(8))
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
