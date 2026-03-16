package components

import (
	"fmt"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/app"
)

// TabBar renders the horizontal tab bar. The active tab is visually
// distinguished with bold styling.
func TabBar(tabs []app.Tab, active app.Tab) *tui.Element {
	row := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Row),
		tui.WithHeight(1),
		tui.WithGap(2),
	)

	for i, tab := range tabs {
		label := fmt.Sprintf("[%d] %s", i+1, tab.String())

		var style tui.Style
		if tab == active {
			style = tui.NewStyle().Bold().Foreground(tui.Cyan).Underline()
		} else {
			style = tui.NewStyle().Dim()
		}

		el := tui.New(
			tui.WithText(label),
			tui.WithTextStyle(style),
		)
		row.AddChild(el)
	}

	return row
}
