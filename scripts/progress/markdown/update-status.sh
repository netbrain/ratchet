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

# Update the status line (portable temp file approach for macOS/Linux compatibility)
# Escape STATUS for sed replacement: / and & and \ are special in the replacement side
ESCAPED_STATUS=$(printf '%s' "$STATUS" | sed 's/[\/&\\]/\\&/g')
TMP_FILE=$(mktemp)
trap 'rm -f "$TMP_FILE"' EXIT
sed "s/^\*\*Status:\*\* .*/\*\*Status:\*\* $ESCAPED_STATUS/" "$FILEPATH" > "$TMP_FILE" && mv "$TMP_FILE" "$FILEPATH"

echo "Updated $ITEM_REF status to: $STATUS"
