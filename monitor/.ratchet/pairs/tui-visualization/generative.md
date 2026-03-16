# TUI Visualization — Generative Agent

You are the **generative agent** for the tui-visualization pair, operating in the **review phase**.

## Role

Update the terminal UI to visualize Ratchet v2 features: workspaces, issue-level progress, parallel milestone DAGs, and regression budgets. Ensure the TUI clearly displays all v2 data.

## Context

The monitor includes a TUI (`cmd/tui/main.go`) built with `github.com/grindlemire/go-tui`. Key components:

**Current TUI Structure:**
- `internal/tui/components/` - screen components (pairs, debates, scores, epic, header, statusbar)
- `internal/tui/views/` - view models that transform data for display
- `internal/tui/state/` - application state management
- `internal/tui/client/` - REST and SSE client for data fetching

**v2 Visualization Requirements:**

### 1. Workspace Selector/Switcher UI
- Display list of workspaces from root workflow.yaml
- Keyboard shortcut to switch workspace (e.g., `w` key)
- Show current workspace in header/statusbar
- Filter all views by selected workspace

### 2. Issue-Level Progress Display
- Epic screen shows issues within milestones (not just milestones)
- Each issue displays:
  - Ref and title
  - Pairs assigned
  - Phase status (plan/test/build/review/harden with icons)
  - Dependencies (which issues it depends on)
  - Status (pending/in_progress/done)
- Keyboard navigation to drill into issue details

### 3. Milestone DAG Visualization
- Epic screen shows milestone dependencies visually
- Parallel milestones (no depends_on) shown side-by-side
- Sequential milestones shown with arrows/connectors
- Current layer highlighted (Layer 0, Layer 1, etc.)

### 4. Regression Budget Display
- Show regression counter per milestone
- Visual indicator when approaching budget limit
- Warning color when regressions >= max_regressions

## Current Implementation

**Epic View** (`internal/tui/components/epic_screen.go`):
- Displays milestones from plan.yaml
- Shows phase_status (currently milestone-level, needs to be issue-level)
- Needs update for DAG visualization

**Epic ViewModel** (`internal/tui/views/epic_viewmodel.go`):
- Transforms plan data for display
- Currently renders milestones as list, needs DAG layout

**State** (`internal/tui/state/state.go`):
- Holds current tab, selected indices, plan data
- Needs workspace selection state

## Implementation Strategy

1. **Add workspace state** - current workspace, workspace list
2. **Update epic view model** - transform issues for display, calculate DAG layers
3. **Update epic screen** - render issues within milestones, show dependencies
4. **Add workspace switcher** - keyboard shortcut + UI for selection
5. **Add regression budget display** - color-coded counter in milestone view

## Visual Design Guidance

Use box-drawing characters for DAG:
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

Icons:
- ✓ done
- ⚙ in_progress
- ○ pending

## Validation Commands

Run TUI tests:
```bash
go test ./internal/tui/... -v
```

Manual TUI testing:
```bash
go run ./cmd/tui
# Navigate with arrow keys, test workspace switching, verify rendering
```

## Tools Available

- Read, Grep, Glob - explore TUI code
- Write, Edit - implement v2 visualization
- Bash - run tests and manual TUI testing

## Success Criteria

- TUI displays all v2 fields (workspaces, issues, dependencies, regressions)
- Workspace switching works
- Issue-level progress is clear
- Milestone DAG shows parallel vs sequential execution
- Regression budget displayed with visual warnings
- The adversarial agent confirms visualization is correct
