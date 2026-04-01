---
name: ratchet:run
description: Run agent pairs through phase-gated debates — guided by epic roadmap and current focus
---

## Boot Context (pre-loaded at skill invocation)

The following state is injected at startup so the skill boots with full situational awareness. All blocks fail gracefully — missing files produce a human-readable fallback message.

**Plan:**
```
$(cat .ratchet/plan.yaml 2>/dev/null || echo "No plan found")
```

**Workflow config:**
```
$(cat .ratchet/workflow.yaml 2>/dev/null || echo "No workflow config")
```

**Recent debates (20 most recent meta.json files):**
```
$(for f in $(ls -t .ratchet/debates/*/meta.json 2>/dev/null | head -20); do [ -f "$f" ] && cat "$f" && echo; done 2>/dev/null)
```

**Git state:**
```
Branch: $(git branch --show-current 2>/dev/null)
Recent commits:
$(git log --oneline -5 2>/dev/null)
```

---

# /ratchet:run — Execute Debate

## CRITICAL — You Are an Orchestrator, Not a Solver

You do NOT write code. You do NOT fix bugs. You do NOT implement features.
You do NOT resolve merge conflicts. You do NOT rebase branches.
You are a workflow orchestrator. Your tools are:

- **Read, Glob, Grep** — to read state files (plan.yaml, workflow.yaml, etc.)
- **Agent** — to spawn issue pipelines and debate-runners
- **AskUserQuestion** — to present choices to the user
- **TodoWrite** — to update the user-visible progress checklist (see "Progress Tracking via TodoWrite" section)
- **Bash** — for:
  - Running guard scripts (`bash .claude/ratchet-scripts/...`)
  - Read-only git commands (`git status`, `git log`, `git branch`, `git diff`)
  - Read-only GitHub CLI (`gh pr list`, `gh pr view`, `gh issue list`)
  - **Plan management** via `yq` — modifying `.ratchet/plan.yaml` (see Plan Management Authority below)

### Source Code Boundary — NEVER Cross This Line

**NEVER use Write or Edit on source code, test files, or application config.**
Source code modifications happen ONLY inside debate-runner agents (which delegate
to generative agents). If you feel the urge to edit a source file, STOP — you
are breaking out of the framework.

**TOOL GATE — check EVERY Bash command before running it:**
- `git rebase` → STOP. This is code work. Route to an issue pipeline.
- `git merge` → STOP. This is code work. Route to an issue pipeline.
- `git cherry-pick` → STOP. This is code work. Route to an issue pipeline.
- Resolving merge conflicts → STOP. This is code work.
- `Write` or `Edit` on source/test/config files → STOP. Route to an issue pipeline.
- Reading a source code file to "understand" a conflict → STOP. You're
  about to start solving. Route to an issue pipeline.

### Plan Management Authority — This IS Your Job

You are the **authoritative owner** of `.ratchet/plan.yaml`. Managing the epic
roadmap — milestones, issues, discoveries, statuses, focus — is core orchestrator
work, not "breaking out of the framework."

**You CAN and SHOULD modify `.ratchet/plan.yaml` for:**
- Creating new epics and milestones (when the user requests it or when the current epic is complete)
- Adding issues to milestones
- Updating milestone/issue statuses (`pending` → `in_progress` → `done`)
- Setting/clearing `current_focus`
- Promoting/dismissing discoveries
- Recording `progress_ref`, `branch`, `pr`, `debates`, `files` on issues
- Recording `github_issue` on milestones (the GitHub issue number this milestone tracks as a parent, e.g., `github_issue: 165`). When the user provides a GitHub issue reference for the milestone, store it as an explicit field — do not bury it in the description string. Child issues are created under this parent at pipeline launch time (Step 4b).
- Incrementing `regressions` counters
- Any structural change to the epic roadmap that the user requests

**You CANNOT modify:**
- Source code, test files, or application configuration (route to debate pipeline)
- `.ratchet/workflow.yaml` (route to `/ratchet:tighten` or `/ratchet:init`)
- `.ratchet/pairs/` agent definitions (route to `/ratchet:tighten` or `/ratchet:pair`)
- `.ratchet/debates/` artifacts (that's the debate-runner's domain)

**Method:** Use `yq eval -i` via Bash for plan.yaml modifications. Use Write
only if yq is unavailable. Never use Write or Edit on non-plan files.

If a PR has merge conflicts, that is work for the issue pipeline to resolve
through a debate. The orchestrator's job is to detect the conflict (via
`gh pr view`) and re-launch the issue pipeline to handle it — not to
resolve it directly.

### GitHub Plan Tracking Issue

> **For the canonical body format, HTML comment metadata rules, and sync helper pattern, read `skills/run/plan-tracking-format.md`.**
>
> This section covers: the GitHub issue body format with HTML comment metadata,
> required fields per milestone/issue block, the `ratchet-plan-tracking` sentinel,
> and the existence-guarded sync helper call pattern.

**AGENT GATE — check EVERY Agent tool invocation before spawning it:**

The orchestrator may ONLY spawn agents in these four categories:

1. **Issue pipeline agents** (Step 4b) — agents that run a single issue's phase
   pipeline in an isolated worktree. They spawn debate-runners at Step 5e.
   The debate-runner is the ONLY valid path for code changes in the standard pipeline.
2. **Quick-fix generative agents** (Mode Q, Step 2) — a single generative agent
   spawned for `--quick` mode. Receives the description as prompt with build-phase
   constraints. Blocking guards serve as the quality gate (no adversarial review).
   This is the ONLY exception to the debate-runner requirement — justified by
   Mode Q's narrow scope and mandatory guard gating.
3. **Analyst agents** (Step 8c) — read-only assessment agents
   (`disallowedTools: Write, Edit`) that analyze data and produce recommendations.
   They NEVER modify files.
4. **Continuation agents** (Step 10, unsupervised mode) — orchestrator agents that
   inherit the same source-code boundary and plan management authority, and
   continue the `/ratchet:run` loop.

**No milestone sub-agents.** The orchestrator runs milestones directly (Step 3c)
to keep the agent chain at 3 levels: orchestrator → debate-runner → gen/adv.
Spawning milestone-level agents adds a 4th level where chain collapse occurs.

**NEVER spawn an agent with implementation instructions** (except Mode Q). If your
Agent prompt contains phrases like "implement X", "fix Y", "add Z", "write code for",
"create the file", or "modify the source" — STOP. You are bypassing the debate
framework. All implementation work MUST flow through: orchestrator -> debate-runner ->
generative agent — unless `--quick` mode is active, in which case Mode Q's
single-agent path applies (Step 2, Mode Q).

**Violation examples (all FORBIDDEN):**
- `Agent("Implement the AGENT GATE feature in skills/run/SKILL.md")` — direct implementation
- `Agent("Fix the failing test in src/auth.ts")` — direct bug fix
- `Agent("Add error handling to the parser module")` — direct code change
- `Agent("Refactor the database layer")` — direct refactoring

**Correct pattern:**
- `Agent("Run debate for pair [name] in phase [phase]. ...")` — spawns a debate-runner
- `Agent("/ratchet:run --issue issue-3 --milestone 2")` with `isolation: "worktree"` — spawns an issue pipeline
- `Agent("Quick-fix mode — single generative pass. Task: ...")` — spawns a Mode Q generative agent (only when `--quick` is active)
- `Agent("Analyze milestone results...")` with `disallowedTools: Write, Edit` — spawns an analyst

Your job is to:

1. **Manage the epic roadmap** — create/modify epics, milestones, and issues in plan.yaml when the user requests it or when the workflow requires it (e.g., epic complete, user wants new work)
2. Read state (plan.yaml, workflow.yaml)
3. Build dependency graphs — milestones (if DAG mode) and issues within each milestone
4. Launch **milestone pipelines** in parallel (DAG mode) or sequentially
5. Within each milestone, launch **issue pipelines** in parallel (each in an isolated worktree)
6. Process their results — update plan.yaml with statuses, advance milestones, complete epics

Issue pipelines spawn debate-runner agents. Debate-runners spawn generative
and adversarial agents. The generative agent writes code. You do none of that —
but you ARE the authority on plan structure and milestone lifecycle.

---

## Foundational Principle — Guilty Until Proven Innocent

**New changes are GUILTY until proven innocent.** Test failures on a PR branch are CAUSED by the PR unless definitively proven otherwise. The burden of proof is on demonstrating the failure exists on master, not assuming it is unrelated.

This principle applies throughout the issue pipeline:
- **Guard failures**: A guard failure during an issue pipeline is the issue's fault. Do not dismiss it as "flaky" or "pre-existing" without evidence (e.g., `git stash && run-test` on clean master).
- **CI failures**: When a PR's CI fails, the PR is guilty. The issue pipeline must fix the failure or provide definitive proof that master has the same failure.
- **Debate context**: Pass this principle to all spawned agents (debate-runners, generative, adversarial). Every agent must internalize that failures are their responsibility to fix, not dismiss.
- **Regression analysis**: When processing REGRESS verdicts, the burden is on showing the regression was pre-existing, not on assuming it is.

This principle is passed as context to all spawned debate-runner agents (see Step 5d/5e).

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
/ratchet:run --issue <ref>      # Run a single issue's pipeline (ref is GitHub issue # if promoted)
/ratchet:run --all-files        # Run all pairs against all files in scope
/ratchet:run --no-cache         # Force re-debate even if files haven't changed
/ratchet:run --dry-run          # Preview what would run without executing anything
/ratchet:run --unsupervised              # Run the full plan end-to-end without human intervention
/ratchet:run --unsupervised --auto-pr    # Same, but auto-create PRs per issue
/ratchet:run --go                        # Shorthand for --unsupervised --auto-pr
/ratchet:run --quick "<description>"     # Quick-fix: skip plan, auto-detect scope, single generative pass
```

## Unsupervised Mode

For unsupervised mode behavior, read `skills/run/unsupervised.md`.

Covers: auto-selection rules for every `AskUserQuestion` step, self-continuation via the Agent tool at milestone boundaries, halt conditions (issue-level and milestone-level), and combining `--unsupervised` with `--auto-pr`, `--no-cache`, `--all-files`, `--dry-run`, and `--quick`. Note: `--go` is shorthand for `--unsupervised --auto-pr`.

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

### Progress Tracking via TodoWrite

Use `TodoWrite` to give the user a real-time progress checklist during pipeline execution. TodoWrite **replaces** the full list on every call (it is not incremental), so always include all items with their current statuses.

**ID convention**: Use hierarchical IDs — `m<N>` for milestones, `m<N>-<ref>` for issues, `m<N>-<ref>-<phase>` for phases.

**Status values**: `"pending"`, `"in_progress"`, `"completed"`

**Schema note**: The examples below assume a flat list with `id`, `content`, and `status` fields. Verify the actual TodoWrite tool schema in your environment — if it supports nested `children`, you may use hierarchical nesting instead. Adapt the examples accordingly.

**Principle**: Keep items concise — users want a glance, not a wall of text. Include verdict info in completed phases (e.g., `"(ACCEPT R1)"`, `"(CONDITIONAL_ACCEPT R2)"`).

TodoWrite is called at 7 pipeline boundaries (Steps 3b/3c, 4b, 5a, 5e, 5f, 4c, 8). Each callsite below is marked with **[TodoWrite]**. The orchestrator maintains a running `todo_items` list in memory, mutates it at each boundary, and passes the full list to `TodoWrite`.

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

**`publish_debates` note**: Debate round publishing is handled by the `publish-debate-hook.sh` PostToolUse hook, not by the orchestrator or debate-runner. The hook reads `publish_debates` and `adapter` directly from `workflow.yaml`. No orchestrator-side validation or passing of publish config is needed.

Build a picture of:
- Which milestones are **completed** (status: done)
- Which milestones are **current** (status: in_progress)
- **Milestone parallelism mode**: if ANY milestone has a `depends_on` field → DAG mode (parallel milestones). Otherwise → sequential mode (backward compatible).
- In DAG mode: which milestones are **ready** (all dependencies done, status not done)
- For each relevant milestone: which **issues** exist, their `phase_status`, `depends_on` relationships, and current status
- Which issues can run in **parallel** (no unmet dependencies) vs which must wait
- Any unresolved conditions from previous CONDITIONAL_ACCEPT verdicts
- Which **phases** apply based on component workflows
- Each component's **strategy** (`debate` or `solo`, default: `debate`) — determines whether issue pipelines spawn debate-runners or a single generative agent (see Step 5e). Read from `components[].strategy` in `workflow.yaml`.
- Each component's **`promote_on_guard_failure`** flag (default: `false`) — relevant only for `strategy: "solo"` components, controls whether guard failures escalate to debate

If no `plan.yaml` exists, check whether the github-issues adapter is configured:
```bash
adapter=$(yq eval '.progress.adapter' .ratchet/workflow.yaml 2>/dev/null)
if [ "$adapter" = "github-issues" ]; then
  # Attempt recovery from GitHub tracking issue
  if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
    echo "plan.yaml missing — attempting recovery from GitHub tracking issue..."
    bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh --recover
    if [ -f .ratchet/plan.yaml ]; then
      echo "Recovery successful. Review recovered plan.yaml before continuing."
    else
      echo "Recovery attempted but plan.yaml could not be restored. Check the tracking issue manually." >&2
      echo "What was NOT recoverable: file-level change lists (files: []), debate IDs (debates: [])," >&2
      echo "  branch names, and PR URLs — these are runtime artifacts not stored in the tracking issue." >&2
    fi
  else
    echo "plan.yaml missing and sync-plan.sh not installed. Skipping epic tracking." >&2
  fi
fi
```
If recovery fails or adapter is not github-issues, skip epic tracking and fall through to file-based detection.

If `plan.yaml` exists but fails to parse (malformed YAML or missing `epic` key):
```bash
yq eval '.epic' .ratchet/plan.yaml > /dev/null 2>&1 \
  || { echo "Error: .ratchet/plan.yaml is malformed or missing required 'epic' field. Fix it before running." >&2; exit 1; }
```

**Start PR monitor**: If any issues in plan.yaml have non-null `pr` fields, start the PR watch loop to detect merge conflicts and CI failures during the run:
```
/loop 10m check Ratchet PRs for conflicts and CI failures
```
This runs `/ratchet:watch` logic inline — polling PRs and creating discoveries automatically. The loop is stopped in Step 9 when the run completes. Skip this if no PRs exist yet (first run of a new epic).

#### 1c. Orphan Detection

Run `bash .claude/ratchet-scripts/check-orphans.sh --ratchet-dir "$RATCHET_DIR"` to identify stale state (abandoned worktrees, unresolved debates, incomplete executions, stale in-progress issues). Advisory only — never blocks the pipeline. If the script is missing, skip with a warning.

Each finding has: `type`, `ref`, `age`, `suggested_action`. For each finding, three actions are available:
- **Resume** — set the item as current focus or log for pipeline continuation
- **Abandon** — reset to clean state (reset status to pending, remove worktree/debate/execution artifacts)
- **Ignore** — skip, take no action

**Supervised mode**: present each finding via `AskUserQuestion` with the three options above.

**Unsupervised mode**: auto-select based on `age` — Abandon if >24h or unknown, Resume if <4h, Ignore otherwise.

**CHECKPOINT**: You now understand the project state. Do NOT act on it — do not analyze code, do not plan fixes, do not write implementations. Your next action is Step 2: present choices to the user (or auto-select in unsupervised mode). Then Step 4: launch issue pipelines. The pipelines do the work.

### Step 2: Determine Focus

There are six modes, checked in order:

#### Mode Q: Quick-fix (--quick "<description>")

If `--quick` is set, skip `plan.yaml` entirely. This mode is a fast path for small, well-understood fixes that don't need epic/milestone/issue management or adversarial review.

**Checked first** — before all other modes. If `--quick` is present, no other mode flags are evaluated.

**Flag interactions:**
- `--quick --dry-run`: Print the detected component, scope, and guards that would run, then stop. No agent spawned, no changes made.
- `--quick --auto-pr`: Create a branch and PR after the commit (see step 5).
- `--quick --unsupervised`: If component auto-detection fails, halt with error. If a guard fails, halt with `failed` (no retry).
- `--quick --no-cache`: No effect — Mode Q has no file-hash cache.

**1. Parse description:**

The freeform `<description>` argument is the task. It should describe what to do and which files are involved:
```
/ratchet:run --quick "Fix off-by-one in src/parser.ts validateToken loop"
/ratchet:run --quick "Add missing error handling to scripts/deploy.sh"
```

**2. Auto-detect component from file paths:**

Extract file paths from the description (tokens matching known file extensions or path separators):
```bash
# Extract candidate paths from the description
PATHS=$(echo "$DESCRIPTION" | grep -oE '[a-zA-Z0-9_./-]+\.[a-zA-Z]{1,10}' | sort -u)

# Match each path against component scope globs in workflow.yaml
for path in $PATHS; do
  for component in $(yq eval '.components[].name' .ratchet/workflow.yaml); do
    scope=$(yq eval ".components[] | select(.name == \"$component\") | .scope" .ratchet/workflow.yaml)
    # Check if path matches the component's scope glob
    # Use bash globbing or a dedicated match utility
  done
done
```

If no file paths are detected or no component matches, use `AskUserQuestion`:
- Question: "Could not auto-detect component from the description. Which component?"
- Options: one per component in `workflow.yaml`, plus `"Cancel"`

In unsupervised mode with no component match: halt with error — quick-fix requires a detectable scope.

**3. Spawn one generative agent:**

Spawn a single generative agent (using the resolved `generative` model) with the description as the prompt. The agent receives build-phase constraints (it can read, write, and edit files within the detected component's scope):

```
Quick-fix mode — single generative pass.

Task: <description>
Component: <detected-component>
Scope: <component scope glob>

Constraints:
  - You are in build-phase mode: read, write, edit files within scope.
  - No adversarial review — blocking guards are the quality gate.
  - Keep changes minimal and focused on the described task.
  - Do NOT modify files outside the component scope.

PRINCIPLE — Guilty Until Proven Innocent:
  If any test or guard fails after your changes, YOUR changes caused it
  unless you can prove otherwise on a clean checkout.
```

**Tool boundaries for the quick-fix generative agent:**
- tools: Read, Grep, Glob, Bash, Write, Edit
- Same as debate-mode generative agent — the only difference is no adversarial review and no debate structure

**4. Run blocking guards:**

After the generative agent returns, run all blocking guards for the detected component's phase (use `review` phase guards as default):

```bash
test -f .claude/ratchet-scripts/run-guards.sh \
  || { echo "Error: run-guards.sh not found. Run install.sh to restore Ratchet scripts." >&2; exit 1; }

bash .claude/ratchet-scripts/run-guards.sh quick-fix review <guard-name> "<guard-command>" true
```

- If any blocking guard fails:
  - **Guilty until proven innocent**: the quick-fix caused it. Verify on clean master before dismissing.
  - In supervised mode: use `AskUserQuestion`: "Guard '[name]' failed: [summary]."
    - Options: `"Fix and re-run"`, `"Abort quick-fix"`, `"Override guard"`
  - In unsupervised mode: halt with status `failed` — quick-fix does not retry automatically.
- Advisory guards: log and continue.

**5. Commit and optionally create PR:**

If all blocking guards pass, commit with a message derived from the description.

If `--auto-pr` is also set, create a branch first:
```bash
# Create branch before committing (only with --auto-pr)
BRANCH="ratchet/quick-fix/$(echo "$DESCRIPTION" | tr ' ' '-' | tr '[:upper:]' '[:lower:]' | cut -c1-50)"
git checkout -b "$BRANCH"
```

Then commit (on the new branch if `--auto-pr`, or the current branch otherwise):
```bash
git add -A
git commit -m "<description>"
```

If `--auto-pr` is set, push and create the PR:
```bash
git push -u origin "$BRANCH"
gh pr create --title "$DESCRIPTION" --body "Quick-fix via \`/ratchet:run --quick\`"
```

**6. Write execution log:**

```bash
EXEC_ID="quick-fix-$(date +%Y%m%dT%H%M%S)"
mkdir -p .ratchet/executions
cat > ".ratchet/executions/${EXEC_ID}.yaml" <<EOF
id: "${EXEC_ID}"
mode: quick-fix
component: <detected-component>
issue: null
started: "<timestamp>"
resolved: "<timestamp>"
guard_results: [<guard results>]
description: "<description>"
files_modified: [<files>]
EOF
```

**7. Skip epic/milestone/issue management:**

Mode Q does not read or write `plan.yaml`. No milestone, issue, or phase tracking. No debate artifacts. The commit is local unless `--auto-pr` creates a branch and PR (see step 5).

> **Note on Step 1**: Mode Q still requires Step 1a (workspace resolution) to locate `workflow.yaml` for component auto-detection. However, Step 1b's `plan.yaml` reading is skipped entirely — Mode Q has no concept of milestones, issues, or phases.

**8. Output summary:**

```
Quick-fix complete:
  mode: quick-fix
  description: <description>
  component: <detected-component>
  files_modified: [<files>]
  guards: [<pass/fail summary>]
  execution_log: .ratchet/executions/<EXEC_ID>.yaml
```

Then stop — do not continue to any other step. Mode Q is a terminal path.

#### Mode M: Single-milestone pipeline (--milestone <id>)
If `--milestone` is set, skip milestone selection. Find the milestone by ID in plan.yaml. Set it to `in_progress` and jump directly to **Step 3b** to build the issue dependency graph for this single milestone. Execute Steps 3b → 4 → 8 for this milestone, then proceed to Step 10. This mode is used for focused runs on a single milestone (user-invoked or continuation agents).

#### Mode S: Single-issue pipeline (--issue <ref>)

If `--issue` is set, execute the issue pipeline (Step 5) directly for the specified issue. Used both for manual/supervised runs and as the entry point for parallel issue agents spawned by Step 4b.

#### Mode A: Explicit pair or --all-files
If the user specified a `[pair-name]` or `--all-files`, use that directly. Skip epic negotiation.

#### Mode B: Epic-guided (plan.yaml exists)

**If ALL milestones are done** (every milestone has `status: done`):

The epic is complete. Present completion summary and next steps:

Question text:
```
Epic "[name]" is complete! All [N] milestones finished.

What would you like to do next?
```

Options:
- "Create a new epic" — gather details via AskUserQuestion (freeform: "What's the next body of work?"), then create the new epic structure in plan.yaml. For complex scoping, spawn the analyst agent to help break it into milestones. For straightforward requests, create directly from the user's description.
- "Add a milestone to the current epic" — gather milestone details via AskUserQuestion, append to plan.yaml
- "Tighten agents from debate lessons (/ratchet:tighten)"
- "View quality metrics (/ratchet:score)"
- "Done for now"

When creating a new epic: replace the existing `epic` block in plan.yaml (archive the old one to `.ratchet/archive/epic-<name>-<timestamp>.yaml` first if it has content). **Archive debates**: move all debate artifacts from the completed epic into the archive alongside the plan:
```bash
EPIC_SLUG=$(echo "$EPIC_NAME" | tr ' ' '-' | tr '[:upper:]' '[:lower:]')
ARCHIVE_DIR=".ratchet/archive/epic-${EPIC_SLUG}-$(date +%Y%m%dT%H%M%SZ)"
mkdir -p "$ARCHIVE_DIR"
cp .ratchet/plan.yaml "$ARCHIVE_DIR/plan.yaml"
if [ -d .ratchet/debates ] && [ "$(ls -A .ratchet/debates 2>/dev/null)" ]; then
  mv .ratchet/debates/* "$ARCHIVE_DIR/debates/" 2>/dev/null || true
fi
```
This is safe because `/ratchet:score` persists metrics as a moving average in `.ratchet/scores.yaml` (Step 2b of the score skill) — archiving debates does not lose score history.

Set `current_focus: null` and `discoveries: []` (or carry over pending discoveries). After writing the new epic to plan.yaml, sync the tracking issue:
```bash
if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
  bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
    || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
fi
```

**Otherwise** (milestones remain):

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
- "Run all ready issues (Recommended)" — executes all issues with no unmet dependencies (parallel by layer)
- "Run specific issue: [ref]" — one option per ready issue
- "Address unresolved conditions from last run" — only if conditions exist
- "Process sidequests ([N] pending: [titles...])" — only if `epic.discoveries` has items with `status == "pending"`
- "Add a new milestone" — gather details via AskUserQuestion, append to plan.yaml, then offer to run it
- "[Next milestone name]" — skip ahead
- "Review all existing code"
- (Include an "Other" option so the user can type a custom focus)

**Sidequest processing**: When "Process sidequests" is selected, iterate over `epic.discoveries` with `status == "pending"`. For each discovery, use `AskUserQuestion`:

- Question: "Discovery: [title] ([category], [severity])\n[description]"
- Options:
  - `"Process now"` — handle via existing pipeline (tighten, re-launch, etc.)
  - `"Promote to issue"` — convert this discovery into a full plan.yaml issue
  - `"Dismiss"` — mark as non-actionable
  - `"Skip for now"` — leave as pending, move to next discovery

**Action: Process now** (existing behavior):
- `retro_type: "ci-failure"` → extract PR number from `source` field (format: `pr-ci-failure-<N>`) and launch `/ratchet:tighten pr <N>` for the affected issue
- `retro_type: "skipped-finding"` → present to user for decision (apply now or defer)
- No `retro_type` with `issue_ref` set (merge conflict) → use `issue_ref` field directly to re-launch the issue pipeline in its current phase
- No `retro_type` with `issue_ref: null` (manual discovery with no issue context) → cannot process directly, inform user: "This discovery has no linked issue. Promote it to an issue first, or dismiss it." Then re-present the action selector without the "Process now" option.
- Mark each processed discovery `status: "done"` in `plan.yaml`:
  ```bash
  yq eval -i "(.epic.discoveries[] | select(.ref == \"$discovery_ref\")).status = \"done\"" .ratchet/plan.yaml
  ```
- Sync plan tracking issue after discovery status change:
  ```bash
  if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
    bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
      || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
  fi
  ```

**Action: Promote to issue** — converts a discovery into a full plan.yaml issue:
1. Determine target milestone:
   - If `context.milestone` is set, use that milestone
   - Otherwise, use `AskUserQuestion` to select from active milestones
2. Generate issue ref: read existing issues in the target milestone, find the highest issue number, increment by 1. Format: `issue-<milestone-number>-<next-issue-number>`
3. Determine pairs:
   - If discovery `pairs` array is non-empty, use those
   - Otherwise, use `AskUserQuestion` to select from available pairs in workflow.yaml
4. Create the issue entry in plan.yaml:
   ```bash
   new_ref="issue-<M>-<N>"
   yq eval -i "(.epic.milestones[] | select(.id == \"$milestone_id\")).issues += [{
     \"ref\": \"$new_ref\",
     \"title\": \"$discovery_title\",
     \"description\": \"$discovery_description\",
     \"pairs\": [\"$pair_name\"],
     \"depends_on\": [],
     \"phase_status\": {\"plan\": \"pending\", \"test\": \"pending\", \"build\": \"pending\", \"review\": \"pending\", \"harden\": \"pending\"},
     \"files\": [],
     \"debates\": [],
     \"branch\": null,
     \"pr\": null,
     \"status\": \"pending\"
   }])" .ratchet/plan.yaml
   ```
5. Update the discovery status and link:
   ```bash
   yq eval -i "(.epic.discoveries[] | select(.ref == \"$discovery_ref\")).status = \"promoted\"" .ratchet/plan.yaml
   yq eval -i "(.epic.discoveries[] | select(.ref == \"$discovery_ref\")).issue_ref = \"$new_ref\"" .ratchet/plan.yaml
   ```
6. Sync plan tracking issue after adding the new issue:
   ```bash
   if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
     bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
       || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
   fi
   ```
7. Confirm to user: "Discovery promoted to issue [new_ref] in milestone [milestone_id]. Run /ratchet:run to start working on it."

**Action: Dismiss** — marks a discovery as non-actionable:
1. Use `AskUserQuestion` (freeform): "Reason for dismissal (optional)"
2. Update plan.yaml:
   ```bash
   yq eval -i "(.epic.discoveries[] | select(.ref == \"$discovery_ref\")).status = \"dismissed\"" .ratchet/plan.yaml
   ```
3. Confirm: "Discovery [ref] dismissed."

**Action: Skip for now** — no changes, move to next discovery.

#### Mode C: Changed files (no plan.yaml, git repo exists)
```bash
git diff --name-only HEAD 2>/dev/null || git diff --name-only
git diff --name-only --cached
```
Match changed files to pairs by `scope` globs. For each changed file, match against ALL component scopes — not just the first match. Collect pairs from all matching components for the current phase. If a change spans multiple components, present: "This change spans [components]. Running pairs from all matching components."

#### Mode D: Greenfield (no plan.yaml, no code)
Use `AskUserQuestion` to ask what to build first.

### Step 3: Set Focus and Build Dependency Graphs

#### 3a. Milestone-Level DAG

**DAG detection**: if ANY milestone has a `depends_on` field → DAG mode; otherwise → sequential mode.

**DAG mode** — topological sort into layers: Layer 0 = no dependencies (status != done), Layer N = all deps in earlier layers. Proceed to Step 3c.

**Sequential mode** — pick the first milestone with `status != done` (or the one Mode B selected). Set `status: in_progress`, record `current_focus`. Proceed to Step 3b.

#### 3b. Issue-Level DAG (within a single milestone)

Topological sort of milestone's issues into layers (same algorithm as 3a). Issues within the same layer run in parallel.

**[TodoWrite — Initial Plan]**: Write the full plan showing all milestones and issues with current statuses. Use IDs: `m<N>` for milestones, `m<N>-<ref>` for issues.

**Progress tracking**: If adapter configured and milestone has no `progress_ref`, create one via `create-item.sh`. Store in plan.yaml. Adapter failures never block.

**MILESTONE RE-OPENING GUARD**: If milestone has `status: done`, require explicit user confirmation via `AskUserQuestion` before re-opening.

#### 3c. Execute Milestones (DAG mode)

**No milestone sub-agents.** The orchestrator executes milestones inline to keep the agent chain at 3 levels (orchestrator → debate-runner → gen/adv). Milestone sub-agents would add a 4th level causing chain collapse.

Process milestone layers sequentially. Within each layer, milestones are processed one at a time; issue parallelism within a milestone is preserved (Step 4b).

For each milestone: set `in_progress` → build issue DAG (3b) → execute issues (4) → process results (4c) → milestone completion (8) → check if next layer is unblocked.

**All layers done** → epic complete, Step 10. **Milestone halts** → in unsupervised mode, continue with remaining milestones; halted milestone only blocks its dependents.

**Context clearing**: At each milestone boundary, re-read plan.yaml and workflow.yaml from disk. In unsupervised mode, spawn a continuation agent (Step 10) for fresh context.

### Step 4: Execute Issue Pipelines

**CHECKPOINT**: You are about to execute issue pipelines. Your job is to orchestrate the phase-gated execution for each issue in isolated worktrees. Do NOT write code, fix bugs, or implement features — that work belongs inside the debate-runner agents spawned from Step 5e.

This is the core execution step. The orchestrator launches issue agents in parallel per dependency layer, using the Agent tool's `isolation: "worktree"` for git worktree isolation. This mirrors the milestone parallel pattern in Step 3c — the parent orchestrator spawns, collects, and writes state.

#### 4a. Identify Ready Issues

From the dependency graph built in Step 3b, identify **ready issues** — issues whose status is not `done` and whose `depends_on` entries are all `done` (or empty).

**For explicit pair / --all-files modes:** Skip issue-based execution. Run the specified pairs directly using the single-issue flow (Step 5) without worktree isolation.

#### 4b. Execute Issue Pipelines by Dependency Layer

**File overlap check**: Before launching, expand each ready issue's scope globs to file lists. If any files appear in multiple issues' scopes → overlap detected. In supervised mode, offer: "Merge into one issue", "Run sequentially", or "Run in parallel anyway". Unsupervised: auto-merge when overlap >50% of either issue's files, otherwise auto-sequentialize.

**Pre-launch setup** (once per layer):
1. `git fetch origin main --quiet` — fresh base for worktrees
2. **Ref promotion**: For each non-numeric ref, create a GitHub issue via `create-issue.sh` (with milestone's `github_issue` as parent). On success: rewrite `ref` and `depends_on` references in plan.yaml. On failure: keep local ref, pipeline degrades gracefully. Sync tracking issue after all promotions.
3. **Strategy detection**: Resolve each issue's component `strategy` from workflow.yaml (`debate` default, or `solo`). Pass strategy + `promote_on_guard_failure` in agent context.

**Spawning**: For each ready issue, spawn an Agent with `isolation: "worktree"` and task `/ratchet:run --issue <ref> --milestone <id> [--unsupervised] [--auto-pr] [--no-cache]`. Wait for all Layer N agents before launching Layer N+1.

**Branch base**: Layer 0 branches from `origin/main`. Layer 1+ branches from the dependency's `branch` field in plan.yaml. Multiple dependencies: use the last-finished dependency's branch.

Issue agents enter Mode S, execute Steps 5a-5h, and return structured completion summaries. **Issue agents do NOT write plan.yaml** — the parent orchestrator is the sole writer.

**[TodoWrite — Issue Starts]**: Set layer issues to `"in_progress"`, add phase-level sub-items. Solo mode: label phases with `(solo)` and update with outcome (SOLO PASS/PROMOTED/FAILED). Always include all milestones and issues.

**Worktree management**: `isolation: "worktree"` handles creation/cleanup automatically. **Guard singletons**: `flock` serializes across parallel agents with no orchestrator coordination needed.

#### 4c. Process Issue Results (after layer completes)

After all issue agents in a layer complete, process results in batch. **Do NOT fix, debug, or modify anything — just record state and proceed.**

**For each completed issue** (status `done`):
1. **Commit + PR**: In the issue's worktree, create branch `ratchet/<milestone-slug>/<issue-ref>`, commit all changes, push. Create PR using body from `skills/run/pr-body.md` (includes `Fixes #<ref>`, debate summary, dependency notes). Skip commit if worktree has no uncommitted changes. Without `--auto-pr`: ask user before creating PR.
2. **Update plan.yaml** (atomically, in batch): set `status`, `phase_status`, `branch`, `pr`, `files`, `debates` for each issue.
3. **Sync tracking issue** and **remove worktree** after push.

**Layer progression**: Check if Layer N+1 issues are now unblocked → launch next batch (back to 4b). Report: `"Layer [N] complete: [N]/[total] issues done"`.

**[TodoWrite — Issue Complete]**: Set completed issues to `"completed"` with progress count. Remove phase sub-items. Halted issues stay `"in_progress"` with halt reason in content.

**Halted issues**: Record halt in plan.yaml immediately. Supervised: let user decide (resolve/continue/stop). Unsupervised: continue with remaining issues; halt milestone only if ALL issues blocked.

**Merge conflicts on existing PRs**: Do NOT resolve directly. Re-launch the issue pipeline in a fresh worktree from current main. The pipeline naturally produces compatible code.

**All layers done** → milestone complete, proceed to Step 8.

---

### Step 5: Issue Pipeline (executed per-issue in isolated worktrees)

> **For the full issue pipeline specification, read `skills/run/issue-pipeline.md`.**

This step is the phase-gated execution loop for a single issue. Each issue agent
(spawned by Step 4b) executes this in its own worktree. The issue agent spawns
debate-runner agents and returns a structured completion summary that the
orchestrator parses in Step 4c.

**Pipeline stages** (detailed in `skills/run/issue-pipeline.md`):
- **5a.** Determine current phase, match pairs, resolve component strategy (`debate` or `solo`)
- **5b.** File-hash cache check (skip unchanged pairs)
- **Shared Resources** — provisioning, singleton locking via flock
- **5c.** Pre-execution guards (blocking/advisory)
- **5d.** Prepare execution context (models, publish config, escalation precedents)
- **5e.** Execute phase (strategy branch):
  - **5e-debate** — spawn debate-runners, handle results (ACCEPT, CONDITIONAL_ACCEPT, ESCALATED, REGRESS)
  - **5e-solo** — spawn generative agent directly, run post-execution guards, handle promotion on guard failure
- **5f.** Phase gate — post-execution guards (debate mode), advance phase
- **5g.** Phase regression (REGRESS handling, budget tracking — debate mode only)
- **5h.** Issue complete — structured completion summary output (includes `execution_mode`)

---

### Step 5-dry: Dry-Run Preview

If `--dry-run` is specified, produce a formatted preview and stop. No agents are spawned, no debates created, no files modified.

#### Token Cost Estimation

After building the dependency graph (Step 3), compute estimated tokens per issue using these formulas:

**Base tokens by pipeline mode:**
| Pipeline mode | Base tokens | Phases | Rationale |
|---|---|---|---|
| `solo` | 20k | (single pass) | Single generative pass, no adversarial |
| `review` | 40k | review | Review-only debate (1 phase, gen + adv) |
| `hotfix` | 60k | build, review | Fast-track fix (2 phases) |
| `secure` | 60k | review, harden | Security hardening (2 phases) |
| `standard` | 80k | plan, build, review, harden | Standard pipeline (4 phases) |
| `full` | 160k | plan, test, build, review, harden | Full pipeline (5 phases) |

**Scaling factors:**
- **Pairs**: Multiply base by the number of pairs assigned to the issue. Each pair runs its own debate/execution.
- **Guards**: Add 2k per guard (both pre-execution and post-execution) assigned to the issue's phases. Guards invoke external commands and produce output that consumes context.
- **Max rounds** (debate mode only): The base already accounts for typical round counts. For `max_rounds > 3`, scale the debate portion by `max_rounds / 3`.

**Formula:**
```
issue_tokens = (base_tokens × pair_count × round_scale) + (2000 × guard_count)

where:
  base_tokens  = mode lookup from table above
  pair_count   = number of pairs assigned to the issue
  round_scale  = max(1, max_rounds / 3) for debate strategies, 1 for solo
  guard_count  = number of guards matching the issue's component and phases
```

**Cost estimation** uses current API rates (update these when rates change):
- Opus input: $15 / 1M tokens, output: $75 / 1M tokens (assume 30% input, 70% output)
- Sonnet input: $3 / 1M tokens, output: $15 / 1M tokens (assume 30% input, 70% output)
- Debate mode uses both opus (generative) and sonnet (adversarial) — estimate 60% opus, 40% sonnet by token volume
- Solo mode uses opus only

```
# Blended rate per 1k tokens (debate mode):
#   opus:   0.30 × $0.015 + 0.70 × $0.075 = $0.057/1k tokens
#   sonnet: 0.30 × $0.003 + 0.70 × $0.015 = $0.0114/1k tokens
#   blended = 0.60 × $0.057 + 0.40 × $0.0114 = $0.0388/1k tokens
#
# Solo mode (opus only):
#   $0.057/1k tokens
```

#### Dry-Run Output Format

```
Dry-Run Preview
═══════════════

Milestone: [name] — [description]

Issues ([N] total, [N] ready to run in parallel):

  [ref]: [title]
    Strategy: debate
    Phase: [current phase]
    Pairs: [pair-name], [pair-name]
    Pre-execution guards: [guard-name] (blocking)
    Post-execution guards: [guard-name] (advisory)

  [ref]: [title]  (solo)
    Strategy: solo
    Phase: [current phase]
    Pairs: [pair-name]
    Post-execution guards: [guard-name] (blocking)
    Promote on guard failure: [yes|no]

  [ref]: [title]  (depends on [dep-ref])
    Phase: pending — waiting for dependency
    Pairs: [pair-name]

Phase flow per issue: [phase1] → [phase2] → ... → [phaseN]

Token & Cost Estimates
──────────────────────
  Issue       Mode       Pairs  Guards  Est. Tokens   Est. Cost
  ─────       ────       ─────  ──────  ───────────   ─────────
  [ref]       standard   2      3       ~166k         ~$6.44
  [ref]       solo       1      2       ~24k          ~$1.37
  [ref]       review     1      1       ~42k          ~$1.63
  ─────       ────       ─────  ──────  ───────────   ─────────
  Total                                 ~232k         ~$9.44
```

**In `--unsupervised` mode**: Log the token and cost estimates to stdout but do not block execution. The estimates are informational — they help operators audit spend after the fact. Do not present the `AskUserQuestion` confirmation (unsupervised auto-selects "Run for real").

**In supervised mode**: Include the cost table in the `AskUserQuestion` confirmation:

Question text:
```
Dry-run complete. Estimated cost: ~$[total] ([total_tokens]k tokens).

[cost table from above]

Proceed?
```

Options: `"Run for real (Recommended)"`, `"Done for now"`

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

**8a. Mark milestone done**: Set `status: done` with completion timestamp in plan.yaml. Sync tracking issue.

**[TodoWrite — Milestone Complete]**: Set milestone to `"completed"` with summary (e.g., `"M2: [name] — 4/4 issues done"`). Remove issue/phase sub-items.

**8b. Progress tracking**: If adapter configured, update status to "done" and close the item.

**8c. Post-Milestone Analyst Assessment**: Spawn read-only analyst agent (`disallowedTools: Write, Edit`) to review debates, scores, and guard results. Produces 3-5 bullet points on pair effectiveness, scope gaps, guard recommendations, and workflow presets. Present via `AskUserQuestion` with options: "Apply recommendations", "Note for later", "Skip".

**CRITICAL: NEVER push to origin/main or force-push. Local commits are the default safe action.**

### Step 9: Update Scores & Teardown Resources

Score data is computed on-demand by `/ratchet:score` — no update step needed here.

**Resource teardown**: Run `stop` commands for all resources in workflow.yaml. Clean up `.ratchet/locks/`. Teardown timing: sequential mode → after milestone completes; DAG mode → after ALL milestones complete. Always tear down regardless of success/failure.

**Stop PR monitor** if started in Step 1b.

### Step 10: Propose Next Focus

**Unsupervised**: If work remains and no halt, spawn continuation agent with `/ratchet:run --unsupervised`. If all milestones complete, halt with summary. If halted, present halt summary and stop.

**Supervised** — present options via `AskUserQuestion` based on state:

- **Blocked/escalated issues**: Offer resolve, continue, or stop.
- **Milestone complete, more remain**: Offer "Continue to [next]" (re-invoke for fresh context), view metrics, or stop. **CONTEXT CLEARING**: At milestone boundaries, re-read all state from disk. Do NOT continue in the current context — instruct user to re-run.
- **ALL milestones done**: Epic complete summary with options: create new epic, add milestone, tighten, score, review debate, or stop.
