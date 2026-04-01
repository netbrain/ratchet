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
4. **Start loop**:
   ```
   /loop 10m check Ratchet PRs for conflicts and CI failures
   ```
5. **Confirm**: "Watching [N] PRs. Checking every 10 minutes. Use /ratchet:watch --stop to end."

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

---

## Review Response Flow

When the watcher detects actionable review comments on a PR, it spawns a response agent to address the feedback directly on the PR branch. This keeps the feedback loop tight without requiring manual intervention.

### Triggering

Each poll cycle, after checking merge conflicts and CI status, check for new review comments:

```bash
pr_num=$(echo "$pr_url" | grep -oP 'pull/\K\d+')

# Fetch all review comments (inline + top-level)
comments_json=$(gh api "repos/{owner}/{repo}/pulls/$pr_num/comments" --paginate 2>/dev/null || echo "[]")
review_comments_json=$(gh api "repos/{owner}/{repo}/pulls/$pr_num/reviews" --paginate 2>/dev/null || echo "[]")
```

**Filter for actionable comments** — skip comments that are:
- Already seen (tracked in `.ratchet/watch-state.json` via `scripts/watch-state.sh`)
- Authored by the bot/current user (no self-response loops)
- Pure praise or acknowledgment (no actionable content)
- Already resolved in the GitHub UI

```bash
# Mark comments as seen to avoid re-processing
scripts/watch-state.sh mark-seen "$pr_num" "$comment_id"

# Check if already seen
scripts/watch-state.sh is-seen "$pr_num" "$comment_id"
```

If no new actionable comments exist, skip to the next PR.

### --dry-run-reviews Flag

When `/ratchet:watch --dry-run-reviews` is active, the watcher reports what it *would* address without taking action:

```
/ratchet:watch --dry-run-reviews
```

Output format:
```
PR #42: 3 actionable review comments detected
  Comment by @reviewer (id: 123456): "This function should validate input before..."
    → Would modify: src/validator.ts (lines 15-22)
  Comment by @reviewer (id: 123457): "Missing error handling for the edge case..."
    → Would modify: src/handler.ts (lines 45-50)
  Comment by @reviewer (id: 123458): "Consider using a Map instead of object..."
    → Would modify: src/cache.ts (lines 8-12)
Dry run complete. No changes made.
```

No worktree is created, no commits are made, no replies are posted.

### Response Pipeline

When actionable comments are detected and `--dry-run-reviews` is NOT set:

#### Step 1: Create Worktree from PR Branch

```bash
pr_branch=$(gh pr view "$pr_num" --json headRefName -q .headRefName)
worktree_path=".claude/worktrees/review-response-pr-$pr_num"

# Clean up any stale worktree from a previous run
if [ -d "$worktree_path" ]; then
    git worktree remove "$worktree_path" 2>/dev/null || git worktree remove --force "$worktree_path"
fi

git fetch origin "$pr_branch"
git worktree add "$worktree_path" "origin/$pr_branch" || {
    echo "Error: Failed to create worktree for PR #$pr_num branch $pr_branch" >&2
    # Skip this PR, continue to next
    continue
}
```

#### Step 2: Reconstruct Agent Context

Gather all context the response agent needs to understand the original work:

```bash
# 1. PR diff — what changed in this PR
pr_diff=$(gh pr diff "$pr_num" 2>/dev/null || echo "")

# 2. Debate transcript — find the debate that produced this PR
issue_ref=$(yq eval ".epic.milestones[].issues[] | select(.pr == \"$pr_url\") | .ref" .ratchet/plan.yaml)
debate_dir=""

# Check active debates first
for d in .ratchet/debates/*/meta.json; do
    [ -f "$d" ] || continue
    debate_issue=$(sed -n 's/.*"issue_ref"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$d" | head -1)
    if [ "$debate_issue" = "$issue_ref" ]; then
        debate_dir=$(dirname "$d")
        break
    fi
done

# Fall back to archive if not found in active debates
if [ -z "$debate_dir" ]; then
    for d in .ratchet/archive/debates/*/meta.json; do
        [ -f "$d" ] || continue
        debate_issue=$(sed -n 's/.*"issue_ref"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$d" | head -1)
        if [ "$debate_issue" = "$issue_ref" ]; then
            debate_dir=$(dirname "$d")
            break
        fi
    done
fi

# 3. Pair definition — same pair used in original debate
pair_name=""
if [ -n "$debate_dir" ] && [ -f "$debate_dir/meta.json" ]; then
    pair_name=$(sed -n 's/.*"pair"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$debate_dir/meta.json" | head -1)
fi

# 4. Review comments — the actionable feedback to address
# (already fetched above in the triggering step)
```

#### Step 3: Spawn Response Agent

The response agent uses the **same generative model and pair definition** as the original debate. It is constrained to the PR's file scope.

```bash
# Determine file scope from the PR diff
pr_files=$(gh pr diff "$pr_num" --name-only 2>/dev/null || echo "")
```

Spawn the agent with:
- **Model**: Same as the original debate's generative model (from `.ratchet/workflow.yaml` or pair definition)
- **Context**: PR diff, debate transcript, pair definition, review comments
- **Constraint**: May only modify files within `$pr_files` scope
- **Task**: Address the actionable review comments

```
Agent(prompt, model: "<generative-model>")
```

The prompt includes:
```
You are a review-response agent. Address the following PR review comments
by making targeted fixes in the codebase.

## Constraints
- You may ONLY modify these files: [pr_files list]
- Make minimal, focused changes that directly address each comment
- Do not refactor beyond what the reviewer requested
- Preserve existing code style and patterns

## Context
### PR Diff
[pr_diff]

### Debate Transcript
[debate transcript content, if available]

### Pair Definition
[pair generative.md content, if available]

### Review Comments to Address
[formatted list of actionable comments with file, line, and body]
```

#### Step 4: Run Blocking Guards

After the agent completes, run all blocking guards for the review phase:

```bash
# Read guards from workflow.yaml for the review phase
# Run each blocking guard in the worktree
cd "$worktree_path" || { echo "Error: Worktree disappeared: $worktree_path" >&2; continue; }

# Use run-guards.sh for each guard
for guard in $(yq eval '.guards[] | select(.timing == "post-review" and .blocking == true) | .name' .ratchet/workflow.yaml 2>/dev/null); do
    guard_command=$(yq eval ".guards[] | select(.name == \"$guard\") | .command" .ratchet/workflow.yaml)
    scripts/run-guards.sh "review-response" "$issue_ref" "review" "$guard" "$guard_command" "true" "standard"
done
```

#### Step 5: Commit and Push (if guards pass)

```bash
cd "$worktree_path" || { echo "Error: Worktree disappeared: $worktree_path" >&2; continue; }

# Check if there are actual changes
if git diff --quiet && git diff --cached --quiet; then
    echo "No changes made by response agent for PR #$pr_num"
else
    # Stage only files within the PR's scope
    echo "$pr_files" | while read -r f; do
        [ -f "$f" ] && git add "$f"
    done

    # Create fixup commit
    git commit -m "fixup! Address review feedback on PR #$pr_num

Addressed comments:
$(echo "$addressed_comments" | sed 's/^/- /')"

    # Push to the PR branch
    git push origin HEAD:"$pr_branch" || {
        echo "Error: Failed to push review response for PR #$pr_num" >&2
        # Do not clean up worktree — leave for manual inspection
        continue
    }
fi
```

#### Step 6: Reply on PR

Post a reply comment on the PR noting which feedback was addressed:

```bash
# Build reply body
reply_body="Addressed the following review feedback:

$(for comment_id in $addressed_comment_ids; do
    comment_body=$(echo "$comments_json" | jq -r ".[] | select(.id == $comment_id) | .body" | head -c 100)
    echo "- **Comment $comment_id**: $comment_body..."
done)

Changes pushed as fixup commit."

# Post as a PR comment
gh pr comment "$pr_num" --body "$reply_body"

# Mark all addressed comments as seen
for comment_id in $addressed_comment_ids; do
    scripts/watch-state.sh mark-seen "$pr_num" "$comment_id"
done
```

#### Step 7: Cleanup Worktree

```bash
git worktree remove "$worktree_path" 2>/dev/null || git worktree remove --force "$worktree_path"
```

If cleanup fails, log a warning but do not block:
```bash
echo "Warning: Failed to remove worktree $worktree_path — manual cleanup may be needed" >&2
```

### Guard Failure Handling

If any blocking guard fails in Step 4:

1. **Do not commit or push** — the changes stay in the worktree
2. **Do not reply on PR** — no misleading "addressed" comment
3. **Create a discovery** in plan.yaml:
   ```bash
   yq eval -i ".epic.discoveries += [{
       \"ref\": \"discovery-review-guard-$(date +%s)\",
       \"title\": \"Review response guard failure for PR #$pr_num\",
       \"description\": \"Guard '$guard' failed when addressing review comments on PR #$pr_num ($issue_ref). Manual intervention required.\",
       \"category\": \"bug\",
       \"severity\": \"major\",
       \"source\": \"review-response-guard-$pr_num\",
       \"created_at\": \"$(date -Iseconds)\",
       \"status\": \"pending\",
       \"issue_ref\": \"$issue_ref\"
   }]" .ratchet/plan.yaml
   ```
4. **Clean up worktree** — remove it since the changes are not viable
5. Report: "Review response for PR #[N] blocked by guard '[name]' — discovery created"

### Error Handling

- **`gh` API failures**: Log to stderr, skip the PR, continue to next. Do not crash the watcher loop.
- **Worktree creation failure**: Log error, skip PR. Common cause: branch already checked out elsewhere.
- **Agent spawn failure**: Log error, clean up worktree, skip PR.
- **Push failure**: Log error, leave worktree for manual inspection (changes may be valuable).
- **Comment parsing failure**: Log warning, skip unparseable comments, process remaining.

---

## Limitations

- **Session-scoped**: stops when you close Claude Code
- **Requires `gh` auth**: uses GitHub CLI for PR status
- **Only watches PRs in plan.yaml**: PRs created outside Ratchet are not monitored
- **Review response is best-effort**: guard failures or push conflicts will skip the response and create a discovery instead
- **No interactive resolution**: the response agent cannot ask clarifying questions to the reviewer
- **File scope constraint**: the response agent can only modify files already in the PR diff
