# Plan: plan.yaml v2 Parser Update

## Objective
Update the `ParsePlan` function and related types in `internal/parser/parser.go` to support Ratchet v2 plan.yaml specification.

## V2 Specification Changes

### Milestone Structure Changes
1. **Add `depends_on` field** - array of milestone IDs for DAG-based parallel execution
2. **Add `regressions` counter** - tracks regression budget usage
3. **Replace `pairs` array** - move from milestone-level to issue-level
4. **Add `issues` array** - collection of issue objects with phase tracking

### New Issue Structure
Each issue within a milestone contains:
- `ref`: string - unique identifier (e.g., "issue-1-1")
- `title`: string - human-readable title
- `pairs`: []string - list of pair names for this issue
- `depends_on`: []string - list of issue refs this depends on
- `phase_status`: map[string]string - status per phase (pending/in_progress/done/skipped)
- `files`: []string - list of modified files
- `debates`: []string - list of debate IDs created
- `branch`: *string - git branch name (nullable)
- `status`: string - overall issue status (pending/in_progress/done/blocked/escalated/failed)

## Implementation Plan

### Phase 1: Test (TDD)
- Write comprehensive tests for v2 plan.yaml parsing
- Test milestone `depends_on` parsing
- Test milestone `regressions` field
- Test issue array parsing with all fields
- Test phase_status map parsing
- Test backward compatibility (v1 plans still parse)

### Phase 2: Build
- Update `Milestone` struct with new fields
- Create new `Issue` struct
- Update `ParsePlan` function logic
- Ensure all tests pass

### Phase 3: Review
- Verify all v2 fields parse correctly
- Verify error handling for malformed input
- Verify defaults are applied correctly

### Phase 4: Harden
- Run race detection tests
- Verify concurrent parsing safety

## Success Criteria
- [ ] All v2 plan.yaml testdata files parse without errors
- [ ] Parser tests encode v2 schema requirements
- [ ] Backward compatibility maintained (v1 plans still work)
- [ ] All guards pass (format, vet, build, test, race)
- [ ] Code review completed
