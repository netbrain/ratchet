package components

import (
	tui "github.com/grindlemire/go-tui"
)

// StatusBar renders the bottom status bar with status text on the left
// and key hints on the right.
func StatusBar(text string, hints string) *tui.Element {
	left := tui.New(
		tui.WithText(text),
		tui.WithTextStyle(tui.NewStyle().Dim()),
	)

	right := tui.New(
		tui.WithText(hints),
		tui.WithTextStyle(tui.NewStyle().Dim()),
	)

	row := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Row),
		tui.WithJustify(tui.JustifySpaceBetween),
		tui.WithHeight(1),
	)
	row.AddChild(left, right)
	return row
}
