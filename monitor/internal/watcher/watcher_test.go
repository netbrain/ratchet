package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/events"
)

func waitForEvent(t *testing.T, ch <-chan events.Event, wantType events.EventType, timeout time.Duration) events.Event {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			if ev.Type == wantType {
				return ev
			}
			// Consume non-matching events (e.g. directory creates)
		case <-deadline:
			t.Fatalf("timed out waiting for %s event", wantType)
			return events.Event{}
		}
	}
}

func TestWatcher_FileCreated(t *testing.T) {
	dir := t.TempDir()

	w, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	go w.Run(t.Context())

	// Give the watcher a moment to start
	time.Sleep(50 * time.Millisecond)

	filePath := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	ev := waitForEvent(t, w.Events(), events.FileCreated, 2*time.Second)
	if ev.Path != filePath {
		t.Errorf("path: got %q, want %q", ev.Path, filePath)
	}
}

func TestWatcher_FileModified(t *testing.T) {
	dir := t.TempDir()

	// Create file before starting watcher
	filePath := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	w, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	go w.Run(t.Context())

	time.Sleep(50 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(filePath, []byte("world"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	ev := waitForEvent(t, w.Events(), events.FileModified, 2*time.Second)
	if ev.Path != filePath {
		t.Errorf("path: got %q, want %q", ev.Path, filePath)
	}
}

func TestWatcher_FileDeleted(t *testing.T) {
	dir := t.TempDir()

	// Create file before starting watcher
	filePath := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	w, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	go w.Run(t.Context())

	time.Sleep(50 * time.Millisecond)

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	ev := waitForEvent(t, w.Events(), events.FileDeleted, 2*time.Second)
	if ev.Path != filePath {
		t.Errorf("path: got %q, want %q", ev.Path, filePath)
	}
}
