#!/usr/bin/env bash
# Markdown progress adapter — update-status
# Updates the status line in a markdown progress file.
# Usage: update-status.sh <item-ref> <status>
set -euo pipefail

ITEM_REF="${1:?Usage: update-status.sh <item-ref> <status>}"
STATUS="${2:?Usage: update-status.sh <item-ref> <status>}"

FILEPATH=".ratchet/progress/$ITEM_REF"

if [ ! -f "$FILEPATH" ]; then
    echo "Error: Progress item not found: $ITEM_REF" >&2
    exit 1
fi

# Update the status line
sed -i "s/^\*\*Status:\*\* .*/\*\*Status:\*\* $STATUS/" "$FILEPATH"

echo "Updated $ITEM_REF status to: $STATUS"
