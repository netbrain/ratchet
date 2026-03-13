---
name: ratchet:status
description: Show milestone and phase progress at a glance
---

# /ratchet:status — Project Status

Display a snapshot of the project's progress through the epic, milestones, and phases.

## Usage
```
/ratchet:status              # Full status overview
/ratchet:status [milestone]  # Detailed view of a specific milestone
```

## Prerequisites
- `.ratchet/` must exist
- `.ratchet/plan.yaml` must exist

If `.ratchet/` or `plan.yaml` does not exist, inform the user:
> "No epic found. Run /ratchet:init to set up a project, or /ratchet:run to start without an epic."

Then use `AskUserQuestion` with options: `"Initialize (/ratchet:init)"`, `"Done for now"`.

## Execution Steps

### Step 1: Read State

Read `.ratchet/plan.yaml` and the workflow config (`.ratchet/workflow.yaml` or `.ratchet/config.yaml`).

Also scan `.ratchet/debates/*/meta.json` to count active/resolved debates per milestone.

### Step 2: Display Overview

Present a compact status view:

```
Epic: [project name] — [description]
Progress: [completed]/[total] milestones

┌─────────────────────────────────────────────────────────┐
│ Milestone 1: [name]                              [DONE] │
│   plan ✓  test ✓  build ✓  review ✓  harden ✓          │
│   Debates: [N] total, [N] consensus, [N] escalated     │
│                                                         │
│ Milestone 2: [name]                       [IN PROGRESS] │
│   plan ✓  test ✓  build ●  review ○  harden ○          │
│   Debates: [N] active, [N] resolved                    │
│   Current phase: build — [N] pairs queued               │
│                                                         │
│ Milestone 3: [name]                           [PENDING] │
│   plan ○  test ○  build ○  review ○  harden ○          │
└─────────────────────────────────────────────────────────┘

✓ = done  ● = in progress  ○ = pending  ✗ = failed/blocked
```

Adapt the phase display based on the component's workflow preset:
- `tdd`: show all 5 phases
- `traditional`: show plan, build, review, harden (skip test)
- `review-only`: show only review

For v1 configs (no phases), show a simplified view without phase tracking.

### Step 3: Detailed Milestone View

If a specific milestone was requested, show expanded detail:

```
Milestone [id]: [name]
Status: [status]
Description: [description]
Done when: [acceptance criteria]

Phase Progress:
  plan:    [done]        — [N] pairs, all consensus
  test:    [done]        — [N] pairs, all consensus
  build:   [in_progress] — [N] pairs ([N] consensus, [N] active)
  review:  [pending]     — [N] pairs queued
  harden:  [pending]     — [N] pairs queued

Pairs:
  [pair-name] — phase: [phase] — [status] — last debate: [debate-id]
  [pair-name] — phase: [phase] — [status] — no debates yet

Guards:
  [guard-name] — phase: [phase] — blocking: [yes/no] — last result: [pass/fail/not run]

Active Debates:
  [debate-id] — [pair-name] — round [N]/[max] — [status]

Unresolved Conditions:
  [condition from CONDITIONAL_ACCEPT, if any]
```

### Step 4: Next Steps

After displaying status, use `AskUserQuestion`:
- Options (adapt based on context):
  - "Continue current phase (/ratchet:run)" — if there's an active milestone
  - "View a specific milestone" — if overview was shown
  - "View debate transcript (/ratchet:debate)" — if debates exist
  - "View quality metrics (/ratchet:score)"
  - "Done for now"
