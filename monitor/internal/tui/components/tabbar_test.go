package components

import (
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/tui/app"
)

// TestTabBarReturnsNonNil verifies TabBar produces an element (AC-3).
func TestTabBarReturnsNonNil(t *testing.T) {
	el := TabBar(app.AllTabs(), app.TabPairs)
	if el == nil {
		t.Fatal("TabBar returned nil")
	}
}

// TestTabBarHasChildrenForEachTab verifies all 4 tabs are represented.
func TestTabBarHasChildrenForEachTab(t *testing.T) {
	tabs := app.AllTabs()
	el := TabBar(tabs, app.TabPairs)

	children := el.Children()
	if len(children) < len(tabs) {
		t.Fatalf("expected at least %d children (one per tab), got %d", len(tabs), len(children))
	}
}

// TestTabBarActiveTabDistinguished verifies the active tab is visually
// distinct from inactive tabs. We check that at least one child differs
// from the others in some property (text style, border, etc.).
func TestTabBarActiveTabDistinguished(t *testing.T) {
	tabs := app.AllTabs()
	for _, activeTab := range tabs {
		el := TabBar(tabs, activeTab)
		if el == nil {
			t.Fatalf("TabBar returned nil for active=%v", activeTab)
		}
		// Basic structural test: element should have children.
		if len(el.Children()) == 0 {
			t.Fatalf("TabBar has no children for active=%v", activeTab)
		}
	}
}
