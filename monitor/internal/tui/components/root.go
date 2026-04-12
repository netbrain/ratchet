// Package components provides TUI view components for ratchet-monitor.
package components

import (
	"fmt"

	tui "github.com/grindlemire/go-tui"
	"github.com/netbrain/ratchet-monitor/internal/tui/app"
	"github.com/netbrain/ratchet-monitor/internal/tui/client"
)

// Root is the top-level TUI component. It implements tui.Component and
// tui.KeyListener.
type Root struct {
	app      *app.App
	showHelp bool
}

// NewRoot creates a new Root component wrapping the given App.
// If a is nil, an empty App with defaults is used to prevent nil panics
// in Render and KeyMap. The fallback App has no Client or Store and
// must not be used with Start() or any method requiring a live connection.
func NewRoot(a *app.App) *Root {
	if a == nil {
		a = &app.App{}
	}
	return &Root{app: a}
}

// Render builds the element tree: header, tab bar, content area, status bar.
func (r *Root) Render(tuiApp *tui.App) *tui.Element {
	var connState client.ConnectionState
	var statusText string
	var workspace string
	if r.app.Store != nil {
		connState = r.app.Store.ConnectionState()
		statusText = r.app.StatusLine()
		workspace = r.app.Store.CurrentWorkspace()
	}

	header := Header(connState, workspace)

	// Stale data banner when disconnected/reconnecting
	var staleBanner *tui.Element
	if r.app.Store != nil {
		switch connState {
		case client.Disconnected:
			staleBanner = tui.New(
				tui.WithText("Disconnected — data may be stale"),
				tui.WithTextStyle(tui.NewStyle().Foreground(tui.Yellow)),
				tui.WithHeight(1),
			)
		case client.Reconnecting:
			staleBanner = tui.New(
				tui.WithText("Reconnecting..."),
				tui.WithTextStyle(tui.NewStyle().Foreground(tui.Yellow)),
				tui.WithHeight(1),
			)
		}
	}

	tabBar := TabBar(app.AllTabs(), r.app.ActiveTab)

	// Mount tab-specific screen components when tui.App is available (live mode).
	// Fall back to static placeholder when tui.App is nil (tests).
	var content *tui.Element
	switch {
	case tuiApp != nil && r.app.Store != nil && r.app.ActiveTab == app.TabPairs:
		content = tuiApp.Mount(r, int(r.app.ActiveTab), func() tui.Component {
			return NewPairsScreen(r.app.Store)
		})
	case tuiApp != nil && r.app.Store != nil && r.app.ActiveTab == app.TabDebates:
		content = tuiApp.Mount(r, int(r.app.ActiveTab), func() tui.Component {
			return NewDebatesScreen(r.app.Store, r.app)
		})
	case tuiApp != nil && r.app.Store != nil && r.app.ActiveTab == app.TabScores:
		content = tuiApp.Mount(r, int(r.app.ActiveTab), func() tui.Component {
			return NewScoresScreen(r.app.Store)
		})
	case tuiApp != nil && r.app.Store != nil && r.app.ActiveTab == app.TabEpic:
		content = tuiApp.Mount(r, int(r.app.ActiveTab), func() tui.Component {
			return NewEpicScreen(r.app.Store)
		})
	default:
		content = Content(r.app.ActiveTab)
	}

	hints := fmt.Sprintf("1-%d:tab  Tab/S-Tab:cycle  w:workspace  j/k:nav  /:filter  ?:help  q:quit", len(app.AllTabs()))
	statusBar := StatusBar(statusText, hints)

	root := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Column),
	)
	root.AddChild(header)
	if staleBanner != nil {
		root.AddChild(staleBanner)
	}
	root.AddChild(tabBar, content)

	// Help overlay
	if r.showHelp {
		helpEl := r.buildHelpOverlay()
		root.AddChild(helpEl)
	}

	root.AddChild(statusBar)
	return root
}

// KeyMap returns the key bindings for the root component.
func (r *Root) KeyMap() tui.KeyMap {
	km := tui.KeyMap{
		// Tab / Shift+Tab: cycle tabs
		tui.On(tui.KeyTab, dirty(func(ke tui.KeyEvent) {
			r.app.NextTab()
		})),
		tui.On(tui.KeyTab.Shift(), dirty(func(ke tui.KeyEvent) {
			r.app.PrevTab()
		})),

		// Quit — no dirty() needed, these exit the app.
		tui.On(tui.Rune('q'), func(ke tui.KeyEvent) {
			r.app.Shutdown()
			if ke.App() != nil {
				ke.App().Stop()
			}
		}),
		tui.On(tui.Rune('c').Ctrl(), func(ke tui.KeyEvent) {
			r.app.Shutdown()
			if ke.App() != nil {
				ke.App().Stop()
			}
		}),
	}

	// Help toggle
	km = append(km, tui.On(tui.Rune('?'), dirty(func(ke tui.KeyEvent) {
		r.showHelp = !r.showHelp
	})))

	// Escape: dismiss help overlay if visible
	km = append(km, tui.On(tui.KeyEscape, dirty(func(ke tui.KeyEvent) {
		if r.showHelp {
			r.showHelp = false
		}
	})))

	// Workspace switcher (v2)
	km = append(km, tui.On(tui.Rune('w'), dirty(func(ke tui.KeyEvent) {
		r.app.CycleWorkspace()
	})))

	// Number keys: direct tab selection, generated from AllTabs()
	// so bindings stay in sync if tabs are reordered or added.
	for i, tab := range app.AllTabs() {
		km = append(km, tui.On(tui.Rune(rune('1'+i)), dirty(func(ke tui.KeyEvent) {
			r.app.SetTab(tab)
		})))
	}

	return km
}

// buildHelpOverlay creates the help content element.
func (r *Root) buildHelpOverlay() *tui.Element {
	help := tui.New(
		tui.WithDisplay(tui.DisplayFlex),
		tui.WithDirection(tui.Column),
		tui.WithFlexGrow(1),
	)

	title := tui.New(
		tui.WithText("Keyboard help"),
		tui.WithTextStyle(tui.NewStyle().Bold()),
		tui.WithHeight(1),
	)
	help.AddChild(title)

	bindings := []struct{ key, desc string }{
		{"1-4", "Switch to tab"},
		{"Tab / Shift+Tab", "Cycle tabs"},
		{"w", "Switch workspace"},
		{"j / k", "Navigate list"},
		{"G", "Jump to last"},
		{"g g", "Jump to first"},
		{"/", "Filter (Pairs/Debates)"},
		{"Enter", "Open detail (Debates)"},
		{"Esc", "Back / dismiss help"},
		{"n / N", "Next / prev round (Debates detail)"},
		{"F", "Toggle follow mode (Debates detail)"},
		{"?", "Toggle this help"},
		{"q / Ctrl+C", "Quit"},
	}

	for _, b := range bindings {
		line := fmt.Sprintf("  %-20s  %s", b.key, b.desc)
		el := tui.New(
			tui.WithText(line),
			tui.WithHeight(1),
		)
		help.AddChild(el)
	}

	return help
}
