// Package app provides the root TUI component and application wiring.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/netbrain/ratchet-monitor/internal/tui/client"
	"github.com/netbrain/ratchet-monitor/internal/tui/state"
)

// Tab identifies one of the main content tabs.
type Tab int

const (
	TabPairs Tab = iota
	TabDebates
	TabScores
	TabEpic
)

func (t Tab) String() string {
	switch t {
	case TabPairs:
		return "Pairs"
	case TabDebates:
		return "Debates"
	case TabScores:
		return "Scores"
	case TabEpic:
		return "Epic"
	default:
		return "?"
	}
}

// AllTabs returns all available tabs in order.
func AllTabs() []Tab { return []Tab{TabPairs, TabDebates, TabScores, TabEpic} }

// App is the root TUI application. It owns the client, state store, and
// drives the render loop. The actual terminal rendering is delegated to
// go-tui once the dependency is available; this struct provides the
// domain logic independent of the rendering framework.
type App struct {
	Client    *client.Client
	Store     *state.Store
	ActiveTab Tab
	cancel    context.CancelFunc
	once      sync.Once
	onUpdate  func() // called after store data changes to trigger re-render
}

// New creates a new App connected to the given server URL.
func New(serverURL string) *App {
	a := &App{
		Store:     state.NewStore(),
		ActiveTab: TabPairs,
	}

	a.Client = client.NewClient(serverURL,
		client.WithStateCallback(func(s client.ConnectionState) {
			a.Store.SetConnectionState(s)
			a.notifyUpdate()
		}),
	)

	return a
}

// SetOnUpdate registers a callback invoked after store data changes.
// Use this to wire tui.App.QueueUpdate/MarkDirty for live re-rendering.
func (a *App) SetOnUpdate(fn func()) {
	a.onUpdate = fn
}

// notifyUpdate calls the onUpdate callback if set.
func (a *App) notifyUpdate() {
	if a.onUpdate != nil {
		a.onUpdate()
	}
}

// Start initiates the SSE subscription and initial data fetch.
// It returns a cancel function to shut everything down.
func (a *App) Start(ctx context.Context) (context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	// Initial data load.
	go a.loadAll(ctx)

	// SSE subscription — events drive store refresh flags.
	ch, err := a.Client.Subscribe(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("subscribe: %w", err)
	}

	go a.processEvents(ctx, ch)

	return cancel, nil
}

// NextTab advances to the next tab (wraps around).
// If ActiveTab is somehow out of range, it resets to the first tab.
func (a *App) NextTab() {
	tabs := AllTabs()
	cur := int(a.ActiveTab)
	if cur < 0 || cur >= len(tabs) {
		a.ActiveTab = tabs[0]
		return
	}
	a.ActiveTab = tabs[(cur+1)%len(tabs)]
}

// PrevTab moves to the previous tab (wraps around).
// If ActiveTab is somehow out of range, it resets to the last tab.
func (a *App) PrevTab() {
	tabs := AllTabs()
	cur := int(a.ActiveTab)
	if cur < 0 || cur >= len(tabs) {
		a.ActiveTab = tabs[len(tabs)-1]
		return
	}
	idx := cur - 1
	if idx < 0 {
		idx = len(tabs) - 1
	}
	a.ActiveTab = tabs[idx]
}

// SetTab switches to a specific tab. Out-of-range values are ignored.
func (a *App) SetTab(t Tab) {
	tabs := AllTabs()
	if int(t) < 0 || int(t) >= len(tabs) {
		return
	}
	a.ActiveTab = t
}

// loadAll fetches all resources from the REST API.
func (a *App) loadAll(ctx context.Context) {
	if pairs, err := a.Client.Pairs(ctx); err == nil {
		a.Store.SetPairs(pairs)
	} else {
		slog.Warn("failed to fetch pairs", "error", err)
	}

	if debates, err := a.Client.Debates(ctx); err == nil {
		a.Store.SetDebates(debates)
	} else {
		slog.Warn("failed to fetch debates", "error", err)
	}

	if plan, err := a.Client.Plan(ctx); err == nil && plan != nil {
		a.Store.SetPlan(*plan)
	} else if err != nil {
		slog.Warn("failed to fetch plan", "error", err)
	}

	if status, err := a.Client.Status(ctx); err == nil && status != nil {
		a.Store.SetStatus(*status)
	} else if err != nil {
		slog.Warn("failed to fetch status", "error", err)
	}

	if workspaces, err := a.Client.Workspaces(ctx); err == nil {
		names := make([]string, len(workspaces))
		for i, ws := range workspaces {
			names[i] = ws.Name
		}
		a.Store.SetWorkspaces(names)
		if len(names) > 0 && a.Store.CurrentWorkspace() == "" {
			a.Store.SetCurrentWorkspace(names[0])
		}
	} else {
		slog.Warn("failed to fetch workspaces", "error", err)
	}

	a.notifyUpdate()
}

// processEvents reads SSE events and applies them to the store, triggering
// refreshes as needed.
func (a *App) processEvents(ctx context.Context, ch <-chan client.SSEEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if err := a.Store.ApplyEvent(ev); err != nil {
				slog.Warn("failed to apply SSE event", "type", ev.Type, "error", err)
				continue
			}
			a.refreshDirty(ctx)
			a.notifyUpdate()
		}
	}
}

// refreshDirty re-fetches any resources flagged dirty by SSE events.
func (a *App) refreshDirty(ctx context.Context) {
	if a.Store.NeedsRefresh("pairs") {
		if pairs, err := a.Client.Pairs(ctx); err == nil {
			a.Store.SetPairs(pairs)
		} else {
			slog.Debug("failed to refresh pairs", "error", err)
		}
	}
	if a.Store.NeedsRefresh("debates") {
		if debates, err := a.Client.Debates(ctx); err == nil {
			a.Store.SetDebates(debates)
		} else {
			slog.Debug("failed to refresh debates", "error", err)
		}
	}
	if a.Store.NeedsRefresh("scores") {
		if scores, err := a.Client.Scores(ctx, ""); err == nil {
			a.Store.SetScores("", scores)
		} else {
			slog.Debug("failed to refresh scores", "error", err)
		}
	}
	if a.Store.NeedsRefresh("plan") || a.Store.NeedsRefresh("config") {
		if plan, err := a.Client.Plan(ctx); err == nil && plan != nil {
			a.Store.SetPlan(*plan)
		} else if err != nil {
			slog.Debug("failed to refresh plan", "error", err)
		}
	}
}

// FetchDebateDetail fetches a debate's full detail from the REST API
// in the background. On success it stores the result and triggers a re-render.
func (a *App) FetchDebateDetail(id string) {
	go func() {
		ctx := context.Background()
		detail, err := a.Client.Debate(ctx, id)
		if err != nil {
			slog.Warn("failed to fetch debate detail", "id", id, "error", err)
			return
		}
		a.Store.SetDebateDetail(detail)
		a.notifyUpdate()
	}()
}

// Shutdown cancels the context created during Start. It is safe to call
// multiple times (idempotent via sync.Once) and safe to call before Start
// (when cancel is nil).
func (a *App) Shutdown() {
	a.once.Do(func() {
		if a.cancel != nil {
			a.cancel()
		}
	})
}

// StatusLine returns a one-line string summarising connection and focus.
// Returns an empty string if Store is nil.
func (a *App) StatusLine() string {
	if a.Store == nil {
		return ""
	}
	conn := a.Store.ConnectionState().String()
	status := a.Store.Status()
	ws := a.Store.CurrentWorkspace()
	prefix := conn
	if ws != "" {
		prefix = fmt.Sprintf("%s | WS:%s", conn, ws)
	}
	if status.MilestoneName != "" {
		return fmt.Sprintf("[%s] M%d: %s (%s)", prefix, status.MilestoneID, status.MilestoneName, status.Phase)
	}
	return fmt.Sprintf("[%s]", prefix)
}

// CycleWorkspace switches to the next workspace (wraps around).
// If no workspaces are configured, this is a no-op.
func (a *App) CycleWorkspace() {
	if a.Store == nil {
		return
	}
	workspaces := a.Store.Workspaces()
	if len(workspaces) == 0 {
		return
	}
	current := a.Store.CurrentWorkspace()
	idx := -1
	for i, ws := range workspaces {
		if ws == current {
			idx = i
			break
		}
	}
	// Move to next (or first if current not found)
	next := (idx + 1) % len(workspaces)
	a.Store.SetCurrentWorkspace(workspaces[next])
	a.notifyUpdate()
}
