---
name: ratchet:watch
description: Watch active PRs for merge conflicts and CI failures, auto-create sidequests
---

# /ratchet:watch — PR Monitor

Watch active Ratchet PRs for merge conflicts and CI failures. When problems are detected, automatically creates discoveries in plan.yaml so they surface in `/ratchet:run` and `/ratchet:status`.

Uses Claude Code's `/loop` feature to poll every 10 minutes.

**Auto-started by `/ratchet:run`** — the run orchestrator starts this loop when PRs exist and stops it on completion. Use `/ratchet:watch` manually only if you want monitoring outside of a run session.

## Usage
```
/ratchet:watch              # Start watching PRs
/ratchet:watch --stop       # Stop watching
```

## Prerequisites
- `.ratchet/plan.yaml` must exist with issues that have `pr` fields populated
- `gh` CLI must be authenticated

If no PRs exist in plan.yaml, inform the user:
> "No PRs found in plan.yaml. PRs are created by /ratchet:run when issues complete."

## Behavior

### Starting

1. **Resolve workspace** (same logic as `/ratchet:run` Step 1a)
2. **Verify PRs exist** — scan plan.yaml for issues with non-null `pr` fields
3. **Start loop**:
   ```
   /loop 10m check Ratchet PRs for conflicts and CI failures
   ```
4. **Confirm**: "Watching [N] PRs. Checking every 10 minutes. Use /ratchet:watch --stop to end."

### Each Poll Cycle

For each PR URL found in plan.yaml (`epic.milestones[].issues[] | select(.pr != null) | .pr`):

```bash
pr_num=$(echo "$pr_url" | grep -oP 'pull/\K\d+')

# Check merge conflict status
mergeable=$(gh pr view "$pr_num" --json mergeable -q .mergeable)

# Check CI status
ci_status=$(gh pr view "$pr_num" --json statusCheckRollup -q '.statusCheckRollup[] | select(.conclusion != "SUCCESS") | .name')
```

**Merge conflict detected** (`mergeable == "CONFLICTING"`):
- Check if discovery already exists: `epic.discoveries[] | select(.source == "pr-conflict-<pr_num>")`
- If not, create discovery:
  ```bash
  issue_ref=$(yq eval ".epic.milestones[].issues[] | select(.pr == \"$pr_url\") | .ref" .ratchet/plan.yaml)
  milestone_id=$(yq eval ".epic.milestones[] | select(.issues[] | .pr == \"$pr_url\") | .id" .ratchet/plan.yaml | head -1)
  yq eval -i ".epic.discoveries += [{
    \"ref\": \"discovery-conflict-$(date +%s)\",
    \"title\": \"Merge conflict in PR #$pr_num\",
    \"description\": \"PR #$pr_num for issue $issue_ref has merge conflicts with main.\",
    \"category\": \"bug\",
    \"severity\": \"critical\",
    \"source\": \"pr-conflict-$pr_num\",
    \"created_at\": \"$(date -Iseconds)\",
    \"status\": \"pending\",
    \"issue_ref\": \"$issue_ref\"
  }]" .ratchet/plan.yaml
  ```
- Report: "Merge conflict detected: PR #[N] (issue [ref]) — discovery created"

**CI failure detected** (non-empty `ci_status`):
- Check if discovery already exists: `epic.discoveries[] | select(.source == "pr-ci-failure-<pr_num>")`
- If not, create discovery:
  ```bash
  yq eval -i ".epic.discoveries += [{
    \"ref\": \"discovery-ci-$(date +%s)\",
    \"title\": \"CI failure in PR #$pr_num\",
    \"description\": \"PR #$pr_num for issue $issue_ref failed: $ci_status\",
    \"category\": \"tech-debt\",
    \"severity\": \"major\",
    \"source\": \"pr-ci-failure-$pr_num\",
    \"created_at\": \"$(date -Iseconds)\",
    \"status\": \"pending\",
    \"issue_ref\": \"$issue_ref\",
    \"retro_type\": \"ci-failure\"
  }]" .ratchet/plan.yaml
  ```
- Report: "CI failure detected: PR #[N] ([failed checks]) — discovery created"

**All clear**: No output (silent when nothing is wrong).

### Stopping

`/ratchet:watch --stop` — cancel the loop. No cleanup needed beyond stopping the loop itself.

## Limitations

- **Session-scoped**: stops when you close Claude Code
- **Requires `gh` auth**: uses GitHub CLI for PR status
- **Only watches PRs in plan.yaml**: PRs created outside Ratchet are not monitored
