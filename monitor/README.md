# Ratchet Monitor

A real-time observability dashboard for [Ratchet](https://github.com/netbrain/ratchet) workflows. Built with Go and Alpine.js, it watches a `.ratchet/` directory for file changes and streams updates to a browser-based dashboard via Server-Sent Events. The dashboard displays pair status, debate history, score trends, and milestone progress without requiring any build tooling or bundler.

## Prerequisites

- Go 1.25+
- A `.ratchet/` directory (created by Ratchet during workflow execution)

## Quick Start

```bash
# Run from a directory containing .ratchet/
go run ./cmd/monitor

# Or specify the watch directory and listen address
WATCH_DIR=/path/to/.ratchet LISTEN_ADDR=:9100 go run ./cmd/monitor
```

Open `http://localhost:9100` in your browser.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WATCH_DIR` | `.` | Root directory to watch (should contain `.ratchet/` structure) |
| `LISTEN_ADDR` | `:9100` | HTTP server listen address |

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Dashboard HTML (single-page Alpine.js app) |
| GET | `/health` | Health check (returns 200) |
| GET | `/events` | SSE stream of real-time file/domain events |
| GET | `/api/pairs` | List all pairs with derived status from workflow.yaml and debates |
| GET | `/api/debates` | List all debate metadata (sorted by started time descending) |
| GET | `/api/debates/{id}` | Single debate detail with full round content |
| GET | `/api/plan` | Parsed plan.yaml (epic, milestones, current focus) |
| GET | `/api/status` | Derived current milestone and phase from plan.yaml |
| GET | `/api/scores` | Score entries from scores.jsonl (optional `?pair=` filter) |
| GET | `/static/*` | Vendored static assets (Alpine.js, marked.js) |

## Architecture

```
.ratchet/ files
    |
    v
[Watcher] -- fsnotify recursive watch, debounced events
    |
    v
[Pipeline] -- reads + parses changed files, classifies into domain events
    |
    v
[Broker] -- fans out events to all SSE subscribers (ring buffer for replay)
    |
    v
[SSE Handler] -- streams events to browser (Last-Event-ID reconnect support)

[FileDataSource] -- reads .ratchet/ files on demand for REST API
    |
    v
[API Handlers] -- /api/pairs, /api/debates, /api/scores, etc.
    |
    v
[Dashboard] -- Alpine.js single-file app, no build step
```

**Data flow:**
- **Real-time path:** Watcher detects file changes, Pipeline enriches with parsed content and domain event types, Broker fans out to SSE clients, dashboard updates reactively.
- **REST path:** FileDataSource reads and parses `.ratchet/` files on each request. The dashboard fetches REST endpoints on initial load and on SSE reconnection.

**Key packages:**
- `internal/watcher` -- fsnotify wrapper with debouncing and recursive directory watching
- `internal/pipeline` -- event enrichment (parse + classify)
- `internal/classifier` -- maps file paths to domain event types
- `internal/parser` -- parsers for workflow.yaml, plan.yaml, project.yaml, meta.json, scores.jsonl
- `internal/sse` -- pub/sub broker with ring buffer and Last-Event-ID replay
- `internal/datasource` -- file-backed read-only data source for API handlers
- `internal/handler` -- HTTP handlers (REST, SSE, health, static, index)
- `internal/events` -- event types and domain constants

## Development

```bash
# Run all tests
make test

# Run tests with race detector
go test -race ./...

# Build
make build

# Clean
make clean
```
