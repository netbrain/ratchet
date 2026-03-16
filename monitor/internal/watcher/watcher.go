package watcher

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/netbrain/ratchet-monitor/internal/events"
)

// Watcher watches a directory tree for file changes and emits events.
type Watcher struct {
	fsw      *fsnotify.Watcher
	ch       chan events.Event
	nextID   atomic.Uint64
	debounce time.Duration
	pending  map[string]*time.Timer

	// done is closed when the event loop exits, preventing timer callbacks
	// from sending on w.ch after it is closed.
	done chan struct{}

	closeOnce sync.Once

	// runOnce ensures Run is only called once.
	runOnce sync.Once
	// runErr is set if Run is called more than once.
	runErr atomic.Bool
}

// New creates a Watcher that recursively watches the given directory.
func New(dir string) (*Watcher, error) {
	return NewWithOptions(dir)
}

// Events returns the channel of file-system events.
func (w *Watcher) Events() <-chan events.Event {
	return w.ch
}

// Run starts the event loop. It blocks until ctx is cancelled or Close is called.
// Run must only be called once; subsequent calls are no-ops.
func (w *Watcher) Run(ctx context.Context) {
	started := false
	w.runOnce.Do(func() {
		started = true
	})
	if !started {
		w.runErr.Store(true)
		slog.Error("watcher.Run called more than once; ignoring")
		return
	}

	if w.debounce > 0 {
		w.runDebounced(ctx)
		return
	}
	defer close(w.done)
	defer close(w.ch)

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			e, valid := w.translate(ev)
			if !valid {
				continue
			}

			w.watchNewDir(ev)

			select {
			case w.ch <- e:
			case <-ctx.Done():
				return
			}
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			slog.Error("fsnotify watcher error", "error", err)
		}
	}
}

// Close stops the underlying fsnotify watcher. It is safe to call multiple times.
func (w *Watcher) Close() error {
	var err error
	w.closeOnce.Do(func() {
		err = w.fsw.Close()
	})
	return err
}

// watchNewDir adds a newly created directory to the watcher.
// It uses Lstat so that symlinks are not followed: Lstat reports
// a symlink's own mode (ModeSymlink), which means IsDir() returns
// false for symlinks to directories, preventing watch loops and
// escapes outside the intended tree.
func (w *Watcher) watchNewDir(ev fsnotify.Event) {
	if !ev.Has(fsnotify.Create) {
		return
	}
	info, err := os.Lstat(ev.Name)
	if err != nil {
		return
	}
	if !info.IsDir() {
		return
	}
	if err := w.fsw.Add(ev.Name); err != nil {
		slog.Error("failed to watch new directory", "path", ev.Name, "error", err)
	}
}

func (w *Watcher) translate(ev fsnotify.Event) (events.Event, bool) {
	var t events.EventType
	switch {
	case ev.Has(fsnotify.Create):
		t = events.FileCreated
	case ev.Has(fsnotify.Write):
		t = events.FileModified
	case ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename):
		t = events.FileDeleted
	default:
		return events.Event{}, false
	}

	id := w.nextID.Add(1)
	return events.Event{
		ID:        id,
		Type:      t,
		Path:      ev.Name,
		Timestamp: time.Now(),
	}, true
}
