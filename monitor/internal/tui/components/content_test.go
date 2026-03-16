package components

import (
	"testing"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/app"
)

// TestContentReturnsNonNil verifies Content produces an element for every tab (AC-6).
func TestContentReturnsNonNil(t *testing.T) {
	for _, tab := range app.AllTabs() {
		el := Content(tab)
		if el == nil {
			t.Fatalf("Content(%v) returned nil", tab)
		}
	}
}

// TestContentShowsTabSpecificPlaceholder verifies each tab produces
// different content text.
func TestContentShowsTabSpecificPlaceholder(t *testing.T) {
	seen := make(map[string]app.Tab)

	for _, tab := range app.AllTabs() {
		el := Content(tab)
		text := collectText(el)
		if text == "" {
			t.Fatalf("Content(%v) produced no text", tab)
		}
		if prev, exists := seen[text]; exists {
			t.Fatalf("Content(%v) and Content(%v) produce identical text %q", prev, tab, text)
		}
		seen[text] = tab
	}
}

// collectText gathers text from an element and its direct children.
func collectText(el *tui.Element) string {
	if el == nil {
		return ""
	}
	text := el.Text()
	for _, child := range el.Children() {
		text += child.Text()
	}
	return text
}
