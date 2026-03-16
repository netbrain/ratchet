package components

import (
	"fmt"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/app"
)

// Content renders the main content area for the given tab.
// Currently returns a placeholder element per tab.
func Content(tab app.Tab) *tui.Element {
	text := fmt.Sprintf("%s content placeholder", tab.String())

	el := tui.New(
		tui.WithText(text),
		tui.WithFlexGrow(1),
	)
	return el
}
