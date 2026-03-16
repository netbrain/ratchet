package components

import (
	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

// PairsScreen renders the pairs table with filtering and selection.
type PairsScreen struct {
	vm         *views.PairsViewModel
	filterMode bool
	lastKey    rune // tracks last key for multi-key sequences (e.g. gg)
}

// NewPairsScreen creates a new PairsScreen backed by the given store.
func NewPairsScreen(store *state.Store) *PairsScreen {
	return &PairsScreen{
		vm: views.NewPairsViewModel(store),
	}
}

// Render builds the element tree for the pairs screen.
func (ps *PairsScreen) Render(app *tui.App) *tui.Element {
	ps.vm.Refresh()

	root := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Column),
		tui.WithFlexGrow(1),
	)

	// Determine column count and viewport height based on terminal size.
	width := 120 // default
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
	cols := columnCountForWidth(width)

	pairs := ps.vm.FilteredPairs()

	// Filter bar (shown when filter is active or in filter mode).
	if ps.filterMode || ps.vm.Filter() != "" {
		filterBar := tui.New(
			tui.WithDisplay(tui.DisplayFlex),
			tui.WithDirection(tui.Row),
			tui.WithHeight(1),
		)
		label := tui.New(
			tui.WithText("Filter: "),
			tui.WithTextStyle(tui.NewStyle().Bold()),
		)
		value := tui.New(
			tui.WithText(ps.vm.Filter()),
		)
		filterBar.AddChild(label, value)
		root.AddChild(filterBar)
	}

	if len(pairs) == 0 {
		msg := "No pairs found"
		if ps.vm.Filter() != "" {
			msg = "No pairs matching filter: " + ps.vm.Filter()
		}
		empty := tui.New(
			tui.WithText(msg),
			tui.WithFlexGrow(1),
		)
		root.AddChild(empty)
		return root
	}

	// Set viewport height: subtract root chrome (header+tabbar+statusbar=3) + table header (1) + filter bar.
	overhead := 4 // root(3) + table header(1)
	if ps.filterMode || ps.vm.Filter() != "" {
		overhead++
	}
	ps.vm.SetViewportHeight(height - overhead)

	// Table container with fixed height to prevent overflow.
	tableHeight := height - overhead + 1 // +1 for the header row included in the table
	table := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Column),
		tui.WithHeight(tableHeight),
	)

	// Header row.
	header := ps.buildRow(cols, "Name", "Component", "Phase", "Status", "Scope",
		tui.NewStyle().Bold(), false)
	table.AddChild(header)

	// Data rows (sliced by scroll offset for viewport scrolling).
	offset := ps.vm.ScrollOffset()
	end := offset + (height - overhead)
	if end > len(pairs) {
		end = len(pairs)
	}
	for i := offset; i < end; i++ {
		pair := pairs[i]
		colorName := ps.vm.StatusColor(pair.Status)
		fg := statusForeground(colorName)
		style := tui.NewStyle().Foreground(fg)
		if colorName == "dim" {
			style = style.Dim()
		}

		selected := i == ps.vm.SelectedIndex()
		row := ps.buildRow(cols, pair.Name, pair.Component, pair.Phase, pair.Status, pair.Scope,
			style, selected)
		table.AddChild(row)
	}

	root.AddChild(table)
	return root
}

// buildRow constructs a flex row with the given column values.
// cols controls how many columns are shown (3=Name,Status,Phase; 4=+Component; 5=+Scope).
func (ps *PairsScreen) buildRow(cols int, name, component, phase, status, scope string,
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

	// Name column: flex-grow.
	nameEl := tui.New(
		tui.WithText(name),
		tui.WithTextStyle(rowStyle),
		tui.WithFlexGrow(1),
	)
	row.AddChild(nameEl)

	// Status column: fixed width (always shown).
	statusEl := tui.New(
		tui.WithText(status),
		tui.WithTextStyle(rowStyle),
		tui.WithWidth(12),
	)
	row.AddChild(statusEl)

	// Phase column: fixed width (always shown).
	phaseEl := tui.New(
		tui.WithText(phase),
		tui.WithTextStyle(rowStyle),
		tui.WithWidth(12),
	)
	row.AddChild(phaseEl)

	if cols >= 4 {
		compEl := tui.New(
			tui.WithText(component),
			tui.WithTextStyle(rowStyle),
			tui.WithWidth(14),
		)
		row.AddChild(compEl)
	}

	if cols >= 5 {
		scopeEl := tui.New(
			tui.WithText(scope),
			tui.WithTextStyle(rowStyle),
			tui.WithWidth(20),
		)
		row.AddChild(scopeEl)
	}

	return row
}

// KeyMap returns dynamic key bindings based on the current mode.
func (ps *PairsScreen) KeyMap() tui.KeyMap {
	if ps.filterMode {
		return ps.filterKeyMap()
	}
	return ps.normalKeyMap()
}

func (ps *PairsScreen) normalKeyMap() tui.KeyMap {
	km := tui.KeyMap{
		tui.On(tui.Rune('j'), dirty(func(ke tui.KeyEvent) {
			ps.lastKey = 0
			ps.vm.SelectNext()
		})),
		tui.On(tui.Rune('k'), dirty(func(ke tui.KeyEvent) {
			ps.lastKey = 0
			ps.vm.SelectPrev()
		})),
		tui.On(tui.Rune('G'), dirty(func(ke tui.KeyEvent) {
			ps.lastKey = 0
			ps.vm.SelectLast()
		})),
		tui.On(tui.Rune('g'), dirty(func(ke tui.KeyEvent) {
			if ps.lastKey == 'g' {
				ps.vm.SelectFirst()
				ps.lastKey = 0
			} else {
				ps.lastKey = 'g'
			}
		})),
		tui.On(tui.Rune('/'), dirty(func(ke tui.KeyEvent) {
			ps.lastKey = 0
			ps.filterMode = true
		})),
		tui.On(tui.KeyEscape, dirty(func(ke tui.KeyEvent) {
			ps.lastKey = 0
			if ps.vm.Filter() != "" {
				ps.vm.SetFilter("")
			}
		})),
	}
	return km
}

func (ps *PairsScreen) filterKeyMap() tui.KeyMap {
	km := tui.KeyMap{
		tui.OnFocused(tui.AnyRune, dirty(func(ke tui.KeyEvent) {
			ps.vm.SetFilter(ps.vm.Filter() + string(ke.Rune))
		})),
		tui.OnStop(tui.KeyBackspace, dirty(func(ke tui.KeyEvent) {
			f := []rune(ps.vm.Filter())
			if len(f) > 0 {
				ps.vm.SetFilter(string(f[:len(f)-1]))
			}
		})),
		tui.OnStop(tui.KeyEscape, dirty(func(ke tui.KeyEvent) {
			ps.filterMode = false
		})),
		tui.OnStop(tui.KeyEnter, dirty(func(ke tui.KeyEvent) {
			ps.filterMode = false
		})),
	}
	return km
}

// columnCountForWidth returns the number of table columns for a given terminal width.
func columnCountForWidth(width int) int {
	switch {
	case width >= 120:
		return 5
	case width >= 80:
		return 4
	default:
		return 3
	}
}

// statusForeground maps a color name string to a tui.Color.
func statusForeground(colorName string) tui.Color {
	switch colorName {
	case "cyan":
		return tui.Cyan
	case "red":
		return tui.Red
	case "green":
		return tui.Green
	case "dim":
		return tui.White
	case "white":
		return tui.White
	default:
		return tui.White
	}
}

// Ensure PairsScreen satisfies the interfaces.
var (
	_ tui.Component   = (*PairsScreen)(nil)
	_ tui.KeyListener = (*PairsScreen)(nil)
)
