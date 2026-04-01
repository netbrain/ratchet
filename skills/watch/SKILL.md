---
name: ratchet:watch
description: Watch active PRs for merge conflicts, CI failures, and review comments
---

# /ratchet:watch — PR Monitor

Watch active Ratchet PRs for merge conflicts, CI failures, and review comments. When problems are detected, automatically creates discoveries in plan.yaml so they surface in `/ratchet:run` and `/ratchet:status`. When actionable review comments are detected, spawns response agents to address feedback directly on the PR branch.

Uses Claude Code's `/loop` feature to poll every 10 minutes.

**Auto-started by `/ratchet:run`** — the run orchestrator starts this loop when PRs exist and stops it on completion. Use `/ratchet:watch` manually only if you want monitoring outside of a run session.

## Usage
```
/ratchet:watch              # Start watching PRs (includes review comment detection)
/ratchet:watch --no-respond # Watch but don't auto-spawn response agents for reviews
/ratchet:watch --stop       # Stop watching
```

## Prerequisites
- `.ratchet/plan.yaml` must exist with issues that have `pr` fields populated
- `gh` CLI must be installed and authenticated

If no PRs exist in plan.yaml, inform the user:
> "No PRs found in plan.yaml. PRs are created by /ratchet:run when issues complete."

## Behavior

### Starting

1. **Verify `gh` authentication** before any GitHub API calls:
   ```bash
   gh auth status >/dev/null 2>&1
   ```
   If the check fails (`exit code != 0`), inform the user via `AskUserQuestion`:
   - Question: "GitHub CLI is not authenticated. /ratchet:watch requires 'gh' to check PR status. Please run 'gh auth login' in your terminal first."
   - Options: `"I've authenticated — retry"`, `"Cancel"`
   If "retry", re-run the `gh auth status` check. If "Cancel", exit the skill.

2. **Resolve workspace** (same logic as `/ratchet:run` Step 1a)
3. **Verify PRs exist** — scan plan.yaml for issues with non-null `pr` fields
4. **Initialize watch state** — load or create `.ratchet/watch-state.json`:
   ```bash
   if [ ! -f .ratchet/watch-state.json ]; then
     echo '{"seen_comment_ids": [], "seen_review_ids": []}' > .ratchet/watch-state.json
   fi
   ```
   The watch state file tracks which review comments and reviews have already been processed to avoid re-processing on subsequent poll cycles.

5. **Parse flags**:
   - `--no-respond`: Disable auto-spawning response agents for review comments. When set, actionable comments are logged as feedback entries but no agent is spawned. Default behavior (without this flag) is to auto-respond when the `github-issues` progress adapter is configured in `workflow.yaml`.
   - `--stop`: Cancel the loop and exit.

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

Skip any comment or review whose ID appears in the seen lists. For each **new** comment or review:

**Classify as actionable or informational:**

| Category | Signal | Action |
|---|---|---|
| **Actionable: Requested change** | Review state `CHANGES_REQUESTED`, or comment contains specific code suggestion, fix request, or "please change/update/fix" | Create feedback entry |
| **Actionable: Question** | Comment ends with `?` or contains "why", "how", "could you explain" | Create feedback entry |
| **Actionable: Code suggestion** | Comment contains a fenced code block with suggested replacement | Create feedback entry |
| **Informational: Approval** | Review state `APPROVED` | Log only, no action |
| **Informational: Acknowledgment** | Comment body matches LGTM, "looks good", "nice", thumbs up | Log only, no action |

**Filter bot comments** — exclude comments from the authenticated `gh` user to avoid self-referential feedback loops:
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
yq eval -i ".epic.discoveries += [{
  \"ref\": \"feedback-review-${pr_num}-$(date +%s)\",
  \"title\": \"Review feedback on PR #${pr_num}: $(echo "$comment_body" | head -c 60)\",
  \"description\": \"$comment_body\",
  \"source\": \"pr-review-${pr_num}\",
  \"created_at\": \"$(date -Iseconds)\",
  \"severity\": \"major\",
  \"status\": \"pending\",
  \"retro_type\": \"review-feedback\",
  \"issue_ref\": \"$issue_ref\",
  \"review_comment_id\": $comment_id,
  \"review_file_path\": \"$comment_path\",
  \"review_author\": \"$comment_author\",
  \"review_category\": \"$category\"
}]" .ratchet/plan.yaml
```

The feedback discovery uses flat top-level fields prefixed with `review_` to remain consistent with the existing discovery schema pattern (all flat key-value pairs, no nested objects). Fields:
- `review_comment_id`: The GitHub comment ID for deduplication and linking
- `review_file_path`: The file path the comment is attached to (null for top-level reviews)
- `review_author`: The GitHub username who left the comment
- `review_category`: One of `requested_change`, `question`, `code_suggestion`

- Report: "Review feedback detected: PR #[N] — [count] actionable comment(s) from [author(s)]"

**Update watch state** — append all newly processed IDs (both actionable and informational) to `.ratchet/watch-state.json`:
```bash
jq --argjson new_comments "$new_comment_ids_json" \
   --argjson new_reviews "$new_review_ids_json" \
   '.seen_comment_ids += $new_comments | .seen_review_ids += $new_reviews' \
   .ratchet/watch-state.json > .ratchet/watch-state.json.tmp \
   && mv .ratchet/watch-state.json.tmp .ratchet/watch-state.json
```

**Auto-respond behavior** (when `--no-respond` is NOT set and `github-issues` adapter is configured):
- For each actionable feedback entry, the watcher notes the feedback for the response agent (see m4-response-agent). The response agent is a separate concern — this issue only detects and classifies review comments.
- When `--no-respond` IS set: feedback entries are created in plan.yaml but no response agent is spawned. The feedback surfaces in `/ratchet:status` for manual handling.

**All clear**: No output (silent when nothing is wrong — no conflicts, no CI failures, no new review comments).

### Stopping

`/ratchet:watch --stop` — cancel the loop. No cleanup needed beyond stopping the loop itself.

## Watch State File

The watcher maintains `.ratchet/watch-state.json` to track which review comments and reviews have been processed. Structure:

```json
{
  "seen_comment_ids": [12345, 12346],
  "seen_review_ids": [67890]
}
```

This file is created automatically on first run if it does not exist. It persists across poll cycles within a session and across sessions (since it lives on disk). To force re-processing of all comments (e.g., after a reset), delete this file:

```bash
rm .ratchet/watch-state.json
```

**Error handling**: If `.ratchet/watch-state.json` exists but contains invalid JSON, the watcher recreates it with empty arrays and logs a warning:
```
Warning: .ratchet/watch-state.json was malformed — reset to empty state
```

## Limitations

- **Session-scoped**: the loop stops when you close Claude Code
- **Requires `gh` auth**: uses GitHub CLI for PR status and review comment fetching
- **Only watches PRs in plan.yaml**: PRs created outside Ratchet are not monitored
- **Comment classification is heuristic**: the actionable vs informational classification uses pattern matching — edge cases may misclassify (e.g., a rhetorical question classified as actionable). When in doubt, comments are classified as actionable to avoid missing feedback.
- **Bot comments are skipped**: comments from the authenticated `gh` user (typically the bot running Ratchet) are excluded from processing to avoid self-referential feedback loops
