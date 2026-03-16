package app

import (
	"context"
	"sync"
	"testing"
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
