#!/usr/bin/env bash
# Pre-commit gate: verify all active debates have reached a terminal state.
# Terminal states: "consensus" (pair agreement) or "resolved" (human/tiebreaker verdict).
# Blocks commit if any debate is "escalated" (no verdict) or "initiated" (in progress).
# Non-invasive: allows commit if no .ratchet/ directory or no debates exist.

set -euo pipefail

command -v python3 >/dev/null 2>&1 || { echo "Error: python3 is required but not found" >&2; exit 1; }

RATCHET_DIR=".ratchet"
DEBATES_DIR="$RATCHET_DIR/debates"

# Non-invasive: if no ratchet setup, allow commit
if [ ! -d "$RATCHET_DIR" ] || [ ! -d "$DEBATES_DIR" ]; then
    exit 0
fi

# Check for debates that need resolution
blocking_debates=()

for meta_file in "$DEBATES_DIR"/*/meta.json; do
    [ -f "$meta_file" ] || continue

    status=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['status'])" "$meta_file" 2>/dev/null || echo "unknown")
    debate_id=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['id'])" "$meta_file" 2>/dev/null || echo "unknown")

    case "$status" in
        escalated)
            # Check if there's a verdict file
            verdict_file="$(dirname "$meta_file")/verdict.json"
            if [ ! -f "$verdict_file" ]; then
                blocking_debates+=("$debate_id (escalated, no verdict)")
            fi
            ;;
        initiated)
            blocking_debates+=("$debate_id (debate in progress)")
            ;;
    esac
done

if [ ${#blocking_debates[@]} -gt 0 ]; then
    echo "╔══════════════════════════════════════════════════════╗"
    echo "║  Ratchet: Unresolved debates block this commit      ║"
    echo "╚══════════════════════════════════════════════════════╝"
    echo ""
    for debate in "${blocking_debates[@]}"; do
        echo "  ✗ $debate"
    done
    echo ""
    echo "Resolve with:"
    echo "  /ratchet:verdict [id] [accept|reject|modify]"
    echo "  /ratchet:debate [id] --continue"
    echo ""
    exit 1
fi

exit 0
