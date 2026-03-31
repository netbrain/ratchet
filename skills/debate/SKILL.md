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

**Error handling**: If `meta.json` is missing or malformed:
```bash
if [ ! -f ".ratchet/debates/<id>/meta.json" ]; then
  echo "Error: Debate '<id>' not found — no meta.json in .ratchet/debates/<id>/" >&2
  # Suggest listing available debates
fi
# Validate JSON is parseable
jq empty .ratchet/debates/<id>/meta.json 2>/dev/null \
  || { echo "Error: meta.json for debate '<id>' is malformed JSON. It may have been corrupted by an interrupted write." >&2; exit 1; }
```

**Recovery procedure for corrupted meta.json**:

If `meta.json` fails JSON validation, attempt recovery before giving up:

1. **Regenerate from round files** — reconstruct meta.json from the round files on disk:
   ```bash
   debate_dir=".ratchet/debates/<id>"
   pair_name=$(echo "<id>" | sed 's/-[0-9T]*$//')
   round_count=$(ls "$debate_dir/rounds"/round-*-adversarial.md 2>/dev/null | wc -l)
   last_round="$debate_dir/rounds/round-${round_count}-adversarial.md"

   # Extract verdict from the last adversarial round (look for verdict keywords)
   verdict=""
   if [ -f "$last_round" ]; then
     verdict=$(grep -oE '(ACCEPT|CONDITIONAL_ACCEPT|TRIVIAL_ACCEPT|REJECT|REGRESS)' \
       "$last_round" | head -1)
   fi

   # Reconstruct minimal meta.json
   cat > "$debate_dir/meta.json" << EOF
   {
     "id": "<id>",
     "pair": "$pair_name",
     "phase": "unknown",
     "status": "$([ -n "$verdict" ] && echo 'consensus' || echo 'initiated')",
     "rounds": $round_count,
     "max_rounds": 3,
     "started": "unknown",
     "resolved": null,
     "verdict": $([ -n "$verdict" ] && echo "\"$verdict\"" || echo 'null'),
     "fast_path": false,
     "recovered": true,
     "recovery_note": "Regenerated from round files after corruption"
   }
   EOF
   echo "meta.json regenerated from $round_count round files. Review and correct any 'unknown' fields." >&2
   ```

2. **If no round files exist** — the debate has no recoverable state:
   ```bash
   if [ "$round_count" -eq 0 ]; then
     echo "Error: No round files found for debate '<id>'. Cannot recover. Delete the debate directory and re-run." >&2
     # Suggest: rm -rf .ratchet/debates/<id> && /ratchet:run
   fi
   ```

After recovery, inform the user via `AskUserQuestion`:
- Question: "Debate '<id>' meta.json was corrupted and has been recovered from round files. Some fields may need manual correction. How would you like to proceed?"
- Options: `"View recovered debate (Recommended)"`, `"Re-run debate from scratch"`, `"Delete and skip"`

> **Verdict storage note**: Verdicts may exist in two locations depending on how the debate was resolved:
> - `meta.json` → `verdict` field: populated by the debate-runner for consensus (ACCEPT, CONDITIONAL_ACCEPT) or tiebreaker verdicts. This is an embedded object.
> - `verdict.json` (separate file in the debate directory): populated by `/ratchet:verdict` for human-cast verdicts. Read both; prefer `verdict.json` if it exists (human decision overrides).

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
  - If "Run another round": increment `max_rounds` by 1 in meta.json using jq:
    ```bash
    jq '.max_rounds += 1' .ratchet/debates/<id>/meta.json > /tmp/meta-tmp.json \
      && mv /tmp/meta-tmp.json .ratchet/debates/<id>/meta.json
    ```
    Then execute one debate round per `/ratchet:run` Step 5e protocol.

- If `initiated`:
  - Question: "Debate [id] was interrupted at round [N]. Resume from where it left off?"
  - Options: `"Resume debate (Recommended)"`, `"Restart debate"`, `"Abandon debate"`
  - If "Restart debate": delete all files in the `rounds/` directory, reset `rounds` to 0 in meta.json, then start fresh from round 1 per `/ratchet:run` Step 5e protocol.

When resuming or running another round, execute the same debate protocol as `/ratchet:run` Step 5e (generative round, adversarial round, check verdict). Read the pair's agent definitions from `.ratchet/pairs/<pair-name>/` and the debate context from `meta.json` and existing round files. If `rounds/` is empty (no prior round files), start from round 1.

**Tool boundaries for resumed debates**: When spawning agents for resumed rounds, enforce the same role boundaries as `/ratchet:run` Step 5e:
- Generative agent: tools = Read, Grep, Glob, Bash, Write, Edit
- Adversarial agent: tools = Read, Grep, Glob, Bash — disallowedTools = Write, Edit
- Tiebreaker agent: tools = Read, Grep, Glob, Bash — disallowedTools = Write, Edit

**Pass the guilty-until-proven-innocent principle** to resumed debate agents: new changes are GUILTY until proven innocent — test failures on a PR branch are caused by the PR unless definitively proven otherwise. The burden of proof is on demonstrating the failure exists on master.

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
