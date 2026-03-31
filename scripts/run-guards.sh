#!/usr/bin/env bash
# Run a single guard and store results.
# Usage: run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command> <blocking>
# The caller (Claude via /ratchet:run) reads workflow.yaml and invokes this
# once per guard, passing arguments directly — no YAML parsing needed.
#
# Exit 0 = guard passed (or non-blocking failure), Exit 1 = blocking guard failed

set -euo pipefail

# JSON output uses printf — no external deps (jq, python3, node).
# CAVEAT: If JSON structures become nested or complex, migrate to jq or similar.

MILESTONE_ID="${1:?Usage: run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command> <blocking>}"
ISSUE_REF="${2:?Usage: run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command> <blocking>}"
PHASE="${3:?Usage: run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command> <blocking>}"
GUARD_NAME="${4:?Usage: run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command> <blocking>}"
GUARD_COMMAND="${5:?Usage: run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command> <blocking>}"
BLOCKING="${6:-true}"  # "true" or "false"

RATCHET_DIR=".ratchet"
GUARDS_DIR="$RATCHET_DIR/guards/$MILESTONE_ID/$ISSUE_REF/$PHASE"

mkdir -p "$GUARDS_DIR" || { echo "Error: Failed to create guards directory: $GUARDS_DIR" >&2; exit 1; }

echo "Running guard: $GUARD_NAME ($GUARD_COMMAND)"

stdout_output=""
stderr_output=""
exit_code=0
# Capture stdout and stderr separately to match the documented JSON schema.
# Uses a temp file for stderr since bash cannot capture both streams in variables directly.
stderr_tmp=$(mktemp)
trap 'rm -f "$stderr_tmp"' EXIT
# Temporarily disable set -e to capture exit code correctly
# Without this, command substitution failure causes immediate exit
set +e
stdout_output=$(bash -c "$GUARD_COMMAND" 2>"$stderr_tmp")
exit_code=$?
stderr_output=$(cat "$stderr_tmp")
rm -f "$stderr_tmp"
set -e

# Write result JSON
# Escape special characters for JSON embedding
# Complete JSON escaping: backslash, quotes, tabs, and control characters (\r, \f, \b, \n)
escaped_stdout=$(printf '%s' "$stdout_output" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g; s/\r/\\r/g; s/\f/\\f/g' | sed 's/\x08/\\b/g' | tr '\n' ' ')
escaped_stderr=$(printf '%s' "$stderr_output" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g; s/\r/\\r/g; s/\f/\\f/g' | sed 's/\x08/\\b/g' | tr '\n' ' ')
# Escape the command string for JSON embedding
escaped_command=$(printf '%s' "$GUARD_COMMAND" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g; s/\r/\\r/g; s/\f/\\f/g' | sed 's/\x08/\\b/g' | tr '\n' ' ')
# Escape the guard name for JSON embedding
escaped_guard_name=$(printf '%s' "$GUARD_NAME" | sed 's/\\/\\\\/g; s/"/\\"/g')
timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S+00:00")
if [ "$BLOCKING" = "true" ]; then blocking_json="true"; else blocking_json="false"; fi
if [ "$exit_code" -eq 0 ]; then passed_json="true"; else passed_json="false"; fi

# Atomic write with advisory locking: flock + temp file + mv
# flock prevents concurrent guards from conflicting on the same directory.
# Falls back to unlocked write if flock is unavailable (e.g., macOS without flock).
LOCK_FILE="$GUARDS_DIR/.guard-write.lock"
tmp_guard=$(mktemp "${GUARDS_DIR}/${GUARD_NAME}.XXXXXX")
trap 'rm -f "$tmp_guard"' EXIT

write_guard_json() {
    cat > "$tmp_guard" <<JSON_EOF
{
  "guard": "$escaped_guard_name",
  "command": "$escaped_command",
  "exit_code": $exit_code,
  "stdout": "$escaped_stdout",
  "stderr": "$escaped_stderr",
  "passed": $passed_json,
  "blocking": $blocking_json,
  "timestamp": "$timestamp",
  "overridden": false,
  "override_reason": null
}
JSON_EOF
    mv "$tmp_guard" "$GUARDS_DIR/$GUARD_NAME.json"
}

if command -v flock >/dev/null 2>&1; then
    # Use fd 9 for flock to avoid interfering with stdin/stdout/stderr
    exec 9>"$LOCK_FILE"
    flock -w 30 9 || { echo "Error: Failed to acquire lock on $LOCK_FILE" >&2; exit 1; }
    write_guard_json
    exec 9>&-
else
    # Fallback: no flock available (macOS without homebrew coreutils)
    write_guard_json
fi

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
