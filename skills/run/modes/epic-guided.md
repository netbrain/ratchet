# Mode B: Epic-guided (plan.yaml exists)

## Epic Complete Flow

**If ALL milestones are done** (every milestone has `status: done`):

The epic is complete. Present completion summary and next steps:

Question text:
```
Epic "[name]" is complete! All [N] milestones finished.

What would you like to do next?
```

Options:
- "Create a new epic" — gather details via AskUserQuestion (freeform: "What's the next body of work?"), then create the new epic structure in plan.yaml. For complex scoping, spawn the analyst agent to help break it into milestones. For straightforward requests, create directly from the user's description.
- "Add a milestone to the current epic" — gather milestone details via AskUserQuestion, append to plan.yaml
- "Tighten agents from debate lessons (/ratchet:tighten)"
- "View quality metrics (/ratchet:score)"
- "Done for now"

When creating a new epic: replace the existing `epic` block in plan.yaml (archive the old one to `.ratchet/archive/epic-<name>-<timestamp>.yaml` first if it has content). **Archive debates**: move all debate artifacts from the completed epic into the archive alongside the plan:
```bash
EPIC_SLUG=$(echo "$EPIC_NAME" | tr ' ' '-' | tr '[:upper:]' '[:lower:]')
ARCHIVE_DIR=".ratchet/archive/epic-${EPIC_SLUG}-$(date +%Y%m%dT%H%M%SZ)"
mkdir -p "$ARCHIVE_DIR"
cp .ratchet/plan.yaml "$ARCHIVE_DIR/plan.yaml"
if [ -d .ratchet/debates ] && [ "$(ls -A .ratchet/debates 2>/dev/null)" ]; then
  mv .ratchet/debates/* "$ARCHIVE_DIR/debates/" 2>/dev/null || true
fi
```
This is safe because `/ratchet:score` persists metrics as a moving average in `.ratchet/scores.yaml` (Step 2b of the score skill) — archiving debates does not lose score history.

Set `current_focus: null` and `discoveries: []` (or carry over pending discoveries). After writing the new epic to plan.yaml, sync the tracking issue:
```bash
if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
  bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
    || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
fi
```

## Focus Selection (milestones remain)

Use `AskUserQuestion` to let the user pick the focus. Include epic status with per-issue progress:

Question text (build from plan.yaml):
```
Epic: [project name] — [completed]/[total] milestones done.

Current milestone: [name] — [description]
[If regressions > 0: "Regressions: [N]/[max_regressions] used"]

Issues:
  [ref]: [title]  [DONE]
    plan ✓  test ✓  build ✓  review ✓  harden ✓
  [ref]: [title]  [IN PROGRESS]
    plan ✓  test ✓  build ●  review ○  harden ○
  [ref]: [title]  [PENDING — depends on [dep-ref]]
    plan ○  test ○  build ○  review ○  harden ○

(✓ = done, ● = current, ○ = pending)

What should we focus on?
```

If there are unresolved conditions from previous CONDITIONAL_ACCEPTs, append:
`"(Unresolved from last run: [condition1], [condition2])"`

Options:
- "Run all ready issues (Recommended)" — executes all issues with no unmet dependencies (parallel by layer)
- "Run specific issue: [ref]" — one option per ready issue
- "Address unresolved conditions from last run" — only if conditions exist
- "Process sidequests ([N] pending: [titles...])" — only if `epic.discoveries` has items with `status == "pending"`
- "Add a new milestone" — gather details via AskUserQuestion, append to plan.yaml, then offer to run it
- "[Next milestone name]" — skip ahead
- "Review all existing code"
- (Include an "Other" option so the user can type a custom focus)

## Sidequest Processing

When "Process sidequests" is selected, iterate over `epic.discoveries` with `status == "pending"`. For each discovery, use `AskUserQuestion`:

- Question: "Discovery: [title] ([category], [severity])\n[description]"
- Options:
  - `"Process now"` — handle via existing pipeline (tighten, re-launch, etc.)
  - `"Promote to issue"` — convert this discovery into a full plan.yaml issue
  - `"Dismiss"` — mark as non-actionable
  - `"Skip for now"` — leave as pending, move to next discovery

### Action: Process now

Existing behavior:
- `retro_type: "ci-failure"` → extract PR number from `source` field (format: `pr-ci-failure-<N>`) and launch `/ratchet:tighten pr <N>` for the affected issue
- `retro_type: "skipped-finding"` → present to user for decision (apply now or defer)
- No `retro_type` with `issue_ref` set (merge conflict) → use `issue_ref` field directly to re-launch the issue pipeline in its current phase
- No `retro_type` with `issue_ref: null` (manual discovery with no issue context) → cannot process directly, inform user: "This discovery has no linked issue. Promote it to an issue first, or dismiss it." Then re-present the action selector without the "Process now" option.
- Mark each processed discovery `status: "done"` in `plan.yaml`:
  ```bash
  yq eval -i "(.epic.discoveries[] | select(.ref == \"$discovery_ref\")).status = \"done\"" .ratchet/plan.yaml
  ```
- Sync plan tracking issue after discovery status change:
  ```bash
  if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
    bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
      || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
  fi
  ```

### Action: Promote to issue

Converts a discovery into a full plan.yaml issue:
1. Determine target milestone:
   - If `context.milestone` is set, use that milestone
   - Otherwise, use `AskUserQuestion` to select from active milestones
2. Generate issue ref: read existing issues in the target milestone, find the highest issue number, increment by 1. Format: `issue-<milestone-number>-<next-issue-number>`
3. Determine pairs:
   - If discovery `pairs` array is non-empty, use those
   - Otherwise, use `AskUserQuestion` to select from available pairs in workflow.yaml
4. Create the issue entry in plan.yaml:
   ```bash
   new_ref="issue-<M>-<N>"
   yq eval -i "(.epic.milestones[] | select(.id == \"$milestone_id\")).issues += [{
     \"ref\": \"$new_ref\",
     \"title\": \"$discovery_title\",
     \"description\": \"$discovery_description\",
     \"pairs\": [\"$pair_name\"],
     \"depends_on\": [],
     \"phase_status\": {\"plan\": \"pending\", \"test\": \"pending\", \"build\": \"pending\", \"review\": \"pending\", \"harden\": \"pending\"},
     \"files\": [],
     \"debates\": [],
     \"branch\": null,
     \"pr\": null,
     \"status\": \"pending\"
   }])" .ratchet/plan.yaml
   ```
5. Update the discovery status and link:
   ```bash
   yq eval -i "(.epic.discoveries[] | select(.ref == \"$discovery_ref\")).status = \"promoted\"" .ratchet/plan.yaml
   yq eval -i "(.epic.discoveries[] | select(.ref == \"$discovery_ref\")).issue_ref = \"$new_ref\"" .ratchet/plan.yaml
   ```
6. Sync plan tracking issue after adding the new issue:
   ```bash
   if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
     bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
       || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
   fi
   ```
7. Confirm to user: "Discovery promoted to issue [new_ref] in milestone [milestone_id]. Run /ratchet:run to start working on it."

### Action: Dismiss

Marks a discovery as non-actionable:
1. Use `AskUserQuestion` (freeform): "Reason for dismissal (optional)"
2. Update plan.yaml:
   ```bash
   yq eval -i "(.epic.discoveries[] | select(.ref == \"$discovery_ref\")).status = \"dismissed\"" .ratchet/plan.yaml
   ```
3. Confirm: "Discovery [ref] dismissed."

### Action: Skip for now

No changes, move to next discovery.
