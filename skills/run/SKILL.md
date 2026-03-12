---
name: ratchet:run
description: Run agent pairs against code — guided by epic roadmap and current focus
---

# /ratchet:run — Execute Debate

The core Ratchet workflow. Operates at two levels of scope:

- **Epic** — the full project roadmap (from `.ratchet/plan.yaml`). Where we've been, where we're going.
- **Focus** — the current iteration. What we're building or reviewing right now.

## Usage
```
/ratchet:run              # Resume from epic — propose next focus or run against changes
/ratchet:run [pair-name]  # Run a specific pair against its scoped files
/ratchet:run --all-files  # Run all pairs against all files in scope
/ratchet:run --no-cache   # Force re-debate even if files haven't changed
```

## Prerequisites
- `.ratchet/` must exist with valid config
- At least one pair must be registered and enabled

If `.ratchet/` does not exist, inform the user:
> "Ratchet is not initialized for this project. Run /ratchet:init to set up."

Then use `AskUserQuestion` with options: `"Initialize now (/ratchet:init)"`, `"Cancel"`.

If no enabled pairs exist in `config.yaml`, inform the user:
> "No active pairs found. Add a pair with /ratchet:pair."

Then use `AskUserQuestion` with options: `"Add a pair (/ratchet:pair)"`, `"Cancel"`.

## Execution Steps

### Step 1: Read Epic Context

Read `.ratchet/plan.yaml` (if it exists), `.ratchet/config.yaml`, and `.ratchet/project.yaml`.

Build a picture of:
- Which milestones are **completed** (status: done)
- Which milestone is **current** (status: in_progress)
- Which milestones are **pending**
- Any unresolved conditions from previous CONDITIONAL_ACCEPT verdicts

If no `plan.yaml` exists (e.g., added to existing project without init), skip epic tracking and fall through to file-based detection.

### Step 2: Determine Focus

There are four modes, checked in order:

#### Mode A: Explicit pair or --all-files
If the user specified a `[pair-name]` or `--all-files`, use that directly. Skip epic negotiation.

#### Mode B: Epic-guided (plan.yaml exists)
Use `AskUserQuestion` to let the user pick the focus. Put the epic status summary directly in the question text so the user sees it in context:

Question text (build from plan.yaml):
`"Epic: [project name] — [completed]/[total] milestones done. Next up: [next milestone name] — [description]. What should we focus on?"`

If there are unresolved conditions from previous CONDITIONAL_ACCEPTs, append:
`"(Unresolved from last run: [condition1], [condition2])"`

Options:
- "[Next milestone name] (Recommended)"
- "Address unresolved conditions from last run" — only if conditions exist
- "Review all existing code"
- (Include an "Other" option so the user can type a custom focus)

#### Mode C: Changed files (no plan.yaml, git repo exists)
```bash
git diff --name-only HEAD 2>/dev/null || git diff --name-only
git diff --name-only --cached
```
Match changed files to pairs by `scope` globs.

#### Mode D: Greenfield (no plan.yaml, no code)
Use `AskUserQuestion` to ask what to build first.

### Step 3: Set Focus in Plan

If using epic mode, update `plan.yaml`:
- Set the chosen milestone to `status: in_progress`
- Record `current_focus` with milestone id and timestamp

**MILESTONE RE-OPENING GUARD**: If the chosen milestone has `status: done`, do NOT silently re-open it. Instead, use `AskUserQuestion`:
- Question: "Milestone '[name]' is already marked done (completed [timestamp]). Re-opening it will reset its status. Are you sure?"
- Options: `"Re-open milestone"`, `"Pick a different milestone"`, `"Cancel"`
- Only set `status: in_progress` if the user explicitly confirms re-opening.

### Step 4: Match Pairs to Focus

Determine which pairs are relevant:
- **Epic mode**: use the pairs listed in the milestone's `pairs` field
- **Changed files mode**: match files to pairs by `scope` globs
- **All-files mode**: all enabled pairs
- **Greenfield mode**: all enabled pairs

Skip disabled pairs (`enabled: false`).

### Step 5: File-Hash Cache Check

For each matched pair, run the cache check script to see if scoped files changed since last consensus:

```bash
bash .claude/ratchet-scripts/cache-check.sh <pair-name> "<scope-glob>"
```

- Exit 0 → files unchanged, **skip this pair**: `Skipping [pair-name] — no changes since last consensus`
- Exit 1 → files changed or no cache, proceed to debate

Use `--no-cache` flag to skip this check and force re-debate of all pairs.

After a debate reaches consensus (Step 7), update the cache:

```bash
bash .claude/ratchet-scripts/cache-update.sh <pair-name> "<scope-glob>" <debate-id>
```

### Step 6a: Create Debate(s)

For each matched pair, create a debate directory:

```
.ratchet/debates/<debate-id>/
├── meta.json
└── rounds/
```

Generate debate ID as: `<pair-name>-<timestamp>` (e.g., `api-contracts-20260312T100000`).

Write initial `meta.json`:
```json
{
  "id": "<debate-id>",
  "pair": "<pair-name>",
  "milestone": "<milestone id if epic mode, null otherwise>",
  "files": ["list", "of", "matched", "files"],
  "status": "initiated",
  "rounds": 0,
  "max_rounds": <from config.yaml>,
  "started": "<ISO timestamp>",
  "resolved": null,
  "verdict": null
}
```

### Step 6b: Static Analysis Pre-Gate

Before starting debates, run the project's static analysis layer (layer 1 from the testing spec in project.yaml). This catches mechanical issues so adversarial agents can focus on semantic problems.

Read `.ratchet/project.yaml` for the static analysis commands (lint, type-check, format-check).

Run each configured command. If any fail:
1. Present the failures in the question text
2. Use `AskUserQuestion` to let the user decide:
   - Question: "Static analysis failed with [N] errors: [summary]. How should we proceed?"
   - Options: `"Fix these before debating"`, `"Proceed to debates anyway"`
   - If fix: stop here, let the user (or agent) fix lint/type errors, then re-run `/ratchet:run`
   - If proceed: continue to debates, but note in the debate context that static analysis had failures

If all pass (or none configured), proceed silently — no output needed.

This gate ensures adversarial agents spend their rounds on real quality issues (architecture, logic, edge cases, security) rather than catching lint violations or type errors that a formatter could fix.

### Step 7: Execute Debates

Run matched pairs. When multiple pairs match, run them **in parallel** using separate agents.

For each pair, execute the debate protocol:

#### Round Loop (up to max_rounds)

**a) Generative Round**

Spawn the pair's generative agent (from `.ratchet/pairs/<name>/generative.md`) with task:
```
You are in round [N] of a debate about code quality.

Epic context: [milestone name and description from plan.yaml]
Focus: [what we're building/reviewing this iteration]

[If greenfield/build mode:]
  No source code exists yet for your scope.
  Project profile: [from .ratchet/project.yaml]
  Your job: CREATE the code for this milestone's scope.
  Build it right from the start — the adversarial agent will review everything you produce.

[If review mode:]
  Files under review: [file list]
  Review the files in your scope. Assess quality along your dimension.

[If round > 1: Previous adversarial critique: [content of round-N-1-adversarial.md]]
[If round > 1: Address the adversarial's critique — fix issues or explain why they're not valid.]

Write your assessment. If you made code changes, describe them.

CRITICAL CONSTRAINT — DEBATE BOUNDARY:
You may ONLY create, modify, or delete code during this debate round.
All code you produce MUST be reviewed by the adversarial agent before it is
considered accepted. Do NOT propose or make code changes outside the debate
loop (e.g., in response to user conversation, post-debate discussion, or
between runs). If the user asks you to make changes outside a debate round,
respond: "Code changes must go through a debate round. Please run
/ratchet:run to start a new debate."
```

Save output to `.ratchet/debates/<id>/rounds/round-<N>-generative.md`.

**b) Adversarial Round**

Spawn the pair's adversarial agent (from `.ratchet/pairs/<name>/adversarial.md`) with task:
```
You are in round [N] of a debate about code quality.

Epic context: [milestone name and description from plan.yaml]
Focus: [what we're building/reviewing this iteration]

Files under review: [file list]
Generative agent's assessment: [content of round-N-generative.md]
[If round > 1: Your previous critique: [content of round-N-1-adversarial.md]]
[If round > 1: Generative's response: [content of round-N-generative.md]]

Review the code and the generative agent's assessment.
Run tests, linters, benchmarks as evidence (use commands from the project's testing spec).
Produce your findings and verdict: ACCEPT, CONDITIONAL_ACCEPT, or REJECT.
```

Save output to `.ratchet/debates/<id>/rounds/round-<N>-adversarial.md`.

**c) Check Verdict**

Parse the adversarial's output for verdict:
- **ACCEPT** or **CONDITIONAL_ACCEPT** → Set status to `"consensus"`, write verdict, update file-hash cache, break loop
- **REJECT** → Continue to next round (or escalate if at max_rounds)

When the orchestrator renders a verdict after escalation, map its output:
- Orchestrator **ACCEPT** → Set status to `"resolved"`, write verdict with `decided_by: "orchestrator"`
- Orchestrator **MODIFY** → Treat as **CONDITIONAL_ACCEPT** — set status to `"resolved"`, write verdict with `decided_by: "orchestrator"`, log `required_changes` as unresolved conditions
- Orchestrator **REJECT** → Set status to `"resolved"`, write verdict with `decided_by: "orchestrator"`, mark milestone as needing re-run

On consensus, update the cache so this pair is skipped next run (if files don't change):
```bash
bash .claude/ratchet-scripts/cache-update.sh <pair-name> "<scope-glob>" <debate-id>
```

Update `meta.json` after each round — increment `rounds`, update `status`.

**IMPORTANT**: The debate loop MUST execute fully. After the generative agent produces code (especially in greenfield mode), the adversarial agent MUST review it. Do not stop after the generative round.

**IMPORTANT**: Code changes are ONLY permitted inside this debate loop (Steps 7a-7c). The generative agent must NOT create, modify, or delete code outside of an active debate round — not in response to user chat, not between runs, not after a verdict. If a user requests code changes outside a debate, redirect them to `/ratchet:run`.

#### Escalation

If max_rounds reached without consensus:
- Set status to `escalated`
- Read `escalation` policy from config.yaml:
  - `orchestrator`: Spawn orchestrator agent with full debate transcript → write verdict
  - `human`: Set status to `escalated`, inform user to use `/ratchet:verdict`
  - `both`: Spawn orchestrator first, then present recommendation to human via `/ratchet:verdict`

### Step 8: Update Epic

After all debates for this focus resolve:
- If all pairs reached a terminal state (`"consensus"` or `"resolved"`) with an ACCEPT, CONDITIONAL_ACCEPT, or MODIFY verdict:
  - Mark the milestone as `status: done` in plan.yaml
  - Record completion timestamp
  - Log any unresolved conditions from CONDITIONAL_ACCEPT or MODIFY verdicts
- If any pair was REJECTED (by orchestrator/human verdict) or remains `"escalated"` without resolution:
  - Keep milestone as `status: in_progress`
  - Note which pairs need re-running

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

This appends a score entry to `.ratchet/scores/scores.jsonl` with fields: timestamp, debate_id, pair, milestone, rounds_to_consensus, escalated, issues_found, issues_resolved.

### Step 11: Propose Next Focus

After reporting results, use `AskUserQuestion` to let the user choose what to do next.

**If more milestones remain**, present a summary line then ask with options:
- Summary: `"Focus complete: [milestone name] — [N] pairs debated, [N] consensus, [N] escalated. Epic progress: [completed]/[total] milestones."`
- Options (adapt based on context):
  - "Continue to [next milestone name]" — the natural next step
  - "Review debate: [debate-id]" — one option per debate that just ran (so the user can inspect transcripts)
  - "View quality metrics" — runs /ratchet:score
  - "Address unresolved conditions" — only if CONDITIONAL_ACCEPTs logged conditions
- Use `multiSelect: false` — the user picks one action.

**If ALL milestones are done**, present a summary then ask:
- Summary: `"Epic complete! All [N] milestones finished. Total debates: [N] | Consensus rate: [%] | Avg rounds: [N]"`
- Options:
  - "View detailed quality metrics"
  - "Tighten agents from debate lessons"
  - "Review a specific debate"
  - "Done for now"
