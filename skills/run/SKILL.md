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
```

## Prerequisites
- `.ratchet/` must exist with valid config
- At least one pair must be registered and enabled

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
Present the epic status and propose the next focus using `AskUserQuestion`:

```
Epic: [project name]
  ✓ Milestone 1: [name] — done
  ✓ Milestone 2: [name] — done
  → Milestone 3: [name] — in progress (or next up)
  ○ Milestone 4: [name] — pending
  ○ Milestone 5: [name] — pending

[If there are unresolved conditions from previous debates:]
  Unresolved from last run:
    - [condition from CONDITIONAL_ACCEPT]
    - [condition from CONDITIONAL_ACCEPT]
```

Ask: "What should we focus on?" with options:
- "[Next milestone name] (Recommended)" — the natural next step
- "Address unresolved conditions from last run" — if any exist
- "Review all existing code" — run all pairs against everything
- (Other — user can type their own focus)

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

### Step 4: Match Pairs to Focus

Determine which pairs are relevant:
- **Epic mode**: use the pairs listed in the milestone's `pairs` field
- **Changed files mode**: match files to pairs by `scope` globs
- **All-files mode**: all enabled pairs
- **Greenfield mode**: all enabled pairs

Skip disabled pairs (`enabled: false`).

### Step 5: Create Debate(s)

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

### Step 6: Static Analysis Pre-Gate

Before starting debates, run the project's static analysis layer (layer 1 from the testing spec in project.yaml). This catches mechanical issues so adversarial agents can focus on semantic problems.

Read `.ratchet/project.yaml` for the static analysis commands (lint, type-check, format-check).

Run each configured command. If any fail:
1. Present the failures to the user
2. Ask: "Fix these before debating, or proceed anyway?"
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
- **ACCEPT** or **CONDITIONAL_ACCEPT** → Set status to `consensus`, write verdict, break loop
- **REJECT** → Continue to next round (or escalate if at max_rounds)

Update `meta.json` after each round — increment `rounds`, update `status`.

**IMPORTANT**: The debate loop MUST execute fully. After the generative agent produces code (especially in greenfield mode), the adversarial agent MUST review it. Do not stop after the generative round.

#### Escalation

If max_rounds reached without consensus:
- Set status to `escalated`
- Read `escalation` policy from config.yaml:
  - `orchestrator`: Spawn orchestrator agent with full debate transcript → write verdict
  - `human`: Set status to `escalated`, inform user to use `/ratchet:verdict`
  - `both`: Spawn orchestrator first, then present recommendation to human via `/ratchet:verdict`

### Step 8: Update Epic

After all debates for this focus resolve:
- If all pairs reached consensus (ACCEPT or CONDITIONAL_ACCEPT):
  - Mark the milestone as `status: done` in plan.yaml
  - Record completion timestamp
  - Log any unresolved conditions from CONDITIONAL_ACCEPTs
- If any pair was REJECTED or ESCALATED without resolution:
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

Append to `.ratchet/scores/scores.jsonl`:
```json
{
  "timestamp": "<ISO timestamp>",
  "debate_id": "<id>",
  "pair": "<pair-name>",
  "milestone": "<milestone id or null>",
  "rounds_to_consensus": <N>,
  "escalated": <true|false>,
  "issues_found": <count>,
  "issues_resolved": <count>
}
```

### Step 11: Propose Next Focus

After reporting results, guide the user to the next iteration:

```
Focus complete: [milestone name]
  [N] pairs debated, [N] consensus, [N] escalated
  [If conditions: Logged [N] conditions for future review]

Epic progress: [completed]/[total] milestones
  → Next up: [next milestone name] — [description]

Run /ratchet:run to continue, or /ratchet:debate [id] to review transcripts.
```

If ALL milestones are done:
```
Epic complete! All [N] milestones finished.

Quality summary:
  Total debates: [N] | Consensus rate: [%] | Avg rounds: [N]

Run /ratchet:score for detailed metrics.
Run /ratchet:tighten to sharpen agents from accumulated lessons.
```
