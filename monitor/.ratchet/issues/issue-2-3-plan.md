# Issue 2-3: SSE Events v2

## Objective
Update Server-Sent Events (SSE) implementation to include v2 context fields (workspace and issue) in event payloads.

## Background
The current SSE implementation (`internal/handler/sse.go`) streams events to clients but doesn't include v2 metadata. For multi-workspace monitoring and issue-specific tracking, events need to carry workspace and issue context.

## Dependencies
- Issue 2-1 (Datasource v2) - DONE: Provides v2 data structures and issue tracking

## Scope

### Event Structure Updates
The `events.Event` struct needs v2 fields:
- `Workspace string` - identifies which workspace this event belongs to
- `Issue string` - identifies the issue context (for debate/phase events)

### Pipeline Integration
The event pipeline (`internal/pipeline/pipeline.go`) needs to:
- Extract workspace context from file paths
- Derive issue reference from debate directory structure
- Populate v2 fields when creating events

### SSE Handler
The SSE handler already serializes events to JSON. No changes needed if the Event struct is updated correctly.

## Test Strategy (TDD)

### Test Phase
1. Add tests for v2 event field serialization (`events_v2_test.go`)
   - Test workspace field in events
   - Test issue field in debate events
   - Test omitempty behavior (backward compatibility)
   - Test JSON roundtrip

2. Add pipeline tests (`pipeline_v2_test.go`)
   - Test workspace extraction from file paths
   - Test issue extraction from debate directories
   - Test event enrichment with v2 fields

### Build Phase
1. Update `events.Event` struct with v2 fields
2. Update pipeline to populate v2 fields
3. Ensure existing SSE tests pass

### Review Phase
API contracts review will verify:
- SSE events include workspace/issue when appropriate
- Events maintain backward compatibility (omitempty)
- JSON schema matches v2 spec

## Files to Modify
- `internal/events/events.go` - add v2 fields to Event struct
- `internal/events/events_v2_test.go` - test v2 serialization
- `internal/pipeline/pipeline.go` - populate v2 fields
- `internal/pipeline/pipeline_v2_test.go` - test v2 enrichment
- `internal/datasource/file.go` - ensure workspace context is available
- `internal/datasource/file_test.go` - test workspace context

## Success Criteria
- All tests pass (existing + new v2 tests)
- SSE events include workspace field when available
- Debate events include issue field
- JSON serialization omits empty v2 fields (backward compatibility)
- No breaking changes to existing SSE clients

## Non-Goals
- No new SSE endpoints (use existing `/events`)
- No changes to SSE reconnection logic (Last-Event-ID already supported)
- No changes to event broker implementation
