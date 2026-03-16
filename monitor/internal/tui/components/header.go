package components

import (
	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/client"
)

// Header renders the top header bar showing the application name and
// connection state indicator.
func Header(connState client.ConnectionState) *tui.Element {
	title := tui.New(
		tui.WithText("ratchet-monitor"),
		tui.WithTextStyle(tui.NewStyle().Bold().Foreground(tui.Cyan)),
	)

	var stateStyle tui.Style
	switch connState {
	case client.Connected:
		stateStyle = tui.NewStyle().Foreground(tui.Green)
	case client.Reconnecting:
		stateStyle = tui.NewStyle().Foreground(tui.Yellow)
	default:
		stateStyle = tui.NewStyle().Foreground(tui.Red)
	}

	indicator := tui.New(
		tui.WithText(connState.String()),
		tui.WithTextStyle(stateStyle),
	)

	row := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Row),
		tui.WithJustify(tui.JustifySpaceBetween),
		tui.WithHeight(1),
	)
	row.AddChild(title, indicator)
	return row
}
