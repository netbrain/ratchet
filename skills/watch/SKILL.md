---
name: ratchet:watch
description: Watch active PRs for merge conflicts, CI failures, and review comments
---

# /ratchet:watch — PR Monitor

Watch active Ratchet PRs for merge conflicts, CI failures, and review comments. When problems detected, automatically creates discoveries in plan.yaml so they surface in `/ratchet:run` and `/ratchet:status`. When actionable review comments detected, spawns response agents to address feedback directly on the PR branch. Uses Claude Code's `/loop` feature to poll every 10 minutes.

**Auto-started by `/ratchet:run`** — run orchestrator starts this loop when PRs exist and stops it on completion. Use `/ratchet:watch` manually only if you want monitoring outside of a run session.

## Usage
```
/ratchet:watch              # Start watching PRs (includes review comment detection)
/ratchet:watch --no-respond # Watch but don't auto-spawn response agents for reviews
/ratchet:watch --stop       # Stop watching
```

## Prerequisites
- `.ratchet/plan.yaml` must exist with issues that have `pr` fields populated
- `gh` CLI must be installed and authenticated

If no PRs exist in plan.yaml, inform user: "No PRs found in plan.yaml. PRs are created by /ratchet:run when issues complete."

## Behavior

### Starting

1. **Verify `gh` authentication** before any GitHub API calls:
   ```bash
   gh auth status >/dev/null 2>&1
   ```
   If check fails (`exit code != 0`), inform user via `AskUserQuestion`:
   - Question: "GitHub CLI is not authenticated. /ratchet:watch requires 'gh' to check PR status. Run 'gh auth login' in your terminal first."
   - Options: `"I've authenticated — retry"`, `"Cancel"`
   If "retry", re-run `gh auth status` check. If "Cancel", exit skill.
2. **Resolve workspace** (same logic as `/ratchet:run` Step 1a)
3. **Verify PRs exist** — scan plan.yaml for issues with non-null `pr` fields
4. **Initialize watch state** — load or create `.ratchet/watch-state.json`:
   ```bash
   if [ ! -f .ratchet/watch-state.json ]; then
     echo '{"seen_comment_ids": [], "seen_review_ids": []}' > .ratchet/watch-state.json
   fi
   ```
   Watch state file tracks which review comments and reviews have been processed to avoid re-processing on subsequent poll cycles.
5. **Parse flags**:
   - `--no-respond`: Disable auto-spawning response agents for review comments. When set, actionable comments are logged as feedback entries but no agent is spawned. Default behavior is to auto-respond when `github-issues` progress adapter is configured in `workflow.yaml`.
   - `--stop`: Cancel loop and exit.
6. **Start loop**:
   ```
   /loop 10m check Ratchet PRs for conflicts, CI failures, and review comments
   ```
7. **Confirm**: "Watching [N] PRs. Checking every 10 minutes. Use /ratchet:watch --stop to end."

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
    \"status\": \"pending\",
    \"issue_ref\": \"$issue_ref\",
    \"context\": {\"milestone\": $milestone_id, \"issue\": \"$issue_ref\", \"debate\": null},
    \"pairs\": [],
    \"affected_scope\": null,
    \"retro_type\": null,
    \"created_at\": \"$(date -Iseconds)\"
  }]" .ratchet/plan.yaml
  ```
- Report: "Merge conflict detected: PR #[N] (issue [ref]) — discovery created"

**CI failure detected** (non-empty `ci_status`):
- Check if discovery already exists: `epic.discoveries[] | select(.source == "pr-ci-failure-<pr_num>")`
- If not, create discovery:
  ```bash
  issue_ref=$(yq eval ".epic.milestones[].issues[] | select(.pr == \"$pr_url\") | .ref" .ratchet/plan.yaml)
  milestone_id=$(yq eval ".epic.milestones[] | select(.issues[] | .pr == \"$pr_url\") | .id" .ratchet/plan.yaml | head -1)
  milestone_id_value=$([ -n "$milestone_id" ] && echo "$milestone_id" || echo "null")
  yq eval -i ".epic.discoveries += [{
    \"ref\": \"discovery-ci-$(date +%s)\",
    \"title\": \"CI failure in PR #$pr_num\",
    \"description\": \"PR #$pr_num for issue $issue_ref failed: $ci_status\",
    \"category\": \"tech-debt\",
    \"severity\": \"major\",
    \"source\": \"pr-ci-failure-$pr_num\",
    \"status\": \"pending\",
    \"issue_ref\": \"$issue_ref\",
    \"context\": {\"milestone\": $milestone_id_value, \"issue\": \"$issue_ref\", \"debate\": null},
    \"pairs\": [],
    \"affected_scope\": null,
    \"retro_type\": \"ci-failure\",
    \"created_at\": \"$(date -Iseconds)\"
  }]" .ratchet/plan.yaml
  ```
- Report: "CI failure detected: PR #[N] ([failed checks]) — discovery created"

**Review comments detected** — fetch and classify new review activity:

```bash
# Get the repo owner/name from gh
repo_nwo=$(gh repo view --json nameWithOwner -q .nameWithOwner)

# Fetch PR review comments (inline code comments)
comments_json=$(gh api "repos/${repo_nwo}/pulls/${pr_num}/comments" \
  --jq '.[] | {id: .id, user: .user.login, body: .body, path: .path, created_at: .created_at, in_reply_to_id: .in_reply_to_id}' 2>/dev/null) || true

# Fetch PR reviews (top-level review verdicts: APPROVED, CHANGES_REQUESTED, COMMENTED)
reviews_json=$(gh pr view "$pr_num" --json reviews \
  -q '.reviews[] | {id: .id, author: .author.login, state: .state, body: .body, submittedAt: .submittedAt}' 2>/dev/null) || true
```

**Filter already-seen comments** — compare against `.ratchet/watch-state.json`:
```bash
seen_comment_ids=$(jq -r '.seen_comment_ids[]' .ratchet/watch-state.json 2>/dev/null)
seen_review_ids=$(jq -r '.seen_review_ids[]' .ratchet/watch-state.json 2>/dev/null)
```

Skip any comment or review whose ID appears in seen lists. For each **new** comment or review, classify as actionable or informational:

| Category | Signal | Action |
|---|---|---|
| **Actionable: Requested change** | Review state `CHANGES_REQUESTED`, or comment contains specific code suggestion, fix request, or "please change/update/fix" | Create feedback entry |
| **Actionable: Question** | Comment ends with `?` or contains "why", "how", "could you explain" | Create feedback entry |
| **Actionable: Code suggestion** | Comment contains a fenced code block with suggested replacement | Create feedback entry |
| **Informational: Approval** | Review state `APPROVED` | Log only, no action |
| **Informational: Acknowledgment** | Comment body matches LGTM, "looks good", "nice", thumbs up | Log only, no action |

**Filter bot comments** — exclude comments from authenticated `gh` user to avoid self-referential feedback loops:
```bash
bot_user=$(gh api user --jq .login 2>/dev/null) || bot_user=""
# Skip any comment where .user == "$bot_user"
# Skip any review where .author == "$bot_user"
```
If `bot_user` cannot be determined (API failure), proceed without filtering but log a warning:
```
Warning: Could not determine authenticated GitHub user — bot comment filtering disabled
```

**For each actionable comment** (after bot filtering), create a structured feedback entry in plan.yaml:
```bash
# Encode review metadata in description since the schema's additionalProperties: false
# prohibits extra top-level fields. Use source field for deduplication.
# Note: use null (no quotes) for empty affected_scope to produce YAML null, not string "null"
affected_scope_value=$([ -n "$comment_path" ] && echo "\"$comment_path\"" || echo "null")
milestone_id=$(yq eval ".epic.milestones[] | select(.issues[] | .pr == \"$pr_url\") | .id" .ratchet/plan.yaml | head -1)
milestone_id_value=$([ -n "$milestone_id" ] && echo "$milestone_id" || echo "null")

yq eval -i ".epic.discoveries += [{
  \"ref\": \"feedback-review-${pr_num}-$(date +%s)\",
  \"title\": \"Review feedback on PR #${pr_num}: $(echo "$comment_body" | head -c 60)\",
  \"description\": \"Author: $comment_author | File: ${comment_path:-top-level} | Category: $category\n\n$comment_body\",
  \"category\": \"tech-debt\",
  \"severity\": \"major\",
  \"source\": \"pr-review-${pr_num}-${comment_id}\",
  \"status\": \"pending\",
  \"retro_type\": \"review-feedback\",
  \"issue_ref\": \"$issue_ref\",
  \"context\": {
    \"milestone\": $milestone_id_value,
    \"issue\": \"$issue_ref\",
    \"debate\": null
  },
  \"pairs\": [],
  \"affected_scope\": $affected_scope_value,
  \"created_at\": \"$(date -Iseconds)\"
}]" .ratchet/plan.yaml
```

Review metadata (author, file path, comment category) encoded in `description` and `source` fields since discovery schema (`additionalProperties: false`) prohibits extra top-level fields. Use `source: "pr-review-<pr_num>-<comment_id>"` for deduplication — comment ID uniquely identifies each feedback entry.

- Report: "Review feedback detected: PR #[N] — [count] actionable comment(s) from [author(s)]"

**Update watch state** — append all newly processed IDs (both actionable and informational) to `.ratchet/watch-state.json`:
```bash
jq --argjson new_comments "$new_comment_ids_json" \
   --argjson new_reviews "$new_review_ids_json" \
   '.seen_comment_ids += $new_comments | .seen_review_ids += $new_reviews' \
   .ratchet/watch-state.json > .ratchet/watch-state.json.tmp \
   && mv .ratchet/watch-state.json.tmp .ratchet/watch-state.json
```

**Auto-respond behavior** (when `--no-respond` is NOT set and `github-issues` adapter is configured): for each actionable feedback entry, watcher notes the feedback for the response agent (see m4-response-agent). Response agent is a separate concern — this issue only detects and classifies review comments. When `--no-respond` IS set: feedback entries are created in plan.yaml but no response agent is spawned. Feedback surfaces in `/ratchet:status` for manual handling.

**All clear**: No output (silent when nothing is wrong — no conflicts, no CI failures, no new review comments).

### Stopping

`/ratchet:watch --stop` — cancel the loop. No cleanup needed beyond stopping the loop itself.

## Watch State File

Watcher maintains `.ratchet/watch-state.json` to track which review comments and reviews have been processed. Structure:
```json
{
  "seen_comment_ids": [12345, 12346],
  "seen_review_ids": [67890]
}
```

File created automatically on first run if it does not exist. Persists across poll cycles within a session and across sessions (since it lives on disk). To force re-processing of all comments (e.g., after a reset), delete this file:
```bash
rm .ratchet/watch-state.json
```

**Error handling**: If `.ratchet/watch-state.json` exists but contains invalid JSON, watcher recreates it with empty arrays and logs a warning:
```
Warning: .ratchet/watch-state.json was malformed — reset to empty state
```

## Limitations

- **Session-scoped**: loop stops when you close Claude Code
- **Requires `gh` auth**: uses GitHub CLI for PR status and review comment fetching
- **Only watches PRs in plan.yaml**: PRs created outside Ratchet not monitored
- **Comment classification is heuristic**: pattern matching may misclassify edge cases (e.g., rhetorical question as actionable). When in doubt, classified as actionable to avoid missing feedback.
- **Bot comments skipped**: comments from authenticated `gh` user (typically Ratchet bot) excluded to avoid self-referential feedback loops
