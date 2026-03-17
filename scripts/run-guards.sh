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
# Temporarily disable set -e to capture exit code correctly
# Without this, command substitution failure causes immediate exit
set +e
output=$(eval "$GUARD_COMMAND" 2>&1)
exit_code=$?
set -e

# Write result JSON
# Escape special characters in output for JSON embedding
# Complete JSON escaping: backslash, quotes, tabs, and control characters (\r, \f, \b, \n)
escaped_output=$(printf '%s' "$output" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g; s/\r/\\r/g; s/\f/\\f/g' | sed 's/\x08/\\b/g' | tr '\n' ' ')
# Escape the command string for JSON embedding
escaped_command=$(printf '%s' "$GUARD_COMMAND" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g; s/\r/\\r/g; s/\f/\\f/g' | sed 's/\x08/\\b/g' | tr '\n' ' ')
# Escape the guard name for JSON embedding
escaped_guard_name=$(printf '%s' "$GUARD_NAME" | sed 's/\\/\\\\/g; s/"/\\"/g')
timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S+00:00")
if [ "$BLOCKING" = "true" ]; then blocking_json="true"; else blocking_json="false"; fi

# Atomic write: temp file + mv to prevent corrupt JSON on crash
tmp_guard=$(mktemp "${GUARDS_DIR}/${GUARD_NAME}.XXXXXX")
trap 'rm -f "$tmp_guard"' EXIT
cat > "$tmp_guard" <<JSON_EOF
{
  "guard": "$escaped_guard_name",
  "phase": "$PHASE",
  "command": "$escaped_command",
  "exit_code": $exit_code,
  "output": "$escaped_output",
  "blocking": $blocking_json,
  "timestamp": "$timestamp"
}
JSON_EOF
mv "$tmp_guard" "$GUARDS_DIR/$GUARD_NAME.json"

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
