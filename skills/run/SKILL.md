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
2. Build dependency graphs — milestones (if DAG mode) and issues within each milestone
3. Launch **milestone pipelines** in parallel (DAG mode) or sequentially
4. Within each milestone, launch **issue pipelines** in parallel (each in an isolated worktree)
5. Process their results (milestone completion, next milestone layer, epic completion)

Issue pipelines spawn debate-runner agents. Debate-runners spawn generative
and adversarial agents. The generative agent writes code. You do none of this.

---

The core Ratchet workflow. Operates at four levels:

- **Epic** — the full project roadmap (from `.ratchet/plan.yaml`)
- **Milestone** — a coherent deliverable, composed of one or more issues
- **Issue** — an independently executable unit of work with its own phase pipeline and PR
- **Phase** — the current stage within an issue (`plan → test → build → review → harden`)

Parallelism exists at two levels:

- **Milestones** run in parallel when they have `depends_on` declarations forming a DAG. Milestones without dependencies are Layer 0 and run concurrently. If no milestones declare `depends_on`, they run sequentially (backward compatible).
- **Issues** within a milestone run in parallel (unless they have explicit dependencies). Each issue progresses through its own phase pipeline independently, in an isolated git worktree.

Phases within an issue are ordered and gated: phase N must complete before phase N+1 begins.

## Usage
```
/ratchet:run                    # Resume from epic — propose next focus or run against changes
/ratchet:run [pair-name]        # Run a specific pair against its scoped files
/ratchet:run [workspace]        # Target a specific workspace
/ratchet:run --milestone <id>   # Run a single milestone's pipeline (used by parallel milestone spawning)
/ratchet:run --issue <ref>      # Run a single issue's pipeline (used by parallel issue spawning)
/ratchet:run --all-files        # Run all pairs against all files in scope
/ratchet:run --no-cache         # Force re-debate even if files haven't changed
/ratchet:run --dry-run          # Preview what would run without executing anything
/ratchet:run --unsupervised              # Run the full plan end-to-end without human intervention
/ratchet:run --unsupervised --auto-pr    # Same, but auto-create PRs per issue
```

## Unsupervised Mode

When `--unsupervised` is set, the run loop executes the entire plan (all milestones, all phases) without human interaction. The principle is simple: **wherever an `AskUserQuestion` has a "(Recommended)" option, auto-select it.**

### Behavior

- **Step 1a (workspace)**: If at workspace root with no workspace specified, **halt** — unsupervised mode requires an explicit workspace target (`/ratchet:run --unsupervised monitor`). Auto-selecting a workspace is too risky.
- **Step 2 (focus)**: Auto-select "Run all ready issues sequentially" for the current milestone. When a milestone completes, auto-advance to the next. In DAG mode, auto-launch all ready milestones in parallel.
- **Step 4 (issue pipelines)**: Execute all ready issues sequentially inline. Each issue pipeline runs in an isolated worktree, spawning only debate-runner agents.
- **Step 5-dry (dry-run)**: Incompatible with `--unsupervised` — if both are set, ignore `--dry-run`.
- **Step 5c (pre-debate guards)**: If a blocking pre-debate guard fails → auto-select "Fix and re-run". The generative agent attempts to fix the issue. If the fix fails after 2 attempts, that issue **halts** (other issues continue).
- **Step 6 (static analysis)**: Auto-select "Fix these before running". Same 2-attempt retry, then halt.
- **Step 5e (debates)**: Run normally. Debates are autonomous by nature. If debate-runner cannot be spawned (tool unavailable), the issue **halts** with status `blocked` — quality gates cannot be compromised.
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
2. Debate-runner cannot be spawned (tool unavailable) — quality gate cannot be enforced
3. A blocking guard fails after 2 auto-fix attempts
4. Regression budget is exhausted and no auto-resolution is possible

**Milestone-level halts** (the entire run stops):
5. A static analysis pre-gate fails after 2 attempts (Step 6)
6. A done milestone would need re-opening
7. All issues in the milestone are halted (no progress possible)
8. All milestones are complete (success)

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
- Which milestones are **current** (status: in_progress)
- **Milestone parallelism mode**: if ANY milestone has a `depends_on` field → DAG mode (parallel milestones). Otherwise → sequential mode (backward compatible).
- In DAG mode: which milestones are **ready** (all dependencies done, status not done)
- For each relevant milestone: which **issues** exist, their `phase_status`, `depends_on` relationships, and current status
- Which issues can run in **parallel** (no unmet dependencies) vs which must wait
- Any unresolved conditions from previous CONDITIONAL_ACCEPT verdicts
- Which **phases** apply based on component workflows

If no `plan.yaml` exists, skip epic tracking and fall through to file-based detection.

**CHECKPOINT**: You now understand the project state. Do NOT act on it — do not analyze code, do not plan fixes, do not write implementations. Your next action is Step 2: present choices to the user (or auto-select in unsupervised mode). Then Step 4: launch issue pipelines. The pipelines do the work.

### Step 2: Determine Focus

There are five modes, checked in order:

#### Mode M: Single-milestone pipeline (--milestone <id>)
If `--milestone` is set, skip milestone selection. Find the milestone by ID in plan.yaml. Set it to `in_progress` and jump directly to **Step 3** to build the issue dependency graph for this single milestone. This mode is used when the orchestrator launches parallel milestones — each agent re-enters the run skill scoped to one milestone. Execute Steps 3 → 4 → 8 for this milestone, then return the result.

#### Mode S: Single-issue pipeline (--issue <ref>) [DEPRECATED]

**Note**: This mode is deprecated in favor of inline execution (Step 4b). The `--issue` flag may still be used for manual/supervised runs but is no longer used by the orchestrator in automated flows.

If `--issue` is set manually, execute the issue pipeline (Step 5) directly for the specified issue without milestone context.

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
- "Run all ready issues sequentially (Recommended)" — executes all issues with no unmet dependencies
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

### Step 3: Set Focus and Build Dependency Graphs

#### 3a. Milestone-Level DAG (parallel milestones)

Check if milestone parallelism is active: **if ANY milestone in plan.yaml has a `depends_on` field → DAG mode**.

**DAG mode** — build milestone dependency layers:
1. **Layer 0**: milestones with `depends_on: []` (or no `depends_on`) whose status is not `done`
2. **Layer 1**: milestones whose `depends_on` entries are all `done`
3. **Layer N**: milestones whose dependencies are all in earlier layers

If multiple milestones are ready (Layer 0 or newly unblocked), proceed to **Step 3c** to launch them in parallel.

**Sequential mode** (no milestone has `depends_on`) — select a single milestone:
- If Mode B selected a specific milestone, use that
- Otherwise, pick the first milestone with `status != done`
- Set it to `status: in_progress`, record `current_focus` with milestone id and timestamp
- Proceed to **Step 3b**

#### 3b. Issue-Level DAG (within a single milestone)

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

#### 3c. Launch Parallel Milestones (DAG mode only)

When multiple milestones are ready, launch them in parallel. Each milestone runs as a separate agent with its own issue DAG:

```
Launching [N] milestones in parallel:
  M[id]: [name] — [N] issues
  M[id]: [name] — [N] issues

[If later layers exist: "[N] more milestones waiting for dependencies"]
```

For each ready milestone, spawn an Agent with:
```
/ratchet:run --milestone <id> [--unsupervised] [--auto-pr] [--no-cache]
```

The spawned agent enters Mode M (Step 2), builds the issue DAG for its milestone, and runs Steps 3b → 4 → 8 independently. Each milestone agent handles its own issue parallelism internally.

**Processing milestone results** (same pattern as issue results in Step 4c):
1. Read each milestone agent's output
2. **Update plan.yaml** with milestone status (done, blocked, halted) and all issue statuses within it
3. Check if any dependent milestones are now unblocked → launch next batch
4. Report overall epic progress

**When all milestones across all layers are done** → epic complete, proceed to Step 10.

**If a milestone halts**: present the halt reason. In supervised mode, let the user decide. In unsupervised mode, continue with other milestones if possible — a halted milestone only blocks milestones that depend on it.

**Context clearing**: each milestone agent starts with fresh context (separate Agent invocation), achieving the same context isolation as sequential milestone execution.

Proceed to **Step 8** for each completed milestone, then **Step 10** for epic-level next steps.

### Step 4: Execute Issue Pipelines

**CHECKPOINT**: You are about to execute issue pipelines. Your job is to orchestrate the phase-gated execution for each issue in isolated worktrees. Do NOT write code, fix bugs, or implement features — that work belongs inside the debate-runner agents spawned from Step 5e.

This is the core execution step. The orchestrator executes pipeline logic for each issue inline (no agent spawning), using git worktrees for isolation. Debate-runner agents are spawned at Step 5e, keeping nesting depth minimal.

#### 4a. Identify Ready Issues

From the dependency graph built in Step 3b, identify **ready issues** — issues whose status is not `done` and whose `depends_on` entries are all `done` (or empty).

**For explicit pair / --all-files modes:** Skip issue-based execution. Run the specified pairs directly using the single-issue flow (Step 5) without worktree isolation.

#### 4b. Execute Issue Pipelines Inline

For each ready issue, execute the issue pipeline (Step 5) **inline** (not as a spawned agent). Use git worktrees for filesystem isolation.

**Execution strategy:**
- **Sequential execution**: Process issues one at a time to maintain simplicity and avoid resource contention
- **Worktree isolation**: Each issue runs in its own git worktree with an isolated branch
- **Inline logic**: Execute Steps 5a-5h directly in the current context (no agent spawning)
- **Debate-runner spawning**: Only spawn agents at Step 5e for debate-runners (depth: orchestrator → debate-runner)

**Worktree setup per issue:**

1. **Determine base branch:**
   - If the issue has no `depends_on` → branch from main (or current branch)
   - If the issue has `depends_on` → branch from the dependency's `branch` field in plan.yaml

2. **Create worktree:**
   ```bash
   git worktree add .ratchet/worktrees/<issue-ref> <base-branch>
   cd .ratchet/worktrees/<issue-ref>
   git checkout -b ratchet/<milestone-slug>/<issue-ref>
   ```

3. **Execute issue pipeline** (Steps 5a-5h) in the worktree context

4. **Record results** in main repo's plan.yaml (worktree plan.yaml is ephemeral)

5. **Clean up worktree** after issue completes or halts:
   ```bash
   cd <main-repo>
   git worktree remove .ratchet/worktrees/<issue-ref>
   ```

**Note on parallelism**: Issues execute sequentially to:
- Avoid agent nesting limits (debate-runners spawn directly from orchestrator, not from nested issue agents)
- Prevent resource contention on shared guards/resources
- Simplify state management and error handling

For true parallelism, use milestone-level DAG (Step 3a) where each milestone runs in a separate agent, and each milestone processes its issues sequentially.

Present a summary before execution:

```
Executing [N] issue pipelines sequentially:
  [ref]: [title] — [N] pairs, starting at [phase]
  [ref]: [title] — [N] pairs, starting at [phase]

[If Layer 1+ exists: "[N] more issues waiting for dependencies"]
```

#### 4c. Process Issue Results (after issue completes)

After each issue pipeline completes (Step 5h), update state and check for newly unblocked issues. **Do NOT fix, debug, or modify anything from the results — just record state and proceed.**

For each completed issue:

1. **Update the MAIN repo's `plan.yaml`** immediately after the issue pipeline completes. Since execution is inline, you have direct access to the results:
   - Set `status` (done, blocked, escalated, failed)
   - Set `phase_status` for all phases
   - Set `branch` (the branch name created by the pipeline)
   - Set `files` (list of modified files)
   - Set `debates` (debate IDs created)
   - Update directly based on Step 5h execution results

2. **Clean up the worktree**: Remove `.ratchet/worktrees/<issue-ref>` after recording results

3. Check if any **Layer 1+ issues** are now unblocked (their dependencies just completed)

4. If newly unblocked issues exist → execute them as the next batch (back to 4b)

5. Report progress after each issue: `"[N]/[total] issues complete in milestone [id]"`

**CRITICAL**: This plan.yaml update is not optional. The orchestrator is the authoritative writer for all issue state.

**When all issues across all layers are done** → milestone is complete, proceed to Step 8.

**If an issue pipeline halts (blocked/escalated/failed):**
- Record the halt status in plan.yaml immediately
- In supervised mode: use `AskUserQuestion` to let the user decide how to proceed
  - Options: `"Resolve now"`, `"Continue with remaining issues (Recommended)"`, `"Done for now"`
- In unsupervised mode: note the halt, continue with remaining issues if possible. A halted issue only blocks issues that depend on it, not unrelated issues. Halt the entire milestone only if all issues are blocked.

**Handling merge conflicts on existing PRs:**

When the orchestrator detects (via `gh pr view`) that an issue's PR has merge conflicts:
1. Do NOT attempt to resolve the conflict yourself (no rebase, no merge, no code editing)
2. Re-launch the issue pipeline (`/ratchet:run --issue <ref>`) in a fresh worktree based on the current main branch
3. The issue pipeline will re-run the build phase, which will naturally produce code that is compatible with the current main branch
4. The old PR branch is replaced — the pipeline creates a new branch and force-pushes (or creates a new PR)

This is not a special case — it's the normal pipeline flow. Merge conflicts mean the issue's code is stale relative to main. The correct response is to re-run the pipeline from the appropriate phase, not to manually patch the conflict.

**IMPORTANT**: Do NOT run debates yourself. Do NOT spawn generative or adversarial agents directly. Issue pipelines handle all debate orchestration. This is a structural constraint.

---

### Step 5: Issue Pipeline (executed inline per-issue in isolated worktrees)

This is the phase-gated loop for a single issue. The orchestrator executes this step **inline** for each issue, using git worktrees for filesystem isolation.

**Execution context:**
- Called from Step 4b for each ready issue
- Runs in the issue's dedicated git worktree (`.ratchet/worktrees/<issue-ref>`)
- Spawns debate-runner agents at Step 5e (only agent spawning in the pipeline)
- Updates the main repo's plan.yaml upon completion (Step 5h)

The issue pipeline progresses through phases sequentially (plan → test → build → review → harden, depending on workflow), then returns control to Step 4c for state persistence.

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
2. **Lock + Run** — if a resource is `singleton: true`, wrap the guard command with `flock` so the lock auto-releases when the command finishes (or crashes)
3. **Teardown** — after all pipelines complete, run `stop` commands for resources that have them

**Provisioning**: run the resource's `start` command. Start commands must be idempotent (e.g., `docker compose up -d postgres` is safe to run multiple times). Track started resources in `.ratchet/locks/resources.json`:
```json
{"postgres": {"started": true, "pid": "$$", "at": "<ISO>"}}
```

**Singleton locking**: uses `flock` — kernel-level file locking in `.ratchet/locks/` (shared across worktrees since they share the same host filesystem). The lock is tied to the file descriptor, so the kernel automatically releases it when the process exits — even on crash or SIGKILL. No stale locks, no timeouts, no cleanup needed.

```bash
# Create lock file (once, idempotent)
mkdir -p .ratchet/locks
touch .ratchet/locks/<resource-name>.lock

# Run the guard command under flock — blocks until lock acquired, auto-releases on exit
flock .ratchet/locks/<resource-name>.lock bash -c '<guard-command>'
```

When multiple singleton resources are required, acquire all locks in alphabetical order to prevent deadlocks:
```bash
flock .ratchet/locks/db.lock flock .ratchet/locks/playwright.lock bash -c '<guard-command>'
```

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
    requires: [postgres, redis]    # postgres is flock'd, redis just starts
```

**Teardown** (Step 9 — after all pipelines complete): for each resource with a `stop` command, run it. Remove `.ratchet/locks/` directory.

#### 5c. Pre-Debate Guards

Run guards where `timing: "pre-debate"` for the current phase. Guards without a `timing` field are treated as `post-debate` (backward compatible).

**If the guard has `requires`**: provision required resources (run `start` if not already running). For singleton resources, wrap the guard command with `flock` — the lock auto-releases when the command exits.

For each pre-debate guard assigned to the current phase:
```bash
# Without singleton resources:
bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>

# With singleton resources (e.g., requires: [postgres]):
flock .ratchet/locks/postgres.lock bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>
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

**If Debate-Runner Cannot Be Spawned**

If the Task/Agent tool is unavailable or debate-runner spawning fails (e.g., nesting limits, execution environment constraints):

1. **In supervised mode**: Escalate to human via `AskUserQuestion`:
   - Question: "Debate-runner unavailable for pair [pair-name] in phase [phase]. How should we proceed?"
   - Options: `"Wait and retry"`, `"Skip this phase"`, `"Escalate issue for manual resolution"`

2. **In unsupervised mode**: Halt the issue pipeline with status `blocked`:
   - Reason: "Debate-runner unavailable - quality gate cannot be enforced"
   - Set issue status to `blocked` in plan.yaml
   - Other issues in the milestone continue (this issue only blocks its dependents)
   - Log: "Issue [ref] blocked at phase [phase]: debate-runner tool unavailable. Manual intervention required."

**Retry Logic** (if "Wait and retry" selected):
- Retry spawning the debate-runner up to 3 times with exponential backoff (5s, 10s, 20s)
- If all retries fail, escalate to human or halt (depending on mode)

**No Fallback Validation**: The debate-runner is the ONLY acceptable path for quality enforcement. Guards alone are insufficient substitutes for adversarial review. Auto-approval without debate violates the quality contract.

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

**IMPORTANT**: Do NOT run debates yourself. The debate-runner is the ONLY path. If unavailable, halt and escalate rather than compromise quality.

**IMPORTANT**: After processing debate results, proceed through ALL of Step 5f — including commit/PR. Do NOT skip to the next phase without packaging the work.

#### 5f. Phase Gate — Guards and Advance

**Check results:**
- All consensus → proceed to guards
- Any escalated → update plan.yaml, output early-exit summary (Step 5h), return
- Any regress → proceed to Step 5g

**Run post-debate guards:**

**If the guard has `requires`**: provision required resources. For singleton resources, wrap with `flock` (same as pre-debate guards).

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

**Resource teardown**: tear down shared resources when no more pipelines need them:
- **Sequential mode**: after all issue pipelines for the milestone complete
- **DAG mode**: after ALL milestones across all layers complete (the top-level orchestrator handles teardown, not individual milestone agents)

For teardown:
1. For each resource in `workflow.yaml` that has a `stop` command, run it
2. Clean up `.ratchet/locks/` directory (remove `resources.json` and any stale lock directories)

Resources are torn down regardless of whether milestones succeeded, halted, or had errors — always clean up.

### Step 10: Propose Next Focus

**If `--milestone` (Mode M)**: This is a milestone sub-agent. Output a structured milestone completion summary (analogous to issue summaries in Step 5h) and return. The top-level orchestrator handles next steps.

```
Milestone [id] complete:
  status: done
  issues: [N] complete, [N] blocked/escalated
  prs: [N] created
  debates: [N] total
```

Or if halted:
```
Milestone [id] [blocked|halted]:
  status: [blocked|halted]
  issues: [N] complete, [N] blocked/escalated, [N] pending
  halted_because: [reason]
```

---

**If `--unsupervised`** (sequential mode): Skip `AskUserQuestion`. If no halt condition was triggered and work remains (more milestones), persist all state to `plan.yaml` and spawn a new agent via the Agent tool with task `/ratchet:run --unsupervised`. If all milestones are complete, halt with the completion summary. If a halt condition was triggered during this iteration, present the halt summary and stop.

**If `--unsupervised`** (DAG mode): The top-level orchestrator processes milestone results from Step 3c. If newly unblocked milestones exist, launch them. If all milestones are done, present the epic completion summary and halt.

**Otherwise**, use `AskUserQuestion` to let the user choose what to do next.

**If the milestone has blocked/escalated issues:**
- Summary: `"Milestone [name]: [N]/[total] issues complete. [N] blocked/escalated."`
- Options:
  - "Resolve escalated debates (/ratchet:verdict)" — if escalated
  - "View issue status" — show per-issue phase progress
  - "Re-run to continue (/ratchet:run) (Recommended)" — picks up unblocked issues
  - "Done for now"

**If milestone complete, more milestones remain:**

**CONTEXT CLEARING**: Milestone boundaries are the primary context clearing point. In sequential mode, persisted state (plan.yaml, debate transcripts, pair definitions, scores) is the source of truth — not context memory. A fresh context forces re-reading actual files, preventing drift from auto-compaction summaries. In DAG mode, context clearing happens naturally — each milestone agent is a separate Agent invocation with fresh context.

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
