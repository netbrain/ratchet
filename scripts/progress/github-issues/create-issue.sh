#!/usr/bin/env bash
# GitHub Issues progress adapter — create-issue
# Creates a GitHub issue for a plan.yaml work item, linked to a parent milestone issue.
# Usage: create-issue.sh <title> <body> [--parent <issue-number>] [--label <label>]
# Returns: the created issue number on stdout
# Requires: gh CLI authenticated
set -euo pipefail

command -v gh >/dev/null 2>&1 || { echo "Error: gh CLI is required but not found" >&2; exit 1; }

TITLE=""
BODY=""
PARENT=""
LABELS=("ratchet")

while [ "$#" -gt 0 ]; do
    case "$1" in
        --parent)
            PARENT="$2"
            shift 2
            ;;
        --label)
            LABELS+=("$2")
            shift 2
            ;;
        *)
            if [ -z "$TITLE" ]; then
                TITLE="$1"
            elif [ -z "$BODY" ]; then
                BODY="$1"
            fi
            shift
            ;;
    esac
done

if [ -z "$TITLE" ]; then
    echo "Error: title is required" >&2
    echo "Usage: create-issue.sh <title> <body> [--parent <issue-number>] [--label <label>]" >&2
    exit 1
fi

# Build the issue body with parent reference and ratchet sentinel
ISSUE_BODY=""
if [ -n "$PARENT" ]; then
    ISSUE_BODY="Part of #${PARENT}

"
fi
if [ -n "$BODY" ]; then
    ISSUE_BODY="${ISSUE_BODY}${BODY}

"
fi
ISSUE_BODY="${ISSUE_BODY}---
<!-- ratchet-managed -->"

# Build label args
LABEL_ARGS=""
for label in "${LABELS[@]}"; do
    LABEL_ARGS="${LABEL_ARGS} --label ${label}"
done

# Create the issue and extract the number
# shellcheck disable=SC2086
ISSUE_URL=$(gh issue create --title "$TITLE" --body "$ISSUE_BODY" $LABEL_ARGS 2>/dev/null) || {
    echo "Error: Failed to create GitHub issue" >&2
    exit 1
}

# Extract issue number from URL (https://github.com/owner/repo/issues/123)
ISSUE_NUM=$(echo "$ISSUE_URL" | grep -oE '[0-9]+$')

if [ -z "$ISSUE_NUM" ]; then
    echo "Error: Could not parse issue number from: $ISSUE_URL" >&2
    exit 1
fi

echo "$ISSUE_NUM"
