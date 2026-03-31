---
name: ratchet:guard
description: Manage guards — list, add, run, and override deterministic checks at phase boundaries
---

# /ratchet:guard — Manage Guards

Guards are deterministic shell commands (lint, test, security scan, benchmarks) that run at phase boundaries. They complement debates — debates are semantic, guards are mechanical.

## Resource Gating with `requires`

Guards can declare shared resource dependencies using the `requires` field. Resources are defined in the top-level `resources` array of `workflow.yaml` and represent shared infrastructure (databases, test servers, etc.) that guards may need exclusive access to.

When a guard specifies `requires: [resource-name]`, the resource is started (via its `start` command) before the guard runs and locked if the resource is `singleton: true`. Singleton locking serializes access so only one guard at a time can use the resource — other guards requiring the same singleton resource wait until the lock is released.

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

The `requires` field accepts an array of resource names (must match entries in `resources`). If a required resource is not defined, the guard fails with an error before execution.

## Parallel Guard Writes and File Locking

When multiple guards run concurrently, `run-guards.sh` uses `flock` (advisory file locking) to prevent race conditions when writing guard result JSON files. Each guard result directory has a `.guard-write.lock` file; the script acquires an exclusive lock (with a 30-second timeout) before writing the result JSON via atomic temp-file-then-`mv`.

On systems where `flock` is unavailable (e.g., macOS without Homebrew coreutils), the script falls back to unlocked writes. This is safe for single-guard execution but may produce corrupted results if multiple guards target the same directory concurrently on such systems.

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

If `workflow.yaml` does not exist, inform the user:
> "No workflow.yaml found. Run /ratchet:init to set up."

Then use `AskUserQuestion` with options: `"Initialize now (/ratchet:init) (Recommended)"`, `"Cancel"`.

## Execution Steps

### No Arguments — List Guards

Read `guards` from `.ratchet/workflow.yaml`. If no guards are configured:
> "No guards configured. Guards are deterministic checks (lint, tests, security scans) that run at phase boundaries."

Then use `AskUserQuestion` with options: `"Add a guard (Recommended)"`, `"Done for now"`.

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
   - Read `components` array from workflow.yaml
   - If components exist: Options are component names + `"All components"`
   - If no components configured: Options are `"All files (no components configured)"`

7. **Resource requirements** — which shared resources does this guard need:
   - Read `resources` array from workflow.yaml
   - If resources exist: Options are resource names + `"None"`
   - If no resources configured: skip this step (no resources available to require)
   - Selected resources are stored in the `requires` field (see [Resource Gating](#resource-gating-with-requires))

8. **Confirm** — present the guard definition:
   - Use `AskUserQuestion`: "[guard summary]. Add this guard?"
   - Options: `"Add (Recommended)"`, `"Modify"`, `"Cancel"`

On approval, append to the `guards` array in `.ratchet/workflow.yaml`.

### run — Execute Guards

Run guards for the specified phase (or all phases if none specified).

**Script validation**: First, check that the guard execution script exists:
```bash
test -f .claude/ratchet-scripts/run-guards.sh || echo "MISSING"
```

If missing:
- Inform user: "Guard execution script not found. This may indicate an incomplete Ratchet installation."
- Use `AskUserQuestion`:
  - Options: `"Re-install Ratchet (./install.sh)"`, `"Run guards manually (I'll show you the format)"`, `"Cancel"`
- If "Run manually": show guard result JSON schema (see Guard Results section below) and let user execute commands and record results

For each matching guard:
```bash
bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> "<command>" <blocking>
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

**Guilty until proven innocent**: Guard failures are caused by the current changes unless proven otherwise. Before overriding a failed guard, verify whether the failure exists on a clean checkout:
```bash
# Test on clean base to determine if failure is pre-existing
git stash && bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> "<command>" <blocking> && git stash pop
# Only override if the guard also fails on clean base (pre-existing failure)
```

If any blocking guards failed, use `AskUserQuestion`:
- Options: `"Fix and re-run (Recommended)"`, `"View full output of [name]"`, `"Verify on clean base first"`, `"Override [name]"`, `"Done for now"`

#### Guard Results

Guard results are stored as JSON in `.ratchet/guards/<milestone-id>/<issue-ref>/<phase>/<guard-name>.json` with this structure:

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

These fields are produced by `scripts/run-guards.sh`. When a guard is overridden (via the `override` subcommand), the following fields are patched onto the existing JSON — the original fields are preserved:

```json
{
  "overridden": true,
  "override_reason": "<human justification>",
  "override_timestamp": "<ISO timestamp>"
}
```

### override — Override a Failed Guard

Read the guard result from `.ratchet/guards/<milestone-id>/<issue-ref>/<phase>/<name>.json`.

Determine `<milestone-id>` from the current active milestone in `.ratchet/plan.yaml`, or require it as an argument: `guard override <name> --milestone <id>`.

Determine `<issue-ref>` from the current active issue context, or require it as an argument: `guard override <name> --issue <ref>`.

**Validation**: Before reading the guard result, validate that the issue-ref exists in `plan.yaml`:
```bash
yq ".epic.milestones[] | select(.id == $MILESTONE_ID) | .issues[] | select(.ref == \"$ISSUE_REF\")" .ratchet/plan.yaml
```
If the issue-ref is not found in the milestone's issues array, report a clear error:
> "Issue ref '[ref]' not found in milestone [id]. Available issues: [list of refs from plan.yaml]. Check the ref and try again."

If no failure exists for this guard:
> "Guard '[name]' has no recent failure to override."

If failure exists, use `AskUserQuestion`:
- Question: "Guard '[name]' failed with exit code [N]: [summary]. Override this and allow phase advancement?"
- Options: `"Override with reason"`, `"Cancel"`

If "Override with reason": use follow-up `AskUserQuestion` (freeform) to capture the reason.

Update the existing guard result JSON (do NOT overwrite — patch only the override fields):
```bash
jq '.overridden = true | .override_reason = "<user reason>" | .override_timestamp = "<ISO timestamp>"' \
  .ratchet/guards/<milestone-id>/<issue-ref>/<phase>/<name>.json > /tmp/guard-override-tmp.json \
  && mv /tmp/guard-override-tmp.json .ratchet/guards/<milestone-id>/<issue-ref>/<phase>/<name>.json
```

This preserves the original `guard`, `command`, `exit_code`, `stdout`, `stderr`, `passed`, `blocking`, and `timestamp` fields.

### remove — Remove a Guard

Read `guards` from `workflow.yaml`, find the named guard.

Use `AskUserQuestion` to confirm:
- Question: "Remove guard '[name]' ([command], [phase], [blocking])? This cannot be undone."
- Options: `"Remove"`, `"Cancel"`

On confirmation, remove from the `guards` array in `.ratchet/workflow.yaml`.
