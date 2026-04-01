# Issue Pipeline Specification

> This file is the extracted Step 5 from `skills/run/SKILL.md`.
> It is loaded on demand when the orchestrator executes an issue pipeline.
> For the orchestrator flow (Steps 1-4, 6-10), see `skills/run/SKILL.md`.

---

### Step 5: Issue Pipeline (executed per-issue in isolated worktrees)

**Execution context:**
- Spawned by Step 4b in an isolated git worktree (one per issue, parallel)
- Strategy: `debate` (default, spawns debate-runner at 5e) or `solo` (generative agent at 5e-solo)
- Returns structured completion summary (Step 5h) — does NOT write plan.yaml
- Phases run sequentially per `pipeline` preset, then returns to Step 4c

#### 5a. Determine Current Phase and Match Pairs

**[TodoWrite — Phase Starts]**: After determining the current phase and matching pairs, update the todo list: set this phase item to `"in_progress"` and include the pair names in the content. Example: `"Build phase (tui-visualization, api-safety)"`. All other items retain their current statuses.

1. Read the issue's `phase_status` — find the first phase that is `pending` or `in_progress`
2. Determine which phases apply based on the component's `pipeline` preset (or explicit `phases` array if present):
   - `full` (was `tdd`): plan → test → build → review → harden
   - `standard` (was `traditional`): plan → build → review → harden (skip test)
   - `review`: review only (skip plan, test, build, harden)
   - `hotfix`: build → review
   - `secure`: review → harden
3. Resolve the component's `strategy` field (`debate` or `solo`, default: `debate`). This determines the execution path at Step 5e.
4. Match pairs from the issue's `pairs` list that are assigned to the current phase
5. Skip disabled pairs (`enabled: false`)

**Scope resolution:**
- If a pair has `scope: "auto"`, resolve it to the parent component's scope glob.

#### 5b. File-Hash Cache Check

For each matched pair, run the cache check script:

```bash
bash .claude/ratchet-scripts/cache-check.sh <pair-name> "<scope-glob>"
```

- Exit 0 → files unchanged, **skip this pair**: `Skipping [pair-name] — no changes since last consensus`
- Exit 1 → files changed or no cache, proceed to execution (debate or solo depending on strategy)

Use `--no-cache` flag to skip this check and force re-debate.

#### Shared Resources

Guards can declare `requires: [resource-name]` referencing shared resources defined in workflow.yaml (databases, test servers, etc.).

**Lifecycle:** Provision (run idempotent `start` command, track in `.ratchet/locks/resources.json`) → Lock + Run (if `singleton: true`, wrap with `flock` on `.ratchet/locks/<resource-name>.lock` — kernel auto-releases on exit/crash) → Teardown (Step 9 runs `stop` commands, removes `.ratchet/locks/`).

**Singleton locking**: `flock` on `.ratchet/locks/<resource-name>.lock`. Multiple singletons: acquire locks in alphabetical order to prevent deadlocks.

#### Guard Execution Pattern

All guard invocations (pre-execution, post-execution, solo mode) use this pattern:

```bash
test -f .claude/ratchet-scripts/run-guards.sh \
  || { echo "Error: run-guards.sh not found. Run install.sh to restore Ratchet scripts." >&2; exit 1; }

# Without singleton resources:
bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>

# With singleton resources (e.g., requires: [postgres]):
flock .ratchet/locks/postgres.lock bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>
```

All subsequent guard steps reference this pattern rather than repeating it.

#### 5c. Pre-Execution Guards

Run guards where `timing: "pre-execution"` (or the deprecated alias `"pre-debate"`) for the current phase. Guards without a `timing` field are treated as `post-execution` (backward compatible).

For each pre-execution guard assigned to the current phase, run the **Guard Execution Pattern** (see Shared Resources section). Provision any `requires` resources first; wrap singletons with `flock`.

- If a **blocking** pre-execution guard fails:
  - **Guilty until proven innocent**: This failure is caused by the current issue's changes unless proven otherwise. Before dismissing as pre-existing, verify on clean master:
    ```bash
    # In the worktree, stash changes and test on clean base
    git stash && bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>
    git stash pop
    # Only if the guard ALSO fails on clean base can the failure be considered pre-existing
    ```
  - Use `AskUserQuestion`: "Pre-execution guard '[name]' failed: [summary]. Execution has NOT started yet."
  - Options: `"Fix and re-run (Recommended)"`, `"Override and proceed"`, `"Cancel — skip this phase"`
  - If fix or cancel: skip execution entirely (no debate or solo run)
- If an **advisory** pre-execution guard fails:
  - Log the failure and pass the output as context to execution (debates/solo agent must address it — guilty until proven innocent)
  - Continue to execution

#### 5d. Prepare Execution Context

For each matched pair, prepare the context for the execution agent (debate-runner in debate mode, generative agent in solo mode):

1. **Resolve `max_rounds`**: Pair-level if set, otherwise global.
2. **Gather escalation precedents**: Scan `.ratchet/escalations/` for matching pair rulings.
3. **Gather phase context**:
   - If phase > plan: read the plan phase spec output
   - If phase > test: read test file locations
   - Collect any unresolved CONDITIONAL_ACCEPT conditions
4. **Resolve models**: Pair-level overrides take precedence over global. For debate mode, pass resolved `generative`, `adversarial`, and `tiebreaker` models. For solo mode, only `generative` is needed.
5. **Resolve progress_ref** (debate mode only): `progress_ref = ref if numeric, else null`. Must be written into `meta.json` at debate creation (before round files trigger the publish hook).

#### 5e. Execute Phase (Strategy Branch)

At this point, the pipeline branches based on the component's `strategy` field resolved in Step 5a:

- **`strategy: "debate"`** (default) → proceed to **Step 5e-debate** below
- **`strategy: "solo"`** → proceed to **Step 5e-solo** below

Both paths converge at **Step 5f** (Phase Gate — Guards and Advance).

---

#### 5e-solo. Solo Execution (strategy: "solo")

A single generative agent executes the phase; post-execution guards serve as the quality gate. Runs in the issue's isolated worktree (same constraints as debate mode).

**1. Create execution log** in `.ratchet/executions/<pair-name>-<timestamp>.yaml` with fields: `id`, `mode: solo`, `component`, `issue`, `started`, `resolved: null`, `guard_results: []`, `promoted: false`, `promotion_reason: null`, `debate_id: null`, `files_modified: []`, `token_estimate: null`.

**2. Spawn generative agent** (resolved `generative` model). Tools: Read, Grep, Glob, Bash, Write, Edit (same as debate-mode generative). Receives: phase, pair definition (`generative.md`), worktree path, milestone/issue context, files in scope, plan/test output if applicable. Includes the "Guilty Until Proven Innocent" principle.

**3. Process result:** Collect `files_modified` via `git diff --name-only` (staged + unstaged + untracked). Update execution log with `files_modified` and `resolved` timestamp.

**4. Run post-execution guards** using the **Guard Execution Pattern**. Record each result in the execution log's `guard_results` array.

**5. Handle guard results:**

| Condition | Action |
|-----------|--------|
| All guards pass | Log complete, proceed to Step 5f |
| Blocking guard fails + `promote_on_guard_failure: true` | Set `promoted: true`, `mode: "promoted"` in log. Fall through to Step 5e-debate for review-only debate on the solo output. Reference `execution_id` in debate `meta.json` |
| Blocking guard fails + no promotion (default) | Return failure → Step 5h. In supervised mode: AskUserQuestion with fix/override/cancel options |
| Advisory guard fails | Log and continue |

**[TodoWrite — Solo Result]**: Update phase item content: `"Build phase — SOLO PASS"`, `"SOLO PROMOTED"`, or `"SOLO FAILED"`.

---

#### 5e-debate. Run Debates (strategy: "debate")

Spawn a **debate-runner** agent for each matched pair. When multiple pairs match the current phase, spawn them **in parallel**.

Use `model` set to the resolved `debate_runner` model (defaults to `sonnet`).

**Tool boundaries:**

| Role | Write/Edit | Scope |
|------|-----------|-------|
| Debate-runner | Yes | `.ratchet/{debates,escalations,reviews}/` only |
| Generative | Yes | Source code, tests, all files |
| Adversarial | No | Read-only (reviews and critiques) |
| Tiebreaker | No | Read-only (evaluates and verdicts) |

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

> **Note:** `progress_ref` must be in `meta.json` before any round files are written — `publish-debate-hook.sh` reads it on each Write to resolve the target GitHub issue.

**If Debate-Runner Cannot Be Spawned:**
- **Supervised**: AskUserQuestion with options: `"Wait and retry"`, `"Skip this phase"`, `"Escalate issue for manual resolution"`. Retry: up to 3 attempts with exponential backoff (5s, 10s, 20s).
- **Unsupervised**: Halt with `blocked` status. Other milestone issues continue; only dependents blocked.

**No Fallback Validation** (debate mode only): The debate-runner is the ONLY acceptable quality path. No auto-approval without debate. For `strategy: "solo"`, guards ARE the quality gate by design.

#### Handle Debate Results (strategy: "debate" or promoted from solo)

Process each debate-runner result:

- **`verdict: "consensus"` (ACCEPT, TRIVIAL_ACCEPT, or CONDITIONAL_ACCEPT)**:
  - Update file-hash cache: `bash .claude/ratchet-scripts/cache-update.sh <pair-name> "<scope-glob>" <debate-id>`
  - Update issue's `files` array with `files_modified`, append debate ID to `debates`
  - If TRIVIAL_ACCEPT: note `fast_path: true`
  - If CONDITIONAL_ACCEPT: also log `conditions` array in issue metadata for traceability
  - Sync plan tracking issue (run `sync-plan.sh` if it exists, non-blocking)

- **`verdict: "escalated"`** (including `escalation_reason: "conditions_unresolved"` when CONDITIONAL_ACCEPT conditions unresolved within max_rounds):
  - Update issue status in plan.yaml, output early-exit summary (Step 5h), return

- **`verdict: "regress"`**: Handle regression (Step 5g)

**[TodoWrite — Debate Results]**: After processing each debate result, update the phase item's content to include the verdict. For example: `"Build phase — ACCEPT R1"` or `"Build phase — CONDITIONAL_ACCEPT R2"`. If escalated: `"Build phase — ESCALATED"`. Do not change the phase status yet (that happens in Step 5f).

**IMPORTANT**: Never run debates yourself — spawn the debate-runner (debate) or generative agent (solo). After processing results, always proceed through all of Step 5f before advancing phases.

#### 5f. Phase Gate — Guards and Advance

**Check results:** Debate: all consensus → guards; any escalated → early exit (5h); any regress → 5g. Solo: all guards passed → phase advance; promoted → process as debate; failed → early exit (5h).

**Run post-execution guards:**

> **Solo mode note:** If `strategy: "solo"`, guards already ran in Step 5e-solo. Skip execution here — only process recorded results.

For each post-execution guard (`timing: "post-execution"`, deprecated `"post-debate"`, or no timing field), run the **Guard Execution Pattern**. Provision `requires` resources first; wrap singletons with `flock`.

Guard result storage: `.ratchet/guards/<milestone-id>/<issue-ref>/<phase>/<guard-name>.json`

- Blocking guard fails → AskUserQuestion: fix/override/view
- Advisory guard fails → log and continue

**Advance phase:**
- Mark current phase as `done` in the issue's `phase_status`
- Auto-advance on fast-path (all TRIVIAL_ACCEPT) without user confirmation
- If next phase exists → set to `in_progress`, loop back to Step 5a
- If all phases done → issue is complete
- Sync plan tracking issue (run `sync-plan.sh` if it exists, non-blocking)

**[TodoWrite — Phase Complete]**: After marking the phase done, update the todo list: set the phase item to `"completed"`. The content should retain the verdict info from Step 5e (e.g., `"Build phase — ACCEPT R1"` for debate mode, `"Build phase — SOLO PASS"` for solo mode). If advancing to the next phase, Step 5a's TodoWrite will set the next phase to `"in_progress"`.

**Commit/PR is the orchestrator's responsibility** (Step 4c), not the issue agent's. The worktree with uncommitted changes is the deliverable.

**Progress tracking**: On phase advancement, add a comment via progress adapter (if configured).

#### 5g. Phase Regression

> **Solo mode note:** Phase regression does not apply to `strategy: "solo"` — there is no adversarial agent to issue a REGRESS verdict. If a solo execution was promoted to debate and the debate issues REGRESS, handle it normally below.

When an adversarial issues REGRESS targeting an earlier phase:

1. Read `max_regressions` from workflow config (default: 2). Budget is tracked per-milestone (shared across issues — the `regressions` counter is on the milestone).
2. If budget exhausted:
   - Use `AskUserQuestion`: regression budget exhausted, allow/reject/escalate
3. If within budget:
   - Increment milestone's `regressions` counter
   - Reset the issue's `phase_status` for target phase and later to `pending`
   - Set target phase to `in_progress`
   - Preserve debate history
   - Sync plan tracking issue (run `sync-plan.sh` if it exists, non-blocking)
   - Loop back to Step 5a

#### 5h. Issue Complete

Set issue status in local plan.yaml (worktree copy — crash recovery; orchestrator writes authoritative update in Step 4c).

**Output a structured completion summary as your final message.** The orchestrator parses this to update plan.yaml (Step 4c). The worktree is cleaned up after return, so this output is the only way results survive.

```
Issue [ref] [complete|blocked|escalated|failed]:
  status: [done|blocked|escalated|failed]
  execution_mode: [debate|solo|promoted]
  phase_status:
    [phase]: [done|failed|blocked|pending|skipped]
    ...
  halted_at: [phase, if early exit]
  halt_reason: [reason, if early exit]
  debates: [debate-id-1, ...]
  executions: [exec-id-1, ...]
  files: [file1, ...]
  worktree: [absolute path to worktree with uncommitted changes, or "none"]
```

- `worktree` path tells the orchestrator where uncommitted code lives for commit/PR in Step 4c
- `execution_mode`: `debate` (adversarial), `solo` (generative + guards), `promoted` (solo escalated to debate)
- `executions`: execution log IDs from `.ratchet/executions/` (empty for debate-only pipelines)
- Include `halted_at`/`halt_reason` only on early exit (escalation, guard failure, regression budget exhausted)

This summary MUST be the last thing you output.
