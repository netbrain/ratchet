---
name: debate
description: View or continue an ongoing debate
user-invocable: true
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

Read all `.ratchet/debates/*/meta.json` files and present a table:

```
ID                              Pair             Status      Rounds  Verdict
api-contracts-20260312T100000   api-contracts    consensus   2       ACCEPT (consensus)
db-perf-20260312T100500         db-performance   escalated   3       pending
input-val-20260312T101000       input-validation consensus   1       ACCEPT (consensus)
```

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
Decision: [accept/reject/modify]
Decided by: [consensus/orchestrator/human]
Reasoning: [...]
```

### With --continue

Only valid for debates with status `escalated` or `initiated`.

Resume the debate protocol from where it left off:
- If `escalated`: offer to run another round (extending max_rounds by 1) or proceed to verdict
- If `initiated`: something went wrong mid-debate, restart from the last completed round

Update `meta.json` accordingly.
