# API Contracts — Generative Agent

**Generative agent** for api-contracts pair, **review phase**.

## Role

Update API handlers and datasource to serve Ratchet v2 data. Ensure responses include all v2 fields, maintain backward compat where possible.

## Context

Monitor HTTP endpoints:
- `GET /api/pairs` - pairs with derived status
- `GET /api/debates` - debate metadata
- `GET /api/debates/{id}` - single debate detail with rounds
- `GET /api/plan` - parsed plan.yaml (epic, milestones, current focus)
- `GET /api/status` - derived current milestone and phase
- `GET /api/scores` - score entries from scores.jsonl
- `GET /events` - SSE stream of real-time events

**v2 API Changes:**

`/api/plan`: add `issues` array, `depends_on`, `regressions` counter to milestones; remove top-level `pairs` (moved to issues).

`/api/status`: include current issue (not just milestone/phase); show workspace context if multi-workspace.

`/events` (SSE): include `workspace` field; `issue` field in debate events; `regression_count` in milestone events.

New endpoints (if applicable): `GET /api/workspaces`, `GET /api/models`, `GET /api/resources`.

## Current Implementation

**Datasource** (`internal/datasource/file.go`): `Plan()` returns `*parser.Plan` (needs v2); `Workflow()` returns `*parser.WorkflowConfig` (needs v2); caching needs invalidation for new files.

**Handlers** (`internal/handler/api.go`): `handlePlan` serves plan.yaml; `handleStatus` derives current state; update JSON serialization for v2.

## Strategy

1. Update datasource to read v2 (depends on parser updates)
2. Update handlers to serialize v2 fields
3. Update SSE events with workspace/issue context
4. Add tests verifying v2 schema

## Validation Commands

```bash
go test ./internal/handler/... -v
go test ./internal/datasource/... -v
```

Manual (requires running monitor):
```bash
go run ./cmd/monitor &
curl http://localhost:9100/api/plan | jq .
curl http://localhost:9100/api/status | jq .
```

## Tools

- Read, Grep, Glob, Write, Edit, Bash

## Lessons from Prior Debates

- Implementing same feature across parallel methods (e.g., Pairs and Debates both getting workspace param): diff implementations after writing. Pitfalls: different error paths, inconsistent graceful degradation, missing tests for second method.
- Scan for redundant I/O when adding validation. If two methods read same file, extract shared helper.
- After new error path in one method, check peer methods for same gap. Consistency critical.

## Success Criteria

- All endpoints return v2 structures
- SSE events include workspace/issue context
- Tests verify v2 fields
- Error handling for missing v2 files
- Adversarial confirms contracts correct
