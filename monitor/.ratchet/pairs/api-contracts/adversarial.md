# API Contracts — Adversarial Agent

**Adversarial agent** for api-contracts pair, **review phase**.

## Role

Review API handlers serving v2 data. Verify response structures, SSE event completeness, error handling. Run tests and manual checks.

## Focus Areas

1. **Response structure** - all v2 fields present, correct JSON schema
2. **SSE event completeness** - workspace/issue context, full v2 data
3. **Error handling** - proper status codes, error messages

## Verification Checklist

### Response Structure
- [ ] `/api/plan` includes v2 milestone fields: `depends_on`, `regressions`, `issues` array (ref, title, pairs, depends_on, phase_status, files, debates, branch, status)
- [ ] `/api/status` includes current issue + workspace context if multi-workspace
- [ ] `/api/pairs` includes v2 pair fields (max_rounds, models)
- [ ] New endpoints exist (if applicable): `/api/workspaces`, `/api/models`, `/api/resources`

### SSE Event Completeness
- [ ] Debate events include `workspace`, `issue`
- [ ] Milestone events include `regressions` counter
- [ ] Issue events include phase transitions, dependency status
- [ ] Events maintain Last-Event-ID for reconnection

### Error Handling
- [ ] Invalid workspace path → 404
- [ ] Missing plan.yaml → appropriate error
- [ ] Parse errors → 500 with clear message
- [ ] SSE clients handle reconnection gracefully

## Validation Commands

```bash
cd /workspace/main/monitor
go test ./internal/handler/... -v
go test ./internal/datasource/... -v
```

Manual API verification:
```bash
go run ./cmd/monitor &
MONITOR_PID=$!
curl -s http://localhost:9100/api/plan | jq '.epic.milestones[0] | keys' | grep -q "issues" && echo "✓ issues field present" || echo "✗ issues field missing"
curl -s http://localhost:9100/api/plan | jq '.epic.milestones[0] | keys' | grep -q "depends_on" && echo "✓ depends_on field present" || echo "✗ depends_on field missing"
curl -s http://localhost:9100/api/status | jq .
timeout 5s curl -s http://localhost:9100/events | head -20
kill $MONITOR_PID
```

Coverage:
```bash
go test ./internal/handler/... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep -E "(handlePlan|handleStatus|handleSSE)"
```

## Tools

- Read, Grep, Glob, Bash. **Disallowed**: Write, Edit

## Review Protocol

1. Read handler implementations
2. Check JSON serialization includes v2 fields
3. Run tests - verify v2 response structure?
4. Manual verification of actual responses
5. Challenge - raise gaps or missing error handling

## Lessons from Prior Debates

- Probe edge cases systematically: null phase_status, empty issues array, unknown status values, missing optional fields (branch, pr = null).
- Alpine.js templates: verify x-if vs x-show, check Object.keys() iteration is deterministic (use explicit ordered arrays).
- Generative introduces helper duplicating existing logic → flag as concrete finding (not suggestion). Duplicated helpers diverge over time.
- Run tests as evidence rather than trusting generative's claims.

## Success Criteria

- All v2 fields present in responses
- SSE events include workspace/issue context
- Error handling robust (invalid workspace, missing files, parse errors)
- Tests verify response matches v2 schema
- No breaking changes (graceful degradation)
