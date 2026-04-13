# Concurrency Safety — Generative Agent

**Generative agent** for concurrency-safety pair, **harden phase**.

## Role

Ensure monitor is thread-safe under concurrent file updates, API requests, SSE streams. Add race detection tests, fix races, validate locking.

## Context

Concurrent operations:
- **File watcher** detects .ratchet/ changes, triggers pipeline
- **Pipeline** reads/parses files (concurrent with watcher)
- **SSE broker** fans out events to multiple clients
- **Datasource** caches parsed data with invalidation
- **TUI** receives SSE while rendering

**Common Race Conditions:**

1. **File Watcher + Parser** - watcher detects change while parser reads; multiple changes trigger concurrent parses; pipeline processes events while broker reads queue.
2. **SSE Broker** - concurrent subscribe/unsubscribe; events pushed while subscribers list modified; ring buffer read/write races.
3. **Datasource Caching** - cache invalidation while being read; concurrent reads trigger duplicate file loads; parse races with cache updates.
4. **TUI State** - SSE updates state while TUI renders; keyboard input modifies state while display reads.

## Current Implementation

- **Watcher** (`internal/watcher/watcher.go`): fsnotify, debouncing. Check: debouncer/channel ops thread-safe?
- **SSE Broker** (`internal/sse/broker.go`): pub/sub with ring buffer. Check: subscriber map, ring buffer.
- **Datasource** (`internal/datasource/file.go`): caches plan.yaml/workflow.yaml. Check: invalidation, concurrent reads.
- **TUI** (`internal/tui/`): receives SSE. Check: state updates while rendering.

## Strategy

1. Add race detection tests - run with `-race`
2. Identify races - find shared state without locks
3. Fix - mutexes, channels, atomics
4. Add stress tests - concurrent file writes + API + SSE
5. Validate locking - no deadlocks, minimal contention

## Locking Patterns

**Good:** `sync.RWMutex` for read-heavy (caching); channels for producer-consumer (events); `sync.Once` for lazy init; short critical sections.

**Bad:** holding locks during I/O; nested locks (deadlock risk); locks in hot paths (contention).

## Validation Commands

```bash
go test -race ./...
go test -race -run=Stress ./...
go test -race -bench=. -benchtime=10s ./...
```

## Tools

- Read, Grep, Glob, Write, Edit, Bash

## Success Criteria

- All tests pass with `-race`
- Stress tests don't trigger races
- Manual review confirms locking
- No deadlocks under load
- Adversarial confirms thread safety
