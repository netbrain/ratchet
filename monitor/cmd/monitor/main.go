package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netbrain/ratchet-monitor/internal/datasource"
	"github.com/netbrain/ratchet-monitor/internal/handler"
	"github.com/netbrain/ratchet-monitor/internal/pipeline"
	"github.com/netbrain/ratchet-monitor/internal/sse"
	"github.com/netbrain/ratchet-monitor/internal/watcher"
)

func main() {
	dir := envOr("WATCH_DIR", ".")
	addr := envOr("LISTEN_ADDR", ":9100")

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

	mux := http.NewServeMux()
	mux.Handle("/", handler.IndexHandler("templates/index.html"))
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

	// Static file serving.
	mux.Handle("/static/", handler.StaticHandler("static"))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		log.Println("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	log.Printf("watching %s, serving on %s", dir, addr)
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
