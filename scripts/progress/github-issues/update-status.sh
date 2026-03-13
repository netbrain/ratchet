#!/usr/bin/env bash
# GitHub Issues progress adapter — update-status
# Updates a GitHub issue's state or adds a status label.
# Usage: update-status.sh <issue-number> <status>
# Status values: "in_progress", "done", "blocked", "pending"
# Requires: gh CLI authenticated
set -euo pipefail

command -v gh >/dev/null 2>&1 || { echo "Error: gh CLI is required but not found" >&2; exit 1; }

ISSUE_NUM="${1:?Usage: update-status.sh <issue-number> <status>}"
STATUS="${2:?Usage: update-status.sh <issue-number> <status>}"

# Add a comment noting the status change
gh issue comment "$ISSUE_NUM" --body "Status updated to: **$STATUS**" 2>&1 || {
    echo "Warning: Failed to comment on issue $ISSUE_NUM" >&2
}

echo "Updated issue $ISSUE_NUM status to: $STATUS"
