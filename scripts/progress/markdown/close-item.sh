#!/usr/bin/env bash
# Markdown progress adapter — close-item
# Marks a markdown progress file as closed.
# Usage: close-item.sh <item-ref>
set -euo pipefail

ITEM_REF="${1:?Usage: close-item.sh <item-ref>}"

FILEPATH=".ratchet/progress/$ITEM_REF"

if [ ! -f "$FILEPATH" ]; then
    echo "Error: Progress item not found: $ITEM_REF" >&2
    exit 1
fi

# Update status to closed (portable temp file approach for macOS/Linux compatibility)
TMP_FILE=$(mktemp)
trap 'rm -f "$TMP_FILE"' EXIT
sed "s/^\*\*Status:\*\* .*/\*\*Status:\*\* closed/" "$FILEPATH" > "$TMP_FILE" && mv "$TMP_FILE" "$FILEPATH"

# Add closing note
cat >> "$FILEPATH" <<EOF

### $(date -u +%Y-%m-%dT%H:%M:%SZ)

**Closed.**

EOF

echo "Closed $ITEM_REF"
