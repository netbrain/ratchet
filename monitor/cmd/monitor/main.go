package main

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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
	flagDir := flag.String("dir", "", "path to .ratchet directory (env: WATCH_DIR, default: \".\")")
	flag.StringVar(flagDir, "d", "", "path to .ratchet directory (shorthand)")

	flagAddr := flag.String("addr", "", "listen address (env: LISTEN_ADDR, default: \":9100\")")
	flag.StringVar(flagAddr, "a", "", "listen address (shorthand)")

	flagWorkspace := flag.String("workspace", "", "workspace name for multi-workspace filtering")
	flag.StringVar(flagWorkspace, "w", "", "workspace name (shorthand)")

	flagVerbose := flag.Bool("verbose", false, "enable debug logging")
	flag.BoolVar(flagVerbose, "v", false, "enable debug logging (shorthand)")

	flag.Parse()

	// Resolve flags: CLI flag > env var > default.
	dir := resolve(*flagDir, "WATCH_DIR", ".")
	addr := resolve(*flagAddr, "LISTEN_ADDR", ":9100")
	workspace := *flagWorkspace

	// Configure slog level.
	logLevel := slog.LevelInfo
	if *flagVerbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	if workspace != "" {
		slog.Info("workspace filter", "workspace", workspace)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start file watcher.
	w, err := watcher.New(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("WATCH_DIR %q does not exist; create it or set WATCH_DIR to a valid .ratchet directory", dir)
		}
		log.Fatalf("failed to create watcher for %q: %v", dir, err)
	}
	defer w.Close()

	broker := sse.NewBroker()
	defer broker.Close()

	// Create the enrichment pipeline instead of a direct watcher->broker pipe.
	pipe := pipeline.New(w, broker, dir)
	go pipe.Run(ctx)

	// Run the watcher event loop.
	go w.Run(ctx)

	// Create the file-backed data source for API handlers.
	ds := datasource.NewFileDataSource(dir)

	// Prepare embedded filesystems: strip the top-level directory prefix so
	// that template names and static paths don't need the "templates/" or
	// "static/" prefix.
	tmplFS, err := fs.Sub(monitor.TemplateFS, "templates")
	if err != nil {
		log.Fatalf("failed to sub templates FS: %v", err)
	}
	stFS, err := fs.Sub(monitor.StaticFS, "static")
	if err != nil {
		log.Fatalf("failed to sub static FS: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler.IndexHandler(tmplFS, "index.html"))
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
	mux.Handle("/static/", handler.StaticHandler(stFS))

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

	slog.Info("starting monitor", "dir", dir, "addr", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// resolve returns the first non-empty value from: flag, env var, default.
func resolve(flagVal, envKey, defaultVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return defaultVal
}
