package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWatcher_DoubleClose(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close must not panic or return an error.
	if err := w.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestWatcher_ConcurrentDoubleClose(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.Close(); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent Close returned error: %v", err)
	}
}

func TestWatcher_CloseBeforeRun(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Close before Run starts -- Run should exit promptly because
	// fsnotify channels will be closed.
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Good, Run exited.
	case <-ctx.Done():
		t.Fatal("Run did not exit after Close")
	}
}

func TestWatcher_RunCalledTwice(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(t.Context())

	// First Run in a goroutine.
	go w.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Second Run should be a no-op and not panic.
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Second Run returned immediately.
	case <-time.After(2 * time.Second):
		t.Fatal("second Run did not return")
	}

	cancel()
}

func TestWatcher_WatchedDirDeleted(t *testing.T) {
	dir := t.TempDir()
	// Create a subdirectory to watch (so we can delete it).
	watchDir := filepath.Join(dir, "watched")
	if err := os.Mkdir(watchDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	w, err := New(watchDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go w.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Delete the watched directory.
	if err := os.RemoveAll(watchDir); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	// The watcher should continue to run (not panic). We just verify it
	// doesn't crash and can be cancelled cleanly.
	time.Sleep(100 * time.Millisecond)
	cancel()
}

func TestWatcher_NewSubdirectory(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	go w.Run(t.Context())
	time.Sleep(50 * time.Millisecond)

	// Create a new subdirectory and a file inside it.
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	time.Sleep(100 * time.Millisecond) // Let the watcher pick up the new dir.

	filePath := filepath.Join(subDir, "nested.yaml")
	if err := os.WriteFile(filePath, []byte("nested"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// We may first receive a CREATE event for the subdirectory itself,
	// so keep consuming until we see the nested file.
	timeout := time.After(2 * time.Second)
	for {
		select {
		case ev := <-w.Events():
			if ev.Path == filePath {
				return // success
			}
		case <-timeout:
			t.Fatal("timed out waiting for nested file event")
		}
	}
}

func TestWatcher_SymlinkDirectorySkipped(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	watchDir := filepath.Join(dir, "watched")
	if err := os.Mkdir(watchDir, 0755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	w, err := New(watchDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go w.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Create a symlink to target directory inside watchDir.
	link := filepath.Join(watchDir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	// Write a file in the target directory.
	filePath := filepath.Join(target, "outside.yaml")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// We should NOT get an event for the file in the target directory since
	// the symlinked directory should not be followed.
	timer := time.After(500 * time.Millisecond)
	for {
		select {
		case ev := <-w.Events():
			if ev.Path == filePath {
				t.Errorf("received event for symlinked target file %q; symlinks should not be followed", filePath)
				return
			}
			// Other events (the symlink creation itself) are fine.
		case <-timer:
			return // Good, no event for the target file.
		}
	}
}

func TestWatcher_EventIDsAreUnique(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	go w.Run(t.Context())
	time.Sleep(50 * time.Millisecond)

	// Create several files.
	for i := 0; i < 5; i++ {
		p := filepath.Join(dir, "file"+string(rune('a'+i))+".yaml")
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	seen := make(map[uint64]bool)
	timeout := time.After(2 * time.Second)
	collected := 0
	for collected < 5 {
		select {
		case ev := <-w.Events():
			if seen[ev.ID] {
				t.Errorf("duplicate event ID: %d", ev.ID)
			}
			seen[ev.ID] = true
			collected++
		case <-timeout:
			// May not get all 5, that's OK for this test.
			if collected == 0 {
				t.Fatal("received no events")
			}
			return
		}
	}
}

func TestWatcher_ChannelClosedAfterRun(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	// The events channel should be closed after Run exits.
	select {
	case _, ok := <-w.Events():
		if ok {
			// Got a buffered event, drain and check again.
			for range w.Events() {
			}
		}
		// Channel is closed -- good.
	case <-time.After(time.Second):
		t.Fatal("events channel not closed after Run returned")
	}

	w.Close()
}
