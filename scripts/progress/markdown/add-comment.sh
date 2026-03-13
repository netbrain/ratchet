#!/usr/bin/env bash
# Markdown progress adapter — add-comment
# Appends a timestamped comment to a markdown progress file.
# Usage: add-comment.sh <item-ref> <body>
set -euo pipefail

ITEM_REF="${1:?Usage: add-comment.sh <item-ref> <body>}"
BODY="${2:?Usage: add-comment.sh <item-ref> <body>}"

FILEPATH=".ratchet/progress/$ITEM_REF"

if [ ! -f "$FILEPATH" ]; then
    echo "Error: Progress item not found: $ITEM_REF" >&2
    exit 1
fi

cat >> "$FILEPATH" <<EOF

### $(date -u +%Y-%m-%dT%H:%M:%SZ)

$BODY

EOF

echo "Comment added to $ITEM_REF"
