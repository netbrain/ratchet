#!/usr/bin/env bash
# Stop gate: warn about unresolved debates when ending a session.
# Advisory only — does not block.

set -euo pipefail

# JSON parsing uses grep/sed for simple key lookups — no external deps (jq, python3, node).
# CAVEAT: If JSON structures become nested or complex, migrate to jq or similar.

# Extract a top-level string value from a JSON file: json_get <file> <key>
json_get() {
    sed -n 's/.*"'"$2"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$1" | head -1
}

RATCHET_DIR=".ratchet"
DEBATES_DIR="$RATCHET_DIR/debates"

if [ ! -d "$RATCHET_DIR" ] || [ ! -d "$DEBATES_DIR" ]; then
    exit 0
fi

active_debates=()

for meta_file in "$DEBATES_DIR"/*/meta.json; do
    [ -f "$meta_file" ] || continue

    status=$(json_get "$meta_file" "status" 2>/dev/null || echo "unknown")
    debate_id=$(json_get "$meta_file" "id" 2>/dev/null || echo "unknown")

    if [ "$status" = "escalated" ] || [ "$status" = "initiated" ]; then
        active_debates+=("$debate_id ($status)")
    fi
done

if [ ${#active_debates[@]} -gt 0 ]; then
    echo ""
    echo "Ratchet: ${#active_debates[@]} unresolved debate(s):"
    for debate in "${active_debates[@]}"; do
        echo "  → $debate"
    done
    echo ""
    echo "Resume with /ratchet:debate [id] or /ratchet:verdict [id]"
fi

exit 0
