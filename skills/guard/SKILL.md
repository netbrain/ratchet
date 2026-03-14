---
name: ratchet:guard
description: Manage guards — list, add, run, and override deterministic checks at phase boundaries
---

# /ratchet:guard — Manage Guards

Guards are deterministic shell commands (lint, test, security scan, benchmarks) that run at phase boundaries. They complement debates — debates are semantic, guards are mechanical.

## Usage
```
/ratchet:guard                     # List all guards with last results
/ratchet:guard add                 # Interactive: add a new guard
/ratchet:guard run [phase]         # Run all guards for a phase (or all phases)
/ratchet:guard override <name>     # Override a failed blocking guard
/ratchet:guard remove <name>       # Remove a guard from workflow.yaml
```

## Prerequisites
- `.ratchet/` must exist
- `.ratchet/workflow.yaml` must exist (guards are a v2 feature)

If `workflow.yaml` does not exist but `config.yaml` does, inform the user:
> "Guards require workflow.yaml (v2). Run /ratchet:migrate to upgrade from v1."

Then use `AskUserQuestion` with options: `"Migrate now (/ratchet:migrate)"`, `"Cancel"`.

## Execution Steps

### No Arguments — List Guards

Read `guards` from `.ratchet/workflow.yaml`. If no guards are configured:
> "No guards configured. Guards are deterministic checks (lint, tests, security scans) that run at phase boundaries."

Then use `AskUserQuestion` with options: `"Add a guard"`, `"Done for now"`.

If guards exist, display:

```
Guards
══════

  [name]
    Command: [command]
    Phase: [phase] | Timing: [pre-debate/post-debate] | Blocking: [yes/no]
    Components: [list or "all"]
    Last result: [pass/fail/not run] ([timestamp])

  [name]
    ...

[N] guards configured ([N] blocking, [N] advisory)
```

Then use `AskUserQuestion`:
- Options: `"Add a guard"`, `"Run all guards"`, `"Run guards for [phase]"`, `"Done for now"`

### add — Add a New Guard

Use `AskUserQuestion` to gather guard details interactively:

1. **What to check?** — freeform or suggest based on project.yaml testing spec:
   - "What should this guard check?"
   - If testing spec has uncovered layers, suggest: "I see you have [tool] configured but no guard for it. Add one?"

2. **Command** — freeform:
   - "What command should run?" (pre-fill if suggesting from testing spec)

3. **Phase** — which phase boundary:
   - Options: `"plan"`, `"test"`, `"build"`, `"review"`, `"harden"`
   - Suggest based on what the command does (lint → build, security → harden, tests → build)

4. **Timing** — when should this guard run:
   - Options: `"Pre-debate (run before debates start — catches issues early)"`, `"Post-debate (run after debates complete — default)"`
   - Suggest based on what the command does: lint/format checks benefit from pre-debate (no point debating code that fails lint), tests/security scans are typically post-debate

5. **Blocking or advisory?**
   - Options: `"Blocking (must pass to advance)"`, `"Advisory (log and continue)"`

6. **Components** — which components:
   - Options: list component names from workflow.yaml + `"All components"`

7. **Confirm** — present the guard definition:
   - Use `AskUserQuestion`: "[guard summary]. Add this guard?"
   - Options: `"Add"`, `"Modify"`, `"Cancel"`

On approval, append to the `guards` array in `.ratchet/workflow.yaml`.

### run — Execute Guards

Run guards for the specified phase (or all phases if none specified).

For each matching guard:
```bash
bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<command>" <blocking>
```

Use the current milestone from `plan.yaml` (or "manual" if no active milestone).

Present results:
```
Guard Results: [phase] phase
═══════════════════════════

  ✓ [name] — passed
  ✗ [name] — FAILED (blocking)
    [first few lines of output]
  ⚠ [name] — failed (advisory)
    [first few lines of output]

[N] passed, [N] failed ([N] blocking)
```

If any blocking guards failed, use `AskUserQuestion`:
- Options: `"View full output of [name]"`, `"Override [name]"`, `"Fix and re-run"`, `"Done for now"`

### override — Override a Failed Guard

Read the guard result from `.ratchet/guards/<milestone>/<phase>/<name>.json`.

If no failure exists for this guard:
> "Guard '[name]' has no recent failure to override."

If failure exists, use `AskUserQuestion`:
- Question: "Guard '[name]' failed with exit code [N]: [summary]. Override this and allow phase advancement?"
- Options: `"Override with reason"`, `"Cancel"`

If "Override with reason": use follow-up `AskUserQuestion` (freeform) to capture the reason.

Write override to the guard result JSON:
```json
{
  "guard": "<name>",
  "phase": "<phase>",
  "overridden": true,
  "override_reason": "<user's reason>",
  "override_timestamp": "<ISO timestamp>"
}
```

### remove — Remove a Guard

Read `guards` from `workflow.yaml`, find the named guard.

Use `AskUserQuestion` to confirm:
- Question: "Remove guard '[name]' ([command], [phase], [blocking])? This cannot be undone."
- Options: `"Remove"`, `"Cancel"`

On confirmation, remove from the `guards` array in `.ratchet/workflow.yaml`.
