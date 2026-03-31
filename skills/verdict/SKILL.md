---
name: ratchet:verdict
description: Human-in-the-loop — cast the deciding vote on an escalated debate
---

# /ratchet:verdict — Human Decides

Cast a human verdict on an escalated debate, overriding or confirming the tiebreaker's recommendation.

## Usage
```
/ratchet:verdict [id] [accept|reject|modify]
/ratchet:verdict [id]   # View debate summary and be prompted for decision
```

## Execution Steps

### Step 1: Load Debate

Read `.ratchet/debates/<id>/meta.json`. Verify status is `escalated` OR (verdict.decided_by == 'tiebreaker' AND status != 'resolved').

If no ID provided, scan all `.ratchet/debates/*/meta.json` for debates with status `escalated`.

If no escalated debates exist, inform the user:
> "No debates need a human verdict right now. All debates are either in progress or already resolved."

Then use `AskUserQuestion` with options: `"View all debates (/ratchet:debate)"`, `"Run next debate (/ratchet:run)"`, `"Done for now"`.

If escalated debates exist, use `AskUserQuestion` to let the user pick:
- Question: "Which debate needs your verdict?"
- Options: one per escalated debate, formatted as `"[debate-id] — [pair-name] ([N] rounds)"`

### Step 2: Present Summary

Show a concise summary of the debate (files, round count, key arguments, tiebreaker recommendation if any) in the question text, then use `AskUserQuestion`:

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

### Step 3.5: Update Plan State

After recording the verdict, update `.ratchet/plan.yaml` to reflect the issue's status:

1. **Read plan.yaml** and locate the issue that contains this debate
2. **Check if other debates for this issue are still pending**:
   - Scan the issue's `phase_status` and `pairs` to see if this was the last debate
3. **Update phase_status** based on verdict and debate count:
   - If `decision: accept` AND this is the last debate → set current phase to `done`, advance to next phase
   - If `decision: reject` → keep current phase as `in_progress` (generative needs to address issues)
   - If `decision: modify` → set status to `in_progress` with conditions logged
4. **Handle partial completion**: If other debates for this issue are still running, do NOT advance the phase yet

Working commands:
```bash
# 1. Get the issue ref from the debate's meta.json
issue_ref=$(jq -r '.issue' .ratchet/debates/<id>/meta.json)

# 2. Count remaining pending debates for this issue
pending_debates=$(for f in .ratchet/debates/*/meta.json; do
  [ -f "$f" ] || continue
  jq -e --arg ref "$issue_ref" \
    '.issue == $ref and (.status == "initiated" or .status == "in_progress")' \
    "$f" 2>/dev/null
done | grep -c "^true$")

# 3. Locate which milestone contains this issue (returns 0-based index)
milestone_idx=$(yq eval '
  .epic.milestones | to_entries | .[] |
  select(.value.issues[] | .ref == "'"$issue_ref"'") | .key
' .ratchet/plan.yaml)

# 4. Locate the issue index within that milestone
issue_idx=$(yq eval '
  .epic.milestones['"$milestone_idx"'].issues | to_entries | .[] |
  select(.value.ref == "'"$issue_ref"'") | .key
' .ratchet/plan.yaml)

# 5. Determine current phase
current_phase=$(yq eval '
  .epic.milestones['"$milestone_idx"'].issues['"$issue_idx"'].phase_status |
  to_entries | .[] | select(.value == "in_progress") | .key
' .ratchet/plan.yaml)

# 6. Update based on verdict
decision="<accept|reject|modify>"  # from verdict.json

if [ "$decision" = "accept" ] && [ "$pending_debates" -eq 0 ]; then
  # Mark current phase done
  yq eval -i '
    .epic.milestones['"$milestone_idx"'].issues['"$issue_idx"'].phase_status.'"$current_phase"' = "done"
  ' .ratchet/plan.yaml

  # Advance to next phase (phase order: plan, test, build, review, harden)
  phases=(plan test build review harden)
  for i in "${!phases[@]}"; do
    if [ "${phases[$i]}" = "$current_phase" ] && [ $((i + 1)) -lt ${#phases[@]} ]; then
      next_phase="${phases[$((i + 1))]}"
      yq eval -i '
        .epic.milestones['"$milestone_idx"'].issues['"$issue_idx"'].phase_status.'"$next_phase"' = "in_progress"
      ' .ratchet/plan.yaml
      break
    fi
  done

elif [ "$decision" = "reject" ]; then
  # Keep current phase as in_progress — generative needs to address issues
  yq eval -i '
    .epic.milestones['"$milestone_idx"'].issues['"$issue_idx"'].phase_status.'"$current_phase"' = "in_progress"
  ' .ratchet/plan.yaml

elif [ "$decision" = "modify" ]; then
  # Keep current phase as in_progress with conditions logged
  yq eval -i '
    .epic.milestones['"$milestone_idx"'].issues['"$issue_idx"'].phase_status.'"$current_phase"' = "in_progress"
  ' .ratchet/plan.yaml
fi

# 7. If accept and all phases done, mark issue as done
if [ "$decision" = "accept" ] && [ "$pending_debates" -eq 0 ]; then
  all_done=$(yq eval '
    .epic.milestones['"$milestone_idx"'].issues['"$issue_idx"'].phase_status |
    to_entries | .[] | select(.value != "done") | .key
  ' .ratchet/plan.yaml)
  if [ -z "$all_done" ]; then
    yq eval -i '
      .epic.milestones['"$milestone_idx"'].issues['"$issue_idx"'].status = "done"
    ' .ratchet/plan.yaml
  fi
fi
```

### Step 4: Update Scores

Run the score update script:
```bash
test -f .claude/ratchet-scripts/update-scores.sh \
  || { echo "Error: update-scores.sh not found. Scores not updated." >&2; }
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
