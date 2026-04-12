// Command tui launches the ratchet-monitor terminal UI.
//
// It connects to a running monitor server via REST+SSE and renders a
// fullscreen dashboard with tabs for Pairs, Debates, Scores, and Epic.
//
// Usage:
//
//	ratchet-tui --server localhost:9100
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	tui "github.com/grindlemire/go-tui"
	tuiapp "github.com/netbrain/ratchet-monitor/internal/tui/app"
	"github.com/netbrain/ratchet-monitor/internal/tui/components"
)

func main() {
	server := flag.String("server", "localhost:9100", "monitor server address (host:port)")
	flag.Parse()

	serverURL := fmt.Sprintf("http://%s", *server)

	app := tuiapp.New(serverURL)

	ctx := context.Background()

	cancel, err := app.Start(ctx)
	if err != nil {
		slog.Error("failed to start", "error", err)
		os.Exit(1)
	}
	defer cancel()

	root := components.NewRoot(app)

	tuiApp, err := tui.NewApp(tui.WithRootComponent(root))
	if err != nil {
		slog.Error("failed to create tui app", "error", err)
		os.Exit(1)
	}
	defer func() { _ = tuiApp.Close() }()

	// Wire SSE-driven re-rendering: store changes trigger TUI re-render.
	app.SetOnUpdate(func() {
		tuiApp.QueueUpdate(func() {
			tuiApp.MarkDirty()
		})
	})

	if err := tuiApp.Run(); err != nil {
		slog.Error("tui error", "error", err)
		os.Exit(1)
	}
}
