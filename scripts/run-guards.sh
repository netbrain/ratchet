#!/usr/bin/env bash
# Run a single guard and store results.
# Usage (standard):
#   run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command> <blocking> [standard]
# Usage (rationalization-check):
#   run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <checks-json> <blocking> rationalization-check
#
# For rationalization-check guards, <checks-json> is a JSON array:
#   [{"assert":"description","command":"shell command"}, ...]
# Each check is run independently; the guard passes only if ALL checks pass.
#
# The caller (Claude via /ratchet:run) reads workflow.yaml and invokes this
# once per guard, passing arguments directly — no YAML parsing needed.
#
# Exit 0 = guard passed (or non-blocking failure), Exit 1 = blocking guard failed

set -euo pipefail

# JSON output uses printf — no external deps for standard guards.
# rationalization-check guards require jq to parse the checks array.

MILESTONE_ID="${1:?Usage: run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command|checks-json> <blocking> [guard-type]}"
ISSUE_REF="${2:?Usage: run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command|checks-json> <blocking> [guard-type]}"
PHASE="${3:?Usage: run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command|checks-json> <blocking> [guard-type]}"
GUARD_NAME="${4:?Usage: run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command|checks-json> <blocking> [guard-type]}"
GUARD_COMMAND="${5:?Usage: run-guards.sh <milestone-id> <issue-ref> <phase> <guard-name> <command|checks-json> <blocking> [guard-type]}"
BLOCKING="${6:-true}"  # "true" or "false"
GUARD_TYPE="${7:-standard}"  # "standard" or "rationalization-check"

RATCHET_DIR=".ratchet"
GUARDS_DIR="$RATCHET_DIR/guards/$MILESTONE_ID/$ISSUE_REF/$PHASE"

mkdir -p "$GUARDS_DIR" || { echo "Error: Failed to create guards directory: $GUARDS_DIR" >&2; exit 1; }

# --- Helper: escape a string for safe JSON embedding ---
# Complete JSON escaping: backslash, quotes, tabs, and control characters (\r, \f, \b, \n)
json_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g; s/\r/\\r/g; s/\f/\\f/g' | sed 's/\x08/\\b/g' | tr '\n' ' '
}

# --- Helper: atomic write of guard result JSON ---
# Uses flock if available, falls back to direct write.
atomic_write_json() {
    local json_content="$1"
    local target_file="$2"
    local lock_file="$GUARDS_DIR/.guard-write.lock"

    local tmp_guard
    tmp_guard=$(mktemp "${GUARDS_DIR}/${GUARD_NAME}.XXXXXX")

    # Write JSON to temp file
    printf '%s\n' "$json_content" > "$tmp_guard"

    if command -v flock >/dev/null 2>&1; then
        # Use fd 9 for flock to avoid interfering with stdin/stdout/stderr
        exec 9>"$lock_file"
        flock -w 30 9 || { echo "Error: Failed to acquire lock on $lock_file" >&2; rm -f "$tmp_guard"; exit 1; }
        mv "$tmp_guard" "$target_file"
        exec 9>&-
    else
        # Fallback: no flock available (macOS without homebrew coreutils)
        mv "$tmp_guard" "$target_file"
    fi
}

# --- Standard guard execution ---
run_standard_guard() {
    echo "Running guard: $GUARD_NAME ($GUARD_COMMAND)"

    local stdout_output=""
    local stderr_output=""
    local exit_code=0
    # Capture stdout and stderr separately to match the documented JSON schema.
    local stderr_tmp
    stderr_tmp=$(mktemp)
    trap 'rm -f "$stderr_tmp"' EXIT

    # Temporarily disable set -e to capture exit code correctly
    set +e
    stdout_output=$(bash -c "$GUARD_COMMAND" 2>"$stderr_tmp")
    exit_code=$?
    stderr_output=$(cat "$stderr_tmp")
    rm -f "$stderr_tmp"
    set -e

    # Build result JSON
    local escaped_stdout escaped_stderr escaped_command escaped_guard_name timestamp blocking_json passed_json
    escaped_stdout=$(json_escape "$stdout_output")
    escaped_stderr=$(json_escape "$stderr_output")
    escaped_command=$(json_escape "$GUARD_COMMAND")
    escaped_guard_name=$(json_escape "$GUARD_NAME")
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S+00:00")
    if [ "$BLOCKING" = "true" ]; then blocking_json="true"; else blocking_json="false"; fi
    if [ "$exit_code" -eq 0 ]; then passed_json="true"; else passed_json="false"; fi

    local json_content
    json_content=$(cat <<JSON_EOF
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
)

    atomic_write_json "$json_content" "$GUARDS_DIR/$GUARD_NAME.json"

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
}

# --- Rationalization-check guard execution ---
run_rationalization_check_guard() {
    echo "Running rationalization-check guard: $GUARD_NAME"

    # Validate jq is available — required for parsing checks JSON
    if ! command -v jq >/dev/null 2>&1; then
        echo "Error: jq is required for rationalization-check guards but not found in PATH" >&2
        exit 1
    fi

    # Validate the checks JSON parses correctly
    local checks_json="$GUARD_COMMAND"
    if ! printf '%s' "$checks_json" | jq empty 2>/dev/null; then
        echo "Error: Invalid checks JSON for guard '$GUARD_NAME'" >&2
        echo "Expected format: [{\"assert\":\"description\",\"command\":\"shell cmd\"}, ...]" >&2
        exit 1
    fi

    local check_count
    check_count=$(printf '%s' "$checks_json" | jq 'length')
    if [ "$check_count" -eq 0 ]; then
        echo "Error: Empty checks array for guard '$GUARD_NAME'" >&2
        exit 1
    fi

    echo "  $check_count assertion(s) to verify"

    local all_passed=true
    local failed_count=0
    local passed_count=0
    local checks_results="["
    local first_check=true

    local i=0
    while [ "$i" -lt "$check_count" ]; do
        local assert_desc command_str
        assert_desc=$(printf '%s' "$checks_json" | jq -r ".[$i].assert")
        command_str=$(printf '%s' "$checks_json" | jq -r ".[$i].command")

        local check_stdout="" check_stderr="" check_exit=0
        local check_stderr_tmp
        check_stderr_tmp=$(mktemp)

        # Run the check command
        set +e
        check_stdout=$(bash -c "$command_str" 2>"$check_stderr_tmp")
        check_exit=$?
        check_stderr=$(cat "$check_stderr_tmp")
        rm -f "$check_stderr_tmp"
        set -e

        local check_passed_json
        if [ "$check_exit" -eq 0 ]; then
            check_passed_json="true"
            passed_count=$((passed_count + 1))
            echo "  PASS: $assert_desc"
        else
            check_passed_json="false"
            all_passed=false
            failed_count=$((failed_count + 1))
            echo "  FAIL: $assert_desc (exit $check_exit)"
        fi

        # Build per-check result JSON fragment
        local escaped_assert escaped_cmd escaped_check_stdout escaped_check_stderr
        escaped_assert=$(json_escape "$assert_desc")
        escaped_cmd=$(json_escape "$command_str")
        escaped_check_stdout=$(json_escape "$check_stdout")
        escaped_check_stderr=$(json_escape "$check_stderr")

        if [ "$first_check" = "true" ]; then
            first_check=false
        else
            checks_results="$checks_results,"
        fi

        checks_results="$checks_results
    {
      \"assert\": \"$escaped_assert\",
      \"command\": \"$escaped_cmd\",
      \"exit_code\": $check_exit,
      \"stdout\": \"$escaped_check_stdout\",
      \"stderr\": \"$escaped_check_stderr\",
      \"passed\": $check_passed_json
    }"

        i=$((i + 1))
    done

    checks_results="$checks_results
  ]"

    # Build overall result
    local escaped_guard_name timestamp blocking_json overall_passed_json
    escaped_guard_name=$(json_escape "$GUARD_NAME")
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S+00:00")
    if [ "$BLOCKING" = "true" ]; then blocking_json="true"; else blocking_json="false"; fi
    if [ "$all_passed" = "true" ]; then overall_passed_json="true"; else overall_passed_json="false"; fi

    local json_content
    json_content=$(cat <<JSON_EOF
{
  "guard": "$escaped_guard_name",
  "type": "rationalization-check",
  "checks": $checks_results,
  "passed": $overall_passed_json,
  "passed_count": $passed_count,
  "failed_count": $failed_count,
  "total_count": $check_count,
  "blocking": $blocking_json,
  "timestamp": "$timestamp",
  "overridden": false,
  "override_reason": null
}
JSON_EOF
)

    atomic_write_json "$json_content" "$GUARDS_DIR/$GUARD_NAME.json"

    # Report summary
    echo "  ---"
    echo "  Results: $passed_count/$check_count passed"

    if [ "$all_passed" = "true" ]; then
        echo "PASSED: Guard '$GUARD_NAME' (all $check_count assertions hold)"
    else
        if [ "$BLOCKING" = "true" ]; then
            echo "BLOCKED: Guard '$GUARD_NAME' failed ($failed_count/$check_count assertions violated)"
            exit 1
        else
            echo "ADVISORY: Guard '$GUARD_NAME' failed ($failed_count/$check_count assertions violated) — non-blocking"
        fi
    fi

    exit 0
}

# --- Dispatch based on guard type ---
case "$GUARD_TYPE" in
    standard)
        run_standard_guard
        ;;
    rationalization-check)
        run_rationalization_check_guard
        ;;
    *)
        echo "Error: Unknown guard type '$GUARD_TYPE'. Expected 'standard' or 'rationalization-check'." >&2
        exit 1
        ;;
esac
