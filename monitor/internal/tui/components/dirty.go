package components

import tui "github.com/grindlemire/go-tui"

// dirty wraps a key handler to mark the tui.App as dirty after execution,
// triggering a re-render. Without this, key handlers that modify component
// state have no visible effect because go-tui only re-renders when dirty.
func dirty(fn func(tui.KeyEvent)) func(tui.KeyEvent) {
	return func(ke tui.KeyEvent) {
		fn(ke)
		if ke.App() != nil {
			ke.App().MarkDirty()
		}
	}
}
