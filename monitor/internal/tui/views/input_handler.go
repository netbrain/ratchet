package views

// Special key constants.
const (
	KeyUp        = "Up"
	KeyDown      = "Down"
	KeyLeft      = "Left"
	KeyRight     = "Right"
	KeyEnter     = "Enter"
	KeyEsc       = "Esc"
	KeyTab       = "Tab"
	KeyBackspace = "Backspace"
	KeySpace     = "Space"
	KeyPgUp      = "PgUp"
	KeyPgDn      = "PgDn"
)

// KeyEvent represents a keyboard event.
type KeyEvent struct {
	Rune    rune
	Special string
	Shift   bool
	Ctrl    bool
	Alt     bool
}

// InputHandler is the interface for handling keyboard input.
type InputHandler interface {
	HandleKey(key KeyEvent) bool
}
