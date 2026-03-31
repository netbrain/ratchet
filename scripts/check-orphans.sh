#!/usr/bin/env bash
# Detect orphaned Ratchet state: stale issues, unresolved debates, orphan worktrees,
# and incomplete execution logs.
# Advisory only — always exits 0.
# Usage: check-orphans.sh [--max-age HOURS] [--ratchet-dir DIR]
#
# Output: JSON array of findings, each with type, ref, age, and suggested_action.

set -euo pipefail

# --- Defaults ---
MAX_AGE_HOURS=24
RATCHET_DIR=".ratchet"

# --- Argument parsing ---
while [ $# -gt 0 ]; do
    case "$1" in
        --max-age)
            if [ $# -lt 2 ]; then
                echo "Error: --max-age requires a value (hours)" >&2
                exit 0  # advisory — do not block
            fi
            MAX_AGE_HOURS="$2"
            if ! printf '%s' "$MAX_AGE_HOURS" | grep -qE '^[0-9]+$'; then
                echo "Error: --max-age must be a positive integer, got: $MAX_AGE_HOURS" >&2
                exit 0
            fi
            shift 2
            ;;
        --ratchet-dir)
            if [ $# -lt 2 ]; then
                echo "Error: --ratchet-dir requires a value" >&2
                exit 0
            fi
            RATCHET_DIR="$2"
            shift 2
            ;;
        *)
            echo "Warning: Unknown argument: $1" >&2
            shift
            ;;
    esac
done

MAX_AGE_SECONDS=$((MAX_AGE_HOURS * 3600))
NOW_EPOCH=$(date +%s)

# --- JSON helpers ---
# Extract a top-level string value from a JSON file: json_get <file> <key>
json_get() {
    sed -n 's/.*"'"$2"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$1" | head -1
}

# Escape a string for safe JSON embedding
json_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g; s/\r/\\r/g; s/\f/\\f/g' | sed 's/\x08/\\b/g' | tr '\n' ' '
}

# --- Output accumulator ---
# Build JSON array incrementally. Each finding is appended as a comma-separated element.
FINDINGS=""
FINDING_COUNT=0

add_finding() {
    local type="$1" ref="$2" age="$3" suggested_action="$4"
    local escaped_ref escaped_action escaped_age
    escaped_ref=$(json_escape "$ref")
    escaped_action=$(json_escape "$suggested_action")
    escaped_age=$(json_escape "$age")

    local entry
    entry=$(printf '{"type":"%s","ref":"%s","age":"%s","suggested_action":"%s"}' \
        "$type" "$escaped_ref" "$escaped_age" "$escaped_action")

    if [ "$FINDING_COUNT" -gt 0 ]; then
        FINDINGS="${FINDINGS},${entry}"
    else
        FINDINGS="${entry}"
    fi
    FINDING_COUNT=$((FINDING_COUNT + 1))
}

# --- Cross-platform file age in seconds ---
# macOS stat uses -f %m, GNU stat uses -c %Y
file_mtime_epoch() {
    local file="$1"
    if stat -c %Y "$file" 2>/dev/null; then
        return
    fi
    if stat -f %m "$file" 2>/dev/null; then
        return
    fi
    echo "0"
}

file_age_seconds() {
    local file="$1"
    local mtime
    mtime=$(file_mtime_epoch "$file")
    if [ -z "$mtime" ] || [ "$mtime" = "0" ]; then
        echo "unknown"
        return
    fi
    echo $((NOW_EPOCH - mtime))
}

format_age() {
    local seconds="$1"
    if [ "$seconds" = "unknown" ]; then
        echo "unknown"
        return
    fi
    local hours=$((seconds / 3600))
    if [ "$hours" -ge 48 ]; then
        echo "$((hours / 24))d"
    else
        echo "${hours}h"
    fi
}

# --- Check 1: Stale in_progress issues in plan.yaml ---
check_stale_issues() {
    local plan_file="$RATCHET_DIR/plan.yaml"
    [ -f "$plan_file" ] || return 0

    # Requires yq to parse YAML
    if ! command -v yq >/dev/null 2>&1; then
        echo "Warning: yq not available, skipping stale issue check" >&2
        return 0
    fi

    local issue_count
    issue_count=$(yq '.milestones[].issues | length' "$plan_file" 2>/dev/null | awk '{s+=$1} END {print s+0}')
    if [ "$issue_count" -eq 0 ] 2>/dev/null; then
        return 0
    fi

    # Extract in_progress issue refs
    local refs
    refs=$(yq -r '.milestones[].issues[] | select(.status == "in_progress") | .ref' "$plan_file" 2>/dev/null) || return 0
    [ -n "$refs" ] || return 0

    while IFS= read -r ref; do
        [ -n "$ref" ] || continue

        # Find the most recent debate meta.json for this issue
        local latest_meta="" latest_mtime=0
        for meta_file in "$RATCHET_DIR"/debates/*/meta.json; do
            [ -f "$meta_file" ] || continue
            local meta_issue
            meta_issue=$(json_get "$meta_file" "issue" 2>/dev/null || true)
            if [ "$meta_issue" = "$ref" ]; then
                local mtime
                mtime=$(file_mtime_epoch "$meta_file")
                if [ "$mtime" -gt "$latest_mtime" ] 2>/dev/null; then
                    latest_mtime=$mtime
                    latest_meta="$meta_file"
                fi
            fi
        done

        if [ -n "$latest_meta" ]; then
            local age_seconds
            age_seconds=$(file_age_seconds "$latest_meta")
            if [ "$age_seconds" != "unknown" ] && [ "$age_seconds" -gt "$MAX_AGE_SECONDS" ]; then
                add_finding "stale_issue" "$ref" "$(format_age "$age_seconds")" \
                    "Issue $ref is in_progress but last debate activity was $(format_age "$age_seconds") ago. Consider resuming or closing."
            fi
        else
            # No debates found for this in_progress issue — flag as stale
            add_finding "stale_issue" "$ref" "unknown" \
                "Issue $ref is in_progress but has no associated debates. Consider starting a debate or closing the issue."
        fi
    done <<< "$refs"
}

# --- Check 2: Unresolved debate directories ---
check_unresolved_debates() {
    local debates_dir="$RATCHET_DIR/debates"
    [ -d "$debates_dir" ] || return 0

    for meta_file in "$debates_dir"/*/meta.json; do
        [ -f "$meta_file" ] || continue

        local status
        status=$(json_get "$meta_file" "status" 2>/dev/null || echo "unknown")

        if [ "$status" = "initiated" ]; then
            local resolved
            resolved=$(json_get "$meta_file" "resolved" 2>/dev/null || true)

            # "null" string or empty means not resolved
            if [ -z "$resolved" ] || [ "$resolved" = "null" ]; then
                local debate_id
                debate_id=$(json_get "$meta_file" "id" 2>/dev/null || echo "unknown")
                local age_seconds
                age_seconds=$(file_age_seconds "$meta_file")
                add_finding "unresolved_debate" "$debate_id" "$(format_age "$age_seconds")" \
                    "Debate $debate_id has status 'initiated' with no resolved timestamp. Resume with /ratchet:debate $debate_id or clean up."
            fi
        fi
    done
}

# --- Check 3: Orphan worktree directories ---
check_orphan_worktrees() {
    local worktrees_dir=".claude/worktrees"
    [ -d "$worktrees_dir" ] || return 0

    local plan_file="$RATCHET_DIR/plan.yaml"

    # Collect in_progress issue refs if plan.yaml exists
    local in_progress_refs=""
    if [ -f "$plan_file" ] && command -v yq >/dev/null 2>&1; then
        in_progress_refs=$(yq -r '.milestones[].issues[] | select(.status == "in_progress") | .ref' "$plan_file" 2>/dev/null || true)
    fi

    for worktree_dir in "$worktrees_dir"/*/; do
        [ -d "$worktree_dir" ] || continue

        local worktree_name
        worktree_name=$(basename "$worktree_dir")

        # Check if this worktree corresponds to an in_progress issue
        local has_match=false
        if [ -n "$in_progress_refs" ]; then
            while IFS= read -r ref; do
                [ -n "$ref" ] || continue
                # Worktree names may contain the issue ref (e.g., "agent-issue-42", "issue-42")
                if printf '%s' "$worktree_name" | grep -qF "$ref"; then
                    has_match=true
                    break
                fi
            done <<< "$in_progress_refs"
        fi

        if [ "$has_match" = "false" ]; then
            local age_seconds
            age_seconds=$(file_age_seconds "$worktree_dir")
            add_finding "orphan_worktree" "$worktree_name" "$(format_age "$age_seconds")" \
                "Worktree $worktree_name exists but has no corresponding in_progress issue. Consider removing with: git worktree remove $worktree_dir"
        fi
    done
}

# --- Check 4: Incomplete execution logs ---
check_incomplete_executions() {
    local exec_dir="$RATCHET_DIR/executions"
    [ -d "$exec_dir" ] || return 0

    for exec_file in "$exec_dir"/*.yaml; do
        [ -f "$exec_file" ] || continue

        # Check for resolved: null using grep (avoid yq dependency for simple check)
        if grep -q 'resolved:[[:space:]]*null' "$exec_file" 2>/dev/null; then
            local exec_id
            exec_id=$(basename "$exec_file" .yaml)
            local age_seconds
            age_seconds=$(file_age_seconds "$exec_file")
            add_finding "incomplete_execution" "$exec_id" "$(format_age "$age_seconds")" \
                "Execution log $exec_id has no resolved timestamp. The execution may have crashed. Review and clean up."
        fi
    done
}

# --- Main ---
check_stale_issues
check_unresolved_debates
check_orphan_worktrees
check_incomplete_executions

# Output JSON array
printf '[%s]\n' "$FINDINGS"

exit 0
