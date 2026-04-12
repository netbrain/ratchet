package components

import (
	"strings"
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
)

// TestHeaderReturnsNonNil verifies that Header returns a valid element (AC-2).
func TestHeaderReturnsNonNil(t *testing.T) {
	el := Header(client.Connected, "")
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
		el := Header(s, "")
		if el == nil {
			t.Fatalf("Header(%v) returned nil", s)
		}
	}
}

// TestHeaderShowsConnectionText verifies the header element tree contains
// text reflecting the connection state. We check the root element or its
// children for non-empty text.
func TestHeaderShowsConnectionText(t *testing.T) {
	el := Header(client.Connected, "")

	// The header should contain some text content — either directly
	// or in children. We just verify the tree is non-trivially populated.
	if el.Text() == "" && len(el.Children()) == 0 {
		t.Fatal("Header element has no text and no children — expected connection indicator content")
	}
}

// TestHeaderShowsWorkspaceName verifies the header displays the active
// workspace name in brackets when a workspace is set.
func TestHeaderShowsWorkspaceName(t *testing.T) {
	el := Header(client.Connected, "engine")
	text := collectAllText(el)
	if !strings.Contains(text, "[engine]") {
		t.Fatalf("expected header to contain '[engine]', got: %q", text)
	}
}

// TestHeaderNoWorkspaceBrackets verifies the header omits workspace brackets
// when no workspace is set.
func TestHeaderNoWorkspaceBrackets(t *testing.T) {
	el := Header(client.Connected, "")
	text := collectAllText(el)
	if strings.Contains(text, "[") || strings.Contains(text, "]") {
		t.Fatalf("expected no brackets in header without workspace, got: %q", text)
	}
}
