#!/usr/bin/env bash
# GitHub Issues progress adapter — close-item
# Closes a GitHub issue.
# Usage: close-item.sh <issue-number>
# Requires: gh CLI authenticated
set -euo pipefail

command -v gh >/dev/null 2>&1 || { echo "Error: gh CLI is required but not found" >&2; exit 1; }

ISSUE_NUM="${1:?Usage: close-item.sh <issue-number>}"

if ! gh issue close "$ISSUE_NUM" >/dev/null 2>&1; then
    echo "Error: Failed to close issue $ISSUE_NUM" >&2
    exit 1
fi

echo "Closed issue $ISSUE_NUM"
