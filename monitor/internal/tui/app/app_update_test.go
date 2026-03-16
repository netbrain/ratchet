package app

import (
	"sync"
	"sync/atomic"
	"testing"
)

// --- SetOnUpdate callback ---

func TestSetOnUpdate(t *testing.T) {
	a := &App{}
	var called atomic.Bool
	a.SetOnUpdate(func() {
		called.Store(true)
	})
	a.notifyUpdate()
	if !called.Load() {
		t.Fatal("SetOnUpdate callback was not called by notifyUpdate")
	}
}

func TestNotifyUpdateNilCallback(t *testing.T) {
	a := &App{}
	a.notifyUpdate() // must not panic when onUpdate is nil
}

func TestSetOnUpdateConcurrent(t *testing.T) {
	a := &App{}
	var count atomic.Int64
	a.SetOnUpdate(func() {
		count.Add(1)
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.notifyUpdate()
		}()
	}
	wg.Wait()

	if count.Load() != 100 {
		t.Fatalf("expected 100 notifications, got %d", count.Load())
	}
}
