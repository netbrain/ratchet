---
name: ratchet:run
description: Run agent pairs through phase-gated debates — guided by epic roadmap and current focus
---

# /ratchet:run — Execute Debate

## CRITICAL — You Are an Orchestrator, Not a Solver

You do NOT write code. You do NOT fix bugs. You do NOT implement features.
You do NOT resolve merge conflicts. You do NOT rebase branches.
You are a workflow orchestrator. Your ONLY tools are:

- **Read, Glob, Grep** — to read state files (plan.yaml, workflow.yaml, etc.)
- **Agent** — to spawn issue pipelines and debate-runners
- **AskUserQuestion** — to present choices to the user
- **Bash** — ONLY for:
  - Running guard scripts (`bash .claude/ratchet-scripts/...`)
  - Read-only git commands (`git status`, `git log`, `git branch`, `git diff`)
  - Read-only GitHub CLI (`gh pr list`, `gh pr view`, `gh issue list`)

**TOOL GATE — check EVERY Bash command before running it:**
- `git rebase` → STOP. This is code work. Route to an issue pipeline.
- `git merge` → STOP. This is code work. Route to an issue pipeline.
- `git cherry-pick` → STOP. This is code work. Route to an issue pipeline.
- Resolving merge conflicts → STOP. This is code work.
- `Write` or `Edit` on ANY file → STOP. Route to an issue pipeline.
- Reading a source code file to "understand" a conflict → STOP. You're
  about to start solving. Route to an issue pipeline.

If a PR has merge conflicts, that is work for the issue pipeline to resolve
through a debate. The orchestrator's job is to detect the conflict (via
`gh pr view`) and re-launch the issue pipeline to handle it — not to
resolve it directly.

Your job is to:

1. Read state (plan.yaml, workflow.yaml)
2. Build dependency graph of issues within the current milestone
3. Launch **issue pipelines** in parallel (each in an isolated worktree)
4. Process their results (milestone completion, next milestone)

Issue pipelines spawn debate-runner agents. Debate-runners spawn generative
and adversarial agents. The generative agent writes code. You do none of this.

---

The core Ratchet workflow. Operates at four levels:

- **Epic** — the full project roadmap (from `.ratchet/plan.yaml`)
- **Milestone** — a coherent deliverable, composed of one or more issues
- **Issue** — an independently executable unit of work with its own phase pipeline and PR
- **Phase** — the current stage within an issue (`plan → test → build → review → harden`)

Issues within a milestone run **in parallel** (unless they have explicit dependencies). Each issue progresses through its own phase pipeline independently, in an isolated git worktree. Phases within an issue are ordered and gated: phase N must complete before phase N+1 begins.

## Usage
```
/ratchet:run                # Resume from epic — propose next focus or run against changes
/ratchet:run [pair-name]    # Run a specific pair against its scoped files
/ratchet:run [workspace]    # Target a specific workspace
/ratchet:run --issue <ref>  # Run a single issue's pipeline (used by parallel spawning)
/ratchet:run --all-files    # Run all pairs against all files in scope
/ratchet:run --no-cache     # Force re-debate even if files haven't changed
/ratchet:run --dry-run      # Preview what would run without executing anything
/ratchet:run --unsupervised           # Run the full plan end-to-end without human intervention
/ratchet:run --unsupervised --auto-pr # Same, but auto-create PRs per issue
```

## Unsupervised Mode

When `--unsupervised` is set, the run loop executes the entire plan (all milestones, all phases) without human interaction. The principle is simple: **wherever an `AskUserQuestion` has a "(Recommended)" option, auto-select it.**

### Behavior

- **Step 1a (workspace)**: If at workspace root with no workspace specified, **halt** — unsupervised mode requires an explicit workspace target (`/ratchet:run --unsupervised monitor`). Auto-selecting a workspace is too risky.
- **Step 2 (focus)**: Auto-select "Run all ready issues in parallel" for the current milestone. When a milestone completes, auto-advance to the next.
- **Step 4 (issue pipelines)**: Launch all ready issues in parallel. Each issue pipeline runs autonomously.
- **Step 5-dry (dry-run)**: Incompatible with `--unsupervised` — if both are set, ignore `--dry-run`.
- **Step 5c (pre-debate guards)**: If a blocking pre-debate guard fails → auto-select "Fix and re-run". The generative agent attempts to fix the issue. If the fix fails after 2 attempts, that issue **halts** (other issues continue).
- **Step 6 (static analysis)**: Auto-select "Fix these before running". Same 2-attempt retry, then halt.
- **Step 5e (debates)**: Run normally. Debates are autonomous by nature.
- **Step 5e (escalation)**: If escalation policy is `tiebreaker` or `both`, auto-escalate to tiebreaker. If policy is `human`, that issue **halts** — this is the primary stop condition.
- **Step 5e (precedent)**: Auto-select "Apply settled pattern" when available.
- **Step 5f (post-debate guards)**: If blocking guard fails → auto-select "Fix and re-run" (2 attempts, then halt issue).
- **Step 5f (advance)**: Auto-advance to next phase. No user confirmation needed.
- **Step 5f (commit/PR)**: Auto-select "Commit locally" by default. If `--auto-pr` is also set, auto-select "Create a pull request" instead — the human pre-approved this by passing the flag.
- **Step 5g (regression)**: If within budget, auto-regress. If budget exhausted, **halt** issue.
- **Step 8c (analyst assessment)**: Auto-select "Note for later" — don't halt for advisory feedback.
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

**Issue-level halts** (the issue stops, other issues continue):
1. A debate requires human escalation (`escalation: human`)
2. A blocking guard fails after 2 auto-fix attempts
3. Regression budget is exhausted and no auto-resolution is possible

**Milestone-level halts** (the entire run stops):
4. A static analysis pre-gate fails after 2 attempts (Step 6)
5. A done milestone would need re-opening
6. All issues in the milestone are halted (no progress possible)
7. All milestones are complete (success)

On halt, present a summary:
```
Unsupervised run [completed|paused]:

  Milestones completed: [N]/[total]
  Issues completed: [N]/[total] ([N] in parallel)
  Debates run: [N] (consensus: [N], escalated: [N], fast-path: [N])
  Guards run: [N] (passed: [N], failed: [N])
  PRs created: [N]

  [If paused: "Stopped at: [reason]. Resume with /ratchet:run or /ratchet:run --unsupervised"]
  [If issues halted: "Halted issues: [ref]: [halt reason], ..."]
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
- Which milestone is **current** (status: in_progress)
- For the current milestone: which **issues** exist, their `phase_status`, `depends_on` relationships, and current status
- Which issues can run in **parallel** (no unmet dependencies) vs which must wait
- Any unresolved conditions from previous CONDITIONAL_ACCEPT verdicts
- Which **phases** apply based on component workflows

If no `plan.yaml` exists, skip epic tracking and fall through to file-based detection.

**CHECKPOINT**: You now understand the project state. Do NOT act on it — do not analyze code, do not plan fixes, do not write implementations. Your next action is Step 2: present choices to the user (or auto-select in unsupervised mode). Then Step 4: launch issue pipelines. The pipelines do the work.

### Step 2: Determine Focus

There are five modes, checked in order:

#### Mode S: Single-issue pipeline (--issue <ref>)
If `--issue` is set, skip all focus negotiation. Find the issue by ref in the current milestone's `issues` array. Jump directly to **Step 5** (Issue Pipeline) for this single issue. This mode is used when the orchestrator spawns parallel agents — each agent re-enters the run skill scoped to one issue. Do NOT spawn further issue-level agents; execute the pipeline directly.

#### Mode A: Explicit pair or --all-files
If the user specified a `[pair-name]` or `--all-files`, use that directly. Skip epic negotiation.

#### Mode B: Epic-guided (plan.yaml exists)
Use `AskUserQuestion` to let the user pick the focus. Include epic status with per-issue progress:

Question text (build from plan.yaml):
```
Epic: [project name] — [completed]/[total] milestones done.

Current milestone: [name] — [description]
[If regressions > 0: "Regressions: [N]/[max_regressions] used"]

Issues:
  [ref]: [title]  [DONE]
    plan ✓  test ✓  build ✓  review ✓  harden ✓
  [ref]: [title]  [IN PROGRESS]
    plan ✓  test ✓  build ●  review ○  harden ○
  [ref]: [title]  [PENDING — depends on [dep-ref]]
    plan ○  test ○  build ○  review ○  harden ○

(✓ = done, ● = current, ○ = pending)

What should we focus on?
```

If there are unresolved conditions from previous CONDITIONAL_ACCEPTs, append:
`"(Unresolved from last run: [condition1], [condition2])"`

Options:
- "Run all ready issues in parallel (Recommended)" — launches all issues with no unmet dependencies
- "Run specific issue: [ref]" — one option per ready issue
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

### Step 3: Set Focus and Build Dependency Graph

If using epic mode, update `plan.yaml`:
- Set the chosen milestone to `status: in_progress`
- Record `current_focus` with milestone id and timestamp

**Build dependency layers** from the milestone's issues:
1. **Layer 0**: issues with no `depends_on` (or all dependencies already `done`)
2. **Layer 1**: issues whose dependencies are all in Layer 0
3. **Layer N**: issues whose dependencies are all in earlier layers

This produces the execution order. Issues within the same layer run in parallel.

**Progress tracking**: If a progress adapter is configured (`.ratchet/workflow.yaml` → `progress.adapter`), and this milestone doesn't have a `progress_ref` yet, create a work item:
```bash
bash .claude/ratchet-scripts/progress/<adapter>/create-item.sh "<milestone name>" "<milestone description>" "ratchet" "milestone"
```
Store the returned reference in `plan.yaml` as `progress_ref` on the milestone. If the adapter fails, log a warning and continue — adapter failures never block debates.

**MILESTONE RE-OPENING GUARD**: If the chosen milestone has `status: done`, do NOT silently re-open it. Instead, use `AskUserQuestion`:
- Question: "Milestone '[name]' is already marked done (completed [timestamp]). Re-opening it will reset its status. Are you sure?"
- Options: `"Re-open milestone"`, `"Pick a different milestone"`, `"Cancel"`
- Only set `status: in_progress` if the user explicitly confirms re-opening.

### Step 4: Launch Issue Pipelines

**CHECKPOINT**: You are about to launch issue pipelines. Your ONLY action here is spawning Agent calls. If you are tempted to "quickly fix" something, write a test, or implement a feature before launching — STOP. That work belongs inside the issue pipeline's debate-runner, not here.

This is the core execution step. The orchestrator launches parallel pipelines for independent issues.

#### 4a. Identify Ready Issues

From the dependency graph built in Step 3, identify **ready issues** — issues whose status is not `done` and whose `depends_on` entries are all `done` (or empty).

**For explicit pair / --all-files modes:** Skip issue-based execution. Run the specified pairs directly using the single-issue flow (Step 5) without worktree isolation.

#### 4b. Launch Parallel Issue Runners

For each ready issue, spawn an Agent with `isolation: "worktree"` that re-invokes the run skill scoped to a single issue. The spawned agent enters Mode S (Step 2) and executes the issue pipeline directly.

**Worktree base branch:**
- If the issue has no `depends_on` → branch from main (or current branch)
- If the issue has `depends_on` → branch from the dependency's `branch` field in plan.yaml. This ensures the dependent issue builds on top of the dependency's changes.

Launch all ready issues **in parallel in a single message** — one Agent call per issue. Each agent's task:

```
/ratchet:run --issue [ref] [--unsupervised] [--auto-pr] [--no-cache]
```

Pass through any flags from the parent invocation. The spawned agent reads plan.yaml, finds the issue by ref, and executes the issue pipeline (Step 5) directly. It does NOT spawn further agents for issues — it IS the issue runner.

**Note on parallelism**: The Agent tool runs parallel calls concurrently but returns all results together — the orchestrator blocks until ALL agents in the batch complete. This means:
- Layer 0 issues (no dependencies) are all launched in one batch
- The orchestrator waits for the entire batch before processing any results
- Layer 1+ issues (with dependencies) are launched in a subsequent batch after their dependencies complete

Present a summary before the batch launches:

```
Launching [N] issue pipelines in parallel:
  [ref]: [title] — [N] pairs, starting at [phase]
  [ref]: [title] — [N] pairs, starting at [phase]

[If Layer 1+ exists: "[N] more issues waiting for dependencies"]
```

#### 4c. Process Issue Results (after batch completes)

When all agents in a batch return, process the results. **Do NOT fix, debug, or modify anything from the results — just report and route.** For each completed issue:

1. Read the issue's completion summary (the agent's final output — see Step 5h)
2. Present all summaries to the user
3. **Update the MAIN repo's `plan.yaml`** based on each issue runner's output. Issue pipelines run in worktrees — their plan.yaml updates are lost when the worktree is cleaned up. The orchestrator MUST write these updates:
   - Set `status` (done, blocked, escalated, failed)
   - Set `phase_status` for all phases
   - Set `branch` (the branch name created by the pipeline)
   - Set `files` (list of modified files)
   - Set `debates` (debate IDs created)
   - Parse these from the issue runner's structured completion summary (Step 5h)
4. Check if any **Layer 1+ issues** are now unblocked (their dependencies just completed)
5. If newly unblocked issues exist → launch them as a new parallel batch (back to 4b)
6. Report overall progress: `"[N]/[total] issues complete"`

**CRITICAL**: This plan.yaml update is not optional. Without it, milestone completion state is lost at context boundaries. The orchestrator is the authoritative writer — issue pipelines output results, the orchestrator persists them.

**When all issues across all layers are done** → milestone is complete, proceed to Step 8.

**If an issue runner returns with a halt (blocked/escalated/failed):**
- Present the early-exit summary
- In supervised mode: use `AskUserQuestion` to let the user decide how to proceed
  - Options: `"Resolve now"`, `"Continue with remaining issues (Recommended)"`, `"Done for now"`
- In unsupervised mode: if `escalation: human`, note the halt. Continue launching subsequent layers if possible — a halted issue only blocks issues that depend on it, not unrelated issues. Halt the entire run only if no progress is possible.

**Handling merge conflicts on existing PRs:**

When the orchestrator detects (via `gh pr view`) that an issue's PR has merge conflicts:
1. Do NOT attempt to resolve the conflict yourself (no rebase, no merge, no code editing)
2. Re-launch the issue pipeline (`/ratchet:run --issue <ref>`) in a fresh worktree based on the current main branch
3. The issue pipeline will re-run the build phase, which will naturally produce code that is compatible with the current main branch
4. The old PR branch is replaced — the pipeline creates a new branch and force-pushes (or creates a new PR)

This is not a special case — it's the normal pipeline flow. Merge conflicts mean the issue's code is stale relative to main. The correct response is to re-run the pipeline from the appropriate phase, not to manually patch the conflict.

**IMPORTANT**: Do NOT run debates yourself. Do NOT spawn generative or adversarial agents directly. Issue pipelines handle all debate orchestration. This is a structural constraint.

---

### Step 5: Issue Pipeline (runs per-issue, typically in a worktree)

This is the phase-gated loop for a single issue. You enter this step in two ways:
- **Mode S** (`--issue <ref>`): You ARE the issue runner. A parent orchestrator spawned you via Agent tool with `isolation: "worktree"`. Execute the pipeline directly — do NOT spawn further issue-level agents.
- **Single ready issue**: If only one issue is ready, the orchestrator can execute Step 5 inline without spawning a separate agent.

The issue pipeline progresses through phases sequentially for ONE issue, then returns a result to the caller (or updates plan.yaml directly if running inline).

#### 5a. Determine Current Phase and Match Pairs

1. Read the issue's `phase_status` — find the first phase that is `pending` or `in_progress`
2. Determine which phases apply based on the component workflows:
   - `tdd`: plan → test → build → review → harden
   - `traditional`: plan → build → review → harden (skip test)
   - `review-only`: review only (skip plan, test, build, harden)
3. Match pairs from the issue's `pairs` list that are assigned to the current phase
4. Skip disabled pairs (`enabled: false`)

**Scope resolution:**
- If a pair has `scope: "auto"`, resolve it to the parent component's scope glob.

#### 5b. File-Hash Cache Check

For each matched pair, run the cache check script:

```bash
bash .claude/ratchet-scripts/cache-check.sh <pair-name> "<scope-glob>"
```

- Exit 0 → files unchanged, **skip this pair**: `Skipping [pair-name] — no changes since last consensus`
- Exit 1 → files changed or no cache, proceed to debate

Use `--no-cache` flag to skip this check and force re-debate.

#### Shared Resources

Guards can declare `requires: [resource-name]` referencing shared resources defined in workflow.yaml. Resources are infrastructure dependencies (databases, test servers, etc.) that need provisioning and optionally singleton access.

**Resource lifecycle:**
1. **Provision** — before a guard runs, start any required resources that aren't already running
2. **Lock** — if a resource is `singleton: true`, acquire a file lock before the guard runs
3. **Release** — after the guard completes, release singleton locks
4. **Teardown** — after all pipelines in a milestone complete, run `stop` commands for resources that have them

**Provisioning**: run the resource's `start` command. Start commands must be idempotent (e.g., `docker compose up -d postgres` is safe to run multiple times). Track started resources in `.ratchet/locks/resources.json`:
```json
{"postgres": {"started": true, "pid": "$$", "at": "<ISO>"}}
```

**Singleton locking**: file-based locks in `.ratchet/locks/` (shared across worktrees since they share the same host filesystem).
```bash
# Acquire singleton lock — waits if another pipeline holds it
while ! mkdir .ratchet/locks/<resource-name>.lock 2>/dev/null; do
  sleep 2
done
echo '{"issue": "<ref>", "pid": "$$", "acquired": "<ISO timestamp>"}' > .ratchet/locks/<resource-name>.lock/info.json
```

After the guard completes:
```bash
rm -rf .ratchet/locks/<resource-name>.lock
```

If a lock is held for more than 10 minutes, consider it stale (crashed pipeline) and force-acquire.

**Example config:**
```yaml
resources:
  - name: postgres
    start: "docker compose up -d postgres"
    stop: "docker compose down postgres"
    singleton: true       # only one pipeline's tests hit the DB at a time

  - name: redis
    start: "docker compose up -d redis"
    singleton: false      # shared freely — no locking

guards:
  - name: integration-tests
    command: "npm run test:integration"
    phase: build
    blocking: true
    requires: [postgres, redis]    # postgres locks, redis just starts
```

When multiple singleton resources are required, acquire all locks in alphabetical order to prevent deadlocks.

**Teardown** (Step 9 — after milestone completes): for each resource with a `stop` command, run it. Clean up `.ratchet/locks/` directory.

#### 5c. Pre-Debate Guards

Run guards where `timing: "pre-debate"` for the current phase. Guards without a `timing` field are treated as `post-debate` (backward compatible).

**If the guard has `requires`**: provision required resources (run `start` if not already running), acquire singleton locks, run the guard, release locks (regardless of pass/fail).

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

#### 5d. Prepare Debate Context

For each matched pair, prepare the context for the **debate-runner** agent:

1. **Resolve `max_rounds`**: Pair-level if set, otherwise global.
2. **Gather escalation precedents**: Scan `.ratchet/escalations/` for matching pair rulings.
3. **Gather phase context**:
   - If phase > plan: read the plan phase spec output
   - If phase > test: read test file locations
   - Collect any unresolved CONDITIONAL_ACCEPT conditions
4. **Resolve models**: Pair-level overrides take precedence over global. Pass resolved `generative`, `adversarial`, and `tiebreaker` models to the debate-runner.

#### 5e. Run Debates

Spawn a **debate-runner** agent for each matched pair. When multiple pairs match the current phase, spawn them **in parallel**.

Use `model` set to the resolved `debate_runner` model (defaults to `sonnet`).

Each debate-runner receives:
```
Run debate for pair [pair-name] in phase [phase].

Pair definitions:
  Generative: .ratchet/pairs/<name>/generative.md
  Adversarial: .ratchet/pairs/<name>/adversarial.md

Context:
  Phase: [current phase]
  Milestone: [id, name, description]
  Issue: [issue ref]
  Files in scope: [matched file list]
  Max rounds: [resolved value]
  Escalation policy: [from workflow.yaml]
  Escalation precedents: [summary or "none"]
  Plan phase output: [path, if phase > plan]
  Test phase output: [paths, if phase > test]
  Previous debate context: [unresolved conditions, if any]
  Models:
    generative: [resolved model]
    adversarial: [resolved model]
    tiebreaker: [resolved model]
```

#### Handle Debate Results

Process each debate-runner result:

- **`verdict: "consensus"`** (ACCEPT, CONDITIONAL_ACCEPT, or TRIVIAL_ACCEPT):
  - Update file-hash cache:
    ```bash
    bash .claude/ratchet-scripts/cache-update.sh <pair-name> "<scope-glob>" <debate-id>
    ```
  - Update the issue's `files` array with `files_modified` and append debate ID to `debates`
  - If CONDITIONAL_ACCEPT: log conditions
  - If TRIVIAL_ACCEPT: note `fast_path: true`

- **`verdict: "escalated"`** (human escalation required):
  - Update issue status in plan.yaml
  - Output the early-exit summary (see Step 5h) and return

- **`verdict: "regress"`** (REGRESS):
  - Handle regression (Step 5g)

**IMPORTANT**: Do NOT run debates yourself. The debate-runner is the ONLY path.

**IMPORTANT**: After processing debate results, proceed through ALL of Step 5f — including commit/PR. Do NOT skip to the next phase without packaging the work.

#### 5f. Phase Gate — Guards and Advance

**Check results:**
- All consensus → proceed to guards
- Any escalated → update plan.yaml, output early-exit summary (Step 5h), return
- Any regress → proceed to Step 5g

**Run post-debate guards:**

**If the guard has `requires`**: provision required resources and acquire singleton locks (same as pre-debate guards).

For each guard where `timing: "post-debate"` (or no timing field) assigned to the current phase:
```bash
bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>
```

Guard result storage: `.ratchet/guards/<milestone-id>/<issue-ref>/<phase>/<guard-name>.json`

- Blocking guard fails → AskUserQuestion: fix/override/view
- Advisory guard fails → log and continue

**Advance phase:**
- Mark current phase as `done` in the issue's `phase_status`
- Auto-advance on fast-path (all TRIVIAL_ACCEPT) without user confirmation
- If next phase exists → set to `in_progress`, loop back to Step 5a
- If all phases done → issue is complete

**Commit/PR at configured boundaries:**
Work is packaged based on `pr_scope`:
- `pr_scope: debate` — after each debate consensus
- `pr_scope: phase` — after each phase completes
- `pr_scope: issue` — after all phases complete (issue done). This is the natural default for parallel execution.
- `pr_scope: milestone` — defer to orchestrator (Step 8)

When creating a PR for an issue:
- Branch name: `ratchet/<milestone-slug>/<issue-ref>`
- Title: issue title
- Body includes:
  - Summary of phases completed, debate outcomes, guard results
  - `Closes [issue ref]` if using a progress adapter with issue tracking
  - **If this issue has `depends_on`**: "Depends on [dep-ref PR URL] being merged first." This tells reviewers the merge order.
- Push and create via `gh pr create`

Store the branch name and PR URL in the issue's `branch` and return them to the orchestrator.

**Progress tracking**: If a progress adapter is configured:
- On phase advancement: add a comment noting the phase completed
- On issue completion: update status

#### 5g. Phase Regression

When an adversarial issues REGRESS targeting an earlier phase:

1. Read `max_regressions` from workflow config (default: 2). Budget is tracked per-milestone (shared across issues — the `regressions` counter is on the milestone).
2. If budget exhausted:
   - Use `AskUserQuestion`: regression budget exhausted, allow/reject/escalate
3. If within budget:
   - Increment milestone's `regressions` counter
   - Reset the issue's `phase_status` for target phase and later to `pending`
   - Set target phase to `in_progress`
   - Preserve debate history
   - Loop back to Step 5a

#### 5h. Issue Complete

When all phases are done:
- Set issue status to `done` in local plan.yaml (worktree copy — useful for crash recovery, but the orchestrator will write the authoritative update in Step 4c)
- Run score updates for all debates in this issue

**Output a structured completion summary as your final message.** This is critical — the orchestrator parses this to update the main repo's plan.yaml (Step 4c). The worktree is cleaned up after the agent returns, so this output is the only way results survive.

```
Issue [ref] complete:
  status: done
  phase_status:
    [phase]: done
    [phase]: done
    [phase]: skipped ([reason, e.g. traditional workflow])
    ...
  debates: [debate-id-1, debate-id-2, ...]
  files: [file1, file2, ...]
  branch: [branch name]
  pr: [URL or "none"]
```

**If the pipeline exits early** (escalation, guard failure, regression budget exhausted), output:

```
Issue [ref] [blocked|escalated|failed]:
  status: [blocked|escalated|failed]
  phase_status:
    [phase]: done
    [phase]: [failed|blocked] — [reason for halt]
    [phase]: pending
    ...
  halted_at: [phase]
  halt_reason: [reason]
  debates: [debate-id-1, ...]
  files: [file1, ...]
  branch: [branch name or "none"]
  pr: [URL or "none"]
```

This summary MUST be the last thing you output. The orchestrator reads plan.yaml for structured state, but this summary provides immediate human-readable feedback.

---

### Step 5-dry: Dry-Run Preview

If `--dry-run` is specified, produce a formatted preview and stop. No agents are spawned, no debates created, no files modified.

```
Dry-Run Preview
═══════════════

Milestone: [name] — [description]

Issues ([N] total, [N] ready to run in parallel):

  [ref]: [title]
    Phase: [current phase]
    Pairs: [pair-name], [pair-name]
    Pre-debate guards: [guard-name] (blocking)
    Post-debate guards: [guard-name] (advisory)

  [ref]: [title]  (depends on [dep-ref])
    Phase: pending — waiting for dependency
    Pairs: [pair-name]

Phase flow per issue: [phase1] → [phase2] → ... → [phaseN]
```

Then use `AskUserQuestion`:
- Options: `"Run for real (Recommended)"`, `"Done for now"`

---

### Step 6: Static Analysis Pre-Gate

Before launching issue pipelines, run any configured static analysis commands from `project.yaml` on the main working tree.

If any fail:
- Use `AskUserQuestion`: "Static analysis failed with [N] errors: [summary]. How should we proceed?"
- Options: `"Fix these before running (Recommended)"`, `"Proceed anyway"`
- If fix: stop here, let the user fix, then re-run

If all pass (or none configured), proceed silently.

---

### Step 7: (Reserved — number kept for reference continuity)

---

### Step 8: Milestone Completion

After all issues in the milestone are `done`:

**8a. Mark milestone done:**
- Set milestone `status: done`, record completion timestamp
- Update `plan.yaml`

**8b. Progress tracking:**
- If adapter configured: update milestone status, close the item
  ```bash
  bash .claude/ratchet-scripts/progress/<adapter>/update-status.sh "<progress_ref>" "done"
  bash .claude/ratchet-scripts/progress/<adapter>/close-item.sh "<progress_ref>"
  ```

**8c. Post-Milestone Analyst Assessment:**

Spawn the analyst agent (with resolved `analyst` model, defaults to `opus`) for a brief assessment. The analyst reviews all issue debates, scores, guard results, and any retro/escalation data to produce 3-5 bullet points covering:
- Pair effectiveness observations
- Scope coverage gaps
- Guard recommendations
- Workflow preset recommendations

Present via `AskUserQuestion`:
- Question: "Post-milestone assessment for [milestone name]:\n[bullet points]"
- Options: `"Apply recommendations (Recommended)"`, `"Note for later"`, `"Skip"`

**CRITICAL: NEVER push to origin/main or force-push. NEVER push unless the user explicitly chose "Create a pull request" within an issue pipeline. Local commits are the default safe action.**

### Step 9: Update Scores & Teardown Resources

Score updates are handled within issue pipelines (Step 5h). The orchestrator does not need to run score updates separately.

**Resource teardown**: after all issue pipelines for the milestone complete, tear down shared resources:
1. For each resource in `workflow.yaml` that has a `stop` command, run it
2. Clean up `.ratchet/locks/` directory (remove `resources.json` and any stale lock directories)

Resources are torn down regardless of whether the milestone succeeded, halted, or had errors — always clean up.

### Step 10: Propose Next Focus

**If `--unsupervised`**: Skip `AskUserQuestion`. If no halt condition was triggered and work remains (more milestones), persist all state to `plan.yaml` and spawn a new agent via the Agent tool with task `/ratchet:run --unsupervised`. If all milestones are complete, halt with the completion summary. If a halt condition was triggered during this iteration, present the halt summary and stop.

**Otherwise**, use `AskUserQuestion` to let the user choose what to do next.

**If the milestone has blocked/escalated issues:**
- Summary: `"Milestone [name]: [N]/[total] issues complete. [N] blocked/escalated."`
- Options:
  - "Resolve escalated debates (/ratchet:verdict)" — if escalated
  - "View issue status" — show per-issue phase progress
  - "Re-run to continue (/ratchet:run) (Recommended)" — picks up unblocked issues
  - "Done for now"

**If milestone complete, more milestones remain:**

**CONTEXT CLEARING**: Milestone boundaries are the primary context clearing point. Persisted state (plan.yaml, debate transcripts, pair definitions, scores) is the source of truth — not context memory. A fresh context forces re-reading actual files, preventing drift from auto-compaction summaries.

- Summary: `"Milestone [name] complete! [N] issues, [N] PRs created. Epic progress: [completed]/[total] milestones.\n\nStarting fresh context for the next milestone — all state is persisted to disk."`
- Options:
  - "Continue to [next milestone name] (/ratchet:run) (Recommended)" — user re-invokes with fresh context
  - "View quality metrics"
  - "View milestone status (/ratchet:status)"
  - "Done for now"

When the user selects "Continue to [next milestone name]", do NOT continue in the current context. Instead, present: "Run `/ratchet:run` to start [next milestone] with a clean context. All progress is saved."

**If ALL milestones are done:**
- Summary: `"Epic complete! All [N] milestones finished. Total issues: [N] | Total debates: [N] | Consensus rate: [%]"`
- Options:
  - "View detailed quality metrics"
  - "Tighten agents from debate lessons"
  - "Review a specific debate"
  - "Done for now"
