package app

import (
	"context"
	"sync"
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// TestShutdownExists verifies that the Shutdown method exists on *App.
// The method should cancel the context created during Start and be safe
// to call multiple times (idempotent).
func TestShutdownCancelsContext(t *testing.T) {
	a := &App{}

	// Give the app a cancel func to work with (simulate Start having been called).
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	// Shutdown should exist and cancel the context.
	a.Shutdown()

	select {
	case <-ctx.Done():
		// success — context was cancelled
	default:
		t.Fatal("Shutdown did not cancel context")
	}
}

// TestShutdownIdempotent verifies Shutdown can be called multiple times
// without panicking.
func TestShutdownIdempotent(t *testing.T) {
	a := &App{}
	_, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	a.Shutdown()
	a.Shutdown() // must not panic
}

// TestShutdownBeforeStart verifies Shutdown is safe to call before Start
// (cancel is nil).
func TestShutdownBeforeStart(t *testing.T) {
	a := &App{}
	a.Shutdown() // must not panic when cancel is nil
}

// TestShutdownConcurrent verifies Shutdown is safe to call from multiple
// goroutines simultaneously (sync.Once protects the cancel call).
func TestShutdownConcurrent(t *testing.T) {
	a := &App{}
	_, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.Shutdown()
		}()
	}
	wg.Wait()
}

// TestSetTabOutOfRange verifies that SetTab ignores invalid tab values.
func TestSetTabOutOfRange(t *testing.T) {
	a := &App{ActiveTab: TabPairs}

	a.SetTab(Tab(99))
	if a.ActiveTab != TabPairs {
		t.Fatalf("SetTab(99) should be ignored, got %v", a.ActiveTab)
	}

	a.SetTab(Tab(-1))
	if a.ActiveTab != TabPairs {
		t.Fatalf("SetTab(-1) should be ignored, got %v", a.ActiveTab)
	}
}

// TestNextTabFromInvalidResets verifies NextTab resets to first tab
// if ActiveTab is somehow out of range.
func TestNextTabFromInvalidResets(t *testing.T) {
	a := &App{ActiveTab: Tab(99)}
	a.NextTab()
	if a.ActiveTab != TabPairs {
		t.Fatalf("NextTab from invalid should reset to TabPairs, got %v", a.ActiveTab)
	}
}

// TestPrevTabFromInvalidResets verifies PrevTab resets to last tab
// if ActiveTab is somehow out of range.
func TestPrevTabFromInvalidResets(t *testing.T) {
	a := &App{ActiveTab: Tab(99)}
	a.PrevTab()
	if a.ActiveTab != TabEpic {
		t.Fatalf("PrevTab from invalid should reset to TabEpic, got %v", a.ActiveTab)
	}
}

// TestStatusLineNilStore verifies StatusLine returns empty string when Store is nil.
func TestStatusLineNilStore(t *testing.T) {
	a := &App{}
	if got := a.StatusLine(); got != "" {
		t.Fatalf("StatusLine with nil Store should return empty string, got %q", got)
	}
}

// TestTabStringUnknown verifies that unknown Tab values return "?".
func TestTabStringUnknown(t *testing.T) {
	if got := Tab(99).String(); got != "?" {
		t.Fatalf("unknown Tab.String() should return \"?\", got %q", got)
	}
}

// TestCycleWorkspacePopulated verifies CycleWorkspace cycles through loaded workspaces.
func TestCycleWorkspacePopulated(t *testing.T) {
	a := &App{Store: state.NewStore()}
	a.Store.SetWorkspaces([]string{"frontend", "backend", "infra"})
	a.Store.SetCurrentWorkspace("frontend")

	a.CycleWorkspace()
	if got := a.Store.CurrentWorkspace(); got != "backend" {
		t.Fatalf("after first cycle: got %q, want %q", got, "backend")
	}

	a.CycleWorkspace()
	if got := a.Store.CurrentWorkspace(); got != "infra" {
		t.Fatalf("after second cycle: got %q, want %q", got, "infra")
	}

	a.CycleWorkspace()
	if got := a.Store.CurrentWorkspace(); got != "frontend" {
		t.Fatalf("after third cycle (wrap): got %q, want %q", got, "frontend")
	}
}

// TestCycleWorkspaceEmpty verifies CycleWorkspace is a no-op with no workspaces.
func TestCycleWorkspaceEmpty(t *testing.T) {
	a := &App{Store: state.NewStore()}
	a.CycleWorkspace() // must not panic
	if got := a.Store.CurrentWorkspace(); got != "" {
		t.Fatalf("expected empty workspace, got %q", got)
	}
}

// TestStatusLineWithWorkspace verifies workspace appears in the status line.
func TestStatusLineWithWorkspace(t *testing.T) {
	a := &App{Store: state.NewStore()}
	a.Store.SetCurrentWorkspace("frontend")

	line := a.StatusLine()
	if !contains(line, "WS:frontend") {
		t.Fatalf("status line should contain 'WS:frontend', got %q", line)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
