#!/usr/bin/env bash
# GitHub Issues progress adapter — add-comment
# Adds a comment to a GitHub issue.
# Usage: add-comment.sh <issue-number> <body>
# Requires: gh CLI authenticated
set -euo pipefail

command -v gh >/dev/null 2>&1 || { echo "Error: gh CLI is required but not found" >&2; exit 1; }

ISSUE_NUM="${1:?Usage: add-comment.sh <issue-number> <body>}"
BODY="${2:?Usage: add-comment.sh <issue-number> <body>}"

gh issue comment "$ISSUE_NUM" --body "$BODY" 2>&1 || {
    echo "Warning: Failed to comment on issue $ISSUE_NUM" >&2
}

echo "Comment added to issue $ISSUE_NUM"
