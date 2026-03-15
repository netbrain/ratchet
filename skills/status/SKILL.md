---
name: ratchet:status
description: Show milestone and phase progress at a glance
---

# /ratchet:status — Project Status

Display a snapshot of the project's progress through the epic, milestones, and phases.

## Usage
```
/ratchet:status              # Full status overview (or workspace overview from root)
/ratchet:status [milestone]  # Detailed view of a specific milestone
/ratchet:status [workspace]  # Status for a specific workspace in a multi-project setup
```

## Prerequisites
- `.ratchet/` must exist
- `.ratchet/plan.yaml` must exist (for workspace-level status)

If `.ratchet/` or `plan.yaml` does not exist, inform the user:
> "No epic found. Run /ratchet:init to set up a project, or /ratchet:run to start without an epic."

Then use `AskUserQuestion` with options: `"Initialize (/ratchet:init) (Recommended)"`, `"Done for now"`.

## Execution Steps

### Step 1: Read State

**Workspace resolution**: Same algorithm as `/ratchet:run` Step 1a. If at workspace root with no workspace specified, show the workspace overview (Step 2b). If a workspace is resolved, show workspace-level status.

Read `plan.yaml` and `workflow.yaml` from the resolved `.ratchet/` directory.

Also scan `debates/*/meta.json` to count active/resolved debates per milestone.

### Step 2b: Workspace Overview (root only)

If at workspace root with no workspace specified, read each workspace's `plan.yaml` and show:

```
Workspaces: [N]
Shared policy: models=[generative:opus, adversarial:sonnet] escalation=[human]

┌───────────────────────────────────────────────────────────────┐
│ monitor                                                       │
│   Epic: [name] — [completed]/[total] milestones               │
│   Current: [milestone name] — [phase] phase                   │
│   Pairs: [N] active, [N] total                                │
│                                                               │
│ engine                                                        │
│   No active milestones                                        │
└───────────────────────────────────────────────────────────────┘
```

Then use `AskUserQuestion`:
- Options: one option per workspace as `"View [name] status"`, plus `"Done for now"`

### Step 2: Display Overview

Present a compact status view:

```
Epic: [project name] — [description]
Progress: [completed]/[total] milestones

┌─────────────────────────────────────────────────────────┐
│ Milestone 1: [name]                              [DONE] │
│   Issues: [N]/[N] complete                              │
│   Debates: [N] total, [N] consensus, [N] escalated     │
│                                                         │
│ Milestone 2: [name]                       [IN PROGRESS] │
│   Issues:                                               │
│     [ref]: [title]                              [DONE]  │
│       plan ✓  test ✓  build ✓  review ✓  harden ✓      │
│     [ref]: [title]                       [IN PROGRESS]  │
│       plan ✓  test ✓  build ●  review ○  harden ○      │
│     [ref]: [title]              [PENDING — dep: [ref]]  │
│       plan ○  test ○  build ○  review ○  harden ○      │
│   Debates: [N] active, [N] resolved                    │
│                                                         │
│ Milestone 3: [name]                           [PENDING] │
│   Issues: [N] total                                     │
└─────────────────────────────────────────────────────────┘

✓ = done  ● = in progress  ○ = pending  ✗ = failed/blocked
```

Adapt the phase display based on the component's workflow preset:
- `tdd`: show all 5 phases
- `traditional`: show plan, build, review, harden (skip test)
- `review-only`: show only review

### Step 3: Detailed Milestone View

If a specific milestone was requested, show expanded detail:

```
Milestone [id]: [name]
Status: [status]
Description: [description]
Done when: [acceptance criteria]
Regressions: [N]/[max]

Issues:
  [ref]: [title]                                        [status]
    Depends on: [dep-refs or "none"]
    Branch: [branch name or "not started"]
    PR: [URL or "none"]
    Phase Progress:
      plan ✓  test ✓  build ●  review ○  harden ○
    Pairs:
      [pair-name] — [phase] — last debate: [debate-id]
    Debates: [N] total ([N] consensus, [N] active)

  [ref]: [title]                                        [status]
    ...

Guards:
  [guard-name] — phase: [phase] — blocking: [yes/no] — last result: [pass/fail/not run]

Unresolved Conditions:
  [condition from CONDITIONAL_ACCEPT, if any]
```

### Step 4: Next Steps

After displaying status, use `AskUserQuestion`:
- Options (adapt based on context):
  - "Continue current phase (/ratchet:run) (Recommended)" — if there's an active milestone
  - "View a specific milestone" — if overview was shown
  - "View debate transcript (/ratchet:debate)" — if debates exist
  - "View quality metrics (/ratchet:score)"
  - "Done for now"
