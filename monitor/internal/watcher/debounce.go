package watcher

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/netbrain/ratchet-monitor/internal/events"
)

// Option configures a Watcher.
type Option func(*watcherConfig)

type watcherConfig struct {
	debounce time.Duration
}

// WithDebounce sets the debounce window for the watcher. Multiple events for
// the same file within this window are coalesced into a single event.
func WithDebounce(d time.Duration) Option {
	return func(cfg *watcherConfig) {
		cfg.debounce = d
	}
}

// NewWithOptions creates a Watcher that recursively watches the given
// directory, with optional configuration such as debouncing.
func NewWithOptions(dir string, opts ...Option) (*Watcher, error) {
	cfg := &watcherConfig{}
	for _, o := range opts {
		o(cfg)
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	// Walk the directory tree and add all directories.
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return fsw.Add(path)
		}
		return nil
	})
	if err != nil {
		_ = fsw.Close()
		return nil, fmt.Errorf("walk directory %q: %w", dir, err)
	}

	w := &Watcher{
		fsw:     fsw,
		ch:      make(chan events.Event, 128),
		done:    make(chan struct{}),
		pending: make(map[string]*time.Timer),
	}

	if cfg.debounce > 0 {
		w.debounce = cfg.debounce
	}

	return w, nil
}

// runDebounced is the event loop when debouncing is enabled.
func (w *Watcher) runDebounced(ctx context.Context) {
	var mu sync.Mutex

	defer func() {
		// Stop all pending timers before closing the channel. We hold mu
		// so no timer callback can be in-flight between the Stop calls and
		// the channel close.
		mu.Lock()
		for path, t := range w.pending {
			t.Stop()
			delete(w.pending, path)
		}
		mu.Unlock()

		close(w.done)
		close(w.ch)
	}()

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

			mu.Lock()
			if t, exists := w.pending[e.Path]; exists {
				t.Stop()
			}
			captured := e
			w.pending[e.Path] = time.AfterFunc(w.debounce, func() {
				mu.Lock()
				delete(w.pending, captured.Path)
				mu.Unlock()

				// Check done to avoid sending on a closed channel. The
				// deferred cleanup stops all timers while holding mu, so
				// this select is only reachable if the timer fired before
				// cleanup. In that case done is still open, and we can
				// safely race on ctx.Done.
				select {
				case <-w.done:
					return
				default:
				}

				select {
				case w.ch <- captured:
				case <-ctx.Done():
				case <-w.done:
				}
			})
			mu.Unlock()

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			slog.Error("fsnotify watcher error", "error", err)
		}
	}
}
