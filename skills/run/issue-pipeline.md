# Issue Pipeline Specification

> Extracted Step 5 from `skills/run/SKILL.md`. Loaded on demand when orchestrator executes an issue pipeline. For orchestrator flow (Steps 1-4, 6-10), see `skills/run/SKILL.md`.

---

### Step 5: Issue Pipeline (executed per-issue in isolated worktrees)

Phase-gated loop for a single issue, spawned by Step 4b as parallel Agent invocation with own git worktree.

**Execution context:**
- **Worktree mode** by dependency layer: **`isolation: worktree`** (Layer 0) — fresh worktree from `origin/main`; **Pre-created worktree** (Layer 1+) — orchestrator creates from dependency's `branch` field in plan.yaml (Step 4b "Branch base resolution"), inheriting committed changes.
- **Strategy** from component's `strategy` field: `debate` (default) spawns debate-runner agents at Step 5e; `solo` spawns generative agent directly at Step 5e-solo.
- Returns structured completion summary (Step 5h) — does NOT write plan.yaml (parent orchestrator handles in Step 4c).

Phases progress sequentially (plan → test → build → review → harden, per pipeline preset), then control returns to Step 4c.

#### Guard Execution Pattern

All guard invocations (Steps 5c, 5e-solo, 5f):

```bash
test -f .claude/ratchet-scripts/run-guards.sh \
  || { echo "Error: run-guards.sh not found. Run install.sh to restore Ratchet scripts." >&2; exit 1; }

# Without singleton resources:
bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>

# With singleton resources (e.g., requires: [postgres]):
flock .ratchet/locks/postgres.lock bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>
```

For multiple singleton resources, acquire locks in alphabetical order to prevent deadlocks:
```bash
flock .ratchet/locks/db.lock flock .ratchet/locks/playwright.lock bash -c '<guard-command>'

Each debate-runner receives:
```
Run debate for pair [pair-name] in phase [phase].

ROLE BOUNDARY — You are a debate-runner, NOT a solver:
  You orchestrate debate rounds between generative and adversarial agents.
  You may use Write/Edit ONLY for debate artifacts in .ratchet/debates/,
  .ratchet/escalations/, and .ratchet/reviews/. You NEVER modify source
  code, tests, or application config. Your tools are: Read, Glob, Write, Edit,
  Agent, AskUserQuestion, TodoWrite — with Write/Edit gated to .ratchet/ paths only.

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
  Caveman:
    generative: [off|lite|full|ultra]
    adversarial: [off|lite|full|ultra]
    tiebreaker: [off|lite|full|ultra]
    debate_runner: [off|lite|full|ultra]
```
```

#### Guilt Verification Pattern

When blocking guard fails, failure is **guilty until proven innocent** — caused by current issue's changes unless proven otherwise. Verify on clean base before dismissing:

```bash
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
git stash
git checkout "$GUILT_BASE_REF" 2>/dev/null
bash .claude/ratchet-scripts/run-guards.sh <milestone-id> <phase> <guard-name> "<guard-command>" <blocking>
GUARD_EXIT=$?
git checkout "$CURRENT_BRANCH" 2>/dev/null
git stash pop 2>/dev/null
# Only if GUARD_EXIT != 0 (also fails on clean base) can failure be considered pre-existing
```

#### 5-pre. Worktree Mode Detection and Dependency Validation

**1. Detect worktree mode** from prompt context fields set by Step 4b: `Worktree mode: isolation` (or absent) → **isolation mode** (Layer 0); `Worktree mode: pre-created` with `Dependency branch`/`Dependency files` → **pre-created mode** (Layer 1+). Log: `Worktree mode: [isolation|pre-created]` (if pre-created, include base branch and dep refs).

**2. Validate dependency changes (pre-created mode only):** For each file in `dependency_files` array, verify it exists and is modified relative to `origin/main`. Outcomes:
- **All files present and modified** → proceed. Log: `"Dependency validation passed: [N] files verified"`
- **Files missing or unmodified** → critical error. **Supervised**: `AskUserQuestion` with options: `"Re-create worktree"`, `"Override and proceed"`, `"Cancel this issue"`. **Unsupervised**: halt with status `blocked`.

**3. Set git baseline for guilt checks:** **Isolation mode**: `GUILT_BASE_REF="origin/main"`. **Pre-created mode**: `GUILT_BASE_REF=$(git rev-parse HEAD)` (captured before any issue work). Replaces the implicit `origin/main` assumption in guard failure verification throughout the pipeline.

#### 5a. Determine Current Phase and Match Pairs

**[TodoWrite — Phase Starts]**: Set this phase item to `"in_progress"` with pair names.

1. Read issue's `phase_status` — find first `pending` or `in_progress` phase
2. Determine applicable phases from component's `pipeline` preset:
   - `full`: plan → test → build → review → harden
   - `standard`: plan → build → review → harden
   - `review`: review only
   - `hotfix`: build → review
   - `secure`: review → harden
3. Resolve component's `strategy` field (`debate` or `solo`, default: `debate`)
4. Match pairs from issue's `pairs` list for current phase; skip disabled pairs
5. If pair has `scope: "auto"`, resolve to parent component's scope glob

#### 5b. File-Hash Cache Check

```bash
bash .claude/ratchet-scripts/cache-check.sh <pair-name> "<scope-glob>"
```

Exit 0 → skip pair (no changes since last consensus). Exit 1 → proceed to execution. `--no-cache` forces re-execution.

#### Shared Resources

Guards can declare `requires: [resource-name]` referencing shared resources in workflow.yaml — infrastructure dependencies needing provisioning and optionally singleton access.

**Lifecycle:**
1. **Provision** — run resource's `start` command (idempotent). Track in `.ratchet/locks/resources.json`.
2. **Lock + Run** — if `singleton: true`, wrap guard command with `flock` (kernel file locking in `.ratchet/locks/`). Auto-releases on exit — no stale locks.
3. **Teardown** (Step 9) — run `stop` commands, remove `.ratchet/locks/`.

#### 5c. Pre-Execution Guards

Run guards where `timing: "pre-execution"` (or deprecated `"pre-debate"`) for current phase. Guards without `timing` default to `post-execution`. If guard has `requires`: provision resources; for singleton, wrap with `flock`. Invoke using **Guard Execution Pattern** above.

- **Blocking guard fails** → verify using **Guilt Verification Pattern**. Then `AskUserQuestion`: `"Fix and re-run"`, `"Override and proceed"`, `"Cancel — skip this phase"`. If fix or cancel: skip execution entirely.
- **Advisory guard fails** → log and pass output as context to execution (guilty until proven innocent). Continue.

#### 5d. Prepare Execution Context

For each matched pair:
1. **Resolve `max_rounds`**: pair-level if set, otherwise global
2. **Gather escalation precedents**: scan `.ratchet/escalations/`
3. **Gather phase context**: plan output (if phase > plan), test locations (if phase > test), unresolved CONDITIONAL_ACCEPT conditions
4. **Resolve models**: pair-level overrides over global. Debate needs `generative`, `adversarial`, `tiebreaker`. Solo needs only `generative`.
5. **Resolve `progress_ref`** (debate only): use `ref` if numeric, else `null`. Pass to debate-runner for `meta.json` at creation (before round files trigger publish hook).
6. **Resolve caveman config** from values computed in Step 1b: if `caveman_enabled` is `true`, pass per-role intensities (`caveman_generative`, `caveman_adversarial`, `caveman_tiebreaker`, `caveman_debate_runner`); if `false` or absent, pass all roles as `off`. The `orchestrator` intensity is NOT passed to debate-runner — governs the run skill's own behavior.

#### 5e. Execute Phase (Strategy Branch)

**`strategy: "debate"`** → Step 5e-debate. **`strategy: "solo"`** → Step 5e-solo. Both paths converge at Step 5f.

---

#### 5e-solo. Solo Execution (strategy: "solo")

Single generative agent executes the phase; post-execution guards serve as quality gate. Same worktree isolation constraints as debate mode.

**1. Create execution log** in `.ratchet/executions/<pair-name>-<timestamp>.yaml` with fields: `id`, `mode: solo`, `component`, `issue`, `started`, `resolved: null`, `guard_results: []`, `promoted: false`, `debate_id: null`, `files_modified: []`.

**2. Spawn generative agent** with resolved `generative` model. Tools: Read, Grep, Glob, Bash, Write, Edit. Receives phase, pair definition, worktree path, milestone, issue, files in scope, prior phase outputs.

**3. Process result**: capture `files_modified` (`git diff --name-only` + staged + untracked). Update execution log with files and `resolved` timestamp.

**4. Run post-execution guards** using **Guard Execution Pattern**. Record each result in log.

**5. Handle guard results:**

| Outcome | Action |
|---------|--------|
| All guards pass | Complete. Proceed to Step 5f (phase advance). |
| Blocking guard fails + `promote_on_guard_failure: true` | Log promotion. Fall through to Step 5e-debate (review-only debate on solo output). |
| Blocking guard fails + no promotion | Return failure → Step 5h. In supervised mode: `AskUserQuestion` first. |
| Advisory guard fails | Log and continue. |

**[TodoWrite — Solo Result]**: Update phase item: `"SOLO PASS"`, `"SOLO PROMOTED"`, or `"SOLO FAILED"`.

---

#### 5e-debate. Run Debates (strategy: "debate")

Spawn a **debate-runner** agent per matched pair (parallel when multiple). Model: resolved `debate_runner` (default `sonnet`).

**Tool boundaries:**
- Debate-runner: Write/Edit only for `.ratchet/debates/`, `.ratchet/escalations/`, `.ratchet/reviews/`. Never modifies source code.
- Generative agent: Read, Grep, Glob, Bash, Write, Edit (the ONLY role that writes code)
- Adversarial agent: Read, Grep, Glob, Bash — no Write/Edit (read-only)
- Tiebreaker agent: Read, Grep, Glob, Bash — no Write/Edit (read-only)

Each debate-runner receives: pair definitions, worktree path, phase, milestone, issue ref, files in scope, max rounds, escalation policy/precedents, prior phase outputs, `progress_ref`, resolved models (generative/adversarial/tiebreaker). The `progress_ref` MUST be written into `meta.json` at debate creation before any round files trigger the publish hook.

**If debate-runner cannot be spawned:** **Supervised** → `AskUserQuestion`: `"Wait and retry"`, `"Skip this phase"`, `"Escalate for manual resolution"`. **Unsupervised** → halt with status `blocked` ("debate-runner unavailable — quality gate cannot be enforced"). Retry up to 3 times with exponential backoff (5s, 10s, 20s).

**No fallback validation** (debate mode): the debate-runner is the ONLY acceptable quality path. Guards alone are insufficient substitutes. For solo mode, guards ARE the quality gate by design.

#### Handle Debate Results (strategy: "debate" or promoted from solo)

- **ACCEPT / TRIVIAL_ACCEPT**: update file-hash cache (`cache-update.sh`), update issue's `files` and `debates`. Sync plan tracking issue. TRIVIAL_ACCEPT: note `fast_path: true`.
- **CONDITIONAL_ACCEPT**: conditions resolved internally by debate-runner. Treat as consensus — update cache, files, debates. Log conditions. Sync plan tracking issue.
- **Escalated (conditions_unresolved)**: follow escalation flow below.
- **Escalated** (human escalation): update issue status, output early-exit summary (Step 5h), return.
- **REGRESS**: handle via Step 5g.

**[TodoWrite — Debate Results]**: Update phase item with verdict (e.g., `"Build phase — ACCEPT R1"`). Do not change phase status yet.

**IMPORTANT**: Do NOT run debates yourself. For debate strategy, debate-runner is the only path; for solo, generative agent is spawned directly. If agent cannot be spawned, halt and escalate. After processing debate results, proceed through ALL of Step 5f — do NOT skip to next phase without packaging.

#### 5f. Phase Gate — Guards and Advance

**Check results:** **Debate mode**: all consensus → guards; any escalated → early exit (Step 5h); any regress → Step 5g. **Solo mode**: all guards passed (already ran in 5e-solo) → phase advance; promoted → process debate results; failed → early exit (Step 5h).

**Run post-execution guards** (debate mode only — solo already ran them in 5e-solo). Invoke using **Guard Execution Pattern**. Results stored in `.ratchet/guards/<milestone-id>/<issue-ref>/<phase>/<guard-name>.json`. Blocking guard fails → `AskUserQuestion`: fix/override/view. Advisory guard fails → log and continue.

**Advance phase:** Mark current phase `done` in issue's `phase_status`. Auto-advance on fast-path (all TRIVIAL_ACCEPT) without confirmation. Next phase exists → set to `in_progress`, loop to Step 5a. All phases done → issue complete. Sync plan tracking issue after phase state change.

**[TodoWrite — Phase Complete]**: Set phase item to `"completed"` retaining verdict info.

**Commit/PR is the orchestrator's responsibility, not the issue agent's.** The issue agent produces code in the worktree but does NOT commit, push, or create PRs. The worktree branch with uncommitted changes is the deliverable — orchestrator handles packaging in Step 4c. This prevents depth-3+ agents from silently dropping the commit step.

#### 5g. Phase Regression

> Solo mode: regression does not apply (no adversarial agent). If promoted-to-debate REGRESSes, handle normally below.

When adversarial issues REGRESS targeting an earlier phase: (1) read `max_regressions` from config (default: 2), tracked per-milestone; (2) budget exhausted → `AskUserQuestion`: allow/reject/escalate; (3) within budget → increment counter, reset target phase and later to `pending`, set target to `in_progress`, preserve debate history, sync plan tracking issue, loop to Step 5a.

#### 5h. Issue Complete

Set issue status to `done` in local plan.yaml (worktree copy — orchestrator writes authoritative update in Step 4c). **Output structured completion summary as final message.** Orchestrator parses this for Step 4c. Worktree is cleaned up after agent return, so this output is the only way results survive.

```
Issue [ref] [complete|blocked|escalated|failed]:
  status: [done|blocked|escalated|failed]
  execution_mode: [debate|solo|promoted]
  worktree_mode: [isolation|pre-created]
  phase_status:
    [phase]: [done|skipped|failed|blocked|pending]
    ...
  halted_at: [phase, if early exit]
  halt_reason: [reason, if early exit]
  debates: [debate-id-1, ...]
  executions: [exec-id-1, ...]
  files: [file1, file2, ...]
  worktree: [absolute path to worktree with uncommitted changes, or "none"]
  dependency_validation: [passed|failed|overridden|skipped]
```

Field semantics:
- `execution_mode`: `debate` (full adversarial), `solo` (generative + guards), `promoted` (solo escalated to debate after guard failure)
- `worktree_mode`: `isolation` (Layer 0, from `origin/main`), `pre-created` (Layer 1+, from dependency branch)
- `dependency_validation`: `passed` (pre-created, all files verified), `failed` (files missing/unmodified), `overridden` (failed but user overrode), `skipped` (isolation mode)
- `executions`: execution log IDs from `.ratchet/executions/`. Empty for debate-only pipelines.

Summary MUST be last output. Orchestrator reads plan.yaml for structured state, but this summary provides immediate human-readable feedback.
