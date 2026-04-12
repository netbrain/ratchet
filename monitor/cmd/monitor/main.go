package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	monitor "github.com/netbrain/ratchet-monitor"
	"github.com/netbrain/ratchet-monitor/internal/datasource"
	"github.com/netbrain/ratchet-monitor/internal/handler"
	"github.com/netbrain/ratchet-monitor/internal/pipeline"
	"github.com/netbrain/ratchet-monitor/internal/sse"
	"github.com/netbrain/ratchet-monitor/internal/watcher"
)

func main() {
	var (
		dir       string
		addr      string
		workspace string
		verbose   bool
	)

	flag.StringVar(&dir, "dir", "", "path to .ratchet directory (env: WATCH_DIR, default: \".\")")
	flag.StringVar(&dir, "d", "", "path to .ratchet directory (shorthand)")
	flag.StringVar(&addr, "addr", "", "listen address (env: LISTEN_ADDR, default: \":9100\")")
	flag.StringVar(&addr, "a", "", "listen address (shorthand)")
	flag.StringVar(&workspace, "workspace", "", "workspace name filter")
	flag.StringVar(&workspace, "w", "", "workspace name filter (shorthand)")
	flag.BoolVar(&verbose, "verbose", false, "enable debug logging")
	flag.BoolVar(&verbose, "v", false, "enable debug logging (shorthand)")
	flag.Parse()

	// CLI flag > env var > default
	if dir == "" {
		dir = envOr("WATCH_DIR", ".")
	}
	if addr == "" {
		addr = envOr("LISTEN_ADDR", ":9100")
	}

	// Configure slog level based on verbose flag.
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})))

	slog.Debug("configuration",
		"dir", dir,
		"addr", addr,
		"workspace", workspace,
		"verbose", verbose,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start file watcher.
	w, err := watcher.New(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("directory %q does not exist; create it or set --dir / WATCH_DIR to a valid .ratchet directory", dir)
		}
		log.Fatalf("failed to create watcher for %q: %v", dir, err)
	}
	defer w.Close()

	var brokerOpts []sse.BrokerOption
	if v := os.Getenv("RATCHET_RING_BUFFER_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("invalid RATCHET_RING_BUFFER_SIZE %q: %v", v, err)
		}
		if n < sse.MinBufferSize || n > sse.MaxBufferSize {
			log.Fatalf("RATCHET_RING_BUFFER_SIZE must be between %d and %d, got %d",
				sse.MinBufferSize, sse.MaxBufferSize, n)
		}
		brokerOpts = append(brokerOpts, sse.WithBufferSize(n))
	}

	broker := sse.NewBroker(brokerOpts...)
	defer broker.Close()

	// Create the enrichment pipeline instead of a direct watcher->broker pipe.
	pipe := pipeline.New(w, broker, dir)
	go pipe.Run(ctx)

	// Run the watcher event loop.
	go w.Run(ctx)

	// Create the file-backed data source for API handlers.
	ds := datasource.NewFileDataSource(dir)

	// Prepare embedded filesystems for templates and static assets.
	templatesFS, err := fs.Sub(monitor.TemplatesFS, "templates")
	if err != nil {
		log.Fatalf("failed to create templates sub-FS: %v", err)
	}
	staticFS, err := fs.Sub(monitor.StaticFS, "static")
	if err != nil {
		log.Fatalf("failed to create static sub-FS: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler.IndexHandler(templatesFS, "index.html"))
	mux.Handle("/health", handler.HealthHandler())
	mux.Handle("/events", handler.SSEHandler(broker))

	// REST API endpoints.
	mux.Handle("/api/pairs", handler.PairsHandler(ds))
	mux.Handle("/api/debates", handler.DebatesHandler(ds))
	mux.Handle("/api/debates/{id}", handler.DebateDetailHandler(ds))
	mux.Handle("/api/plan", handler.PlanHandler(ds))
	mux.Handle("/api/status", handler.StatusHandler(ds))
	mux.Handle("/api/scores", handler.ScoresHandler(ds))
	mux.Handle("/api/workspaces", handler.WorkspacesHandler(ds))

	// Static file serving from embedded FS.
	mux.Handle("/static/", handler.StaticHandler(staticFS))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	if workspace != "" {
		fmt.Fprintf(os.Stderr, "watching %s (workspace=%s), serving on %s\n", dir, workspace, addr)
	} else {
		fmt.Fprintf(os.Stderr, "watching %s, serving on %s\n", dir, addr)
	}
	slog.Info("server starting", "dir", dir, "addr", addr, "workspace", workspace, "ring_buffer_size", broker.BufferSize())
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
