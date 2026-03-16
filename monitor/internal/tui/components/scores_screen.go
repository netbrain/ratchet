package components

import (
	"fmt"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

// ScoresScreen renders the scores summary table.
type ScoresScreen struct {
	vm      *views.ScoresViewModel
	lastKey rune
}

// NewScoresScreen creates a new ScoresScreen backed by the given store.
func NewScoresScreen(store *state.Store) *ScoresScreen {
	return &ScoresScreen{
		vm: views.NewScoresViewModel(store),
	}
}

// Render builds the element tree for the scores screen.
func (ss *ScoresScreen) Render(app *tui.App) *tui.Element {
	ss.vm.Refresh()

	root := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Column),
		tui.WithFlexGrow(1),
	)

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
	cols := scoreColumnCountForWidth(width)

	summaries := ss.vm.PairSummaries()

	if len(summaries) == 0 {
		empty := tui.New(
			tui.WithText("No score data"),
			tui.WithFlexGrow(1),
		)
		root.AddChild(empty)
		return root
	}

	// Set viewport height: subtract root chrome (3) + table header (1).
	overhead := 4
	ss.vm.SetViewportHeight(height - overhead)

	tableHeight := height - overhead + 1 // +1 for header row
	table := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Column),
		tui.WithHeight(tableHeight),
	)

	header := ss.buildRow(cols, "Pair", "Debates", "Consensus%", "Avg Rounds", "Found", "Resolved", "Escalated", "Trend",
		tui.NewStyle().Bold(), false)
	table.AddChild(header)

	// Data rows (sliced by scroll offset for viewport scrolling).
	offset := ss.vm.ScrollOffset()
	end := offset + (height - overhead)
	if end > len(summaries) {
		end = len(summaries)
	}
	for i := offset; i < end; i++ {
		s := summaries[i]
		consensus := fmt.Sprintf("%.0f%%", s.ConsensusRate*100)
		avgRounds := fmt.Sprintf("%.1f", s.AvgRounds)
		found := fmt.Sprintf("%d", s.IssuesFound)
		resolved := fmt.Sprintf("%d", s.IssuesResolved)
		escalated := fmt.Sprintf("%d", s.Escalated)
		debates := fmt.Sprintf("%d", s.DebateCount)
		trend := views.Sparkline(s.Trend, 8)

		style := ss.consensusStyle(s.ConsensusRate, s.Escalated)
		selected := i == ss.vm.SelectedIndex()
		row := ss.buildRow(cols, s.Pair, debates, consensus, avgRounds, found, resolved, escalated, trend,
			style, selected)
		table.AddChild(row)
	}

	root.AddChild(table)
	return root
}

// buildRow constructs a flex row for the scores table.
func (ss *ScoresScreen) buildRow(cols int, pair, debates, consensus, avgRounds, found, resolved, escalated, trend string,
	style tui.Style, selected bool) *tui.Element {

	rowStyle := style
	if selected {
		rowStyle = rowStyle.Reverse()
	}

	row := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Row),
		tui.WithHeight(1),
	)

	pairEl := tui.New(
		tui.WithText(pair),
		tui.WithTextStyle(rowStyle),
		tui.WithFlexGrow(1),
	)
	row.AddChild(pairEl)

	debatesEl := tui.New(
		tui.WithText(debates),
		tui.WithTextStyle(rowStyle),
		tui.WithWidth(9),
	)
	row.AddChild(debatesEl)

	consensusEl := tui.New(
		tui.WithText(consensus),
		tui.WithTextStyle(rowStyle),
		tui.WithWidth(12),
	)
	row.AddChild(consensusEl)

	trendEl := tui.New(
		tui.WithText(trend),
		tui.WithTextStyle(rowStyle),
		tui.WithWidth(10),
	)
	row.AddChild(trendEl)

	if cols >= 6 {
		avgEl := tui.New(
			tui.WithText(avgRounds),
			tui.WithTextStyle(rowStyle),
			tui.WithWidth(12),
		)
		row.AddChild(avgEl)

		foundEl := tui.New(
			tui.WithText(found),
			tui.WithTextStyle(rowStyle),
			tui.WithWidth(8),
		)
		row.AddChild(foundEl)
	}

	if cols >= 8 {
		resolvedEl := tui.New(
			tui.WithText(resolved),
			tui.WithTextStyle(rowStyle),
			tui.WithWidth(10),
		)
		row.AddChild(resolvedEl)

		escalatedEl := tui.New(
			tui.WithText(escalated),
			tui.WithTextStyle(rowStyle),
			tui.WithWidth(10),
		)
		row.AddChild(escalatedEl)
	}

	return row
}

// consensusStyle returns a style colored by consensus rate and escalation count.
func (ss *ScoresScreen) consensusStyle(rate float64, escalated int) tui.Style {
	if escalated > 0 {
		return tui.NewStyle().Foreground(tui.Red)
	}
	switch {
	case rate >= 0.8:
		return tui.NewStyle().Foreground(tui.Green)
	case rate >= 0.5:
		return tui.NewStyle().Foreground(tui.Yellow)
	default:
		return tui.NewStyle().Foreground(tui.Red)
	}
}

// KeyMap returns key bindings for the scores screen.
func (ss *ScoresScreen) KeyMap() tui.KeyMap {
	return tui.KeyMap{
		tui.On(tui.Rune('j'), dirty(func(ke tui.KeyEvent) {
			ss.lastKey = 0
			ss.vm.SelectNext()
		})),
		tui.On(tui.Rune('k'), dirty(func(ke tui.KeyEvent) {
			ss.lastKey = 0
			ss.vm.SelectPrev()
		})),
		tui.On(tui.Rune('G'), dirty(func(ke tui.KeyEvent) {
			ss.lastKey = 0
			ss.vm.SelectLast()
		})),
		tui.On(tui.Rune('g'), dirty(func(ke tui.KeyEvent) {
			if ss.lastKey == 'g' {
				ss.vm.SelectFirst()
				ss.lastKey = 0
			} else {
				ss.lastKey = 'g'
			}
		})),
	}
}

// scoreColumnCountForWidth returns the number of table columns for a given terminal width.
func scoreColumnCountForWidth(width int) int {
	switch {
	case width >= 120:
		return 8
	case width >= 80:
		return 6
	default:
		return 4
	}
}

// Ensure ScoresScreen satisfies the interfaces.
var (
	_ tui.Component   = (*ScoresScreen)(nil)
	_ tui.KeyListener = (*ScoresScreen)(nil)
)
