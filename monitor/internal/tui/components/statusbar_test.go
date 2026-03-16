package components

import (
	"testing"
)

// TestStatusBarReturnsNonNil verifies StatusBar produces an element (AC-7).
func TestStatusBarReturnsNonNil(t *testing.T) {
	el := StatusBar("test status line", "q:quit")
	if el == nil {
		t.Fatal("StatusBar returned nil")
	}
}

// TestStatusBarContainsText verifies the status bar has children for
// status text and key hints.
func TestStatusBarContainsText(t *testing.T) {
	el := StatusBar("connected", "1-4:tab  q:quit")

	// The element should have two children (left status, right hints).
	children := el.Children()
	if len(children) < 2 {
		t.Fatalf("expected at least 2 children (status + hints), got %d", len(children))
	}
}

// TestStatusBarEmptyInput verifies StatusBar handles empty input gracefully.
func TestStatusBarEmptyInput(t *testing.T) {
	el := StatusBar("", "")
	if el == nil {
		t.Fatal("StatusBar returned nil for empty input")
	}
}
