---
name: ratchet:debate
description: View or continue an ongoing debate
---

# /ratchet:debate — View or Continue a Debate

View the full transcript of a debate, or continue an unresolved one.

## Usage
```
/ratchet:debate              # List all debates with status
/ratchet:debate [id]         # View a specific debate's full transcript
/ratchet:debate [id] --continue  # Continue an escalated debate with another round
```

## Execution Steps

### No Arguments — List Debates

Read all `.ratchet/debates/*/meta.json` files. If no debates exist (directory is empty or no meta.json files found), inform the user:
> "No debates found. Run /ratchet:run to start your first debate."

Then use `AskUserQuestion` with options: `"Start a debate (/ratchet:run) (Recommended)"`, `"Done for now"`.

If debates exist, use `AskUserQuestion` to let the user pick a debate to view:

- Question: "Which debate do you want to view?"
- Options: one per debate, formatted as `"[debate-id] — [pair-name] | [status] | [N] rounds | [verdict or 'pending']"`
- Include a `"Cancel"` option

### With ID — View Transcript

Read the debate's `meta.json` and all round files. Present the full transcript:

```
Debate: [id]
Pair: [pair-name]
Files: [file list]
Status: [status]
Started: [timestamp]

--- Round 1 ---

[Generative]:
[contents of round-1-generative.md]

[Adversarial]:
[contents of round-1-adversarial.md]

--- Round 2 ---
...

[If verdict exists:]
--- Verdict ---
Decision: [ACCEPT/CONDITIONAL_ACCEPT/REJECT] (or [accept/modify/reject] for human/tiebreaker verdicts)
Decided by: [consensus/tiebreaker/human]
Reasoning: [...]
```

### With --continue

Only valid for debates with status `escalated` or `initiated`.

Resume the debate protocol from where it left off. Use `AskUserQuestion` to let the user decide:

- If `escalated`:
  - Question: "Debate [id] escalated after [N] rounds (max was [max_rounds]). How do you want to proceed?"
  - Options: `"Run another round (extend max by 1)"`, `"Proceed to verdict"`, `"View full transcript first"`
  - If "Run another round": increment `max_rounds` by 1 in meta.json, then execute one debate round per `/ratchet:run` Step 7 protocol.

- If `initiated`:
  - Question: "Debate [id] was interrupted at round [N]. Resume from where it left off?"
  - Options: `"Resume debate (Recommended)"`, `"Restart debate"`, `"Abandon debate"`
  - If "Restart debate": delete all files in the `rounds/` directory, reset `rounds` to 0 in meta.json, then start fresh from round 1 per `/ratchet:run` Step 7 protocol.

When resuming or running another round, execute the same debate protocol as `/ratchet:run` Step 7 (generative round, adversarial round, check verdict). Read the pair's agent definitions from `.ratchet/pairs/<pair-name>/` and the debate context from `meta.json` and existing round files. If `rounds/` is empty (no prior round files), start from round 1.

If the user picks "Abandon debate", set status to `"resolved"` with verdict `{"decision": "reject", "decided_by": "human", "reasoning": "Debate abandoned by user"}`.

Update `meta.json` accordingly.

### After Viewing — Next Steps

After showing a transcript (or completing a --continue action), use `AskUserQuestion` to guide the user:
- Options (adapt based on debate status):
  - "Continue this debate" — only if status is `escalated` or `initiated`
  - "Render verdict (/ratchet:verdict)" — only if status is `escalated`
  - "View another debate" — if more debates exist
  - "Back to main flow (/ratchet:run) (Recommended)"
  - "Done for now"
