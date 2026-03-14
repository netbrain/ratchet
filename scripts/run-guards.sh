#!/usr/bin/env bash
# Run a single guard and store results.
# Usage: run-guards.sh <milestone-id> <phase> <guard-name> <command> <blocking>
# The caller (Claude via /ratchet:run) reads workflow.yaml and invokes this
# once per guard, passing arguments directly — no YAML parsing needed.
#
# Exit 0 = guard passed (or non-blocking failure), Exit 1 = blocking guard failed

set -euo pipefail

# JSON output uses printf — no external deps (jq, python3, node).
# CAVEAT: If JSON structures become nested or complex, migrate to jq or similar.

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
# Escape special characters in output for JSON embedding
escaped_output=$(printf '%s' "$output" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g' | tr '\n' ' ')
timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S+00:00")
if [ "$BLOCKING" = "true" ]; then blocking_json="true"; else blocking_json="false"; fi

cat > "$GUARDS_DIR/$GUARD_NAME.json" <<JSON_EOF
{
  "guard": "$GUARD_NAME",
  "phase": "$PHASE",
  "command": "$GUARD_COMMAND",
  "exit_code": $exit_code,
  "output": "$escaped_output",
  "blocking": $blocking_json,
  "timestamp": "$timestamp"
}
JSON_EOF

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
