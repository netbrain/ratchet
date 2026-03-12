---
name: verdict
description: Human-in-the-loop — cast the deciding vote on an escalated debate
user-invocable: true
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

If no ID provided, list all debates needing human input.

### Step 2: Present Summary

Show a concise summary of the debate:
- Files under review
- Number of rounds
- Key arguments from each side
- Orchestrator's recommendation (if `escalation: both`)

Ask the human: "What's your verdict? [accept / reject / modify]"

If `modify`, ask: "What specific changes are needed?"

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
- Set `status` to `verdict`
- Set `resolved` timestamp
- Set `verdict` object

### Step 4: Update Scores

Append to `.ratchet/scores/scores.jsonl` with the final outcome.

### Step 5: Report
```
Verdict recorded for [id]: [decision] (human)
[If modify: Required changes: [list]]
[If accept: Pair consensus not needed — human override]
```
