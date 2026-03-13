#!/usr/bin/env bash
# Run a single guard and store results.
# Usage: run-guards.sh <milestone-id> <phase> <guard-name> <command> <blocking>
# The caller (Claude via /ratchet:run) reads workflow.yaml and invokes this
# once per guard, passing arguments directly — no YAML parsing needed.
#
# Exit 0 = guard passed (or non-blocking failure), Exit 1 = blocking guard failed

set -euo pipefail

command -v python3 >/dev/null 2>&1 || { echo "Error: python3 is required but not found" >&2; exit 1; }

MILESTONE_ID="${1:?Usage: run-guards.sh <milestone-id> <phase> <guard-name> <command> <blocking>}"
PHASE="${2:?Usage: run-guards.sh <milestone-id> <phase> <guard-name> <command> <blocking>}"
GUARD_NAME="${3:?Usage: run-guards.sh <milestone-id> <phase> <guard-name> <command> <blocking>}"
GUARD_COMMAND="${4:?Usage: run-guards.sh <milestone-id> <phase> <guard-name> <command> <blocking>}"
BLOCKING="${5:-true}"  # "true" or "false"

RATCHET_DIR=".ratchet"
GUARDS_DIR="$RATCHET_DIR/guards/$MILESTONE_ID/$PHASE"

mkdir -p "$GUARDS_DIR"

echo "Running guard: $GUARD_NAME ($GUARD_COMMAND)"

output=""
exit_code=0
output=$(eval "$GUARD_COMMAND" 2>&1) || exit_code=$?

# Write result JSON
python3 -c "
import json, sys
from datetime import datetime, timezone

result = {
    'guard': sys.argv[1],
    'phase': sys.argv[2],
    'command': sys.argv[3],
    'exit_code': int(sys.argv[4]),
    'output': sys.argv[5],
    'blocking': sys.argv[6] == 'true',
    'timestamp': datetime.now(timezone.utc).isoformat()
}

with open(sys.argv[7], 'w') as f:
    json.dump(result, f, indent=2)
    f.write('\n')
" "$GUARD_NAME" "$PHASE" "$GUARD_COMMAND" "$exit_code" "$output" "$BLOCKING" "$GUARDS_DIR/$GUARD_NAME.json"

if [ "$exit_code" -ne 0 ]; then
    if [ "$BLOCKING" = "true" ]; then
        echo "BLOCKED: Guard '$GUARD_NAME' failed (exit $exit_code)"
        exit 1
    else
        echo "ADVISORY: Guard '$GUARD_NAME' failed (exit $exit_code) — non-blocking"
    fi
else
    echo "PASSED: Guard '$GUARD_NAME'"
fi

exit 0
