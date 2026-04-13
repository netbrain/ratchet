# Mode B: Epic-guided (plan.yaml exists)

## Epic Complete Flow

**If ALL milestones are done** (every milestone has `status: done`), epic is complete. Present completion summary and next steps:

Question text:
```
Epic "[name]" is complete! All [N] milestones finished.

What would you like to do next?
```

Options:
- "Create a new epic" — gather details via AskUserQuestion (freeform: "What's the next body of work?"), then create new epic structure in plan.yaml. For complex scoping, spawn analyst agent to break into milestones; for straightforward requests, create directly from user's description.
- "Add a milestone to the current epic" — gather details via AskUserQuestion, append to plan.yaml
- "Tighten agents from debate lessons (/ratchet:tighten)"
- "View quality metrics (/ratchet:score)"
- "Done for now"

When creating a new epic: replace existing `epic` block in plan.yaml (archive old one to `.ratchet/archive/epic-<name>-<timestamp>.yaml` first if it has content). **Archive debates**: move all debate artifacts from completed epic into archive alongside the plan:
```bash
EPIC_SLUG=$(echo "$EPIC_NAME" | tr ' ' '-' | tr '[:upper:]' '[:lower:]')
ARCHIVE_DIR=".ratchet/archive/epic-${EPIC_SLUG}-$(date +%Y%m%dT%H%M%SZ)"
mkdir -p "$ARCHIVE_DIR"
cp .ratchet/plan.yaml "$ARCHIVE_DIR/plan.yaml"
if [ -d .ratchet/debates ] && [ "$(ls -A .ratchet/debates 2>/dev/null)" ]; then
  mv .ratchet/debates/* "$ARCHIVE_DIR/debates/" 2>/dev/null || true
fi
```
Safe because `/ratchet:score` persists metrics as moving average in `.ratchet/scores.yaml` (Step 2b of score skill) — archiving debates does not lose score history.

Set `current_focus: null` and `discoveries: []` (or carry over pending). After writing the new epic to plan.yaml, sync tracking issue:
```bash
if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
  bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
    || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
fi
```

## Focus Selection (milestones remain)

Use `AskUserQuestion` to let user pick focus. Include epic status with per-issue progress:

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
- (Include "Other" option so user can type a custom focus)

## Sidequest Processing

When "Process sidequests" selected, iterate over `epic.discoveries` with `status == "pending"`. For each, use `AskUserQuestion`:

- Question: "Discovery: [title] ([category], [severity])\n[description]"
- Options: `"Process now"` (handle via existing pipeline — tighten, re-launch, etc.), `"Promote to issue"` (convert to full plan.yaml issue), `"Dismiss"` (mark non-actionable), `"Skip for now"` (leave pending, move to next)

### Action: Process now

- `retro_type: "ci-failure"` → extract PR number from `source` field (format: `pr-ci-failure-<N>`) and launch `/ratchet:tighten pr <N>`
- `retro_type: "skipped-finding"` → present to user (apply now or defer)
- No `retro_type` with `issue_ref` set (merge conflict) → use `issue_ref` to re-launch issue pipeline in current phase
- No `retro_type` with `issue_ref: null` (manual discovery) → cannot process directly. Inform user: "This discovery has no linked issue. Promote it to an issue first, or dismiss it." Then re-present action selector without "Process now" option.
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

Converts discovery into a full plan.yaml issue:
1. Target milestone: if `context.milestone` set, use it; else `AskUserQuestion` to select from active milestones
2. Generate issue ref: find highest issue number in target milestone, increment by 1. Format: `issue-<milestone-number>-<next-issue-number>`
3. Pairs: if discovery `pairs` array non-empty, use those; else `AskUserQuestion` to select from available pairs in workflow.yaml
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
5. Update discovery status and link:
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

Marks discovery as non-actionable:
1. `AskUserQuestion` (freeform): "Reason for dismissal (optional)"
2. Update plan.yaml:
   ```bash
   yq eval -i "(.epic.discoveries[] | select(.ref == \"$discovery_ref\")).status = \"dismissed\"" .ratchet/plan.yaml
   ```
3. Confirm: "Discovery [ref] dismissed."

### Action: Skip for now

No changes, move to next discovery.
