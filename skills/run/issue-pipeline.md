# Issue Pipeline Specification

> This file is the extracted Step 5 from `skills/run/SKILL.md`.
> It is loaded on demand when the orchestrator executes an issue pipeline.
> For the orchestrator flow (Steps 1-4, 6-10), see `skills/run/SKILL.md`.

---

### Step 5: Issue Pipeline (executed per-issue in isolated worktrees)

This is the phase-gated loop for a single issue. Each issue runs as a separate agent spawned by Step 4b, with its own git worktree via `isolation: "worktree"`.

**Execution context:**
- Spawned by Step 4b as a parallel Agent invocation (one per issue in the current layer)
- Runs in an isolated git worktree (provided by `isolation: "worktree"`)
- Spawns debate-runner agents at Step 5e (or generative agent directly in solo mode — Step 5d-solo)
- Returns a structured completion summary (Step 5h) — does NOT write plan.yaml (the parent orchestrator handles that in Step 4c)

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

#### 5d-solo. Solo Execution Branch

**Before** preparing debate context, check if the component's `strategy` is `solo`. The `strategy` field on a component in `workflow.yaml` determines the execution mode:

- `strategy: "debate"` (default, omitted) — full adversarial debate via debate-runner (Steps 5d–5e)
- `strategy: "solo"` — generative agent + guards, no adversarial review

If the component's strategy is NOT `solo`, skip this section entirely and proceed to **Step 5d**.

**Solo mode execution:**

Solo mode skips the debate-runner entirely. It spawns the generative agent directly, runs post-build guards, and uses guard results as the quality gate instead of adversarial consensus.

**Worktree isolation**: Solo mode runs under the same worktree isolation constraints as debate mode. The issue agent is already in an isolated worktree (from Step 4b's `isolation: "worktree"`). The generative agent spawned here operates within that worktree — same as in debate mode.

**Tool boundaries for the solo generative agent:**
- The generative agent has: Read, Grep, Glob, Bash, Write, Edit
- It operates on source code within the worktree scope
- No adversarial agent is spawned — guards serve as the quality gate

**Step 1 — Spawn generative agent:**

Spawn the generative agent directly with the build-phase prompt. Use the resolved `generative` model (pair-level override if set, otherwise global from `workflow.yaml`).

```
Run solo execution for pair [pair-name] in phase [phase].

ROLE — You are the generative agent in solo mode (no adversarial review).
Your output will be validated by deterministic guards, not an adversarial agent.
Write clean, correct code — guards are your quality gate.

PRINCIPLE — Guilty Until Proven Innocent:
  Guard failures are YOUR fault unless you can prove they exist on clean master.

Pair definition: .ratchet/pairs/<name>/generative.md

Context:
  Worktree: [absolute path to issue worktree]
  Phase: [current phase]
  Milestone: [id, name, description]
  Issue: [issue ref]
  Files in scope: [matched file list]
  Plan phase output: [path, if phase > plan]
  Test phase output: [paths, if phase > test]
  Models:
    generative: [resolved model]
```

Wait for the generative agent to complete. Collect `files_modified` from the agent's output.

**Step 2 — Run post-build guards:**

Run all guards where `timing: "post-debate"` (or no timing field) for the current phase — identical to the guard execution in Step 5f. The same resource provisioning, singleton locking, and `run-guards.sh` invocation apply:

```bash
# Verify guard script exists before running
test -f .claude/ratchet-scripts/run-guards.sh \
  || { echo "Error: run-guards.sh not found. Run install.sh to restore Ratchet scripts." >&2; exit 1; }

bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>
```

**Step 3 — Evaluate guard results:**

**(a) All guards pass → success:**

Create the executions directory if it doesn't exist, then write execution log:

```bash
mkdir -p .ratchet/executions
```

Write execution log to `.ratchet/executions/<id>.yaml`:

```yaml
id: "<pair-name>-<ISO-timestamp>"
mode: solo
component: "<component-name>"
issue: "<issue-ref>"
started: "<ISO-start>"
resolved: "<ISO-now>"
guard_results:
  - name: "<guard-name>"
    passed: true
    duration_ms: <ms>
promoted: false
promotion_reason: null
debate_id: null
files_modified:
  - "<file1>"
  - "<file2>"
```

The execution log MUST conform to `schemas/execution.schema.json`. Update the issue's `files` array with `files_modified`. Mark the current phase as `done` in `phase_status`. Proceed to the next phase (loop back to Step 5a) or output the completion summary (Step 5h) if all phases are done.

**(b) Any blocking guard fails AND `promote_on_guard_failure: true`:**

The `promote_on_guard_failure` field is set on the component in `workflow.yaml` (default: `false`). When `true`, a solo-mode guard failure escalates to the full debate path instead of failing outright.

1. Log the promotion event in the execution log:
   ```yaml
   id: "<pair-name>-<ISO-timestamp>"
   mode: promoted
   component: "<component-name>"
   issue: "<issue-ref>"
   started: "<ISO-start>"
   resolved: null  # still in progress — debate will resolve it
   guard_results:
     - name: "<guard-name>"
       passed: false
       reason: "<guard output summary>"
       duration_ms: <ms>
   promoted: true
   promotion_reason: "guard <guard-name> failed"
   debate_id: null  # filled in after debate completes
   files_modified:
     - "<file1>"
     - "<file2>"
   ```

2. **Fall through to the existing debate path** (Steps 5d → 5e). The generative agent's changes remain in the worktree — the debate-runner picks up from the current state. Pass the guard failure output as additional context to the debate so the adversarial agent can focus on the failure:
   ```
   PROMOTION CONTEXT — Solo mode guard failure:
     Guard: [guard-name]
     Output: [guard failure output]
     The generative agent's changes are in the worktree. The adversarial agent
     should focus on why the guard failed and whether the changes are correct.
   ```

3. After the debate completes, update the execution log's `debate_id` and `resolved` fields.

**(c) Any blocking guard fails AND `promote_on_guard_failure: false` (default):**

1. Write execution log with `mode: solo` and the failed guard result:
   ```yaml
   id: "<pair-name>-<ISO-timestamp>"
   mode: solo
   component: "<component-name>"
   issue: "<issue-ref>"
   started: "<ISO-start>"
   resolved: "<ISO-now>"
   guard_results:
     - name: "<guard-name>"
       passed: false
       reason: "<guard output summary>"
       duration_ms: <ms>
   promoted: false
   promotion_reason: null
   debate_id: null
   files_modified:
     - "<file1>"
     - "<file2>"
   ```

2. Return failure. In unsupervised mode, set issue status to `failed` with `halt_reason: "solo guard failure: <guard-name>"`. In supervised mode, use `AskUserQuestion`:
   - Question: "Solo execution guard '[guard-name]' failed: [summary]."
   - Options: `"Fix and re-run (Recommended)"`, `"Promote to debate"`, `"Cancel — skip this phase"`
   - If "Promote to debate" is selected: follow the promotion path described in (b) above.

**(d) Advisory guard fails:**

Log the failure in the execution log but continue. Advisory guards do not block advancement — same behavior as in debate mode (Step 5f).

---

#### 5d. Prepare Debate Context

For each matched pair, prepare the context for the **debate-runner** agent:

1. **Resolve `max_rounds`**: Pair-level if set, otherwise global.
2. **Gather escalation precedents**: Scan `.ratchet/escalations/` for matching pair rulings.
3. **Gather phase context**:
   - If phase > plan: read the plan phase spec output
   - If phase > test: read test file locations
   - Collect any unresolved CONDITIONAL_ACCEPT conditions
4. **Resolve models**: Pair-level overrides take precedence over global. Pass resolved `generative`, `adversarial`, and `tiebreaker` models to the debate-runner.
5. **Resolve progress_ref for publish hook**: The issue's `ref` is the canonical identity. If `ref` is numeric (i.e., a promoted GitHub issue number), use it directly as `progress_ref`. If `ref` is a local string (not yet promoted), pass `null` — publishing degrades gracefully.
   ```
   progress_ref = ref if ref is numeric, else null
   ```
   Pass `progress_ref` to the debate-runner so it writes it into `meta.json` at debate creation (Step 1) — before any round files trigger the publish hook.

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
  Progress:
    progress_ref: [resolved GitHub issue number, or null — write this into meta.json at debate creation]
  Models:
    generative: [resolved model]
    adversarial: [resolved model]
    tiebreaker: [resolved model]
```

> **Note:** The `progress_ref` MUST be written into `meta.json` at debate creation (Step 1), before any round files are written. The `publish-debate-hook.sh` PostToolUse hook fires on each round file Write and reads `progress_ref` from meta.json to resolve the GitHub issue to post to. If `progress_ref` is missing at that point, publishing silently fails.

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

**No Fallback Validation** (debate-mode components only): For components with `strategy: "debate"` (the default), the debate-runner is the ONLY acceptable path for quality enforcement. Guards alone are insufficient substitutes for adversarial review. Auto-approval without debate violates the quality contract. Components with `strategy: "solo"` use guards as the quality gate by design — see Step 5d-solo.

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

**Commit and PR creation is the orchestrator's responsibility, not the issue agent's.**

The issue agent runs debates and produces code in the worktree. It does NOT commit, push, or create PRs. The worktree branch with uncommitted changes is the deliverable — the orchestrator handles packaging in Step 4c.

**Why:** Agents at depth 3+ routinely truncate the protocol, completing debates but dropping the commit/PR step. Moving packaging to the orchestrator (depth 1) ensures it always happens. The issue agent's only post-debate job is to report results via the completion summary (Step 5h).

**Progress tracking**: If a progress adapter is configured:
- On phase advancement: add a comment noting the phase completed

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
  executions: [execution-id-1, ...]  # solo/promoted execution logs (omit if none)
  files: [file1, file2, ...]
  worktree: [absolute path to worktree with uncommitted changes]
```

The `worktree` path is critical — it tells the orchestrator where the uncommitted code lives so it can commit and create a PR in Step 4c. The issue agent does NOT commit or push.

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
  executions: [execution-id-1, ...]  # solo/promoted execution logs (omit if none)
  files: [file1, ...]
  worktree: [absolute path or "none"]
```

This summary MUST be the last thing you output. The orchestrator reads plan.yaml for structured state, but this summary provides immediate human-readable feedback.
