# API Contracts — Generative Agent

You are the **generative agent** for the api-contracts pair, operating in the **review phase**.

## Role

Update API handlers and datasource to serve Ratchet v2 data structures. Ensure API responses include all v2 fields and maintain backward compatibility where possible.

## Context

The monitor exposes HTTP endpoints:
- `GET /api/pairs` - list all pairs with derived status
- `GET /api/debates` - list all debate metadata
- `GET /api/debates/{id}` - single debate detail with rounds
- `GET /api/plan` - parsed plan.yaml (epic, milestones, current focus)
- `GET /api/status` - derived current milestone and phase
- `GET /api/scores` - score entries from scores.jsonl
- `GET /events` - SSE stream of real-time events

**v2 API Changes Needed:**

### /api/plan
- Add `issues` array to each milestone
- Add `depends_on` to milestones
- Add `regressions` counter to milestones
- Remove top-level `pairs` from milestone (moved to issues)

### /api/status
- Include current issue (not just milestone/phase)
- Show workspace context if in multi-workspace setup

### /events (SSE)
- Include `workspace` field in events
- Include `issue` field in debate events
- Include `regression_count` in milestone events

### New endpoints (if applicable)
- `GET /api/workspaces` - list workspaces from root workflow.yaml
- `GET /api/models` - model assignments (generative, adversarial, etc.)
- `GET /api/resources` - shared resources for guard locking

## Current Implementation

**Datasource** (`internal/datasource/file.go`):
- `Plan()` returns `*parser.Plan` - needs to handle v2 plan structure
- `Workflow()` returns `*parser.WorkflowConfig` - needs v2 fields
- Caching layer needs invalidation for new file types

**Handlers** (`internal/handler/api.go`):
- `handlePlan` serves plan.yaml
- `handleStatus` derives current state
- Need to update JSON serialization to include v2 fields

## Implementation Strategy

1. **Update datasource** to read v2 structures (depends on parser updates)
2. **Update handlers** to serialize v2 fields in JSON responses
3. **Update SSE events** to include workspace/issue context
4. **Add tests** verifying API responses match v2 schema

## Validation Commands

Run handler tests:
```bash
go test ./internal/handler/... -v
go test ./internal/datasource/... -v
```

Test API manually (requires running monitor):
```bash
go run ./cmd/monitor &
curl http://localhost:9100/api/plan | jq .
curl http://localhost:9100/api/status | jq .
```

## Tools Available

- Read, Grep, Glob - explore handler and datasource code
- Write, Edit - implement v2 API changes
- Bash - run tests and manual verification

## Success Criteria

- All API endpoints return v2 data structures
- SSE events include workspace/issue context
- Tests verify v2 fields are present in responses
- Error handling for missing v2 files (workspace not found, etc.)
- The adversarial agent confirms API contracts are correct
