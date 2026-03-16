package components

import (
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
)

// TestHeaderReturnsNonNil verifies that Header returns a valid element (AC-2).
func TestHeaderReturnsNonNil(t *testing.T) {
	el := Header(client.Connected)
	if el == nil {
		t.Fatal("Header returned nil")
	}
}

// TestHeaderAcceptsAllConnectionStates verifies Header handles every
// ConnectionState value without panicking.
func TestHeaderAcceptsAllConnectionStates(t *testing.T) {
	states := []client.ConnectionState{
		client.Disconnected,
		client.Connected,
		client.Reconnecting,
	}
	for _, s := range states {
		el := Header(s)
		if el == nil {
			t.Fatalf("Header(%v) returned nil", s)
		}
	}
}

// TestHeaderShowsConnectionText verifies the header element tree contains
// text reflecting the connection state. We check the root element or its
// children for non-empty text.
func TestHeaderShowsConnectionText(t *testing.T) {
	el := Header(client.Connected)

	// The header should contain some text content — either directly
	// or in children. We just verify the tree is non-trivially populated.
	if el.Text() == "" && len(el.Children()) == 0 {
		t.Fatal("Header element has no text and no children — expected connection indicator content")
	}
}
