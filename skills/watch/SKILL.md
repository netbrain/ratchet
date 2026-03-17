# /ratchet:watch — Background Monitoring

Monitor Ratchet epic progress in the background using Claude Code's `/loop` feature.

## Usage

```bash
/ratchet:watch              # Start monitoring current workspace
/ratchet:watch monitor      # Monitor specific workspace
/ratchet:watch --stop       # Stop all monitoring loops
/ratchet:watch --list       # List active monitoring loops
```

## What It Does

Sets up background loops to monitor epic progress without interrupting your workflow:

- **Every 5 minutes**: Display epic status summary
- **Every 10 minutes**: Check PRs for merge conflicts and CI failures
- **Every 1 hour**: Update quality scores and metrics
- **Every 30 minutes**: Check for blocked/escalated issues requiring attention

All monitoring output is displayed between your turns at low priority.

### PR Monitoring Features

When PR monitoring is enabled, `/ratchet:watch` proactively:

1. **Detects merge conflicts**: Checks all open Ratchet PRs for merge conflicts
2. **Monitors CI status**: Watches GitHub Actions/CI checks for failures
3. **Auto-creates sidequests**: When issues are detected, automatically adds discoveries to `plan.yaml`:
   - **CI failure** → Creates retro sidequest to learn from the failure
   - **Merge conflict** → Creates fix sidequest to resolve the conflict
4. **Notifies you**: Surfaces issues between your turns for awareness

## Requirements

- Claude Code v2.1.72+ (requires `/loop` feature)
- Active Ratchet workspace with `plan.yaml`

## Behavior

When you run `/ratchet:watch`:

1. **Resolve workspace** (same logic as `/ratchet:run`):
   - If workspace specified → use that
   - If CWD inside workspace → auto-detect
   - If at workspace root → present selector

2. **Set up monitoring loops**:
   ```bash
   /loop 5m /ratchet:status [workspace] --quiet
   /loop 10m check Ratchet PRs for conflicts and CI failures
   /loop 1h /ratchet:score [workspace] --summary
   /loop 30m check for blocked issues in [workspace]
   ```

3. **Track loop IDs** in `/tmp/ratchet-watch-loops-[workspace].json` for later cleanup

4. **Confirm to user**: "Monitoring [workspace] epic in background. Loops will expire after 72 hours or when session closes. Use /ratchet:watch --stop to end monitoring."

## Stopping Monitoring

`/ratchet:watch --stop` cancels all Ratchet monitoring loops. It does NOT stop the main `/ratchet:run` workflow if running.

## Listing Active Monitors

`/ratchet:watch --list` shows active monitoring loops with their last execution time.

## Implementation

### Loop Management

```bash
# For --stop flag:
# Cancel all loops tracked in /tmp/ratchet-watch-loops-[workspace].json
# Use Claude Code's loop cancellation API

# Store loop metadata:
{
  "workspace": "monitor",
  "started": "2026-03-17T00:30:00Z",
  "loops": [
    {"id": "loop-123", "command": "/ratchet:status monitor --quiet", "interval": "5m"},
    {"id": "loop-124", "command": "check PRs", "interval": "10m"},
    {"id": "loop-125", "command": "/ratchet:score monitor --summary", "interval": "1h"},
    {"id": "loop-126", "command": "check blocked issues", "interval": "30m"}
  ]
}
```

### Blocked Issue Check (every 30 minutes)

```bash
if [ -f .ratchet/plan.yaml ]; then
  blocked=$(yq eval '.epic.milestones[].issues[] | select(.status == "blocked") | .ref' .ratchet/plan.yaml)
  escalated=$(yq eval '.epic.milestones[].issues[] | select(.status == "escalated") | .ref' .ratchet/plan.yaml)

  if [ -n "$blocked" ] || [ -n "$escalated" ]; then
    echo "⚠️  Issues requiring attention:"
    [ -n "$blocked" ] && echo "  Blocked: $blocked"
    [ -n "$escalated" ] && echo "  Escalated: $escalated"
  fi
fi
```

### PR Monitoring (every 10 minutes)

```bash
# Get all PRs created by Ratchet (from plan.yaml)
prs=$(yq eval '.epic.milestones[].issues[] | select(.pr != null) | .pr' .ratchet/plan.yaml)

for pr_url in $prs; do
  pr_num=$(echo "$pr_url" | grep -oP 'pull/\K\d+')

  # Check merge conflict status
  mergeable=$(gh pr view "$pr_num" --json mergeable -q .mergeable)

  # Check CI status
  ci_status=$(gh pr view "$pr_num" --json statusCheckRollup -q '.statusCheckRollup[] | select(.conclusion != "SUCCESS") | .name')

  # Handle merge conflicts
  if [ "$mergeable" = "CONFLICTING" ]; then
    issue_ref=$(yq eval ".epic.milestones[].issues[] | select(.pr == \"$pr_url\") | .ref" .ratchet/plan.yaml)

    # Check if discovery already exists
    existing=$(yq eval ".epic.discoveries[] | select(.source == \"pr-conflict-$pr_num\")" .ratchet/plan.yaml)

    if [ -z "$existing" ]; then
      echo "🔴 Merge conflict detected: PR #$pr_num (issue $issue_ref)"
      echo "   Creating sidequest to resolve conflict..."

      # Add discovery to plan.yaml
      discovery_ref="discovery-conflict-$(date +%s)"
      yq eval -i ".epic.discoveries += [{
        \"ref\": \"$discovery_ref\",
        \"title\": \"Resolve merge conflict in PR #$pr_num\",
        \"description\": \"PR #$pr_num for issue $issue_ref has merge conflicts with main. Re-run issue pipeline to regenerate code on current main branch.\",
        \"source\": \"pr-conflict-$pr_num\",
        \"created_at\": \"$(date -Iseconds)\",
        \"severity\": \"high\",
        \"affected_scope\": \"issue $issue_ref\"
      }]" .ratchet/plan.yaml

      echo "   ✓ Created discovery: $discovery_ref"
    fi
  fi

  # Handle CI failures
  if [ -n "$ci_status" ]; then
    issue_ref=$(yq eval ".epic.milestones[].issues[] | select(.pr == \"$pr_url\") | .ref" .ratchet/plan.yaml)

    # Check if retro already exists
    existing=$(yq eval ".epic.discoveries[] | select(.source == \"pr-ci-failure-$pr_num\")" .ratchet/plan.yaml)

    if [ -z "$existing" ]; then
      echo "❌ CI failure detected: PR #$pr_num (issue $issue_ref)"
      echo "   Failed checks: $ci_status"
      echo "   Creating retro sidequest..."

      # Add retro discovery
      discovery_ref="discovery-retro-$(date +%s)"
      yq eval -i ".epic.discoveries += [{
        \"ref\": \"$discovery_ref\",
        \"title\": \"Retro: CI failure in PR #$pr_num\",
        \"description\": \"PR #$pr_num for issue $issue_ref failed CI checks: $ci_status. Review failure, update guards/tests, and improve agent prompts to prevent recurrence.\",
        \"source\": \"pr-ci-failure-$pr_num\",
        \"created_at\": \"$(date -Iseconds)\",
        \"severity\": \"medium\",
        \"affected_scope\": \"issue $issue_ref\",
        \"retro_type\": \"ci-failure\"
      }]" .ratchet/plan.yaml

      echo "   ✓ Created retro discovery: $discovery_ref"
    fi
  fi
done
```

### Discovery Schema

Discoveries created by PR monitoring follow this structure:

```yaml
epic:
  discoveries:
    - ref: "discovery-conflict-1710633000"
      title: "Resolve merge conflict in PR #20"
      description: "PR #20 for issue issue-2-2 has merge conflicts with main. Re-run issue pipeline to regenerate code on current main branch."
      source: "pr-conflict-20"
      created_at: "2026-03-17T00:30:00Z"
      severity: "high"           # high for conflicts, medium for CI failures
      affected_scope: "issue issue-2-2"

    - ref: "discovery-retro-1710633100"
      title: "Retro: CI failure in PR #19"
      description: "PR #19 for issue issue-2-3 failed CI checks: test-suite. Review failure, update guards/tests, and improve agent prompts to prevent recurrence."
      source: "pr-ci-failure-19"
      created_at: "2026-03-17T00:31:40Z"
      severity: "medium"
      affected_scope: "issue issue-2-3"
      retro_type: "ci-failure"
```

These discoveries can later be implemented using `/ratchet:run` - they'll show up as available work in the epic.

## Limitations

- **Session-scoped**: Monitoring stops when you close Claude Code
- **72-hour expiry**: Loops auto-delete after 3 days
- **50 loop limit**: Don't run `/ratchet:watch` on too many workspaces simultaneously
- **Not for critical path**: Monitoring is advisory — don't rely on it for workflow correctness

## Use Cases

**Good:**
```bash
# Monitor long-running epic while working on other tasks
/ratchet:watch monitor

# Check in periodically during unsupervised run
/ratchet:run --unsupervised --auto-pr monitor &
/ratchet:watch monitor
```

**Bad:**
```bash
# Don't use for workflow orchestration
# (use /ratchet:run --unsupervised instead)
```

## See Also

- `/ratchet:status` — One-time status check
- `/ratchet:score` — Quality metrics
- `/ratchet:run` — Main workflow execution
