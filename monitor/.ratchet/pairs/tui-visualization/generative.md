# TUI Visualization — Generative Agent

**Generative agent** for tui-visualization pair, **review phase**.

## Role

Update terminal UI to visualize Ratchet v2: workspaces, issue-level progress, parallel milestone DAGs, regression budgets.

## Context

TUI in `cmd/tui/main.go` built with `github.com/grindlemire/go-tui`.

**Current Structure:**
- `internal/tui/components/` - screens (pairs, debates, scores, epic, header, statusbar)
- `internal/tui/views/` - view models that transform data
- `internal/tui/state/` - app state
- `internal/tui/client/` - REST and SSE client

**v2 Visualization Requirements:**

1. **Workspace Selector/Switcher** - display workspaces from root workflow.yaml; keyboard shortcut (`w`); show current in header/statusbar; filter views by selected workspace.
2. **Issue-Level Progress** - epic screen shows issues within milestones; each issue: ref/title, pairs assigned, phase status (plan/test/build/review/harden with icons), dependencies, status (pending/in_progress/done); keyboard navigation drills into details.
3. **Milestone DAG** - epic screen shows milestone deps visually; parallel (no depends_on) side-by-side; sequential with arrows; current layer highlighted (Layer 0, Layer 1, etc.).
4. **Regression Budget** - counter per milestone; visual indicator near limit; warning color when regressions >= max_regressions.

## Current Implementation

- **Epic View** (`internal/tui/components/epic_screen.go`) - displays milestones, shows phase_status (currently milestone-level, needs issue-level), needs DAG update.
- **Epic ViewModel** (`internal/tui/views/epic_viewmodel.go`) - transforms plan data, currently list, needs DAG layout.
- **State** (`internal/tui/state/state.go`) - holds current tab, indices, plan; needs workspace state.

## Strategy

1. Add workspace state - current + list
2. Update epic view model - transform issues, calculate DAG layers
3. Update epic screen - render issues within milestones, show deps
4. Add workspace switcher - keyboard + UI
5. Add regression budget display - color-coded counter

## Visual Design Guidance

Box-drawing characters for DAG:
```
Layer 0:  ┌─ Milestone 1 ─┐    ┌─ Milestone 2 ─┐
          │  Issue 1-1 ✓  │    │  Issue 2-1 ⚙  │
          │  Issue 1-2 ⚙  │    │  Issue 2-2 ○  │
          └───────┬────────┘    └───────┬────────┘
                  │                     │
                  └─────────┬───────────┘
                            ↓
Layer 1:              ┌─ Milestone 3 ─┐
                      │  Issue 3-1 ○  │
                      └────────────────┘
```

Icons: ✓ done, ⚙ in_progress, ○ pending

## Validation Commands

```bash
go test ./internal/tui/... -v
```

Manual:
```bash
go run ./cmd/tui
# Navigate, test workspace switching, verify rendering
```

## Tools

- Read, Grep, Glob, Write, Edit, Bash

## Lessons from Prior Debates

- Ensure test assertions are unambiguous: if same symbol (e.g., tree connector) can come from multiple paths, create isolated fixtures.
- Test single-item AND multi-item fixtures to cover all rendering branches.
- Check duplication when adding helpers — if similar exists, unify rather than parallel implementation.

## Success Criteria

- TUI displays all v2 fields (workspaces, issues, dependencies, regressions)
- Workspace switching works
- Issue-level progress clear
- DAG shows parallel vs sequential
- Regression budget displayed with warnings
- Adversarial confirms visualization correct
