---
name: ratchet:guard
description: Manage guards — list, add, run, and override deterministic checks at phase boundaries
---

# /ratchet:guard — Manage Guards

Guards are deterministic shell commands (lint, test, security scan, benchmarks) that run at phase boundaries. They complement debates — debates are semantic, guards are mechanical.

## Resource Gating with `requires`

Guards can declare shared resource dependencies via `requires` field. Resources are defined in top-level `resources` array of `workflow.yaml` and represent shared infrastructure (databases, test servers, etc.) that guards may need exclusive access to.

When a guard specifies `requires: [resource-name]`, the resource is started (via its `start` command) before the guard runs and locked if `singleton: true`. Singleton locking serializes access so only one guard at a time can use the resource — others wait until the lock is released.

**Example** in `workflow.yaml`:
```yaml
resources:
  - name: test-db
    start: "docker compose up -d test-db"
    stop: "docker compose down test-db"
    singleton: true

guards:
  - name: integration-tests
    command: "npm run test:integration"
    phase: build
    blocking: true
    requires: [test-db]
```

`requires` accepts an array of resource names (must match entries in `resources`). If a required resource is not defined, the guard fails with an error before execution.

## Parallel Guard Writes and File Locking

When multiple guards run concurrently, `run-guards.sh` uses `flock` (advisory file locking) to prevent race conditions when writing guard result JSON files. Each result directory has a `.guard-write.lock` file; the script acquires an exclusive lock (30-second timeout) before writing the result JSON via atomic temp-file-then-`mv`.

On systems where `flock` is unavailable (e.g., macOS without Homebrew coreutils), the script falls back to unlocked writes. Safe for single-guard execution but may produce corrupted results if multiple guards target same directory concurrently on such systems.

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

If `workflow.yaml` does not exist, inform user: "No workflow.yaml found. Run /ratchet:init to set up." Then `AskUserQuestion` with options: `"Initialize now (/ratchet:init) (Recommended)"`, `"Cancel"`.

## Execution Steps

### No Arguments — List Guards

Read `guards` from `.ratchet/workflow.yaml`. If no guards configured: "No guards configured. Guards are deterministic checks (lint, tests, security scans) that run at phase boundaries." Then `AskUserQuestion` with options: `"Add a guard (Recommended)"`, `"Done for now"`.

If guards exist, display:

```
Guards
══════

  [name]
    Command: [command]
    Phase: [phase] | Timing: [pre-debate/post-debate] | Blocking: [yes/no]
    Components: [list or "all"]
    Requires: [resource list or "none"]
    Last result: [pass/fail/not run] ([timestamp])

  [name]
    ...

[N] guards configured ([N] blocking, [N] advisory)
```

Then `AskUserQuestion`:
- Options: `"Add a guard"`, `"Run all guards"`, `"Run guards for [phase]"`, `"Done for now"`

### add — Add a New Guard

Use `AskUserQuestion` to gather guard details interactively:

1. **What to check?** — freeform: "What should this guard check?" If testing spec has uncovered layers, suggest: "I see you have [tool] configured but no guard for it. Add one?"
2. **Command** — freeform: "What command should run?" (pre-fill if suggesting from testing spec)
3. **Phase** — which phase boundary. Options: `"plan"`, `"test"`, `"build"`, `"review"`, `"harden"`. Suggest based on command (lint → build, security → harden, tests → build)
4. **Timing** — Options: `"Pre-debate (run before debates start — catches issues early)"`, `"Post-debate (run after debates complete — default)"`. Suggest: lint/format → pre-debate (no point debating code that fails lint), tests/security → post-debate
5. **Blocking or advisory?** Options: `"Blocking (must pass to advance)"`, `"Advisory (log and continue)"`
6. **Components** — read `components` array from workflow.yaml. If exist: Options are component names + `"All components"`. If none: `"All files (no components configured)"`
7. **Resource requirements** — read `resources` array from workflow.yaml. If exist: Options are resource names + `"None"`. If none: skip. Selected resources stored in `requires` field (see [Resource Gating](#resource-gating-with-requires))
8. **Confirm** — `AskUserQuestion`: "[guard summary]. Add this guard?" Options: `"Add (Recommended)"`, `"Modify"`, `"Cancel"`

On approval, append to `guards` array in `.ratchet/workflow.yaml`.

### run — Execute Guards

Run guards for specified phase (or all phases if none specified).

**Script validation**: First, check that guard execution script exists:
```bash
test -f .claude/ratchet-scripts/run-guards.sh || echo "MISSING"
```

If missing:
- Inform user: "Guard execution script not found. May indicate incomplete Ratchet installation."
- `AskUserQuestion`: Options: `"Re-install Ratchet (./install.sh)"`, `"Run guards manually (I'll show you the format)"`, `"Cancel"`
- If "Run manually": show guard result JSON schema (see Guard Results section) and let user execute commands and record results

For each matching guard:
```bash
bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> "<command>" <blocking>
```

Use current milestone from `plan.yaml` (or "manual" if no active milestone).

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

**Guilty until proven innocent**: Guard failures are caused by current changes unless proven otherwise. Before overriding a failed guard, verify whether failure exists on a clean checkout:
```bash
# Test on clean base to determine if failure is pre-existing
git stash && bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> "<command>" <blocking> && git stash pop
# Only override if the guard also fails on clean base (pre-existing failure)
```

If any blocking guards failed, `AskUserQuestion`:
- Options: `"Fix and re-run (Recommended)"`, `"View full output of [name]"`, `"Verify on clean base first"`, `"Override [name]"`, `"Done for now"`

#### Guard Results

Guard results stored as JSON in `.ratchet/guards/<milestone-id>/<issue-ref>/<phase>/<guard-name>.json` with this structure:

```json
{
  "guard": "<guard-name>",
  "command": "<command that was executed>",
  "exit_code": 0,
  "stdout": "<stdout output>",
  "stderr": "<stderr output>",
  "passed": true,
  "blocking": true,
  "timestamp": "<ISO timestamp>",
  "overridden": false,
  "override_reason": null
}
```

These fields are produced by `.claude/ratchet-scripts/run-guards.sh`. When a guard is overridden (via `override` subcommand), following fields are patched onto existing JSON — original fields preserved:

```json
{
  "overridden": true,
  "override_reason": "<human justification>",
  "override_timestamp": "<ISO timestamp>"
}
```

### override — Override a Failed Guard

Read guard result from `.ratchet/guards/<milestone-id>/<issue-ref>/<phase>/<name>.json`.

Determine `<milestone-id>` from current active milestone in `.ratchet/plan.yaml`, or require it as argument: `guard override <name> --milestone <id>`. Determine `<issue-ref>` from current active issue context, or require it as argument: `guard override <name> --issue <ref>`.

**Validation**: Before reading guard result, validate that issue-ref exists in `plan.yaml`:
```bash
yq ".epic.milestones[] | select(.id == $MILESTONE_ID) | .issues[] | select(.ref == \"$ISSUE_REF\")" .ratchet/plan.yaml
```
If issue-ref not found in milestone's issues array, report a clear error: "Issue ref '[ref]' not found in milestone [id]. Available issues: [list of refs from plan.yaml]. Check the ref and try again."

If no failure exists for this guard: "Guard '[name]' has no recent failure to override."

If failure exists, `AskUserQuestion`:
- Question: "Guard '[name]' failed with exit code [N]: [summary]. Override this and allow phase advancement?"
- Options: `"Override with reason"`, `"Cancel"`

If "Override with reason": follow-up `AskUserQuestion` (freeform) to capture reason.

Update existing guard result JSON (do NOT overwrite — patch only override fields):
```bash
jq '.overridden = true | .override_reason = "<user reason>" | .override_timestamp = "<ISO timestamp>"' \
  .ratchet/guards/<milestone-id>/<issue-ref>/<phase>/<name>.json > /tmp/guard-override-tmp.json \
  && mv /tmp/guard-override-tmp.json .ratchet/guards/<milestone-id>/<issue-ref>/<phase>/<name>.json
```

Preserves original `guard`, `command`, `exit_code`, `stdout`, `stderr`, `passed`, `blocking`, and `timestamp` fields.

### remove — Remove a Guard

Read `guards` from `workflow.yaml`, find named guard.

`AskUserQuestion` to confirm:
- Question: "Remove guard '[name]' ([command], [phase], [blocking])? Cannot be undone."
- Options: `"Remove"`, `"Cancel"`

On confirmation, remove from `guards` array in `.ratchet/workflow.yaml`.
