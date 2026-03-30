# PR Body and Debate Summary Builder

> This file is the extracted PR body construction logic from `skills/run/issue-pipeline.md`.
> It is loaded on demand when the issue pipeline creates a PR at commit/PR boundaries (Step 5f).
> For the full issue pipeline, see `skills/run/issue-pipeline.md`.

---

## Building the Debate Summary Section

The issue pipeline tracks debate IDs in the issue's `debates` array in plan.yaml (recorded by the debate-runner via `yq` after each debate completes). Read them at PR creation time:

```bash
# Step 1: Read debate IDs from the issue's debates array in plan.yaml
# issue_ref is the ref string for this issue (e.g., "#46"), available in the
# issue pipeline's context from when the orchestrator launched this pipeline.
export ISSUE_REF="${issue_ref}"
mapfile -t debate_ids < <(
  yq -r '.epic.milestones[].issues[] | select(.ref == env(ISSUE_REF)) | .debates[]?' \
    .ratchet/plan.yaml
)

# If no debates recorded, omit the section entirely
if [ "${#debate_ids[@]}" -eq 0 ]; then
  debate_summary_section=""
else
  # Step 2: Build the table rows — one row per debate ID
  table_rows=""
  conditions_block=""
  has_conditional=false

  for debate_id in "${debate_ids[@]}"; do
    meta_path=".ratchet/debates/${debate_id}/meta.json"
    if [ ! -f "$meta_path" ]; then
      echo "Warning: could not load debate metadata for ${debate_id}" >&2
      continue
    fi
    meta=$(cat "$meta_path")
    pair=$(echo "$meta" | jq -r '.pair')
    phase=$(echo "$meta" | jq -r '.phase')
    rounds=$(echo "$meta" | jq -r '.rounds_completed')
    verdict=$(echo "$meta" | jq -r '.verdict_detail // .verdict')
    decided_by=$(echo "$meta" | jq -r '
      if .decided_by then .decided_by
      elif (.verdict_detail == "ACCEPT" or .verdict_detail == "TRIVIAL_ACCEPT" or .verdict == "consensus") then "consensus"
      elif (.verdict_detail == "ESCALATED" or .verdict == "ESCALATED") then "tiebreaker"
      else "consensus"
      end')
    table_rows="${table_rows}| ${pair} | ${phase} | ${rounds} | ${verdict} | ${decided_by} |\n"

    # Collect conditions for CONDITIONAL_ACCEPT debates
    if [ "$verdict" = "CONDITIONAL_ACCEPT" ]; then
      has_conditional=true
      while IFS= read -r condition; do
        conditions_block="${conditions_block}- ${pair}: ${condition}\n"
      done < <(echo "$meta" | jq -r '.conditions[]? // empty')
    fi
  done

  # Step 3: Assemble the section
  debate_summary_section="## Debate Summary

| Pair | Phase | Rounds | Verdict | Decided By |
|------|-------|--------|---------|------------|
$(printf "%b" "$table_rows")"

  if [ "$has_conditional" = true ]; then
    debate_summary_section="${debate_summary_section}

<details>
<summary>Conditions</summary>
$(printf "%b" "$conditions_block")
</details>"
  fi
fi
```

The resulting `$debate_summary_section` is appended to the PR body at `gh pr create` time:

```bash
# Step 4: Build GitHub issue linking line
# The issue's ref IS the GitHub issue number (if promoted). Use it directly.
# If ref is a local string (not yet promoted), no linking line is emitted.
github_issue_line=""
if [[ "${ISSUE_REF}" =~ ^[0-9]+$ ]]; then
  github_issue_line="Fixes #${ISSUE_REF}"
fi

# Step 5: Create the PR with the assembled body
pr_body="$(cat <<EOF
## Summary

$(echo "$phase_summaries")

${github_issue_line}
${depends_on_line}

${debate_summary_section}
EOF
)"

git push -u origin "${branch_name}"
gh pr create \
  --title "${issue_title}" \
  --body "${pr_body}" \
  --base main
```

**Example output — two debates, one CONDITIONAL_ACCEPT:**

```markdown
## Debate Summary

| Pair | Phase | Rounds | Verdict | Decided By |
|------|-------|--------|---------|------------|
| api-contracts | review | 2 | ACCEPT | consensus |
| skill-coherence | build | 1 | CONDITIONAL_ACCEPT | consensus |

<details>
<summary>Conditions</summary>
- skill-coherence: Verify cross-reference to skills/run/SKILL.md exists
</details>
```

(Each debate in the issue's `debates` array produces one row. Multiple pairs -> multiple rows. Conditions block only appears when at least one CONDITIONAL_ACCEPT verdict exists.)

**Rules:**
- Data sourced exclusively from `meta.json` `conditions` array — do NOT summarize adversarial narrative or round text
- Conditions block only shown when at least one `CONDITIONAL_ACCEPT` verdict exists (`$has_conditional = true`)
- If `meta.json` is missing or unreadable for a debate ID, skip that row and log a warning: `"Warning: could not load debate metadata for [id]"` to stderr
- Section omitted entirely when `debates` array is empty (`${#debate_ids[@]} -eq 0`)
- Multiple debates -> multiple table rows; a debate with no conditions contributes no lines to the conditions block
