# Issue Pipeline Specification

> This file is the extracted Step 5 from `skills/run/SKILL.md`.
> It is loaded on demand when the orchestrator executes an issue pipeline.
> For the orchestrator flow (Steps 1-4, 6-10), see `skills/run/SKILL.md`.

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

**[TodoWrite — Phase Starts]**: After determining the current phase and matching pairs, update the todo list: set this phase item to `"in_progress"` and include the pair names in the content. Example: `"Build phase (tui-visualization, api-safety)"`. All other items retain their current statuses.

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
# Verify guard script exists before running
test -f .claude/ratchet-scripts/run-guards.sh \
  || { echo "Error: run-guards.sh not found. Run install.sh to restore Ratchet scripts." >&2; exit 1; }

# Without singleton resources:
bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>

# With singleton resources (e.g., requires: [postgres]):
flock .ratchet/locks/postgres.lock bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>
```

- If a **blocking** pre-debate guard fails:
  - **Guilty until proven innocent**: This failure is caused by the current issue's changes unless proven otherwise. Before dismissing as pre-existing, verify on clean master:
    ```bash
    # In the worktree, stash changes and test on clean base
    git stash && bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>
    git stash pop
    # Only if the guard ALSO fails on clean base can the failure be considered pre-existing
    ```
  - Use `AskUserQuestion`: "Pre-debate guard '[name]' failed: [summary]. Debates have NOT started yet."
  - Options: `"Fix and re-run (Recommended)"`, `"Override and proceed to debates"`, `"Cancel — skip this phase"`
  - If fix or cancel: skip debate creation entirely
- If an **advisory** pre-debate guard fails:
  - Log the failure and pass the output as context to the debates (the debates must address it — guilty until proven innocent)
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
5. **Resolve publish config**: Read from the values already computed in Step 1b:
   - `publish_debates`: the validated value (already corrected for adapter mismatch)
   - `progress_ref`: use the **issue-level** progress ref, NOT the epic-level one. Debate comments belong on the specific issue being worked on, not the epic tracking issue:
     ```bash
     # Issue-level ref (e.g., the GitHub issue number for this specific work item).
     # NOTE: progress_ref is not yet auto-populated on issues by the adapter framework.
     # The yq path is forward-compatible for when adapters populate it; the #N regex
     # fallback is the current working path for github-issues adapter.
     issue_progress_ref=$(yq eval "
       .epic.milestones[].issues[] | select(.ref == \"$ISSUE_REF\") | .progress_ref // null
     " .ratchet/plan.yaml)

     # Fallback: if the issue has no progress_ref (not yet created by adapter),
     # check if the issue ref itself is a GitHub issue number (e.g., "#42")
     if [ "$issue_progress_ref" = "null" ] && [[ "$ISSUE_REF" =~ ^#[0-9]+$ ]]; then
       issue_progress_ref="$ISSUE_REF"
     fi

     # Diagnostic: warn when no progress_ref could be resolved
     if [ "$issue_progress_ref" = "null" ]; then
       echo "Note: No progress_ref resolved for issue $ISSUE_REF — debate comments will not be published." >&2
     fi
     ```
     The epic-level `progress_ref` (`epic.progress_ref`) is for plan sync operations only — it should NOT be passed to debate-runners. Debate round comments posted to the epic tracking issue create noise and make individual issues harder to follow.
   - `adapter`: the resolved adapter from Step 1b
   - If `adapter` is `"none"` or empty: pass all three as `null` (adapter not configured)

#### 5e. Run Debates

Spawn a **debate-runner** agent for each matched pair. When multiple pairs match the current phase, spawn them **in parallel**.

Use `model` set to the resolved `debate_runner` model (defaults to `sonnet`).

**Tool boundaries for the debate-runner agent:**
- The debate-runner has Write and Edit tools but may ONLY use them for debate artifacts inside `.ratchet/debates/`, `.ratchet/escalations/`, and `.ratchet/reviews/`. It MUST NOT write to any other path — no source code, no tests, no application config.
- When the debate-runner spawns a **generative agent**, it MUST grant Write and Edit tools (the generative agent is the ONLY role that writes code).
- When the debate-runner spawns an **adversarial agent**, it MUST NOT grant Write or Edit tools. The adversarial agent is read-only — it reviews, validates, and critiques but never modifies files.
- When the debate-runner spawns a **tiebreaker agent**, it MUST NOT grant Write or Edit tools. The tiebreaker is read-only — it evaluates arguments and renders a verdict.

Each debate-runner receives:
```
Run debate for pair [pair-name] in phase [phase].

ROLE BOUNDARY — You are a debate-runner, NOT a solver:
  You orchestrate debate rounds between generative and adversarial agents.
  You may use Write/Edit ONLY for debate artifacts in .ratchet/debates/,
  .ratchet/escalations/, and .ratchet/reviews/. You NEVER modify source
  code, tests, or application config. Your tools are: Read, Write, Edit,
  Agent, AskUserQuestion — with Write/Edit gated to .ratchet/ paths only.

  When spawning agents, enforce these tool boundaries:
    Generative agent: tools = Read, Grep, Glob, Bash, Write, Edit
    Adversarial agent: tools = Read, Grep, Glob, Bash — disallowedTools = Write, Edit
    Tiebreaker agent: tools = Read, Grep, Glob, Bash — disallowedTools = Write, Edit

  If you feel the urge to edit source code or tests, STOP — spawn the generative agent instead.

PRINCIPLE — Guilty Until Proven Innocent:
  New changes are GUILTY until proven innocent. Test failures on a PR
  branch are CAUSED by the PR unless definitively proven otherwise.
  The burden of proof is on demonstrating the failure exists on master,
  not assuming it is unrelated. If a test fails, fix it — do not dismiss
  it without running the same test on a clean master checkout as evidence.

Pair definitions:
  Generative: .ratchet/pairs/<name>/generative.md
  Adversarial: .ratchet/pairs/<name>/adversarial.md

Context:
  Worktree: [absolute path to issue worktree, e.g. /workspace/main/.ratchet/worktrees/issue-43]
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
  Publish:
    publish_debates: [false | per-round | summary, or null if adapter is none]
    progress_ref: [issue-level progress_ref — the GitHub issue for THIS issue, or null]
    adapter: [adapter name from workflow.yaml, or null if adapter is none]
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

- **`verdict: "consensus"` with `verdict_detail: "ACCEPT"` or `verdict_detail: "TRIVIAL_ACCEPT"`**:
  - Update file-hash cache:
    ```bash
    bash .claude/ratchet-scripts/cache-update.sh <pair-name> "<scope-glob>" <debate-id>
    ```
  - Update the issue's `files` array with `files_modified` and append debate ID to `debates`
  - If TRIVIAL_ACCEPT: note `fast_path: true`
  - Sync plan tracking issue:
    ```bash
    if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
      bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
        || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
    fi
    ```

- **`verdict: "consensus"` with `verdict_detail: "CONDITIONAL_ACCEPT"`**:
  - This means the debate-runner resolved conditions internally (generative addressed them, adversarial confirmed in a follow-up round). Treat as consensus with noted conditions:
    - Update file-hash cache (same as ACCEPT)
    - Update the issue's `files` array with `files_modified` and append debate ID to `debates`
    - Log the conditions from the debate result's `conditions` array in the issue's metadata for traceability
    - Sync plan tracking issue (same as ACCEPT — files/debates updated):
      ```bash
      if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
        bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
          || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
      fi
      ```

- **`verdict: "escalated"` with `escalation_reason: "conditions_unresolved"`**:
  - CONDITIONAL_ACCEPT conditions were never fully resolved within max_rounds
  - Follow the same escalation flow as other escalated verdicts (below)
  - Log: "Debate [id] escalated: CONDITIONAL_ACCEPT conditions unresolved after [N] rounds"

- **`verdict: "escalated"`** (human escalation required):
  - Update issue status in plan.yaml
  - Output the early-exit summary (see Step 5h) and return

- **`verdict: "regress"`** (REGRESS):
  - Handle regression (Step 5g)

**[TodoWrite — Debate Results]**: After processing each debate result, update the phase item's content to include the verdict. For example: `"Build phase — ACCEPT R1"` or `"Build phase — CONDITIONAL_ACCEPT R2"`. If escalated: `"Build phase — ESCALATED"`. Do not change the phase status yet (that happens in Step 5f).

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
# Verify guard script exists (same check as pre-debate guards)
test -f .claude/ratchet-scripts/run-guards.sh \
  || { echo "Error: run-guards.sh not found. Run install.sh to restore Ratchet scripts." >&2; exit 1; }

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
- Sync plan tracking issue after phase state change:
  ```bash
  if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
    bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
      || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
  fi
  ```

**[TodoWrite — Phase Complete]**: After marking the phase done, update the todo list: set the phase item to `"completed"`. The content should retain the verdict info from Step 5e (e.g., `"Build phase — ACCEPT R1"`). If advancing to the next phase, Step 5a's TodoWrite will set the next phase to `"in_progress"`.

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
  - **Debate Summary section** (see below)
- Push and create via `gh pr create`

**Building the Debate Summary section:**

The issue pipeline tracks debate IDs in the issue's `debates` array in plan.yaml (recorded by the debate-runner via `yq` after each debate completes). Read them at PR creation time:

```bash
# Step 1: Read debate IDs from the issue's debates array in plan.yaml
# issue_ref is the ref string for this issue (e.g., "#46"), available in the
# issue pipeline's context from when the orchestrator launched this pipeline.
export ISSUE_REF="${issue_ref}"
mapfile -t debate_ids < <(
  yq -r '.epic.milestones[].issues[] | select(.ref == env(ISSUE_REF)) | .debates[]?' \
    .ratchet/plan.yaml
)

# If no debates recorded, omit the section entirely
if [ "${#debate_ids[@]}" -eq 0 ]; then
  debate_summary_section=""
else
  # Step 2: Build the table rows — one row per debate ID
  table_rows=""
  conditions_block=""
  has_conditional=false

  for debate_id in "${debate_ids[@]}"; do
    meta_path=".ratchet/debates/${debate_id}/meta.json"
    if [ ! -f "$meta_path" ]; then
      echo "Warning: could not load debate metadata for ${debate_id}" >&2
      continue
    fi
    meta=$(cat "$meta_path")
    pair=$(echo "$meta" | jq -r '.pair')
    phase=$(echo "$meta" | jq -r '.phase')
    rounds=$(echo "$meta" | jq -r '.rounds_completed')
    verdict=$(echo "$meta" | jq -r '.verdict_detail // .verdict')
    decided_by=$(echo "$meta" | jq -r '
      if .decided_by then .decided_by
      elif (.verdict_detail == "ACCEPT" or .verdict_detail == "TRIVIAL_ACCEPT" or .verdict == "consensus") then "consensus"
      elif (.verdict_detail == "ESCALATED" or .verdict == "ESCALATED") then "tiebreaker"
      else "consensus"
      end')
    table_rows="${table_rows}| ${pair} | ${phase} | ${rounds} | ${verdict} | ${decided_by} |\n"

    # Collect conditions for CONDITIONAL_ACCEPT debates
    if [ "$verdict" = "CONDITIONAL_ACCEPT" ]; then
      has_conditional=true
      while IFS= read -r condition; do
        conditions_block="${conditions_block}- ${pair}: ${condition}\n"
      done < <(echo "$meta" | jq -r '.conditions[]? // empty')
    fi
  done

  # Step 3: Assemble the section
  debate_summary_section="## Debate Summary

| Pair | Phase | Rounds | Verdict | Decided By |
|------|-------|--------|---------|------------|
$(printf "%b" "$table_rows")"

  if [ "$has_conditional" = true ]; then
    debate_summary_section="${debate_summary_section}

<details>
<summary>Conditions</summary>
$(printf "%b" "$conditions_block")
</details>"
  fi
fi
```

The resulting `$debate_summary_section` is appended to the PR body at `gh pr create` time:

```bash
# Step 4: Create the PR with the assembled body
pr_body="$(cat <<EOF
## Summary

$(echo "$phase_summaries")

Closes ${ISSUE_REF}
${depends_on_line}

${debate_summary_section}
EOF
)"

git push -u origin "${branch_name}"
gh pr create \
  --title "${issue_title}" \
  --body "${pr_body}" \
  --base main
```

**Example output — two debates, one CONDITIONAL_ACCEPT:**

```markdown
## Debate Summary

| Pair | Phase | Rounds | Verdict | Decided By |
|------|-------|--------|---------|------------|
| api-contracts | review | 2 | ACCEPT | consensus |
| skill-coherence | build | 1 | CONDITIONAL_ACCEPT | consensus |

<details>
<summary>Conditions</summary>
- skill-coherence: Verify cross-reference to skills/run/SKILL.md exists
</details>
```

(Each debate in the issue's `debates` array produces one row. Multiple pairs → multiple rows. Conditions block only appears when at least one CONDITIONAL_ACCEPT verdict exists.)

**Rules:**
- Data sourced exclusively from `meta.json` `conditions` array — do NOT summarize adversarial narrative or round text
- Conditions block only shown when at least one `CONDITIONAL_ACCEPT` verdict exists (`$has_conditional = true`)
- If `meta.json` is missing or unreadable for a debate ID, skip that row and log a warning: `"Warning: could not load debate metadata for [id]"` to stderr
- Section omitted entirely when `debates` array is empty (`${#debate_ids[@]} -eq 0`)
- Multiple debates → multiple table rows; a debate with no conditions contributes no lines to the conditions block

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
   - Sync plan tracking issue (phase_status changed):
     ```bash
     if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
       bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
         || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
     fi
     ```
   - Loop back to Step 5a

#### 5h. Issue Complete

When all phases are done:
- Set issue status to `done` in local plan.yaml (worktree copy — useful for crash recovery, but the orchestrator will write the authoritative update in Step 4c)
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
