package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/events"
)

func TestNewWithOptions_Debounce_RapidWritesSameFile(t *testing.T) {
	dir := t.TempDir()

	// Pre-create the file so modifications are detected.
	filePath := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(filePath, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	w, err := NewWithOptions(dir, WithDebounce(200*time.Millisecond))
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Close()

	go w.Run(t.Context())
	time.Sleep(50 * time.Millisecond)

	// Rapid writes to the same file.
	for i := range 5 {
		if err := os.WriteFile(filePath, []byte("update "+string(rune('0'+i))), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for debounce window to pass.
	time.Sleep(400 * time.Millisecond)

	// Should receive exactly one event (debounced).
	count := 0
	timeout := time.After(500 * time.Millisecond)
	for done := false; !done; {
		select {
		case ev := <-w.Events():
			if ev.Type == events.FileModified || ev.Type == events.FileCreated {
				count++
			}
		case <-timeout:
			done = true
		}
	}

	if count != 1 {
		t.Errorf("expected 1 debounced event, got %d", count)
	}
}

func TestNewWithOptions_Debounce_DifferentFiles(t *testing.T) {
	dir := t.TempDir()

	// Pre-create files.
	for _, name := range []string{"a.yaml", "b.yaml", "c.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("initial"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	w, err := NewWithOptions(dir, WithDebounce(200*time.Millisecond))
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Close()

	go w.Run(t.Context())
	time.Sleep(50 * time.Millisecond)

	// Write to three different files.
	for _, name := range []string{"a.yaml", "b.yaml", "c.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("changed"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce window.
	time.Sleep(400 * time.Millisecond)

	// Should receive separate events for each file.
	paths := make(map[string]int)
	timeout := time.After(500 * time.Millisecond)
	for done := false; !done; {
		select {
		case ev := <-w.Events():
			paths[ev.Path]++
		case <-timeout:
			done = true
		}
	}

	if len(paths) < 3 {
		t.Errorf("expected events for 3 different files, got %d unique paths: %v", len(paths), paths)
	}
}

func TestNewWithOptions_DebounceConfigurable(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(filePath, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Use a very short debounce.
	w, err := NewWithOptions(dir, WithDebounce(50*time.Millisecond))
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Close()

	go w.Run(t.Context())
	time.Sleep(50 * time.Millisecond)

	// Write and wait for debounce.
	if err := os.WriteFile(filePath, []byte("changed"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Event should arrive within 150ms (50ms debounce + margin).
	select {
	case ev := <-w.Events():
		if ev.Path != filePath {
			t.Errorf("path: got %q, want %q", ev.Path, filePath)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("timed out waiting for debounced event with short window")
	}
}
