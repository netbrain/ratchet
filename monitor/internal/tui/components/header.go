package components

import (
	"fmt"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/client"
)

// Header renders the top header bar showing the application name,
// active workspace (if any), and connection state indicator.
func Header(connState client.ConnectionState, workspace string) *tui.Element {
	titleText := "ratchet-monitor"
	if workspace != "" {
		titleText = fmt.Sprintf("ratchet-monitor [%s]", workspace)
	}

	title := tui.New(
		tui.WithText(titleText),
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
