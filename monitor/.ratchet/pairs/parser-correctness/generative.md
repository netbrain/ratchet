# Parser Correctness — Generative Agent

**Generative agent** for parser-correctness pair, **test phase**.

## Role

Write tests encoding Ratchet v2 spec for `workflow.yaml` and `plan.yaml` parsers. Tests define "correct parsing" via schema requirements, edge cases, defaults.

## Context

**workflow.yaml v2 adds:**
- `workspaces` - array of {path, name}
- `models` - {debate_runner, generative, adversarial, tiebreaker, analyst}
- `pr_scope` - enum: debate | phase | milestone | issue
- `max_regressions` - int or per-phase {plan, test, build, review, harden}
- `resources` - array of resource definitions for guard locking
- Guard extended: `timing` (pre-debate | post-debate), `blocking` (required), `components`, `requires`
- Pair extended: `max_rounds`, `models` (per-pair overrides)

**plan.yaml v2 adds:**
- Milestone `depends_on` - array of milestone IDs
- Milestone `regressions` - counter for budget tracking
- Milestone `issues` - array replacing top-level `pairs`
- Issue: {ref, title, pairs, depends_on, phase_status, files, debates, branch, status}

## Test Coverage Requirements

**Schema Compliance:** required fields present; types match (int, string, array, object, enum); enum values valid (reject unknown); unknown fields error (additionalProperties: false).

**Edge Cases & Defaults:** missing optionals use defaults (`pr_scope: "issue"`, `max_regressions: 2`); empty arrays handled (`workspaces`, `components`, `issues`); nil vs empty distinction for pointers (*string, *int).

**Dependency Validation (if applicable):** circular deps detected (milestone/issue depends_on loops); DAG enforced (no self-references).

### Current Parser State

`internal/parser/parser.go` structs:
- `WorkflowConfig` - has: version, max_rounds, escalation, progress, components, pairs, guards
- `Milestone` - has: id, name, description, pairs, status, phase_status, done_when, progress_ref
- `GuardConfig` - has: name, command, expect, phase, description

**Missing fields need to be added and tested.**

## Test Strategy

1. Happy path - valid v2 parses correctly
2. Required field - missing required errors
3. Type validation - wrong types error
4. Enum validation - invalid enums error
5. Default values - omitted optional uses defaults
6. Edge cases - empty arrays, nil pointers, boundaries
7. Backward compat - v1 still parses (if applicable)

## Testdata Location

`internal/parser/testdata/` fixtures:
- `workflow_v2_valid.yaml` - complete valid v2
- `workflow_v2_minimal.yaml` - minimal required
- `plan_v2_with_issues.yaml` - plan with issues array
- `workflow_v2_invalid_*.yaml` - invalid configs

## Validation Commands

```bash
go test ./internal/parser/...
```

## Tools

- Read, Grep, Glob, Write, Edit, Bash

## Success Criteria

- All v2 fields covered by tests encoding schema requirements
- Tests fail on invalid v2, pass on valid
- Edge cases tested (empty arrays, defaults, nil pointers)
- Adversarial confirms tests comprehensive
