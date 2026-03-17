#!/usr/bin/env bash
# Markdown progress adapter — create-item
# Creates a markdown file in .ratchet/progress/ for tracking work items.
# Usage: create-item.sh <title> <body> [labels...]
# Outputs: item reference (filename)
set -euo pipefail

TITLE="${1:?Usage: create-item.sh <title> <body> [labels...]}"
BODY="${2:-}"
shift 2 || true
LABELS="$*"

PROGRESS_DIR=".ratchet/progress"
mkdir -p "$PROGRESS_DIR" || { echo "Error: Failed to create progress directory: $PROGRESS_DIR" >&2; exit 1; }

# Generate filename from title
SLUG=$(echo "$TITLE" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g' | sed 's/--*/-/g' | sed 's/^-//;s/-$//')
TIMESTAMP=$(date -u +%Y%m%dT%H%M%S)
FILENAME="${SLUG}-${TIMESTAMP}.md"
FILEPATH="$PROGRESS_DIR/$FILENAME"

TMP_FILE=$(mktemp)
trap 'rm -f "$TMP_FILE"' EXIT

cat > "$TMP_FILE" <<EOF
# $TITLE

**Status:** open
**Created:** $(date -u +%Y-%m-%dT%H:%M:%SZ)
**Labels:** ${LABELS:-none}

---

$BODY

---

## Updates

EOF

mv "$TMP_FILE" "$FILEPATH" || {
    echo "Error: Failed to create progress file: $FILEPATH" >&2
    exit 1
}

echo "$FILENAME"
