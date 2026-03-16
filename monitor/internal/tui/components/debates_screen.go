package components

import (
	"fmt"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/markdown"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

type screenMode int

const (
	modeList screenMode = iota
	modeDetail
)

// DebateDetailFetcher can fetch a debate's detail by ID.
type DebateDetailFetcher interface {
	FetchDebateDetail(id string)
}

// DebatesScreen renders the debates list and detail views.
type DebatesScreen struct {
	listVM     *views.DebatesViewModel
	detailVM   *views.DebateDetailViewModel
	store      *state.Store
	fetcher    DebateDetailFetcher
	mode       screenMode
	filterMode bool
	lastKey    rune
}

// NewDebatesScreen creates a new DebatesScreen backed by the given store and fetcher.
func NewDebatesScreen(store *state.Store, fetcher DebateDetailFetcher) *DebatesScreen {
	return &DebatesScreen{
		listVM:  views.NewDebatesViewModel(store),
		store:   store,
		fetcher: fetcher,
	}
}

// Render builds the element tree for the debates screen.
func (ds *DebatesScreen) Render(app *tui.App) *tui.Element {
	if ds.mode == modeDetail {
		return ds.renderDetail(app)
	}
	return ds.renderList(app)
}

// renderList builds the list view.
func (ds *DebatesScreen) renderList(app *tui.App) *tui.Element {
	ds.listVM.Refresh()

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
	cols := debateColumnCountForWidth(width)

	debates := ds.listVM.FilteredDebates()

	// Filter bar row (text filter left, status filter right).
	if ds.filterMode || ds.listVM.Filter() != "" || ds.listVM.StatusFilter() != "" {
		filterBar := tui.New(
			tui.WithDisplay(tui.DisplayFlex),
			tui.WithDirection(tui.Row),
			tui.WithHeight(1),
		)
		if ds.filterMode || ds.listVM.Filter() != "" {
			label := tui.New(
				tui.WithText("Filter: "),
				tui.WithTextStyle(tui.NewStyle().Bold()),
			)
			value := tui.New(
				tui.WithText(ds.listVM.Filter()),
			)
			filterBar.AddChild(label, value)
		}
		if ds.listVM.StatusFilter() != "" {
			spacer := tui.New(tui.WithFlexGrow(1))
			colorName := ds.listVM.DebateStatusColor(ds.listVM.StatusFilter())
			fg := debateStatusForeground(colorName)
			badge := tui.New(
				tui.WithText("[Status: "+ds.listVM.StatusFilter()+"]"),
				tui.WithTextStyle(tui.NewStyle().Foreground(fg).Bold()),
			)
			filterBar.AddChild(spacer, badge)
		}
		root.AddChild(filterBar)
	}

	if len(debates) == 0 {
		msg := "No debates found"
		if ds.listVM.Filter() != "" {
			msg = "No debates matching filter: " + ds.listVM.Filter()
		}
		empty := tui.New(
			tui.WithText(msg),
			tui.WithFlexGrow(1),
		)
		root.AddChild(empty)
		return root
	}

	// Set viewport height: subtract root chrome (3) + table header (1) + filter bar.
	overhead := 4 // root(3) + table header(1)
	if ds.filterMode || ds.listVM.Filter() != "" || ds.listVM.StatusFilter() != "" {
		overhead++
	}
	ds.listVM.SetViewportHeight(height - overhead)

	// Table with fixed height to prevent overflow.
	tableHeight := height - overhead + 1 // +1 for header row
	table := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Column),
		tui.WithHeight(tableHeight),
	)

	header := ds.buildListRow(cols, "ID", "Pair", "Phase", "Status", "Milestone",
		tui.NewStyle().Bold(), false)
	table.AddChild(header)

	// Data rows (sliced by scroll offset for viewport scrolling).
	offset := ds.listVM.ScrollOffset()
	end := offset + (height - overhead)
	if end > len(debates) {
		end = len(debates)
	}
	for i := offset; i < end; i++ {
		d := debates[i]
		colorName := ds.listVM.DebateStatusColor(d.Status)
		fg := debateStatusForeground(colorName)
		style := tui.NewStyle().Foreground(fg)
		if colorName == "dim" {
			style = style.Dim()
		}

		milestone := ""
		if d.Milestone > 0 {
			milestone = fmt.Sprintf("M%d", d.Milestone)
		}

		selected := i == ds.listVM.SelectedIndex()
		row := ds.buildListRow(cols, d.ID, d.Pair, d.Phase, d.Status, milestone,
			style, selected)
		table.AddChild(row)
	}

	root.AddChild(table)
	return root
}

// buildListRow constructs a flex row for the debate list table.
func (ds *DebatesScreen) buildListRow(cols int, id, pair, phase, status, milestone string,
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

	idEl := tui.New(
		tui.WithText(id),
		tui.WithTextStyle(rowStyle),
		tui.WithFlexGrow(1),
	)
	row.AddChild(idEl)

	statusEl := tui.New(
		tui.WithText(status),
		tui.WithTextStyle(rowStyle),
		tui.WithWidth(14),
	)
	row.AddChild(statusEl)

	phaseEl := tui.New(
		tui.WithText(phase),
		tui.WithTextStyle(rowStyle),
		tui.WithWidth(12),
	)
	row.AddChild(phaseEl)

	if cols >= 4 {
		pairEl := tui.New(
			tui.WithText(pair),
			tui.WithTextStyle(rowStyle),
			tui.WithWidth(18),
		)
		row.AddChild(pairEl)
	}

	if cols >= 5 {
		msEl := tui.New(
			tui.WithText(milestone),
			tui.WithTextStyle(rowStyle),
			tui.WithWidth(8),
		)
		row.AddChild(msEl)
	}

	return row
}

// renderDetail builds the detail view for a single debate.
func (ds *DebatesScreen) renderDetail(app *tui.App) *tui.Element {
	root := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Column),
		tui.WithFlexGrow(1),
	)

	// Check if the async fetch has completed and populated the store.
	if ds.detailVM == nil && ds.store != nil {
		selected := ds.listVM.SelectedDebate()
		if selected != nil {
			if detail := ds.store.DebateDetail(selected.ID); detail != nil {
				ds.detailVM = views.NewDebateDetailViewModel(detail)
			}
		}
	}

	if ds.detailVM == nil {
		loading := tui.New(
			tui.WithText("Loading debate..."),
			tui.WithFlexGrow(1),
		)
		root.AddChild(loading)
		return root
	}

	// Header: debate ID, rounds count, status.
	headerRow := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Row),
		tui.WithHeight(1),
	)

	selected := ds.listVM.SelectedDebate()
	debateID := ""
	debateStatus := ""
	if selected != nil {
		debateID = selected.ID
		debateStatus = selected.Status
	}

	idEl := tui.New(
		tui.WithText(debateID),
		tui.WithTextStyle(tui.NewStyle().Bold()),
		tui.WithFlexGrow(1),
	)
	headerRow.AddChild(idEl)

	roundInfo := fmt.Sprintf("%d rounds", ds.detailVM.RoundCount())
	roundEl := tui.New(
		tui.WithText(roundInfo),
		tui.WithWidth(12),
	)
	headerRow.AddChild(roundEl)

	statusEl := tui.New(
		tui.WithText(debateStatus),
		tui.WithWidth(14),
	)
	headerRow.AddChild(statusEl)

	root.AddChild(headerRow)

	// Navigation hint bar.
	hintEl := tui.New(
		tui.WithText("Esc/Backspace:back  j/k:scroll  d/u:page  gg/G:top/bottom"),
		tui.WithTextStyle(tui.NewStyle().Dim()),
		tui.WithHeight(1),
	)
	root.AddChild(hintEl)

	// Build full thread: all rounds concatenated with role separators.
	var allLines []markdown.StyledLine
	for _, r := range ds.detailVM.Rounds() {
		// Role separator.
		label := fmt.Sprintf("── %s (round %d) ──", r.Role, r.Number)
		allLines = append(allLines, markdown.StyledLine{Text: "", Kind: markdown.KindNormal})
		allLines = append(allLines, markdown.StyledLine{Text: label, Kind: markdown.KindRule})
		allLines = append(allLines, markdown.StyledLine{Text: "", Kind: markdown.KindNormal})
		// Round content.
		allLines = append(allLines, markdown.RenderLines(r.Content)...)
	}

	// Get terminal height for viewport.
	width := 120
	height := 24
	if app != nil {
		w, h := app.Size()
		if w > 0 {
			width = w
		}
		_ = width // used only for future responsive layout
		if h > 0 {
			height = h
		}
	}
	overhead := 5 // root(3) + header(1) + hint(1)
	viewportH := height - overhead
	if viewportH < 1 {
		viewportH = 1
	}
	ds.detailVM.SetViewportHeight(viewportH)

	offset := ds.detailVM.ContentScrollOffset()
	if offset > len(allLines) {
		offset = len(allLines)
	}
	end := offset + viewportH
	if end > len(allLines) {
		end = len(allLines)
	}
	visible := allLines[offset:end]

	contentArea := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Column),
		tui.WithHeight(viewportH),
	)

	for _, sl := range visible {
		lineEl := tui.New(
			tui.WithText(sl.Text),
			tui.WithTextStyle(markdownLineStyle(sl.Kind)),
			tui.WithHeight(1),
		)
		contentArea.AddChild(lineEl)
	}

	root.AddChild(contentArea)
	return root
}

// KeyMap returns dynamic key bindings based on the current mode.
func (ds *DebatesScreen) KeyMap() tui.KeyMap {
	if ds.mode == modeDetail {
		return ds.detailKeyMap()
	}
	if ds.filterMode {
		return ds.listFilterKeyMap()
	}
	return ds.listNormalKeyMap()
}

func (ds *DebatesScreen) listNormalKeyMap() tui.KeyMap {
	return tui.KeyMap{
		tui.On(tui.Rune('j'), dirty(func(ke tui.KeyEvent) {
			ds.lastKey = 0
			ds.listVM.SelectNext()
		})),
		tui.On(tui.Rune('k'), dirty(func(ke tui.KeyEvent) {
			ds.lastKey = 0
			ds.listVM.SelectPrev()
		})),
		tui.On(tui.Rune('G'), dirty(func(ke tui.KeyEvent) {
			ds.lastKey = 0
			ds.listVM.SelectLast()
		})),
		tui.On(tui.Rune('g'), dirty(func(ke tui.KeyEvent) {
			if ds.lastKey == 'g' {
				ds.listVM.SelectFirst()
				ds.lastKey = 0
			} else {
				ds.lastKey = 'g'
			}
		})),
		tui.On(tui.Rune('/'), dirty(func(ke tui.KeyEvent) {
			ds.lastKey = 0
			ds.filterMode = true
		})),
		tui.On(tui.Rune('s'), dirty(func(ke tui.KeyEvent) {
			ds.lastKey = 0
			ds.listVM.CycleStatusFilter()
		})),
		tui.On(tui.KeyEnter, dirty(func(ke tui.KeyEvent) {
			ds.lastKey = 0
			ds.openDetail()
		})),
		tui.On(tui.KeyEscape, dirty(func(ke tui.KeyEvent) {
			ds.lastKey = 0
			if ds.listVM.Filter() != "" {
				ds.listVM.SetFilter("")
			}
		})),
	}
}

func (ds *DebatesScreen) listFilterKeyMap() tui.KeyMap {
	return tui.KeyMap{
		tui.OnFocused(tui.AnyRune, dirty(func(ke tui.KeyEvent) {
			ds.listVM.SetFilter(ds.listVM.Filter() + string(ke.Rune))
		})),
		tui.OnStop(tui.KeyBackspace, dirty(func(ke tui.KeyEvent) {
			f := []rune(ds.listVM.Filter())
			if len(f) > 0 {
				ds.listVM.SetFilter(string(f[:len(f)-1]))
			}
		})),
		tui.OnStop(tui.KeyEscape, dirty(func(ke tui.KeyEvent) {
			ds.filterMode = false
		})),
		tui.OnStop(tui.KeyEnter, dirty(func(ke tui.KeyEvent) {
			ds.filterMode = false
		})),
	}
}

func (ds *DebatesScreen) detailKeyMap() tui.KeyMap {
	goBack := func() {
		ds.mode = modeList
		ds.detailVM = nil
		ds.lastKey = 0
	}
	return tui.KeyMap{
		tui.On(tui.Rune('j'), dirty(func(ke tui.KeyEvent) {
			if ds.detailVM != nil {
				ds.lastKey = 0
				ds.detailVM.ScrollDown()
			}
		})),
		tui.On(tui.Rune('k'), dirty(func(ke tui.KeyEvent) {
			if ds.detailVM != nil {
				ds.lastKey = 0
				ds.detailVM.ScrollUp()
			}
		})),
		tui.On(tui.Rune('d'), dirty(func(ke tui.KeyEvent) {
			if ds.detailVM != nil {
				ds.lastKey = 0
				ds.detailVM.PageDown()
			}
		})),
		tui.On(tui.Rune('u'), dirty(func(ke tui.KeyEvent) {
			if ds.detailVM != nil {
				ds.lastKey = 0
				ds.detailVM.PageUp()
			}
		})),
		tui.On(tui.Rune('G'), dirty(func(ke tui.KeyEvent) {
			if ds.detailVM != nil {
				ds.lastKey = 0
				ds.detailVM.ScrollToBottom()
			}
		})),
		tui.On(tui.Rune('g'), dirty(func(ke tui.KeyEvent) {
			if ds.lastKey == 'g' {
				if ds.detailVM != nil {
					ds.detailVM.ScrollToTop()
				}
				ds.lastKey = 0
			} else {
				ds.lastKey = 'g'
			}
		})),
		tui.On(tui.KeyEscape, dirty(func(ke tui.KeyEvent) {
			goBack()
		})),
		tui.On(tui.KeyBackspace, dirty(func(ke tui.KeyEvent) {
			goBack()
		})),
	}
}

// openDetail switches to detail mode for the selected debate.
func (ds *DebatesScreen) openDetail() {
	selected := ds.listVM.SelectedDebate()
	if selected == nil {
		return
	}

	ds.mode = modeDetail
	ds.lastKey = 0

	if ds.store != nil {
		detail := ds.store.DebateDetail(selected.ID)
		if detail != nil {
			ds.detailVM = views.NewDebateDetailViewModel(detail)
			return
		}
	}

	// No detail data cached — trigger an async fetch.
	ds.detailVM = nil
	if ds.fetcher != nil {
		ds.fetcher.FetchDebateDetail(selected.ID)
	}
}

// debateColumnCountForWidth returns the number of table columns for a given terminal width.
func debateColumnCountForWidth(width int) int {
	switch {
	case width >= 120:
		return 5
	case width >= 80:
		return 4
	default:
		return 3
	}
}

// debateStatusForeground maps a color name string to a tui.Color.
func debateStatusForeground(colorName string) tui.Color {
	switch colorName {
	case "cyan":
		return tui.Cyan
	case "red":
		return tui.Red
	case "green":
		return tui.Green
	case "yellow":
		return tui.Yellow
	case "dim":
		return tui.White
	case "white":
		return tui.White
	default:
		return tui.White
	}
}

// markdownLineStyle maps a markdown LineKind to a tui.Style for native rendering.
func markdownLineStyle(kind markdown.LineKind) tui.Style {
	switch kind {
	case markdown.KindH1:
		return tui.NewStyle().Bold().Underline()
	case markdown.KindH2:
		return tui.NewStyle().Bold()
	case markdown.KindH3:
		return tui.NewStyle().Italic()
	case markdown.KindCode:
		return tui.NewStyle().Foreground(tui.Cyan)
	case markdown.KindBlockquote:
		return tui.NewStyle().Dim().Italic()
	case markdown.KindRule:
		return tui.NewStyle().Dim()
	case markdown.KindList:
		return tui.NewStyle()
	default:
		return tui.NewStyle()
	}
}

// Ensure DebatesScreen satisfies the interfaces.
var (
	_ tui.Component   = (*DebatesScreen)(nil)
	_ tui.KeyListener = (*DebatesScreen)(nil)
)
