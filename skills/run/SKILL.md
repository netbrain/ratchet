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

The orchestrator may ONLY spawn agents in these three categories:

1. **Issue pipeline agents** (Step 4b) — agents that run a single issue's phase
   pipeline in an isolated worktree. They spawn debate-runners at Step 5e.
   The debate-runner is the ONLY valid path for code changes.
2. **Analyst agents** (Step 8c) — read-only assessment agents
   (`disallowedTools: Write, Edit`) that analyze data and produce recommendations.
   They NEVER modify files.
3. **Continuation agents** (Step 10, unsupervised mode) — orchestrator agents that
   inherit the same source-code boundary and plan management authority, and
   continue the `/ratchet:run` loop.

**No milestone sub-agents.** The orchestrator runs milestones directly (Step 3c)
to keep the agent chain at 3 levels: orchestrator → debate-runner → gen/adv.
Spawning milestone-level agents adds a 4th level where chain collapse occurs.

**NEVER spawn an agent with implementation instructions.** If your Agent prompt
contains phrases like "implement X", "fix Y", "add Z", "write code for", "create
the file", or "modify the source" — STOP. You are bypassing the debate framework.
All implementation work MUST flow through: orchestrator -> debate-runner ->
generative agent. There are no shortcuts.

**Violation examples (all FORBIDDEN):**
- `Agent("Implement the AGENT GATE feature in skills/run/SKILL.md")` — direct implementation
- `Agent("Fix the failing test in src/auth.ts")` — direct bug fix
- `Agent("Add error handling to the parser module")` — direct code change
- `Agent("Refactor the database layer")` — direct refactoring

**Correct pattern:**
- `Agent("Run debate for pair [name] in phase [phase]. ...")` — spawns a debate-runner
- `Agent("/ratchet:run --issue issue-3 --milestone 2")` with `isolation: "worktree"` — spawns an issue pipeline
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
```

## Unsupervised Mode

For unsupervised mode behavior, read `skills/run/unsupervised.md`.

Covers: auto-selection rules for every `AskUserQuestion` step, self-continuation via the Agent tool at milestone boundaries, halt conditions (issue-level and milestone-level), and combining `--unsupervised` with `--auto-pr`, `--no-cache`, `--all-files`, and `--dry-run`. Note: `--go` is shorthand for `--unsupervised --auto-pr`.

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

**CHECKPOINT**: You now understand the project state. Do NOT act on it — do not analyze code, do not plan fixes, do not write implementations. Your next action is Step 2: present choices to the user (or auto-select in unsupervised mode). Then Step 4: launch issue pipelines. The pipelines do the work.

### Step 2: Determine Focus

There are five modes, checked in order:

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

**[TodoWrite — Initial Plan]**: After building the issue DAG (and milestone DAG in DAG mode), write the initial todo list showing the full plan. Include all milestones and their issues with current statuses:

```
TodoWrite([
  {"id": "m2", "content": "M2: [milestone name]", "status": "in_progress"},
  {"id": "m2-issue32", "content": "[ref]: [title]", "status": "pending"},
  {"id": "m2-issue33", "content": "[ref]: [title]", "status": "pending"},
  {"id": "m3", "content": "M3: [milestone name]", "status": "pending"}
])
```

Show all milestones (not just the current one) so the user sees the full roadmap. Mark already-completed milestones/issues as `"completed"`. In DAG mode with parallel milestones (Step 3c), show all ready milestones as `"in_progress"`.

**Progress tracking**: If a progress adapter is configured (`.ratchet/workflow.yaml` → `progress.adapter`), and this milestone doesn't have a `progress_ref` yet, create a work item:
```bash
bash .claude/ratchet-scripts/progress/<adapter>/create-item.sh "<milestone name>" "<milestone description>" "ratchet" "milestone"
```
Store the returned reference in `plan.yaml` as `progress_ref` on the milestone. If the adapter fails, log a warning and continue — adapter failures never block debates.

**MILESTONE RE-OPENING GUARD**: If the chosen milestone has `status: done`, do NOT silently re-open it. Instead, use `AskUserQuestion`:
- Question: "Milestone '[name]' is already marked done (completed [timestamp]). Re-opening it will reset its status. Are you sure?"
- Options: `"Re-open milestone"`, `"Pick a different milestone"`, `"Cancel"`
- Only set `status: in_progress` if the user explicitly confirms re-opening.

#### 3c. Execute Milestones (DAG mode — sequential with parallel issues)

**Design decision — no milestone sub-agents.** The orchestrator executes milestones directly instead of spawning milestone-level agents. This keeps the agent chain at 3 levels:

```
orchestrator → debate-runner → generative + adversarial
```

Spawning milestone sub-agents adds a 4th level that causes agent chain collapse — agents at depth 3+ routinely skip the debate framework and write code directly. By running milestones inline, the debate-runner stays at depth 2 where compliance is reliable.

**Milestone execution order (DAG mode):**

Process milestone layers sequentially. Within each layer, all milestones are processed one at a time (the orchestrator handles one milestone's issue DAG before moving to the next). Issue parallelism within a milestone is preserved (Step 4b).

```
Milestone execution plan:
  Layer 0: M[id] ([N] issues), M[id] ([N] issues)
  Layer 1: M[id] ([N] issues) — depends on Layer 0
  [...]
```

For each milestone in the current layer:
1. Set milestone to `status: in_progress` in plan.yaml
2. Build issue dependency graph (Step 3b)
3. Execute issue pipelines (Step 4) — issues within the milestone run in parallel via Agent tool
4. Process issue results (Step 4c)
5. Run milestone completion (Step 8) if all issues done
6. Check if any Layer N+1 milestones are now unblocked → continue to next layer

**When all milestones across all layers are done** → epic complete, proceed to Step 10.

**If a milestone halts**: present the halt reason. In supervised mode, let the user decide. In unsupervised mode, continue with remaining milestones in the same layer — a halted milestone only blocks milestones that depend on it.

**Context clearing**: At each milestone boundary, the orchestrator re-reads plan.yaml and workflow.yaml from disk. This prevents accumulated context drift. In unsupervised mode with many milestones, the orchestrator spawns a continuation agent (Step 10) after completing each milestone to get a fresh context window.

### Step 4: Execute Issue Pipelines

**CHECKPOINT**: You are about to execute issue pipelines. Your job is to orchestrate the phase-gated execution for each issue in isolated worktrees. Do NOT write code, fix bugs, or implement features — that work belongs inside the debate-runner agents spawned from Step 5e.

This is the core execution step. The orchestrator launches issue agents in parallel per dependency layer, using the Agent tool's `isolation: "worktree"` for git worktree isolation. This mirrors the milestone parallel pattern in Step 3c — the parent orchestrator spawns, collects, and writes state.

#### 4a. Identify Ready Issues

From the dependency graph built in Step 3b, identify **ready issues** — issues whose status is not `done` and whose `depends_on` entries are all `done` (or empty).

**For explicit pair / --all-files modes:** Skip issue-based execution. Run the specified pairs directly using the single-issue flow (Step 5) without worktree isolation.

#### 4b. Execute Issue Pipelines by Dependency Layer

**File overlap check (before launching):**

Before spawning parallel issue agents, check whether issues in the same layer have overlapping file scopes. Overlapping issues produce conflicting changes in parallel worktrees — wasted work.

For each pair of ready issues in the current layer:
1. Resolve each issue's file scope (from its pairs' `scope` globs in workflow.yaml)
2. Expand the globs to actual file lists
3. If any files appear in more than one issue's scope → overlap detected

If overlap is detected, use `AskUserQuestion`:
- Question: "Issues [ref-A] and [ref-B] overlap on [N] files: [file list]. Running them in parallel will likely produce conflicting changes."
- Options:
  - `"Merge into one issue (Recommended)"` — combine the issues in plan.yaml (merge titles, pairs, and depends_on), then run as one
  - `"Run sequentially instead"` — run ref-A first, then ref-B in the next layer (add ref-A to ref-B's depends_on)
  - `"Run in parallel anyway"` — proceed, accept possible conflicts

In unsupervised mode: auto-select "Merge into one issue" when overlap exceeds 50% of either issue's files, otherwise "Run sequentially instead".

---

For each dependency layer (from Step 3b), launch all ready issues **in parallel** as separate Agent invocations. This mirrors the milestone parallel pattern in Step 3c.

**Execution strategy:**
- **Layer-parallel execution**: All issues in the same dependency layer run concurrently as separate Agent invocations
- **Worktree isolation**: Each issue agent uses `isolation: "worktree"` on the Agent tool, giving it an isolated copy of the repository
- **Agent-per-issue**: Each issue is spawned with `/ratchet:run --issue <ref> --milestone <id>`
- **Layer synchronization**: Wait for all issues in Layer N to complete before launching Layer N+1
- **Sequential fallback**: If no issues declare `depends_on`, issues still run in parallel as a single Layer 0. For milestones with only 1 issue, this naturally degrades to sequential.

**Parallelism detection** (mirrors milestone DAG detection in Step 3a):
- All ready issues (from Step 4a) in the same layer are launched simultaneously
- Issues with unmet `depends_on` wait in later layers

**Issue ref promotion (lazy GitHub issue creation):**

Before spawning issue agents, promote any non-numeric refs to GitHub issue numbers. This is just-in-time — planning stays offline-capable.

For each issue in the current ready layer whose `ref` is NOT already a number:
1. If the `github-issues` adapter is configured and `gh` is available:
   ```bash
   PARENT_ISSUE=$(yq eval ".epic.milestones[] | select(.id == \"$MS_ID\") | .github_issue // null" .ratchet/plan.yaml)
   MS_NAME=$(yq eval ".epic.milestones[] | select(.id == \"$MS_ID\") | .name" .ratchet/plan.yaml)
   MS_DESC=$(yq eval ".epic.milestones[] | select(.id == \"$MS_ID\") | .description" .ratchet/plan.yaml)
   ISSUE_DESC=$(yq eval ".epic.milestones[] | select(.id == \"$MS_ID\") | .issues[] | select(.ref == \"$ISSUE_REF\") | .description // \"\"" .ratchet/plan.yaml)
   ISSUE_PAIRS=$(yq eval ".epic.milestones[] | select(.id == \"$MS_ID\") | .issues[] | select(.ref == \"$ISSUE_REF\") | .pairs | join(\", \")" .ratchet/plan.yaml)
   ISSUE_FILES=$(yq eval ".epic.milestones[] | select(.id == \"$MS_ID\") | .issues[] | select(.ref == \"$ISSUE_REF\") | .files | join(\", \")" .ratchet/plan.yaml)

   # Build a rich issue body with context
   ISSUE_BODY="## Context

   Milestone: **${MS_NAME}** — ${MS_DESC}

   ## Description

   ${ISSUE_DESC:-$ISSUE_TITLE}

   ## Scope

   **Pairs:** ${ISSUE_PAIRS:-none assigned}
   **Files:** ${ISSUE_FILES:-to be determined during plan phase}"

   ISSUE_NUM=$(bash .claude/ratchet-scripts/progress/github-issues/create-issue.sh \
     "$ISSUE_TITLE" "$ISSUE_BODY" \
     ${PARENT_ISSUE:+--parent "$PARENT_ISSUE"} 2>/dev/null) || true
   ```

   **Issue descriptions in plan.yaml**: When creating epics and milestones, always include a `description` field on each issue (not just `title`). The description should explain *what* and *why* — enough context for someone reading the GitHub issue to understand the problem and approach without needing plan.yaml.
2. If creation succeeds: rewrite `ref` to the returned number, update any `depends_on` arrays in the same milestone that referenced the old ref, write plan.yaml
   ```bash
   OLD_REF="$ISSUE_REF"
   NEW_REF="$ISSUE_NUM"
   # Rewrite ref
   yq eval -i "(.epic.milestones[] | select(.id == \"$MS_ID\") | .issues[] | select(.ref == \"$OLD_REF\")).ref = \"$NEW_REF\"" .ratchet/plan.yaml
   # Rewrite depends_on references across all issues in this milestone
   yq eval -i "(.epic.milestones[] | select(.id == \"$MS_ID\") | .issues[].depends_on) |= map(select(. == \"$OLD_REF\") = \"$NEW_REF\" // .)" .ratchet/plan.yaml
   ```
3. If creation fails: keep the local ref. Pipeline runs normally — publishing degrades gracefully (no comments posted, but no crash). The ref gets promoted on the next successful run.

Sync the plan tracking issue after all promotions in the layer:
```bash
if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
  bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
    || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
fi
```

**Fresh base fetch**: Before spawning any issue agents in a layer, fetch the latest remote state so worktrees branch from the current `origin/main` — not a stale local HEAD:
```bash
git fetch origin main --quiet
```
This runs once per layer, not per issue. All worktrees in the layer share the fetched ref.

**Spawning parallel issue agents:**

For each issue in the current ready layer, spawn an Agent with:
- `isolation`: `"worktree"` — each agent gets its own git worktree automatically
- Task: `/ratchet:run --issue <ref> --milestone <id> [--unsupervised] [--auto-pr] [--no-cache]`
- The issue's `ref` is now the GitHub issue number (if promoted) or the local ref (if promotion failed/skipped). The issue pipeline uses numeric refs directly as `progress_ref` for debate publishing and `Fixes #<ref>` in PR bodies.

The issue agent enters Mode S, executes Steps 5a-5h independently in its worktree, and returns a structured completion summary (Step 5h). The **parent orchestrator** collects all results and writes plan.yaml — issue agents do NOT write plan.yaml.

**Branch base resolution for dependent issues:**
- Layer 0 issues: branch from `origin/main` (freshly fetched above)
- Layer 1+ issues: branch from the dependency's `branch` field in plan.yaml (written by the orchestrator after Layer N-1 completes)
- Multiple dependencies: branch from the dependency that finished last

**Layer dispatch presentation:**
```
Executing issue dependency layer 0 ([N] issues in parallel):
  [ref]: [title] — [N] pairs, starting at [phase]
  [ref]: [title] — [N] pairs, starting at [phase]

[If Layer 1+ exists: "[N] more issues in [N] layers waiting for dependencies"]
```

After Layer 0 completes, the orchestrator processes results (Step 4c), checks which Layer 1 issues are now unblocked, and launches the next batch.

**[TodoWrite — Issue Starts]**: When launching a layer of issue pipelines, update the todo list: set all issues in the layer to `"in_progress"` and add phase-level items. Include all phases from each issue's workflow:

```
TodoWrite([
  {"id": "m2", "content": "M2: [milestone name]", "status": "in_progress"},
  {"id": "m2-issue32", "content": "[ref]: [title]", "status": "in_progress"},
  {"id": "m2-issue32-plan", "content": "Plan phase", "status": "completed"},
  {"id": "m2-issue32-build", "content": "Build phase ([pair-name])", "status": "in_progress"},
  {"id": "m2-issue32-review", "content": "Review phase", "status": "pending"},
  {"id": "m2-issue32-harden", "content": "Harden phase", "status": "pending"},
  {"id": "m2-issue33", "content": "[ref]: [title]", "status": "pending"},
  {"id": "m3", "content": "M3: [milestone name]", "status": "pending"}
])
```

Always include all milestones, all issues, and the phase breakdown for all active issues in the layer.

**Note on worktree management**: The Agent tool's `isolation: "worktree"` handles worktree creation and cleanup automatically. The issue agent receives an isolated copy of the repository. If the agent makes changes, the worktree path and branch are returned in the result. The orchestrator uses this to record the branch in plan.yaml.

**Note on plan.yaml write authority**: Issue agents do NOT write plan.yaml. They return structured completion summaries (Step 5h). The parent orchestrator is the sole plan.yaml writer — it collects results from all parallel issue agents in a layer, then writes all updates atomically. This eliminates write contention by construction.

**Note on guard singleton resources**: Guards with `singleton: true` resources use `flock` for serialization. When multiple issue agents run in parallel, each agent's guards independently acquire locks via `flock .ratchet/locks/<resource>.lock`. This means singleton-guarded operations serialize across parallel issues automatically — correct behavior, with no orchestrator coordination needed.

[If Layer 1+ exists: "[N] more issues waiting for dependencies"]
```

#### 4c. Process Issue Results (after issue completes)

After all issue agents in a layer complete, the orchestrator processes results in batch. **Do NOT fix, debug, or modify anything from the results — just record state and proceed.**

**Processing a layer's results:**

1. **Collect all agent results**: Each issue agent returns a structured completion summary (Step 5h). Read all results from the layer.

2. **Package each completed issue (commit + PR)**: For each issue with status `done`, the orchestrator creates the commit and PR from the issue agent's worktree. This is the orchestrator's job — not the issue agent's — because agents at depth 3+ routinely truncate the protocol and skip packaging.

   For each completed issue with a `worktree` path:
   ```bash
   cd "<worktree>"

   # Branch name
   BRANCH="ratchet/<milestone-slug>/<issue-ref>"
   git checkout -b "$BRANCH" 2>/dev/null || git checkout "$BRANCH"

   # Commit all changes
   git add -A
   git commit -m "<issue title>

   Debated by: <pair-names>
   Verdicts: <verdict summaries>"

   # Push and create PR
   git push -u origin "$BRANCH"
   ```

   PR creation uses `gh pr create` with the body from `skills/run/pr-body.md`:
   - `Fixes #<ref>` when ref is a numeric (promoted) GitHub issue number
   - Debate summary table from all debates in this issue
   - Dependency note if `depends_on` references exist

   > **For the PR body construction, read `skills/run/pr-body.md`.**

   Store the branch name and PR URL on the issue in plan.yaml.

   **If the worktree has no uncommitted changes** (agent already committed, or no code was modified): skip the commit step, just check if a branch exists.

   **If `--auto-pr` is not set** (supervised mode): use `AskUserQuestion` before creating the PR:
   - Question: "Issue [ref] complete. Create PR?"
   - Options: `"Create PR (Recommended)"`, `"Skip PR — just keep the branch"`, `"View changes first"`

3. **Update plan.yaml in batch**: For each issue in the layer, update the MAIN repo's plan.yaml:
   - Set `status` (done, blocked, escalated, failed)
   - Set `phase_status` for all phases
   - Set `branch` (from the commit step above)
   - Set `pr` (from the PR creation step above, or null)
   - Set `files` (list of modified files)
   - Set `debates` (debate IDs created)
   - Write all updates atomically — the orchestrator is the sole writer

4. **Sync plan tracking issue** (if github-issues adapter configured):
   ```bash
   if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
     bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
       || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
   fi
   ```

5. **Worktree cleanup**: After committing and pushing, the worktree can be removed. The branch on the remote is the durable deliverable.
   ```bash
   git worktree remove "<worktree>" 2>/dev/null || true
   ```

5. Check if any **Layer 1+ issues** are now unblocked (their dependencies just completed)

6. If newly unblocked issues exist → launch them as the next layer (back to 4b)

7. Report progress after each layer: `"Layer [N] complete: [N]/[total] issues done in milestone [id]"`

**[TodoWrite — Issue Complete]**: After recording the issue result, update the todo list: set the issue item to `"completed"` and update its content to include the progress count (e.g., `"[ref]: [title] (2/4 issues done)"`). Remove the phase sub-items for the completed issue to keep the list concise. If the issue halted, keep its status as `"in_progress"` and append the halt reason in the content (e.g., `"[ref]: [title] -- BLOCKED: guard failure"`). Note: TodoWrite has no `"blocked"` status; `"in_progress"` is the closest approximation for halted issues — the content string communicates the actual state.

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

**IMPORTANT (AGENT GATE enforcement)**: Do NOT run debates yourself. Do NOT spawn generative or adversarial agents directly. Do NOT spawn any agent with implementation instructions ("implement X", "fix Y", "add Z"). All code changes flow through debate-runners only. See the AGENT GATE section at the top of this document.

---

### Step 5: Issue Pipeline (executed per-issue in isolated worktrees)

> **For the full issue pipeline specification, read `skills/run/issue-pipeline.md`.**

This step is the phase-gated execution loop for a single issue. Each issue agent
(spawned by Step 4b) executes this in its own worktree. The issue agent spawns
debate-runner agents and returns a structured completion summary that the
orchestrator parses in Step 4c.

**Pipeline stages** (detailed in `skills/run/issue-pipeline.md`):
- **5a.** Determine current phase and match pairs
- **5b.** File-hash cache check (skip unchanged pairs)
- **Shared Resources** — provisioning, singleton locking via flock
- **5c.** Pre-debate guards (blocking/advisory)
- **5d-solo.** Solo execution branch — if component `strategy: "solo"`, spawn generative agent directly + guards (skip debate-runner)
- **5d.** Prepare debate context (models, publish config, escalation precedents)
- **5e.** Run debates — spawn debate-runners, handle results (ACCEPT, CONDITIONAL_ACCEPT, ESCALATED, REGRESS)
- **5f.** Phase gate — post-debate guards, advance phase, commit/PR at configured boundaries
- **5g.** Phase regression (REGRESS handling, budget tracking)
- **5h.** Issue complete — structured completion summary output

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
- Sync plan tracking issue:
  ```bash
  if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
    bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
      || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
  fi
  ```

**[TodoWrite — Milestone Complete]**: After marking the milestone done, update the todo list: set the milestone item to `"completed"`. Remove all issue/phase sub-items for this milestone to keep the list compact. Update the milestone content to include the summary (e.g., `"M2: [name] — 4/4 issues done"`). If other milestones remain, they retain their current statuses.

**8b. Progress tracking:**
- If adapter configured: update milestone status, close the item
  ```bash
  bash .claude/ratchet-scripts/progress/<adapter>/update-status.sh "<progress_ref>" "done"
  bash .claude/ratchet-scripts/progress/<adapter>/close-item.sh "<progress_ref>"
  ```

**8c. Post-Milestone Analyst Assessment:**

Spawn the analyst agent (with resolved `analyst` model, defaults to `opus`) for a brief assessment. Agent configuration:
- `tools`: Read, Grep, Glob, Bash
- `disallowedTools`: Write, Edit

The post-milestone analyst is **read-only** — it analyzes data and produces recommendations. Any file modifications from recommendations are applied by the orchestrator's parent skill, not the analyst.

The analyst reviews all issue debates, scores, guard results, and any escalation data to produce 3-5 bullet points covering:
- Pair effectiveness observations
- Scope coverage gaps
- Guard recommendations
- Workflow preset recommendations

Present via `AskUserQuestion`:
- Question: "Post-milestone assessment for [milestone name]:\n[bullet points]"
- Options: `"Apply recommendations (Recommended)"`, `"Note for later"`, `"Skip"`

**CRITICAL: NEVER push to origin/main or force-push. NEVER push unless the user explicitly chose "Create a pull request" within an issue pipeline. Local commits are the default safe action.**

### Step 9: Update Scores & Teardown Resources

Score data is computed on-demand by `/ratchet:score` from debate artifacts (`debates/*/meta.json` and `reviews/**/*.json`) and persisted as an exponential moving average in `.ratchet/scores.yaml`. The EMA survives debate archival across epics — no score update step is needed here.

**Resource teardown**: tear down shared resources when no more pipelines need them:
- **Sequential mode**: after all issue pipelines for the milestone complete
- **DAG mode**: after ALL milestones across all layers complete (the orchestrator handles teardown after the last milestone)

For teardown:
1. For each resource in `workflow.yaml` that has a `stop` command, run it
2. Clean up `.ratchet/locks/` directory (remove `resources.json` and any stale lock directories)

Resources are torn down regardless of whether milestones succeeded, halted, or had errors — always clean up.

**Stop PR monitor**: If the PR watch loop was started in Step 1b, stop it now. The run is complete — any new PR issues will be caught on the next `/ratchet:run` invocation or by a manual `/ratchet:watch`.

### Step 10: Propose Next Focus

**If `--unsupervised`**: Skip `AskUserQuestion`. If no halt condition was triggered and work remains (more milestones), persist all state to `plan.yaml` and spawn a new agent via the Agent tool with task `/ratchet:run --unsupervised`. The continuation agent inherits the same source-code boundary and plan management authority. If all milestones are complete, halt with the completion summary. If a halt condition was triggered during this iteration, present the halt summary and stop.

**Otherwise**, use `AskUserQuestion` to let the user choose what to do next.

**If the milestone has blocked/escalated issues:**
- Summary: `"Milestone [name]: [N]/[total] issues complete. [N] blocked/escalated."`
- Options:
  - "Resolve escalated debates (/ratchet:verdict)" — if escalated
  - "View issue status" — show per-issue phase progress
  - "Re-run to continue (/ratchet:run) (Recommended)" — picks up unblocked issues
  - "Done for now"

**If milestone complete, more milestones remain:**

**CONTEXT CLEARING**: Milestone boundaries are the primary context clearing point. Persisted state (plan.yaml, debate transcripts, pair definitions, scores) is the source of truth — not context memory. At each milestone boundary, re-read state files from disk. In unsupervised mode, the continuation agent (Step 10) starts with fresh context, preventing drift from auto-compaction summaries.

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
  - "Create a new epic" — use AskUserQuestion (freeform) to ask what the next body of work is, then create the new epic structure in plan.yaml (new epic name, description, milestones with issues). Use the analyst agent to help scope if the user wants a thorough analysis, or create directly from the user's description for straightforward requests.
  - "Add a milestone to the current epic" — use AskUserQuestion to gather milestone details, then append to plan.yaml
  - "Tighten agents from debate lessons (/ratchet:tighten)"
  - "View detailed quality metrics (/ratchet:score)"
  - "Review a specific debate (/ratchet:debate)"
  - "Done for now"
