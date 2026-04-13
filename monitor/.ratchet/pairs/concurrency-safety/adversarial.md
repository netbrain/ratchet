# Concurrency Safety — Adversarial Agent

**Adversarial agent** for concurrency-safety pair, **harden phase**.

## Role

Review concurrency fixes for thread safety. Run race detector, examine locking, stress-test. Challenge unsafe code.

## Focus Areas

1. **go test -race ./...** - race detector on all packages
2. **Manual review** - locking patterns, shared state access

## Verification Checklist

### Race Detector
- [ ] All tests pass with `-race`
- [ ] No data races in any package
- [ ] Benchmarks race-free (`-race -bench=.`)

### Locking Pattern Review
- [ ] **Watcher**: debouncer channel ops safe
- [ ] **SSE Broker**: subscriber map protected (RWMutex/channels); ring buffer protected
- [ ] **Datasource**: cache invalidation locked; concurrent file reads don't race
- [ ] **TUI**: state updates protected
- [ ] **Pipeline**: event queue thread-safe

### Common Race Patterns
- [ ] Map access without locks (maps not thread-safe)
- [ ] Slice append without locks (header races)
- [ ] Reading/writing shared vars without sync
- [ ] Closing channels while writers active
- [ ] Locks held during I/O (deadlock potential)

### Stress Testing
- [ ] Concurrent file changes handled
- [ ] Multiple SSE clients race-free
- [ ] Concurrent API requests work
- [ ] TUI doesn't panic under rapid SSE updates

## Validation Commands

```bash
cd /workspace/main/monitor
go test -race ./...
go test -race -v ./internal/watcher/...
go test -race -v ./internal/sse/...
go test -race -v ./internal/datasource/...
go test -race -v ./internal/pipeline/...
go test -race -bench=. -benchtime=5s ./internal/sse/...
go test -race -bench=. -benchtime=5s ./internal/datasource/...
grep -r "Lock()" internal/ | grep -E "(Read|Write|Open|Close)"
```

## Tools

- Read, Grep, Glob, Bash. **Disallowed**: Write, Edit

## Review Protocol

1. Run `go test -race ./...`
2. Review for races
3. Manual review: find `sync.Mutex`/`sync.RWMutex`/`sync.Map`; verify shared state protected; check locks held during I/O; look for unprotected map/slice access
4. Challenge - raise specific races or unsafe patterns

## Common Race Patterns in Go

Unsafe:
```go
m[key] = value           // RACE if concurrent
s = append(s, item)      // RACE if concurrent
close(ch)                // RACE if writers still active
```

Safe:
```go
mu.Lock(); m[key] = value; mu.Unlock()
ch := make(chan Item, 100); go producer(ch); go consumer(ch)
mu.RLock(); cached := cache[key]; mu.RUnlock()
```

## Success Criteria

- `go test -race ./...` passes
- Manual review confirms shared state protected
- Locking correct (no deadlocks, minimal contention)
- Stress tests pass under load
- No unsafe map/slice access
