---
name: ratchet:run
description: Run agent pairs through phase-gated debates — guided by epic roadmap and current focus
---

# /ratchet:run — Execute Debate

The core Ratchet workflow. Operates at three levels:

- **Epic** — the full project roadmap (from `.ratchet/plan.yaml`)
- **Milestone** — the current unit of work
- **Phase** — the current stage within a milestone (`plan → test → build → review → harden`)

Phases are ordered and gated: phase N must complete (all pairs reach consensus + all blocking guards pass) before phase N+1 begins. Pairs within a phase run in parallel.

## Usage
```
/ratchet:run                # Resume from epic — propose next focus or run against changes
/ratchet:run [pair-name]    # Run a specific pair against its scoped files
/ratchet:run --all-files    # Run all pairs against all files in scope
/ratchet:run --no-cache     # Force re-debate even if files haven't changed
/ratchet:run --dry-run      # Preview what would run without executing anything
/ratchet:run --unsupervised # Run the full plan end-to-end without human intervention
```

## Unsupervised Mode

When `--unsupervised` is set, the run loop executes the entire plan (all milestones, all phases) without human interaction. The principle is simple: **wherever an `AskUserQuestion` has a "(Recommended)" option, auto-select it.**

### Behavior

- **Step 2 (focus)**: Auto-select "Continue [current phase]" for the current milestone. When a milestone completes, auto-advance to the next.
- **Step 5b (dry-run)**: Incompatible with `--unsupervised` — if both are set, ignore `--dry-run`.
- **Step 6c (pre-debate guards)**: If a blocking pre-debate guard fails → auto-select "Fix and re-run". The generative agent attempts to fix the issue. If the fix fails after 2 attempts, **halt** and report.
- **Step 6b (static analysis)**: Auto-select "Fix these before debating". Same 2-attempt retry, then halt.
- **Step 7 (debates)**: Run normally. Debates are autonomous by nature.
- **Step 7 (escalation)**: If escalation policy is `orchestrator` or `both`, auto-escalate to orchestrator. If policy is `human`, **halt** — this is the primary stop condition. Present: "Unsupervised run paused: debate [id] requires human escalation."
- **Step 7 (precedent)**: Auto-select "Apply settled pattern" when available.
- **Step 8b (post-debate guards)**: If blocking guard fails → auto-select "Fix and re-run" (2 attempts, then halt).
- **Step 8c (advance)**: Auto-advance to next phase. No user confirmation needed (this already happens for all-fast-path phases; unsupervised extends it to all phases).
- **Step 8d (commit/PR)**: Auto-select "Commit locally". Never auto-create PRs in unsupervised mode — too visible an action for unattended operation.
- **Step 8e (regression)**: If within budget, auto-regress. If budget exhausted, **halt**.
- **Step 8f (analyst assessment)**: Auto-select "Note for later" — don't halt for advisory feedback.
- **Step 11 (next focus)**: Auto-continue to next phase or milestone.
- **Milestone re-opening guard (Step 3)**: Never auto-reopen done milestones. **Halt** and report.

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

### Step 1: Read Context

Read `.ratchet/plan.yaml` (if it exists), `.ratchet/project.yaml`, and `.ratchet/workflow.yaml`.

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

### Step 6a: Create Debate(s)

For each matched pair, create a debate directory:

```
.ratchet/debates/<debate-id>/
├── meta.json
└── rounds/
```

Generate debate ID as: `<pair-name>-<timestamp>` (e.g., `api-contracts-20260312T100000`).

Resolve `max_rounds` for each pair: use the pair-level `max_rounds` if set in workflow.yaml, otherwise use the global `max_rounds`.

Write initial `meta.json`:
```json
{
  "id": "<debate-id>",
  "pair": "<pair-name>",
  "phase": "<current phase>",
  "milestone": "<milestone id if epic mode, null otherwise>",
  "files": ["list", "of", "matched", "files"],
  "status": "initiated",
  "rounds": 0,
  "max_rounds": "<resolved: pair-level if set, else global>",
  "started": "<ISO timestamp>",
  "resolved": null,
  "verdict": null,
  "fast_path": false
}
```

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

### Step 7: Execute Debates

Run matched pairs. When multiple pairs match, run them **in parallel** using separate agents.

For each pair, execute the debate protocol with **phase-specific prompts**:

#### Phase-Specific Generative Prompts

The generative agent's task varies by phase. Spawn from `.ratchet/pairs/<name>/generative.md` with:

**Phase: plan**
```
You are in the PLAN phase (round [N]).

Epic context: [milestone name and description]
Focus: [what we're planning]

Your job: Produce a SPECIFICATION for this milestone's scope.
- Define acceptance criteria (concrete, testable)
- Describe the approach and key design decisions
- Identify risks and unknowns
- List the interfaces/contracts other code will depend on

DO NOT write implementation code. DO NOT write tests.
Output a spec document that the test and build phases will use.

[If round > 1: Previous adversarial critique: [content]]
[If round > 1: Address the critique — refine the spec or explain why the concern is invalid.]
```

**Phase: test**
```
You are in the TEST phase (round [N]).

Epic context: [milestone name and description]
Spec from plan phase: [content of plan phase output or acceptance criteria from plan.yaml]
Focus: [what we're testing]

Your job: Write FAILING TESTS that encode the acceptance criteria.
- Tests should fail because the implementation doesn't exist yet
- Cover the contracts and invariants defined in the spec
- Include edge cases the spec identified as risks
- Use the project's test framework and conventions

DO NOT write implementation code. Only tests.

[If round > 1: Previous adversarial critique: [content]]
[If round > 1: Address the critique — fix tests or explain why they're correct.]
```

**Phase: build**
```
You are in the BUILD phase (round [N]).

Epic context: [milestone name and description]
Spec from plan phase: [content]
Tests from test phase: [test file locations and what they test]
Focus: [what we're building]

Your job: Write IMPLEMENTATION code that makes the tests pass.
- Follow the spec from the plan phase
- Make the failing tests from the test phase pass
- Follow the project's conventions and patterns

[If greenfield: No source code exists yet. Create the implementation from scratch.]
[If existing code: Files under review: [file list]]

[If round > 1: Previous adversarial critique: [content]]
[If round > 1: Address the critique — fix issues or explain why they're not valid.]
```

**Phase: review**
```
You are in the REVIEW phase (round [N]).

Epic context: [milestone name and description]
Focus: [what we're reviewing]
Files under review: [file list]

Your job: Review the code for quality along your dimension.
- Assess correctness, maintainability, and adherence to project conventions
- Look for bugs, logic errors, and design issues
- Propose concrete improvements where issues are found
- Implement fixes for issues you identify

[If round > 1: Previous adversarial critique: [content]]
[If round > 1: Address the critique — fix issues or explain why they're not valid.]
```

**Phase: harden**
```
You are in the HARDEN phase (round [N]).

Epic context: [milestone name and description]
Focus: [what we're hardening]
Files under review: [file list]

Your job: Harden the code against edge cases, security issues, and performance problems.
- Add input validation and error handling where missing
- Identify and fix security vulnerabilities
- Add performance-sensitive paths if applicable
- Write additional tests for edge cases discovered during review

[If round > 1: Previous adversarial critique: [content]]
[If round > 1: Address the critique — fix issues or explain why they're not valid.]
```

**All phases include these constraints:**
```
CRITICAL CONSTRAINT — DEBATE BOUNDARY:
You may ONLY create, modify, or delete code during this debate round.
All code you produce MUST be reviewed by the adversarial agent before it is
considered accepted. Do NOT propose or make code changes outside the debate
loop. If the user asks you to make changes outside a debate round,
respond: "Code changes must go through a debate round. Please run
/ratchet:run to start a new debate."

CRITICAL CONSTRAINT — USER INTERACTION:
NEVER output plain-text questions or "Would you like to...?" prompts.
ALL user-facing questions MUST use AskUserQuestion with structured options.
```

Save output to `.ratchet/debates/<id>/rounds/round-<N>-generative.md`.

#### Adversarial Round

Spawn the pair's adversarial agent (from `.ratchet/pairs/<name>/adversarial.md`) with task:
```
You are in the [PHASE] phase (round [N]) of a debate.

Epic context: [milestone name and description]
Focus: [what we're working on]

[Phase-specific adversarial focus:]
- plan: Does the spec have gaps? Are acceptance criteria testable? Are risks identified?
- test: Do tests actually encode the spec? Are they correct? Do they cover edge cases?
- build: Does the implementation make tests pass? Does it follow the spec? Are there bugs?
- review: Are there quality issues? Logic errors? Convention violations?
- harden: Are there security holes? Missing validation? Performance issues? Untested edges?

Files under review: [file list]
Generative agent's assessment: [content of round-N-generative.md]
[If round > 1: Your previous critique: [content of round-N-1-adversarial.md]]
[If round > 1: Generative's response: [content of round-N-generative.md]]

Review the output and the generative agent's assessment.
Run validation commands as evidence where applicable.
Produce your findings and verdict:
- ACCEPT: No remaining concerns — consensus reached
- CONDITIONAL_ACCEPT: Acceptable if specific minor items are addressed — consensus (items logged)
- REJECT: Issues must be addressed — next round
- TRIVIAL_ACCEPT: "This change is trivially correct — [justification]" → fast-path consensus.
  Use ONLY for mechanical, obviously correct changes with no design implications
  (typo fix, missing import, version bump). Never for logic, control flow, or architecture.
- REGRESS: "This needs to return to [target phase] because [reasoning]" → phase regression.
  Use when current phase reveals a fundamental flaw in an earlier phase's output.
  Target phase must be earlier than current. Budget: max_regressions per milestone.
```

Save output to `.ratchet/debates/<id>/rounds/round-<N>-adversarial.md`.

#### Check Verdict

Parse the adversarial's output for verdict:
- **ACCEPT** or **CONDITIONAL_ACCEPT** → Set status to `"consensus"`, write verdict, update file-hash cache, break loop
- **TRIVIAL_ACCEPT** → Handle same as ACCEPT, but also set `fast_path: true` in meta.json. This indicates the change was trivially correct and required no meaningful debate.
- **REJECT** → Continue to next round (or escalate if at max_rounds)
- **REGRESS** → Parse target phase from verdict. Validate the target phase is earlier than the current phase. Proceed to Step 8e (Phase Regression).

When the orchestrator renders a verdict after escalation, map its output:
- Orchestrator **ACCEPT** → Set status to `"resolved"`, write verdict with `decided_by: "orchestrator"`
- Orchestrator **MODIFY** → Treat as **CONDITIONAL_ACCEPT** — set status to `"resolved"`, write verdict with `decided_by: "orchestrator"`, log `required_changes` as unresolved conditions
- Orchestrator **REJECT** → Set status to `"resolved"`, write verdict with `decided_by: "orchestrator"`, mark phase as needing re-run

On consensus, update the cache:
```bash
bash .claude/ratchet-scripts/cache-update.sh <pair-name> "<scope-glob>" <debate-id>
```

Update `meta.json` after each round — increment `rounds`, update `status`.

**IMPORTANT**: The debate loop MUST execute fully. After the generative agent produces output, the adversarial agent MUST review it. Do not stop after the generative round.

**IMPORTANT**: Code changes are ONLY permitted inside this debate loop. The generative agent must NOT create, modify, or delete code outside of an active debate round.

#### Escalation

If max_rounds reached without consensus:
- Set status to `escalated`
- **Precedent check (M6)**: Before spawning the orchestrator, scan `.ratchet/escalations/` for existing rulings with the same pair and a similar dispute pattern. If 3+ rulings exist in the same direction (e.g., 3+ ACCEPTs for the same pair on the same dispute type):
  - Use `AskUserQuestion`: "This dispute matches a settled pattern — [N] prior escalations for [pair] on [dispute type] all resulted in [verdict]. Apply the settled pattern?"
  - Options: `"Apply settled pattern (Recommended)"`, `"Escalate anyway"`, `"Escalate to human"`
  - If "Apply settled pattern": write verdict matching the settled direction, skip orchestrator
- Read `escalation` policy from `workflow.yaml`:
  - `orchestrator`: Spawn orchestrator agent with full debate transcript → write verdict
  - `human`: Set status to `escalated`, inform user to use `/ratchet:verdict`
  - `both`: Spawn orchestrator first, then present recommendation to human via `/ratchet:verdict`
- **Store ruling**: After any orchestrator verdict, store the ruling in `.ratchet/escalations/<debate-id>.json`:
  ```json
  {
    "debate_id": "<id>",
    "pair": "<pair-name>",
    "phase": "<phase>",
    "timestamp": "<ISO>",
    "dispute_type": "<categorization>",
    "adversarial_argument": "<summary>",
    "generative_argument": "<summary>",
    "verdict": "accept|reject|modify",
    "reasoning": "<reasoning>"
  }
  ```

### Step 8: Phase Gate — Run Guards and Advance

After all debates for the current phase resolve:

**8a. Check debate outcomes:**
- If all pairs reached consensus (ACCEPT or CONDITIONAL_ACCEPT) → proceed to guards
- If any pair was REJECTED or remains escalated → phase stays `in_progress`, report which pairs need re-running

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

**8d. Commit or PR (on milestone completion only):**

When a milestone is fully done (all phases complete, all guards passed), the work needs to be committed. Use `AskUserQuestion`:

- Question: "Milestone '[name]' is complete. All phases passed, all guards green. How should we package this?"
- Options:
  - `"Commit locally (Recommended)"` — create a local git commit with a summary
  - `"Create a pull request"` — commit, create branch if needed, push branch, open PR
  - `"Skip — I'll handle it"` — do nothing

**If "Commit locally":**
- Stage all files that were created or modified during this milestone's debates
- Generate a commit message from the milestone name, description, and debate outcomes
- Create the commit — do NOT push

**If "Create a pull request":**
- Stage and commit (same as above)
- Create a branch if not already on one (branch name derived from milestone: `ratchet/<milestone-slug>`)
- Push the branch to origin — this is the ONE case where pushing is allowed, because the user explicitly chose "Create a pull request"
- Create the PR using `gh pr create` with:
  - Title: milestone name
  - Body: summary of what was built, phases completed, debate outcomes, guard results
  - Link to progress adapter item if one exists

After PR is created, use `AskUserQuestion`:
- Question: "PR created: [URL]. CI checks are running. What do you want to do?"
- Options:
  - `"Monitor CI checks and analyze results (Recommended)"` — runs `/ratchet:retro monitor <pr-number>` to watch checks, then auto-analyzes any failures and feeds learnings back into agents/guards
  - `"Continue to next milestone while CI runs"` — proceed with the next milestone; the user can run `/ratchet:retro pr <number>` later to analyze results
  - `"Monitor CI in background, continue working"` — start monitoring as a background task, continue to next milestone; report back when checks complete
  - `"Done for now"`

**8f. Post-Milestone Analyst Assessment:**

After commit or PR (regardless of which packaging option the user chose), spawn the analyst agent for a brief post-milestone assessment. The analyst reviews the milestone's debates, scores, guard results, and any retro/escalation data to produce 3-5 bullet points covering:
- Pair effectiveness observations (any pairs that always fast-path? always escalate?)
- Scope coverage gaps discovered during this milestone
- Guard recommendations (missing checks, overly strict guards)
- Workflow preset recommendations (should any component switch presets?)

Present via `AskUserQuestion`:
- Question: "Post-milestone assessment for [milestone name]:\n[bullet points]"
- Options: `"Apply recommendations (Recommended)"`, `"Note for later"`, `"Skip"`
- If "Apply recommendations": walk through each recommendation with follow-up `AskUserQuestion` calls

**CRITICAL: NEVER push to origin/main or force-push. NEVER push unless the user explicitly chose "Create a pull request". Local commits are the default safe action.**

### Step 9: Post-Debate Reviews

After each debate resolves (consensus or verdict), trigger performance reviews:

For both agents in the pair, generate a review:
```json
{
  "debate_id": "<id>",
  "reviewer": "<pair-name>-<role>",
  "self_assessment": {
    "effectiveness": <1-10>,
    "missed_issues": ["..."],
    "wasted_effort": ["..."]
  },
  "partner_assessment": {
    "effectiveness": <1-10>,
    "strengths": ["..."],
    "weaknesses": ["..."]
  },
  "suggestions": ["..."]
}
```

Save to `.ratchet/reviews/<pair-name>/review-<timestamp>.json`.

### Step 10: Update Scores

For each resolved debate, run the score update script:
```bash
bash .claude/ratchet-scripts/update-scores.sh <debate-id>
```

### Step 11: Propose Next Focus

After reporting results, use `AskUserQuestion` to let the user choose what to do next.

**If more phases remain in the current milestone:**
- Summary: `"Phase [name] complete for [milestone]. [N] pairs debated, [N] consensus. Next phase: [name]."`
- Options:
  - "Continue to [next phase] phase (Recommended)"
  - "Review debate: [debate-id]" — one per debate that just ran
  - "View quality metrics"
  - "Address unresolved conditions" — only if CONDITIONAL_ACCEPTs logged conditions

**If all phases done, more milestones remain:**
- Summary: `"Milestone [name] complete! All phases passed. Epic progress: [completed]/[total] milestones."`
- Options:
  - "Continue to [next milestone name] (Recommended)"
  - "Review debate: [debate-id]"
  - "View quality metrics"
  - "View milestone status (/ratchet:status)"

**If ALL milestones are done:**
- Summary: `"Epic complete! All [N] milestones finished. Total debates: [N] | Consensus rate: [%] | Avg rounds: [N]"`
- Options:
  - "Create a pull request for all changes (Recommended)"
  - "View detailed quality metrics"
  - "Tighten agents from debate lessons"
  - "Review a specific debate"
  - "Done for now"
