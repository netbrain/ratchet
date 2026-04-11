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

## Boundaries

You are an orchestrator, not a solver. You do NOT write code, fix bugs, implement features, resolve merge conflicts, or rebase branches.

| Rule | Scope | Notes |
|---|---|---|
| **Source Code Boundary** | NEVER use Write/Edit on source, test, or config files | Exception: `--here` mode in top-level human-interactive sessions only. Spawned agents CANNOT claim `--here`. Git rebase/merge/cherry-pick blocked even under `--here`. |
| **TOOL GATE** | Check EVERY Bash command before running | `git rebase/merge/cherry-pick` → STOP, route to issue pipeline (blocked even under `--here`). Write/Edit on source → STOP (except `--here`). Reading source to "understand" a conflict → STOP. |
| **AGENT GATE** | Check EVERY Agent invocation before spawning | Only 4 valid agent types: (1) Issue pipeline agents (Step 4b), (2) Quick-fix generative agents (Mode Q), (3) Analyst agents (read-only, `disallowedTools: Write, Edit`), (4) Continuation agents (Step 10). NEVER spawn agents with implementation instructions (except Mode Q). No milestone sub-agents — keeps chain at 3 levels. |
| **Plan Management** | You ARE the authority on `.ratchet/plan.yaml` | CAN modify: epics, milestones, issues, statuses, focus, discoveries, progress_ref, branch, pr, debates, files, regressions, github_issue. CANNOT modify: source code, workflow.yaml, pairs/, debates/. Use `yq eval -i` for plan.yaml changes. |

**`--here` mode** bypasses the Agent tool entirely — orchestrator executes directly in-session. Only top-level human-interactive sessions; spawned agents MUST NOT claim it. It is a modifier, not a mode — modifies how the resolved mode executes.

### Caveman Mode (Self)

If the workflow config has `caveman.enabled: true` and `caveman.intensity.orchestrator` is not `off`, read `caveman/snippets.md` from the repo root, extract the section matching the resolved intensity, and apply that compression style to your own user-facing output — messages, question text in AskUserQuestion calls, and log output. This does NOT affect structured data (plan.yaml updates, yq commands, agent spawn prompts, TodoWrite entries — those are always precise). Read `caveman.intensity.orchestrator` from the values computed in Step 1b.

### GitHub Plan Tracking Issue

> **For the canonical body format, HTML comment metadata rules, and sync helper pattern, read `skills/run/plan-tracking-format.md`.**

### Guilty Until Proven Innocent

New changes are GUILTY until proven innocent. Test/guard/CI failures on a PR branch are CAUSED by the PR unless definitively proven otherwise (e.g., `git stash && run-test` on clean master). This principle is passed to all spawned agents.

---

### Allowed Tools

- **Read, Glob, Grep** — read state files
- **Agent** — spawn issue pipelines and debate-runners
- **AskUserQuestion** — present choices
- **TodoWrite** — update progress checklist (see "Progress Tracking via TodoWrite" below)
- **Bash** — guard scripts, read-only git/gh commands, `yq eval -i` on plan.yaml

Your job is to:
1. **Manage the epic roadmap** — create/modify epics, milestones, issues in plan.yaml
2. Read state (plan.yaml, workflow.yaml)
3. Build dependency graphs — milestones (DAG mode) and issues within each milestone
4. Launch milestone pipelines (parallel DAG or sequential)
5. Within each milestone, launch issue pipelines in parallel (each in an isolated worktree)
6. Process results — update plan.yaml, advance milestones, complete epics

Issue pipelines spawn debate-runners. Debate-runners spawn generative and adversarial agents. The generative agent writes code. You do none of that — but you ARE the authority on plan structure and milestone lifecycle.

If a PR has merge conflicts, re-launch the issue pipeline to handle it — never resolve conflicts directly.

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
/ratchet:run --here                     # In-session execution — work directly in the current session, no worktree
/ratchet:run --here --issue <ref>       # In-session issue — skip worktree, work on current branch
/ratchet:run --here --quick "<desc>"    # In-session quick-fix — follows Mode Q auto-commit behavior
/ratchet:run --here --auto-pr           # In-session with auto-commit + auto-PR (no prompt)
/ratchet:run --no-auto-merge            # Disable auto-merging of prerequisite PRs
```

## Unsupervised Mode

For unsupervised mode behavior, read `skills/run/unsupervised.md`.

Covers: auto-selection rules for every `AskUserQuestion` step, self-continuation via the Agent tool at milestone boundaries, halt conditions (issue-level and milestone-level), combining `--unsupervised` with `--auto-pr`, `--no-cache`, `--all-files`, `--dry-run`, and `--quick`, and forbidden combinations (`--here --unsupervised`, `--here --go`). Note: `--go` is shorthand for `--unsupervised --auto-pr`.

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

### Sync Convention

**Sync plan tracking issue** — used at multiple pipeline boundaries. The canonical call pattern:

```bash
if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
  bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
    || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
fi
```

All subsequent references to "Sync plan tracking issue." mean: run the above pattern. Non-blocking — failures never halt the pipeline.

### Progress Tracking via TodoWrite

TodoWrite **replaces** the full list on every call — always include all items with current statuses. The orchestrator maintains a running `todo_items` list in memory, mutates it at each of 7 pipeline boundaries (marked **[TodoWrite]** below), and passes the full list to `TodoWrite`.

**Pattern**: `{id: "<hierarchical-id>", content: "<label>", status: "pending|in_progress|completed"}`

| Level | ID format | Example |
|---|---|---|
| Milestone | `m<N>` | `m2` |
| Issue | `m<N>-<ref>` | `m2-issue32` |
| Phase | `m<N>-<ref>-<phase>` | `m2-issue32-build` |

Keep items concise. Include verdict info in completed phases (e.g., `"Build phase — ACCEPT R1"`). For solo strategy, suffix with `(solo)` on the issue and use `SOLO PASS/PROMOTED/FAILED` on phases.

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
5. **Inherit policy from root**: Read the root `workflow.yaml` for shared policy fields (`models`, `escalation`, `max_rounds`, `max_regressions`, `pr_scope`, `caveman`). The workspace's own `workflow.yaml` overrides these per-field (not all-or-nothing — e.g., a workspace can override just `models.adversarial` or `caveman.intensity.generative` and inherit everything else)
6. **No `workspaces` key** → single-project mode, use `.ratchet/` as-is (no change from current behavior)

#### 1b. Read State

Read `plan.yaml` (if it exists), `project.yaml`, and `workflow.yaml` from the resolved `.ratchet/` directory.

**`publish_debates` note**: Debate round publishing is handled by the `publish-debate-hook.sh` PostToolUse hook, not by the orchestrator or debate-runner. The hook reads `publish_debates` and `adapter` directly from `workflow.yaml`. No orchestrator-side validation or passing of publish config is needed.

**Caveman config resolution**: After reading `workflow.yaml`, extract per-role caveman intensities:
```bash
caveman_enabled=$(yq eval '.caveman.enabled // false' .ratchet/workflow.yaml)
if [ "$caveman_enabled" = "true" ]; then
  caveman_generative=$(yq eval '.caveman.intensity.generative // "full"' .ratchet/workflow.yaml)
  caveman_adversarial=$(yq eval '.caveman.intensity.adversarial // "full"' .ratchet/workflow.yaml)
  caveman_tiebreaker=$(yq eval '.caveman.intensity.tiebreaker // "full"' .ratchet/workflow.yaml)
  caveman_orchestrator=$(yq eval '.caveman.intensity.orchestrator // "full"' .ratchet/workflow.yaml)
  caveman_debate_runner=$(yq eval '.caveman.intensity.debate_runner // "full"' .ratchet/workflow.yaml)
else
  caveman_generative=off
  caveman_adversarial=off
  caveman_tiebreaker=off
  caveman_orchestrator=off
  caveman_debate_runner=off
fi
```
These resolved values are used when spawning issue pipelines (Step 5) and for the orchestrator's own output style.

Build a picture of:
- Which milestones are **completed** (status: done)
- Which milestones are **current** (status: in_progress)
- **Milestone parallelism mode**: if ANY milestone has a `depends_on` field → DAG mode. Otherwise → sequential mode.
- In DAG mode: which milestones are **ready** (all dependencies done, status not done)
- For each relevant milestone: which **issues** exist, their `phase_status`, `depends_on` relationships, and current status
- Which issues can run in **parallel** (no unmet dependencies) vs which must wait
- Any unresolved conditions from previous CONDITIONAL_ACCEPT verdicts
- Which **phases** apply based on component workflows
- Each component's **strategy** (`debate` or `solo`, default: `debate`)
- Each component's **`promote_on_guard_failure`** flag (default: `false`)

If no `plan.yaml` exists, check whether the github-issues adapter is configured. If `progress.adapter` is `github-issues` and `sync-plan.sh` exists, attempt recovery via `bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh --recover`. If recovery fails or adapter is not github-issues, skip epic tracking and fall through to file-based detection.

If `plan.yaml` exists but fails to parse (malformed YAML or missing `epic` key), halt with an error.

**Start PR monitor**: If any issues in plan.yaml have non-null `pr` fields, start the PR watch loop:
```
/loop 10m check Ratchet PRs for conflicts and CI failures
```
Skip this if no PRs exist yet (first run of a new epic).

#### 1c. Orphan Detection

Run `bash .claude/ratchet-scripts/check-orphans.sh --ratchet-dir "$RATCHET_DIR"` to identify stale state (abandoned worktrees, unresolved debates, incomplete executions, stale in-progress issues). Orphan detection is advisory — it never blocks the pipeline.

**If findings exist**: Each finding has `type` (stale_issue, unresolved_debate, orphan_worktree, incomplete_execution), `ref`, `age`, and `suggested_action`.

**In supervised mode**, present each via `AskUserQuestion` with options: `"Resume"`, `"Abandon"`, `"Ignore"`.

**In unsupervised mode**, auto-select based on age: >24h → Abandon, <4h → Resume, else → Ignore. Unknown age → Abandon.

**Abandon actions**: stale_issue → reset status to pending; unresolved_debate → `rm -rf`; orphan_worktree → `git worktree remove`; incomplete_execution → `rm -f`.

**Resume actions**: stale_issue → set as current_focus; others → log for continuation.

**CHECKPOINT**: You now understand the project state. Do NOT act on it — proceed to Step 2.

### Step 2: Determine Focus

**`--here` pre-check (before mode resolution):** If `--here` is present, validate it
immediately — before evaluating any mode. Check forbidden combinations first:
`--here --unsupervised` and `--here --go` halt with an error. If valid, set an
internal `here_mode = true` flag. This flag does NOT change which mode is selected —
it modifies how the selected mode EXECUTES. Mode resolution proceeds normally
(Q → M → S → A → B → C → D). After a mode is resolved, the `here_mode` flag
changes its execution: no worktree isolation, no agent spawning, direct in-session
work. See `skills/run/modes/here.md` for full details.

There are six modes and one modifier, checked in this order:

| Priority | Flag | Mode | Action |
|---|---|---|---|
| pre-check | `--here` | modifier | Validate combinations, set `here_mode = true`. For full spec, read `skills/run/modes/here.md` |
| 1 | `--quick "<desc>"` | Q | Single generative pass, no plan.yaml. Terminal path. For full spec, read `skills/run/modes/quick-fix.md` |
| 2 | `--milestone <id>` | M | Jump to Step 3b for this milestone. One-liner below. |
| 3 | `--issue <ref>` | S | Execute issue pipeline (Step 5) directly. One-liner below. |
| 4 | `[pair-name]` / `--all-files` | A | Run specified pairs directly. Skip epic negotiation. |
| 5 | plan.yaml exists | B | Epic-guided focus selection. For full spec, read `skills/run/modes/epic-guided.md` |
| 6 | git repo, no plan | C | Match changed files to pairs by scope globs. One-liner below. |
| 7 | no plan, no code | D | Ask what to build first. |

**`--dry-run`** intercepts after Step 3 (dependency graph built) and before Step 4 (execution). For full spec, read `skills/run/modes/dry-run.md`.

#### Mode Q: Quick-fix (`--quick "<description>"`)
Skip plan.yaml. Auto-detect component, spawn one generative agent, run guards, commit. Terminal path. For full spec, read `skills/run/modes/quick-fix.md`.

#### --here Modifier (in-session execution)
Not a mode — modifies how the resolved mode executes. No worktree isolation, no agent spawning, human serves as quality gate. Forbidden with `--unsupervised` and `--go`. For full spec, read `skills/run/modes/here.md`.

#### Mode M: Single-milestone pipeline (--milestone <id>)
If `--milestone` is set, skip milestone selection. Find the milestone by ID in plan.yaml. Set it to `in_progress` and jump directly to **Step 3b** to build the issue dependency graph for this single milestone. Execute Steps 3b → 4 → 8 for this milestone, then proceed to Step 10. This mode is used for focused runs on a single milestone (user-invoked or continuation agents).

#### Mode S: Single-issue pipeline (--issue <ref>)
If `--issue` is set, execute the issue pipeline (Step 5) directly for the specified issue. Used both for manual/supervised runs and as the entry point for parallel issue agents spawned by Step 4b.

#### Mode A: Explicit pair or --all-files
If the user specified a `[pair-name]` or `--all-files`, use that directly. Skip epic negotiation.

#### Mode B: Epic-guided (plan.yaml exists)
If all milestones are done, present epic-complete flow (new epic, add milestone, tighten, score). Otherwise, present focus selector with issue status, sidequest processing, and milestone options. For full spec, read `skills/run/modes/epic-guided.md`.

#### Mode C: Changed files (no plan.yaml, git repo exists)
Run `git diff --name-only HEAD` and `git diff --name-only --cached`. Match changed files to pairs by `scope` globs. For each changed file, match against ALL component scopes — not just the first match. If a change spans multiple components, present: "This change spans [components]. Running pairs from all matching components."

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

Build dependency layers from the milestone's issues (Layer 0 = no unmet deps, Layer N = deps in earlier layers). This produces the execution order. Issues within the same layer run in parallel.

**[TodoWrite]**: Write initial plan — all milestones and their issues with current statuses.

**Progress tracking**: If a progress adapter is configured and this milestone doesn't have a `progress_ref` yet, create one via `create-item.sh` and store in plan.yaml. Adapter failures never block debates.

**MILESTONE RE-OPENING GUARD**: If the chosen milestone has `status: done`, use `AskUserQuestion` before re-opening: "Milestone '[name]' is already marked done. Re-opening will reset its status. Are you sure?" Options: `"Re-open milestone"`, `"Pick a different milestone"`, `"Cancel"`.

#### 3c. Execute Milestones (DAG mode — sequential with parallel issues)

**Design decision — no milestone sub-agents.** The orchestrator executes milestones directly to keep the agent chain at 3 levels (orchestrator → debate-runner → gen/adv). Spawning milestone sub-agents adds a 4th level where chain collapse occurs.

**Milestone execution order (DAG mode):**

Process milestone layers sequentially. Within each layer, milestones are processed one at a time. Issue parallelism within a milestone is preserved (Step 4b).

For each milestone in the current layer:
1. Set milestone to `status: in_progress` in plan.yaml
2. **Pre-launch: auto-merge prerequisite PRs** (dependent milestones only — see below)
3. Build issue dependency graph (Step 3b)
4. Execute issue pipelines (Step 4)
5. Process issue results (Step 4c)
6. Run milestone completion (Step 8) if all issues done
7. Check if any Layer N+1 milestones are now unblocked → continue to next layer

**Auto-merge prerequisite PRs**: Before starting a milestone that `depends_on` another milestone, check if the prerequisite milestone has unmerged PRs (from `plan.yaml` issue `.pr` fields). For each unmerged PR:

- **Supervised mode**: Confirm via `AskUserQuestion`: "Prerequisite PR [url] from milestone [name] is unmerged. Merge it?" Options: `"Merge (squash)"`, `"Skip — use stacked branch fallback"`, `"Halt"`.
- **Unsupervised mode**: Auto-merge via `gh pr merge --squash` if all checks pass.
- **`--no-auto-merge` flag**: Skip auto-merge entirely, fall through to stacked branch fallback.

If merge succeeds, `git fetch origin main --quiet` to update the base. If merge fails (checks failing, conflicts, permissions), fall through to **stacked branch fallback**.

**Stacked branch fallback**: When auto-merge fails or is skipped, create a temporary integration branch that merges all prerequisite branches:

```bash
git checkout -b integration/<milestone-slug> origin/main
for branch in <prerequisite-branches>; do
  git merge --no-edit "origin/$branch" || { echo "WARN: Cannot integrate $branch — conflicts exist" >&2; break; }
done
```

Use this integration branch as the base for dependent milestone's issue worktrees (instead of `origin/main`). Add warnings to spawned issue agents: "You are working on a stacked branch. Your PR will target the integration branch, not main. Prerequisite PRs must merge first."

If integration branch creation fails (conflicting prerequisites), halt the milestone with a clear error.

**When all milestones across all layers are done** → epic complete, proceed to Step 10.

**If a milestone halts**: present the halt reason. In supervised mode, let the user decide. In unsupervised mode, continue with remaining milestones — a halted milestone only blocks milestones that depend on it.

**Context clearing**: At each milestone boundary, the orchestrator re-reads plan.yaml and workflow.yaml from disk. In unsupervised mode, spawn a continuation agent (Step 10) after each milestone for a fresh context window.

### Step 4: Execute Issue Pipelines

**CHECKPOINT**: You are about to execute issue pipelines. Do NOT write code, fix bugs, or implement features — that work belongs inside debate-runner agents spawned from Step 5e.

This is the core execution step. The orchestrator launches issue agents in parallel per dependency layer, using git worktree isolation — either automatic (via `isolation: "worktree"` for Layer 0) or manual (via `git worktree add` from a dependency's branch for Layer 1+).

#### 4a. Identify Ready Issues

From the dependency graph (Step 3b), identify ready issues — status not `done`, all `depends_on` entries `done` (or empty).

**For explicit pair / --all-files modes:** Skip issue-based execution. Run pairs directly via Step 5 without worktree isolation.

#### 4b. Execute Issue Pipelines by Dependency Layer

**File overlap check**: Before spawning parallel agents, check for overlapping file scopes between issues in the same layer. If overlap detected, use `AskUserQuestion` with options: `"Merge into one issue (Recommended)"`, `"Run sequentially instead"`, `"Run in parallel anyway"`. In unsupervised mode: auto-merge when overlap >50%, otherwise run sequentially.

For each dependency layer, launch all ready issues **in parallel** as separate Agent invocations:

- **Layer-parallel execution**: All issues in the same layer run concurrently
- **Layer 0 issues**: Use `isolation: "worktree"` on the Agent tool (automatic worktree from `origin/main`)
- **Layer 1+ issues**: Manually create worktree from dependency's branch via `git worktree add`, spawn Agent WITHOUT `isolation: "worktree"`, pass worktree path in prompt
- **Layer synchronization**: Wait for all Layer N issues before launching Layer N+1

**Issue ref promotion (lazy GitHub issue creation):** Before spawning, promote non-numeric refs to GitHub issue numbers via `create-issue.sh` with rich body (milestone context, description, scope). Rewrite `ref` and `depends_on` arrays. Sync plan tracking issue.

**Fresh base fetch**: `git fetch origin main --quiet` once per layer before spawning.

**Component strategy detection**: Resolve each issue's component `strategy` (`debate` or `solo`) from `workflow.yaml`. Pass to agent context:
```
Component: [name]
Strategy: [debate|solo]
Promote on guard failure: [true|false]
```

**Issue descriptions in plan.yaml**: Always include a `description` field on each issue — enough context for someone reading the GitHub issue to understand the problem and approach.

The issue agent enters Mode S, executes Steps 5a-5h independently, and returns a structured completion summary (Step 5h). The **parent orchestrator** collects all results and writes plan.yaml — issue agents do NOT write plan.yaml.

**Note on guard singleton resources**: Guards with `singleton: true` use `flock` for serialization. Parallel agents' guards independently acquire locks — correct behavior with no orchestrator coordination needed.

**[TodoWrite]**: Set launched issues to `"in_progress"`, add phase-level items.

#### 4c. Process Issue Results (after issue completes)

After all issue agents in a layer complete, process results in batch. **Do NOT fix, debug, or modify anything — just record state and proceed.**

1. **Collect all agent results** from completion summaries (Step 5h).

2. **Package each completed issue (commit + PR)**: For each `done` issue, create commit and PR from the agent's worktree. Branch name: `ratchet/<milestone-slug>/<issue-ref>`. Commit, push, create PR via `gh pr create` (see `skills/run/pr-body.md` for body format). If `--auto-pr` not set, confirm via `AskUserQuestion`.

3. **GUARD GATE (post-rebase)**: After ANY rebase or conflict resolution in this step, ALL blocking guards for the issue's component MUST run before pushing. Run guards via the component's guard scripts. If any guard fails, the push is blocked — re-launch the issue pipeline to fix. This prevents the gap where rebase agents skip guards (evidence: PR 242 failed linting, PR 199 CI failure). The sequence is: rebase → run ALL guards → push (only if guards pass).

4. **Update plan.yaml in batch**: For each issue, set `status`, `phase_status`, `branch`, `pr`, `files`, `debates`. Write all updates atomically.

5. **Sync plan tracking issue.**

6. **Worktree cleanup**: Layer 0 (automatic via Agent tool). Layer 1+ (manual `git worktree remove`). Always clean up regardless of success/failure.

7. Check if Layer N+1 issues are now unblocked → launch next layer (back to 4b).

8. Report: `"Layer [N] complete: [N]/[total] issues done in milestone [id]"`

**[TodoWrite]**: Set completed issues to `"completed"`, halted issues stay `"in_progress"` with halt reason.

**When all issues across all layers are done** → milestone complete, proceed to Step 8.

**If an issue pipeline halts**: Record halt in plan.yaml. In supervised mode, use `AskUserQuestion` (Resolve/Continue/Done). In unsupervised mode, continue — halted issues only block dependents.

**Handling merge conflicts on existing PRs**: Re-launch the issue pipeline in a fresh worktree from current main. The pipeline re-runs from the appropriate phase, producing code compatible with current main. Do NOT resolve conflicts directly.

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

If `--dry-run` is specified, produce a formatted preview with token/cost estimates and stop. No agents spawned, no files modified. For full spec, read `skills/run/modes/dry-run.md`.

---

### Step 6: Static Analysis Pre-Gate

Before launching issue pipelines, run configured static analysis commands from `project.yaml`. If any fail, use `AskUserQuestion`: "Static analysis failed with [N] errors. How should we proceed?" Options: `"Fix these before running (Recommended)"`, `"Proceed anyway"`.

---

### Step 7: (Reserved — number kept for reference continuity)

---

### Step 8: Milestone Completion

After all issues in the milestone are `done`:

**8a. Mark milestone done:** Set `status: done`, record timestamp, update plan.yaml. Sync plan tracking issue.

**[TodoWrite]**: Set milestone to `"completed"` with summary.

**8b. Progress tracking:** If adapter configured, update status and close the item via `update-status.sh` and `close-item.sh`.

**8c. Post-Milestone Analyst Assessment:**

Spawn analyst agent (resolved `analyst` model, defaults to `opus`; `disallowedTools: Write, Edit`). The analyst reviews all issue debates, scores, guard results, and escalation data to produce 3-5 bullet points covering pair effectiveness, scope gaps, guard recommendations, and workflow preset recommendations.

Present via `AskUserQuestion`: "Post-milestone assessment for [name]:\n[bullets]" Options: `"Apply recommendations (Recommended)"`, `"Note for later"`, `"Skip"`.

**CRITICAL: NEVER push to origin/main or force-push. NEVER push unless the user explicitly chose "Create a pull request" within an issue pipeline.**

### Step 9: Update Scores & Teardown Resources

Score data is computed on-demand by `/ratchet:score` from debate artifacts and persisted as an EMA in `.ratchet/scores.yaml`. No score update step needed here.

**Resource teardown**: After all pipelines complete (sequential: after milestone; DAG: after all milestones), run `stop` commands for each resource in `workflow.yaml`, clean up `.ratchet/locks/`. Teardown runs regardless of success/failure.

**Stop PR monitor**: If started in Step 1b, stop it now.

### Step 10: Propose Next Focus

**If `--unsupervised`**: Skip `AskUserQuestion`. If work remains, persist state and spawn continuation agent via Agent tool with `/ratchet:run --unsupervised`. If all milestones complete, halt with summary. If halt condition triggered, present summary and stop.

**If milestone has blocked/escalated issues:**
- Options: "Resolve escalated debates (/ratchet:verdict)", "View issue status", "Re-run (/ratchet:run) (Recommended)", "Done for now"

**If milestone complete, more remain:**

**CONTEXT CLEARING**: Milestone boundaries are the primary context clearing point. Re-read state files from disk. Continuation agents start fresh.

- Summary: `"Milestone [name] complete! Epic progress: [completed]/[total] milestones.\n\nStarting fresh context for the next milestone."`
- Options: "Continue to [next milestone] (/ratchet:run) (Recommended)", "View quality metrics", "View milestone status (/ratchet:status)", "Done for now"

When the user selects "Continue to [next milestone name]", present: "Run `/ratchet:run` to start [next milestone] with a clean context. All progress is saved."

**If ALL milestones are done:**
- Summary: `"Epic complete! All [N] milestones finished. Total issues: [N] | Total debates: [N] | Consensus rate: [%]"`
- Options: "Create a new epic", "Add a milestone to the current epic", "Tighten agents (/ratchet:tighten)", "View metrics (/ratchet:score)", "Review a debate (/ratchet:debate)", "Done for now"
