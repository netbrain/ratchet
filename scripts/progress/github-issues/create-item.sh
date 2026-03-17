#!/usr/bin/env bash
# GitHub Issues progress adapter — create-item
# Creates a GitHub issue for tracking a work item.
# Usage: create-item.sh <title> <body> [labels...]
# Outputs: issue number (e.g., "42")
# Requires: gh CLI authenticated
set -euo pipefail

command -v gh >/dev/null 2>&1 || { echo "Error: gh CLI is required but not found" >&2; exit 1; }

TITLE="${1:?Usage: create-item.sh <title> <body> [labels...]}"
BODY="${2:-}"
shift 2 || true

# Build label args as array to handle labels with spaces correctly
LABEL_ARGS=()
for label in "$@"; do
    LABEL_ARGS+=(--label "$label")
done

# Create issue and capture the number
ISSUE_URL=$(gh issue create --title "$TITLE" --body "$BODY" ${LABEL_ARGS[@]+"${LABEL_ARGS[@]}"} 2>&1) || {
    echo "Error: Failed to create GitHub issue: $ISSUE_URL" >&2
    exit 1
}

# Extract issue number from URL (https://github.com/owner/repo/issues/42 -> 42)
ISSUE_NUM=$(echo "$ISSUE_URL" | grep -oE '[0-9]+$')
echo "$ISSUE_NUM"
