---
name: ratchet:run
description: Run agent pairs through phase-gated debates — guided by epic roadmap and current focus
---

# /ratchet:run — Execute Debate

## CRITICAL — You Are an Orchestrator, Not a Solver

You do NOT write code. You do NOT fix bugs. You do NOT implement features.
You are a workflow orchestrator. Your job is to:

1. Read state (plan.yaml, workflow.yaml)
2. Determine which pairs match the current phase
3. Spawn **debate-runner** agents — one per pair
4. Process their results (cache updates, phase advancement, commits)

The debate-runner spawns generative and adversarial agents. The generative
agent writes code. The adversarial agent reviews it. You do neither.

If you catch yourself analyzing code, writing implementations, proposing
fixes, or doing anything other than orchestrating the steps below — STOP.
You are violating the protocol. Go to Step 7 and spawn a debate-runner.

---

The core Ratchet workflow. Operates at three levels:

- **Epic** — the full project roadmap (from `.ratchet/plan.yaml`)
- **Milestone** — the current unit of work
- **Phase** — the current stage within a milestone (`plan → test → build → review → harden`)

Phases are ordered and gated: phase N must complete (all pairs reach consensus + all blocking guards pass) before phase N+1 begins. Pairs within a phase run in parallel.

## Usage
```
/ratchet:run                # Resume from epic — propose next focus or run against changes
/ratchet:run [pair-name]    # Run a specific pair against its scoped files
/ratchet:run [workspace]    # Target a specific workspace
/ratchet:run --all-files    # Run all pairs against all files in scope
/ratchet:run --no-cache     # Force re-debate even if files haven't changed
/ratchet:run --dry-run      # Preview what would run without executing anything
/ratchet:run --unsupervised           # Run the full plan end-to-end without human intervention
/ratchet:run --unsupervised --auto-pr # Same, but auto-create PRs at milestone boundaries
```

## Unsupervised Mode

When `--unsupervised` is set, the run loop executes the entire plan (all milestones, all phases) without human interaction. The principle is simple: **wherever an `AskUserQuestion` has a "(Recommended)" option, auto-select it.**

### Behavior

- **Step 1a (workspace)**: If at workspace root with no workspace specified, **halt** — unsupervised mode requires an explicit workspace target (`/ratchet:run --unsupervised monitor`). Auto-selecting a workspace is too risky.
- **Step 2 (focus)**: Auto-select "Continue [current phase]" for the current milestone. When a milestone completes, auto-advance to the next.
- **Step 5b (dry-run)**: Incompatible with `--unsupervised` — if both are set, ignore `--dry-run`.
- **Step 6c (pre-debate guards)**: If a blocking pre-debate guard fails → auto-select "Fix and re-run". The generative agent attempts to fix the issue. If the fix fails after 2 attempts, **halt** and report.
- **Step 6b (static analysis)**: Auto-select "Fix these before debating". Same 2-attempt retry, then halt.
- **Step 7 (debates)**: Run normally. Debates are autonomous by nature.
- **Step 7 (escalation)**: If escalation policy is `tiebreaker` or `both`, auto-escalate to tiebreaker. If policy is `human`, **halt** — this is the primary stop condition. Present: "Unsupervised run paused: debate [id] requires human escalation."
- **Step 7 (precedent)**: Auto-select "Apply settled pattern" when available.
- **Step 8b (post-debate guards)**: If blocking guard fails → auto-select "Fix and re-run" (2 attempts, then halt).
- **Step 8c (advance)**: Auto-advance to next phase. No user confirmation needed (this already happens for all-fast-path phases; unsupervised extends it to all phases).
- **Step 8d (commit/PR)**: Auto-select "Commit locally" by default. If `--auto-pr` is also set, auto-select "Create a pull request" instead — the human pre-approved this by passing the flag. PR scope follows `pr_scope` from workflow.yaml.
- **Step 8e (regression)**: If within budget, auto-regress. If budget exhausted, **halt**.
- **Step 8f (analyst assessment)**: Auto-select "Note for later" — don't halt for advisory feedback.
- **Step 10 (next focus)**: Do not present options. Instead, use the **self-continuation mechanism** (see below).
- **Milestone re-opening guard (Step 3)**: Never auto-reopen done milestones. **Halt** and report.

### Self-Continuation via Agent Tool

The unsupervised loop is driven by `plan.yaml` as a state machine and the Agent tool as the continuation mechanism.

At **Step 10**, if `--unsupervised` is set and no halt condition was triggered:
1. Write all state to `plan.yaml` (phase_status, current_focus, regressions, etc.)
2. Spawn a new agent via the Agent tool with the task: `/ratchet:run --unsupervised`
3. The spawned agent reads `plan.yaml`, finds the next pending phase/milestone, and continues from Step 1

This creates a chain: each agent handles one milestone, persists state, and spawns the next. `plan.yaml` is the continuity mechanism — if the session crashes at any point, a manual `/ratchet:run --unsupervised` picks up from the last persisted state.

**Context clearing at milestone boundaries**: Self-continuation MUST happen at milestone boundaries — the spawned agent starts with a fresh context and re-reads all state from disk. This prevents context drift from auto-compaction summaries corrupting downstream work. Within a milestone, phases run in the same context (cross-phase continuity has value). Between milestones, fresh context forces the agent to rely on persisted state (plan.yaml, debate transcripts, scores) rather than compressed memories.

**Why Agent tool, not a shell loop**: Agents start with fresh context, can read/write project files, and the state machine (`plan.yaml`) handles crash recovery naturally. No external tooling or hook configuration required.

### Halt Conditions

Unsupervised mode **halts** (stops the loop and reports to the user) when:
1. A debate requires human escalation (`escalation: human`)
2. A blocking guard fails after 2 auto-fix attempts
3. A static analysis fix fails after 2 attempts
4. Regression budget is exhausted and no auto-resolution is possible
5. A done milestone would need re-opening
6. All milestones are complete (success)

On halt, present a summary:
```
Unsupervised run [completed|paused]:

  Milestones completed: [N]/[total]
  Phases completed: [N]
  Debates run: [N] (consensus: [N], escalated: [N], fast-path: [N])
  Guards run: [N] (passed: [N], failed: [N])
  Commits created: [N]

  [If paused: "Stopped at: [reason]. Resume with /ratchet:run or /ratchet:run --unsupervised"]
```

### Combining with Other Flags

- `--unsupervised --auto-pr`: Auto-create PRs at milestone boundaries (human pre-approves by passing this flag)
- `--unsupervised --no-cache`: Force re-debate all files, unsupervised
- `--unsupervised --all-files`: Run all pairs against all files, unsupervised
- `--unsupervised --dry-run`: Dry-run takes precedence (preview only, no execution)

## Prerequisites
- `.ratchet/` must exist with valid config
- At least one pair must be registered and enabled

If `.ratchet/` does not exist, inform the user:
> "Ratchet is not initialized for this project. Run /ratchet:init to set up."

Then use `AskUserQuestion` with options: `"Initialize now (/ratchet:init) (Recommended)"`, `"Cancel"`.

If no enabled pairs exist in the workflow config (`workflow.yaml`), inform the user:
> "No active pairs found. Add a pair with /ratchet:pair."

Then use `AskUserQuestion` with options: `"Add a pair (/ratchet:pair) (Recommended)"`, `"Cancel"`.

## Execution Steps

### Step 1: Resolve Workspace and Read Context

#### 1a. Workspace Resolution

Determine which `.ratchet/` directory to use:

1. **Walk up from CWD** looking for `.ratchet/workflow.yaml`
2. Read the first `workflow.yaml` found. If it has a `workspaces` key → **workspace root**
3. **Workspace root resolution**:
   - If the user specified a workspace name as an argument (e.g., `/ratchet:run monitor`) → use that workspace
   - If CWD is inside a workspace's `path` → auto-select that workspace
   - Otherwise → present workspace selector via `AskUserQuestion`:
     ```
     Workspaces: [N]

       [name] — [status summary from workspace plan.yaml]
       [name] — [status summary]

     Which workspace?
     ```
     Options: one per workspace name, plus `"Done for now"`
4. **Workspace selected** → set the effective `.ratchet/` path to `<workspace-path>/.ratchet/` and prepend `<workspace-path>/` to all file operations for the rest of this run
5. **Inherit policy from root**: Read the root `workflow.yaml` for shared policy fields (`models`, `escalation`, `max_rounds`, `max_regressions`, `pr_scope`). The workspace's own `workflow.yaml` overrides these per-field (not all-or-nothing — e.g., a workspace can override just `models.adversarial` and inherit everything else)
6. **No `workspaces` key** → single-project mode, use `.ratchet/` as-is (no change from current behavior)

#### 1b. Read State

Read `plan.yaml` (if it exists), `project.yaml`, and `workflow.yaml` from the resolved `.ratchet/` directory.

Build a picture of:
- Which milestones are **completed** (status: done)
- Which milestone is **current** (status: in_progress) and its **phase_status**
- Which milestones are **pending**
- Any unresolved conditions from previous CONDITIONAL_ACCEPT verdicts
- Which **phases** apply to the current milestone's component workflows

If no `plan.yaml` exists, skip epic tracking and fall through to file-based detection.

### Step 2: Determine Focus

There are four modes, checked in order:

#### Mode A: Explicit pair or --all-files
If the user specified a `[pair-name]` or `--all-files`, use that directly. Skip epic negotiation.

#### Mode B: Epic-guided (plan.yaml exists)
Use `AskUserQuestion` to let the user pick the focus. Include epic status AND phase progress:

Question text (build from plan.yaml):
```
Epic: [project name] — [completed]/[total] milestones done.

Current milestone: [name] — [description]
Phase progress: plan ✓ → test ✓ → build ● → review ○ → harden ○
(✓ = done, ● = current, ○ = pending)
[If regressions > 0: "Regressions: [N]/[max_regressions] used"]

What should we focus on?
```

If there are unresolved conditions from previous CONDITIONAL_ACCEPTs, append:
`"(Unresolved from last run: [condition1], [condition2])"`

Options:
- "Continue [current phase] phase for [milestone name] (Recommended)"
- "Address unresolved conditions from last run" — only if conditions exist
- "[Next milestone name]" — skip ahead
- "Review all existing code"
- (Include an "Other" option so the user can type a custom focus)

#### Mode C: Changed files (no plan.yaml, git repo exists)
```bash
git diff --name-only HEAD 2>/dev/null || git diff --name-only
git diff --name-only --cached
```
Match changed files to pairs by `scope` globs. For each changed file, match against ALL component scopes — not just the first match. Collect pairs from all matching components for the current phase. If a change spans multiple components, present: "This change spans [components]. Running pairs from all matching components."

#### Mode D: Greenfield (no plan.yaml, no code)
Use `AskUserQuestion` to ask what to build first.

### Step 3: Set Focus in Plan

If using epic mode, update `plan.yaml`:
- Set the chosen milestone to `status: in_progress`
- Record `current_focus` with milestone id and timestamp
- Determine the **current phase** — the first phase in `phase_status` that is not `done`

**Progress tracking**: If a progress adapter is configured (`.ratchet/workflow.yaml` → `progress.adapter`), and this milestone doesn't have a `progress_ref` yet, create a work item:
```bash
bash .claude/ratchet-scripts/progress/<adapter>/create-item.sh "<milestone name>" "<milestone description>" "ratchet" "milestone"
```
Store the returned reference in `plan.yaml` as `progress_ref` on the milestone. If the adapter fails, log a warning and continue — adapter failures never block debates.

**MILESTONE RE-OPENING GUARD**: If the chosen milestone has `status: done`, do NOT silently re-open it. Instead, use `AskUserQuestion`:
- Question: "Milestone '[name]' is already marked done (completed [timestamp]). Re-opening it will reset its status. Are you sure?"
- Options: `"Re-open milestone"`, `"Pick a different milestone"`, `"Cancel"`
- Only set `status: in_progress` if the user explicitly confirms re-opening.

### Step 4: Determine Active Phase and Match Pairs

**For v2 (workflow.yaml with phases):**

1. Identify the current phase from `phase_status` — the first phase that is `pending` or `in_progress`
2. Determine which phases apply based on the component workflows:
   - `tdd`: plan → test → build → review → harden
   - `traditional`: plan → build → review → harden (skip test)
   - `review-only`: review only (skip plan, test, build, harden)
3. Match pairs assigned to the current phase (from the pair's `phase` field)
4. Skip disabled pairs (`enabled: false`)

**Scope resolution (all modes):**
- If a pair has `scope: "auto"`, resolve it to the parent component's scope glob before matching files.

**For explicit pair / changed files / all-files modes:**
- Run the specified pairs regardless of phase assignment

### Step 5: File-Hash Cache Check

For each matched pair, run the cache check script:

```bash
bash .claude/ratchet-scripts/cache-check.sh <pair-name> "<scope-glob>"
```

- Exit 0 → files unchanged, **skip this pair**: `Skipping [pair-name] — no changes since last consensus`
- Exit 1 → files changed or no cache, proceed to debate

Use `--no-cache` flag to skip this check and force re-debate.

### Step 5b: Dry-Run Preview

If `--dry-run` is specified, produce a formatted preview and stop. No agents are spawned, no debates created, no files modified.

Present:
```
Dry-Run Preview
═══════════════

Milestone: [name] — [description]
Phase: [current phase]

Matched pairs:
  [pair-name] → [scope] ([N] files)
  [pair-name] → [scope] ([N] files)

Pre-debate guards:
  [guard-name] — [command] (blocking: [yes/no])

Post-debate guards:
  [guard-name] — [command] (blocking: [yes/no])

Phase flow: [phase1] → [phase2] → ... → [phaseN]

[If cross-cutting: "This change spans [components]"]
```

Then use `AskUserQuestion`:
- Options: `"Run for real (Recommended)"`, `"Done for now"`
- If "Run for real": restart from Step 6c without `--dry-run`

After a debate reaches consensus, update the cache:
```bash
bash .claude/ratchet-scripts/cache-update.sh <pair-name> "<scope-glob>" <debate-id>
```

### Step 6c: Pre-Debate Guards

Before creating debates, run guards where `timing: "pre-debate"` for the current phase. Guards without a `timing` field are treated as `post-debate` (backward compatible).

For each pre-debate guard assigned to the current phase:
```bash
bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>
```

- If a **blocking** pre-debate guard fails:
  - Use `AskUserQuestion`: "Pre-debate guard '[name]' failed: [summary]. Debates have NOT started yet."
  - Options: `"Fix and re-run (Recommended)"`, `"Override and proceed to debates"`, `"Cancel — skip this phase"`
  - If fix or cancel: skip debate creation entirely
- If an **advisory** pre-debate guard fails:
  - Log the failure and pass the output as context to the debates
  - Continue to debate creation

### Step 6a: Prepare Debate Context

For each matched pair, prepare the context that the **debate-runner** agent needs:

1. **Resolve `max_rounds`**: Use the pair-level `max_rounds` if set in workflow.yaml, otherwise use the global `max_rounds`.

2. **Resolve issue association**: If the current milestone has an `issues` array, determine which issue this pair belongs to by matching the pair name against each issue's `pairs` list.

3. **Gather escalation precedents**: Scan `.ratchet/escalations/` for existing rulings with the same pair name. Summarize any matching precedents (pair, dispute type, verdict direction, count).

4. **Gather phase context**:
   - If phase > plan: read the plan phase spec output
   - If phase > test: read test file locations
   - Collect any unresolved CONDITIONAL_ACCEPT conditions from previous debates

5. **Resolve models**: Read `models` from workflow.yaml (global defaults). For each pair, check if the pair has a `models` override — pair-level overrides take precedence over global defaults. If no `models` section exists at all, all agents inherit the parent conversation's model. Pass the resolved `generative`, `adversarial`, and `tiebreaker` models to the debate-runner.

### Step 6b: Static Analysis Pre-Gate

Before starting debates, run any configured static analysis commands from `project.yaml`.

Run each configured command. If any fail:
1. Present the failures in the question text
2. Use `AskUserQuestion` to let the user decide:
   - Question: "Static analysis failed with [N] errors: [summary]. How should we proceed?"
   - Options: `"Fix these before debating (Recommended)"`, `"Proceed to debates anyway"`
   - If fix: stop here, let the user fix, then re-run `/ratchet:run`
   - If proceed: note failures in debate context

If all pass (or none configured), proceed silently.

### Step 7: Run Debates

Spawn a **debate-runner** agent (from `agents/debate-runner.md`) for each matched pair. When multiple pairs match the current phase, spawn them **in parallel** using separate Agent calls.

Use `model` set to the resolved `debate_runner` model from Step 6a.5 (defaults to `sonnet` if no `models` config exists).

Each debate-runner receives:

```
Run debate for pair [pair-name] in phase [phase].

Pair definitions:
  Generative: .ratchet/pairs/<name>/generative.md
  Adversarial: .ratchet/pairs/<name>/adversarial.md

Context:
  Phase: [current phase]
  Milestone: [id, name, description]
  Issue: [issue ref or null]
  Files in scope: [matched file list]
  Max rounds: [resolved value from Step 6a]
  Escalation policy: [from workflow.yaml]
  Escalation precedents: [summary from Step 6a, or "none"]
  Plan phase output: [path, if phase > plan]
  Test phase output: [paths, if phase > test]
  Previous debate context: [unresolved conditions, if any]
  Models:
    generative: [resolved model from Step 6a.5]
    adversarial: [resolved model from Step 6a.5]
    tiebreaker: [resolved model from Step 6a.5]
```

The debate-runner handles all round management, generative/adversarial agent spawning, verdict parsing, escalation, and artifact persistence. See `agents/debate-runner.md` for the full protocol.

#### Handle Results

Each debate-runner returns a result object. Process each result:

- **`verdict: "consensus"`** (ACCEPT, CONDITIONAL_ACCEPT, or TRIVIAL_ACCEPT):
  - Update file-hash cache:
    ```bash
    bash .claude/ratchet-scripts/cache-update.sh <pair-name> "<scope-glob>" <debate-id>
    ```
  - If issue association exists, update the issue's `files` array in `plan.yaml` with `files_modified` from the result, and append the debate ID to the issue's `debates` array
  - If CONDITIONAL_ACCEPT: log conditions for tracking
  - If TRIVIAL_ACCEPT: note `fast_path: true` for auto-advance logic in Step 8c

- **`verdict: "escalated"`** (human escalation required):
  - Phase stays `in_progress`
  - Use `AskUserQuestion`: "Debate [id] requires human escalation."
  - Options: `"Resolve now (/ratchet:verdict [id]) (Recommended)"`, `"Continue with other pairs"`, `"Done for now"`

- **`verdict: "regress"`** (REGRESS):
  - Extract `regress_target` and `regress_reasoning` from the result
  - Proceed to Step 8e (Phase Regression)

**IMPORTANT**: Do NOT run debates yourself. Do NOT spawn generative or adversarial agents directly. The debate-runner agent is the ONLY path to running debates. If no debate-runner is spawned, no code gets written. This is a structural constraint, not a suggestion.

**IMPORTANT**: After processing debate results, you MUST proceed through ALL of Step 8 — including 8d (commit/PR). Do NOT skip to the next issue, phase, or milestone without packaging the work. Every completed scope boundary (debate, phase, milestone, or issue — per `pr_scope`) MUST produce a commit or PR before moving on. If you find yourself starting the next piece of work without having committed the previous one, STOP and go back to Step 8d.

### Step 8: Phase Gate — Run Guards and Advance

After all debates for the current phase resolve:

**8a. Check debate-runner results:**
- If all debate-runners returned `verdict: "consensus"` → proceed to guards
- If any returned `verdict: "escalated"` → phase stays `in_progress`, report which debates need resolution
- If any returned `verdict: "regress"` → proceed to Step 8e before checking guards

**8b. Run post-debate guards for this phase (v2 only):**

Read `guards` from `workflow.yaml`. Filter to guards where `timing: "post-debate"` or where `timing` is not set (backward compatible — guards without a timing field default to post-debate). For each matching guard assigned to the current phase, run the guard script:

```bash
bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>
```

This stores results in `.ratchet/guards/<milestone-id>/<phase>/<guard-name>.json`:
```json
{
  "guard": "<name>",
  "phase": "<phase>",
  "command": "<command>",
  "exit_code": 0,
  "output": "<stdout+stderr>",
  "blocking": true,
  "timestamp": "<ISO timestamp>"
}
```

- If a **blocking** guard fails:
  - Use `AskUserQuestion`: "Blocking guard '[name]' failed: [summary of output]. How should we proceed?"
  - Options: `"Fix and re-run (Recommended)"`, `"Override and advance anyway"`, `"View full output"`
  - If fix: phase stays `in_progress`, user fixes the issue, re-runs `/ratchet:run`
  - If override: log the override, advance anyway

- If an **advisory** guard fails:
  - Log the failure
  - Pass the output as context to the next phase's debates
  - Do NOT block advancement

**8c. Advance phase:**

If all debates passed and all blocking guards passed (or were overridden):
- Mark current phase as `done` in `phase_status`
- **Auto-advance on fast-path**: If ALL debates in the phase had `fast_path: true` (TRIVIAL_ACCEPT), auto-advance without `AskUserQuestion`. Still run post-debate guards. Present: "All pairs fast-pathed. Auto-advancing to [next phase]."
- Check if there's a next phase (based on the component's workflow preset):
  - If yes: set next phase to `in_progress`
  - If no (all phases done): mark milestone as `status: done`, record completion timestamp

Update `plan.yaml` with the new `phase_status`.

**Progress tracking**: If a progress adapter is configured and the milestone has a `progress_ref`:
- On phase advancement: add a comment noting the phase completed
  ```bash
  bash .claude/ratchet-scripts/progress/<adapter>/add-comment.sh "<progress_ref>" "Phase [name] complete. [N] pairs, all consensus. Moving to [next phase]."
  ```
- On milestone completion: update status and close the item
  ```bash
  bash .claude/ratchet-scripts/progress/<adapter>/update-status.sh "<progress_ref>" "done"
  bash .claude/ratchet-scripts/progress/<adapter>/close-item.sh "<progress_ref>"
  ```
- Adapter failures never block — log a warning and continue.

**8e. Phase Regression (on REGRESS verdict):**

When an adversarial agent issues a REGRESS verdict targeting an earlier phase:

1. Read `max_regressions` from workflow config (default: 2). If it's an integer, that limit applies to all phases. If it's an object with per-phase keys (e.g., `{ "build": 3, "review": 1 }`), use the limit for the current phase, falling back to 2 for unspecified phases. The budget is tracked per-milestone.
2. Track `regressions` counter in `plan.yaml` on the current milestone (initialize to 0 if absent). Also track per-phase regression counts if per-phase limits are configured.
3. If budget exhausted (`regressions >= max_regressions` for the current phase):
   - Use `AskUserQuestion`: "Regression budget exhausted ([N]/[max]). The adversarial wants to regress from [current] to [target] because: [reasoning]."
   - Options: `"Allow one more regression"`, `"Reject regression — continue current phase"`, `"Escalate to human"`
4. If within budget (or human allowed):
   - Increment `regressions` counter in `plan.yaml`
   - Reset `phase_status` for the target phase and all later phases to `pending`
   - Set the target phase to `in_progress`
   - Preserve all debate history (do not delete previous rounds)
   - Present: "Regressing from [current] to [target]. Reason: [reasoning]. Regression [N]/[max]."
   - Return to Step 4 (re-match pairs for the target phase)

**8d. Commit or PR:**

Work is packaged based on `pr_scope` from `workflow.yaml` (default: `debate`). This step triggers at the boundary matching the configured scope:

- `pr_scope: debate` — after each debate reaches consensus (Step 7, after verdict)
- `pr_scope: phase` — after all debates in a phase complete and guards pass (Step 8c)
- `pr_scope: milestone` — after all phases complete (milestone done)
- `pr_scope: issue` — one PR per individual issue tracked in the milestone's `issues` array. A milestone with 4 issues produces 4 PRs. Each PR contains exactly the files recorded in that issue's `files` list in `plan.yaml` (populated during debates via the `issue` field in `meta.json`). The PR is created when all debates for that issue reach consensus. If an issue's changes are too large for a single PR (more than 3 phases with substantive file changes), split into per-phase PRs and link them all to the issue.

**Auto-detection** (when `pr_scope` is not explicitly set):
1. If a progress adapter is configured (`github-issues`, `linear`, `jira`) → default to `issue`
2. If no adapter is configured, but the project has GitHub Issues activity (check via `gh issue list --limit 5`), use `AskUserQuestion`:
   - Question: "This project uses GitHub Issues. Would you like PRs scoped to issues? This links each PR to its corresponding issue for traceability."
   - Options: `"Yes — one PR per issue (Recommended)"`, `"No — one PR per debate"`, `"No — one PR per phase"`, `"No — one PR per milestone"`
   - If "Yes": set `pr_scope: issue` and suggest enabling the `github-issues` progress adapter
3. Otherwise → default to `debate`

This check runs once on the first `/ratchet:run` when `pr_scope` is unset. The user's choice is persisted to `workflow.yaml` so it's not asked again.

When the boundary is reached, use `AskUserQuestion`:

- Question: "[Scope] complete: [context]. How should we package this?"
  - For `debate`: "Debate [pair-name] reached consensus in [phase] phase."
  - For `phase`: "Phase [name] complete for [milestone]. [N] pairs, all consensus."
  - For `milestone`: "Milestone '[name]' complete. All phases passed, all guards green."
  - For `issue`: "Issue [ref]: [title] — all debates resolved. [N] files changed."
- Options:
  - `"Commit locally (Recommended)"` — create a local git commit with a summary
  - `"Create a pull request"` — commit, create branch if needed, push branch, open PR
  - `"Skip — I'll handle it"` — do nothing

**If "Commit locally":**
- Stage files that were created or modified during this scope's debates
- Generate a commit message scoped to what was done:
  - `debate`: pair name, phase, and verdict summary
  - `phase`: phase name, pairs involved, and outcome
  - `milestone`: milestone name, description, and debate outcomes
  - `issue`: issue reference, title, and summary of changes
- Create the commit — do NOT push

**If "Create a pull request":**
- Stage and commit (same as above)
- Create a branch if not already on one:
  - `debate`: `ratchet/<milestone-slug>/<pair-name>`
  - `phase`: `ratchet/<milestone-slug>/<phase>`
  - `milestone`: `ratchet/<milestone-slug>`
  - `issue`: `ratchet/<issue-ref>` (or `ratchet/<issue-ref>/<phase>` if split)
- Push the branch to origin — this is the ONE case where pushing is allowed, because the user explicitly chose "Create a pull request"
- Create the PR using `gh pr create` with:
  - Title scoped to the boundary (pair name, phase name, milestone name, or issue title)
  - Body: summary of what was done, debate outcomes, guard results
  - For `issue` scope: include `Closes [issue ref]` in the body so the issue is auto-closed on merge
  - Link to progress adapter item if one exists

After PR is created, use `AskUserQuestion`:
- Question: "PR created: [URL]. CI checks are running. What do you want to do?"
- Options:
  - `"Monitor CI checks and analyze results (Recommended)"` — runs `/ratchet:retro monitor <pr-number>` to watch checks, then auto-analyzes any failures and feeds learnings back into agents/guards
  - `"Continue while CI runs"` — proceed; the user can run `/ratchet:retro pr <number>` later
  - `"Monitor CI in background, continue working"` — background monitoring, continue working; report back when checks complete
  - `"Done for now"`

**8f. Post-Milestone Analyst Assessment:**

After commit or PR (regardless of which packaging option the user chose), spawn the analyst agent (with `model` set to the resolved `analyst` model from workflow.yaml, defaults to `opus`) for a brief post-milestone assessment. The analyst reviews the milestone's debates, scores, guard results, and any retro/escalation data to produce 3-5 bullet points covering:
- Pair effectiveness observations (any pairs that always fast-path? always escalate?)
- Scope coverage gaps discovered during this milestone
- Guard recommendations (missing checks, overly strict guards)
- Workflow preset recommendations (should any component switch presets?)

Present via `AskUserQuestion`:
- Question: "Post-milestone assessment for [milestone name]:\n[bullet points]"
- Options: `"Apply recommendations (Recommended)"`, `"Note for later"`, `"Skip"`
- If "Apply recommendations": walk through each recommendation with follow-up `AskUserQuestion` calls

**CRITICAL: NEVER push to origin/main or force-push. NEVER push unless the user explicitly chose "Create a pull request". Local commits are the default safe action.**

### Step 9: Update Scores

Post-debate reviews are generated by the debate-runner agent (Step 4 in `agents/debate-runner.md`) — they are NOT the main thread's responsibility.

For each resolved debate, run the score update script:
```bash
bash .claude/ratchet-scripts/update-scores.sh <debate-id>
```

### Step 10: Propose Next Focus

**If `--unsupervised`**: Skip `AskUserQuestion`. If no halt condition was triggered and work remains (more phases or milestones), persist all state to `plan.yaml` and spawn a new agent via the Agent tool with task `/ratchet:run --unsupervised`. If all milestones are complete, halt with the completion summary. If a halt condition was triggered during this iteration, present the halt summary and stop.

**Otherwise**, use `AskUserQuestion` to let the user choose what to do next.

**If more phases remain in the current milestone:**
- Summary: `"Phase [name] complete for [milestone]. [N] pairs debated, [N] consensus. Next phase: [name]."`
- Options:
  - "Continue to [next phase] phase (Recommended)"
  - "Review debate: [debate-id]" — one per debate that just ran
  - "View quality metrics"
  - "Address unresolved conditions" — only if CONDITIONAL_ACCEPTs logged conditions

**If all phases done, more milestones remain:**

**CONTEXT CLEARING**: Milestone boundaries are the primary context clearing point. Persisted state (plan.yaml, debate transcripts, pair definitions, scores) is the source of truth — not context memory. A fresh context forces re-reading actual files, preventing drift from auto-compaction summaries.

- Summary: `"Milestone [name] complete! All phases passed. Epic progress: [completed]/[total] milestones.\n\nStarting fresh context for the next milestone — all state is persisted to disk."`
- Options:
  - "Continue to [next milestone name] (/ratchet:run) (Recommended)" — user re-invokes with fresh context
  - "Review debate: [debate-id]"
  - "View quality metrics"
  - "View milestone status (/ratchet:status)"
  - "Done for now"

When the user selects "Continue to [next milestone name]", do NOT continue in the current context. Instead, present: "Run `/ratchet:run` to start [next milestone] with a clean context. All progress is saved." This ensures the next milestone starts from disk state, not from a potentially degraded context window.

**If ALL milestones are done:**
- Summary: `"Epic complete! All [N] milestones finished. Total debates: [N] | Consensus rate: [%] | Avg rounds: [N]"`
- Options:
  - "Create a pull request for all changes (Recommended)"
  - "View detailed quality metrics"
  - "Tighten agents from debate lessons"
  - "Review a specific debate"
  - "Done for now"
