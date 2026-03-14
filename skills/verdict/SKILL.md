---
name: ratchet:verdict
description: Human-in-the-loop — cast the deciding vote on an escalated debate
---

# /ratchet:verdict — Human Decides

Cast a human verdict on an escalated debate, overriding or confirming the orchestrator's recommendation.

## Usage
```
/ratchet:verdict [id] [accept|reject|modify]
/ratchet:verdict [id]   # View debate summary and be prompted for decision
```

## Execution Steps

### Step 1: Load Debate

Read `.ratchet/debates/<id>/meta.json`. Verify status is `escalated` or has an orchestrator verdict pending human review.

If no ID provided, scan all `.ratchet/debates/*/meta.json` for debates with status `escalated`.

If no escalated debates exist, inform the user:
> "No debates need a human verdict right now. All debates are either in progress or already resolved."

Then use `AskUserQuestion` with options: `"View all debates (/ratchet:debate)"`, `"Run next debate (/ratchet:run)"`, `"Done for now"`.

If escalated debates exist, use `AskUserQuestion` to let the user pick:
- Question: "Which debate needs your verdict?"
- Options: one per escalated debate, formatted as `"[debate-id] — [pair-name] ([N] rounds)"`

### Step 2: Present Summary

Show a concise summary of the debate (files, round count, key arguments, orchestrator recommendation if any) in the question text, then use `AskUserQuestion`:

- Question: "[summary text]. What's your verdict?"
- Options: `"Accept"`, `"Reject"`, `"Modify"`

If the user picks `Modify`, follow up with `AskUserQuestion` (freeform):
- Question: "What specific changes are needed?"

### Step 3: Record Verdict

Write or update `.ratchet/debates/<id>/verdict.json`:
```json
{
  "decision": "accept|reject|modify",
  "decided_by": "human",
  "reasoning": "human's stated reasoning",
  "required_changes": ["if modify, list of changes"],
  "timestamp": "<ISO timestamp>"
}
```

Update `meta.json`:
- Set `status` to `"resolved"` (terminal state — human has decided)
- Set `resolved` timestamp
- Set `verdict` object

### Step 4: Update Scores

Run the score update script:
```bash
bash .claude/ratchet-scripts/update-scores.sh <debate-id>
```

This appends the final outcome to `.ratchet/scores/scores.jsonl`.

### Step 5: Report
```
Verdict recorded for [id]: [decision] (human)
[If modify: Required changes: [list]]
[If accept: Pair consensus not needed — human override]
```

After reporting, use `AskUserQuestion` to guide the user:
- Options (adapt based on context):
  - "Continue to next milestone (/ratchet:run) (Recommended)" — if the verdict resolved all debates for the current focus
  - "Verdict another debate" — if more escalated debates exist
  - "View quality metrics (/ratchet:score)"
  - "Done for now"
