# Unsupervised Mode

> This file is the extracted Unsupervised Mode section from `skills/run/SKILL.md`.
> It is loaded on demand when `--unsupervised` is passed to `/ratchet:run`.
> For the main orchestrator flow, see `skills/run/SKILL.md`.

---

When `--unsupervised` is set, the run loop executes the entire plan (all milestones, all phases) without human interaction. The principle is simple: **wherever an `AskUserQuestion` has a "(Recommended)" option, auto-select it.**

> **`--go` flag**: `--go` is shorthand for `--unsupervised --auto-pr`. It is equivalent in every way — the same behavior, halt conditions, and self-continuation rules apply.

## Behavior

- **Step 1a (workspace)**: If at workspace root with no workspace specified, **halt** — unsupervised mode requires an explicit workspace target (`/ratchet:run --unsupervised monitor`). Auto-selecting a workspace is too risky.
- **Step 1c (orphan detection)**: Auto-select based on finding age: Abandon for stale items (>24h or unknown age), Resume for recent items (<4h), Ignore for ambiguous (4-24h). See Step 1c in SKILL.md for the full decision matrix.
- **Step 2 (focus)**: Auto-select "Run all ready issues sequentially" for the current milestone. When a milestone completes, auto-advance to the next. In DAG mode, auto-launch all ready milestones in parallel.
- **Step 3c (auto-merge prerequisite PRs)**: Auto-merge prerequisite milestone PRs if all CI checks pass. If any PR has failing checks, skip that PR and fall through to the stacked branch fallback (Step 4b). If `--no-auto-merge` is set, skip the entire auto-merge step.
- **Step 4 (issue pipelines)**: Execute all ready issues sequentially inline. Each issue pipeline runs in an isolated worktree, spawning only debate-runner agents.
- **Step 5-dry (dry-run)**: Dry-run takes precedence — produce the preview (including token and cost estimates), log the estimates to stdout, and stop. No agents are spawned. The `AskUserQuestion` confirmation is skipped (estimates are informational only).
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

## Self-Continuation via Agent Tool

The unsupervised loop is driven by `plan.yaml` as a state machine and the Agent tool as the continuation mechanism.

At **Step 10**, if `--unsupervised` is set and no halt condition was triggered:
1. Write all state to `plan.yaml` (phase_status, current_focus, regressions, etc.)
2. Spawn a new agent via the Agent tool with the task: `/ratchet:run --unsupervised`
3. The spawned agent reads `plan.yaml`, finds the next pending phase/milestone, and continues from Step 1

This creates a chain: each agent handles one milestone, persists state, and spawns the next. `plan.yaml` is the continuity mechanism — if the session crashes at any point, a manual `/ratchet:run --unsupervised` picks up from the last persisted state.

**Context clearing at milestone boundaries**: Self-continuation MUST happen at milestone boundaries — the spawned agent starts with a fresh context and re-reads all state from disk. This prevents context drift from auto-compaction summaries corrupting downstream work. Within a milestone, phases run in the same context (cross-phase continuity has value). Between milestones, fresh context forces the agent to rely on persisted state (plan.yaml, debate transcripts, scores) rather than compressed memories.

**Why Agent tool, not a shell loop**: Agents start with fresh context, can read/write project files, and the state machine (`plan.yaml`) handles crash recovery naturally. No external tooling or hook configuration required.

## Halt Conditions

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

## Combining with Other Flags

- `--go`: Shorthand for `--unsupervised --auto-pr` (identical behavior)
- `--unsupervised --auto-pr`: Auto-create PRs at milestone boundaries (human pre-approves by passing this flag)
- `--unsupervised --no-cache`: Force re-debate all files, unsupervised
- `--unsupervised --all-files`: Run all pairs against all files, unsupervised
- `--unsupervised --dry-run`: Dry-run takes precedence (preview only, no execution)
- `--go --no-cache`: Combines `--go` with `--no-cache` (force re-debate, unsupervised, auto-PR)
- `--go --all-files`: Combines `--go` with `--all-files` (all pairs, unsupervised, auto-PR)
- `--unsupervised --no-auto-merge`: Skip auto-merging prerequisite milestone PRs. Dependent milestones will use the stacked branch fallback (branching from prerequisite branches instead of main).
- `--quick "<description>"`: Compatible with `--unsupervised`. In unsupervised mode, if component auto-detection fails, Mode Q halts with error (no interactive fallback). If a blocking guard fails, Mode Q halts with status `failed` (no retry). Combinable with `--auto-pr` to auto-create a branch and PR from the quick-fix commit.
