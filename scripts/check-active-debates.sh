#!/usr/bin/env bash
# Stop gate: warn about unresolved debates when ending a session.
# Advisory only — does not block.

set -euo pipefail

command -v python3 >/dev/null 2>&1 || { echo "Error: python3 is required but not found" >&2; exit 1; }

RATCHET_DIR=".ratchet"
DEBATES_DIR="$RATCHET_DIR/debates"

if [ ! -d "$RATCHET_DIR" ] || [ ! -d "$DEBATES_DIR" ]; then
    exit 0
fi

active_debates=()

for meta_file in "$DEBATES_DIR"/*/meta.json; do
    [ -f "$meta_file" ] || continue

    status=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['status'])" "$meta_file" 2>/dev/null || echo "unknown")
    debate_id=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['id'])" "$meta_file" 2>/dev/null || echo "unknown")

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
