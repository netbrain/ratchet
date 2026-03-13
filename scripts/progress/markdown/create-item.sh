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
mkdir -p "$PROGRESS_DIR"

# Generate filename from title
SLUG=$(echo "$TITLE" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g' | sed 's/--*/-/g' | sed 's/^-//;s/-$//')
TIMESTAMP=$(date -u +%Y%m%dT%H%M%S)
FILENAME="${SLUG}-${TIMESTAMP}.md"
FILEPATH="$PROGRESS_DIR/$FILENAME"

cat > "$FILEPATH" <<EOF
# $TITLE

**Status:** open
**Created:** $(date -u +%Y-%m-%dT%H:%M:%SZ)
**Labels:** ${LABELS:-none}

---

$BODY

---

## Updates

EOF

echo "$FILENAME"
