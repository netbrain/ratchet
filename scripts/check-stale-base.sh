#!/usr/bin/env bash
# Verify a worktree contains expected changes from declared dependencies.
# Detects stale-base conditions where dependency changes are missing.
#
# Usage: check-stale-base.sh --issue <ref> --plan <plan.yaml> --worktree <path> [--verbose]
#
# Inputs:
#   --issue     Issue ref to check (e.g., "129")
#   --plan      Path to plan.yaml
#   --worktree  Path to the worktree to verify
#   --verbose   Show detailed diff output for each checked file
#
# Output: JSON object with dependency satisfaction findings.
# Exit 0 if all dependencies satisfied, exit 1 if any missing.

set -euo pipefail

# --- Defaults ---
ISSUE_REF=""
PLAN_FILE=""
WORKTREE_PATH=""
VERBOSE=false

# --- Argument parsing ---
while [ $# -gt 0 ]; do
    case "$1" in
        --issue)
            if [ $# -lt 2 ]; then
                echo "Error: --issue requires a value" >&2
                exit 1
            fi
            ISSUE_REF="$2"
            shift 2
            ;;
        --plan)
            if [ $# -lt 2 ]; then
                echo "Error: --plan requires a value" >&2
                exit 1
            fi
            PLAN_FILE="$2"
            shift 2
            ;;
        --worktree)
            if [ $# -lt 2 ]; then
                echo "Error: --worktree requires a value" >&2
                exit 1
            fi
            WORKTREE_PATH="$2"
            shift 2
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            sed -n '2,/^$/s/^# \{0,1\}//p' "$0"
            exit 0
            ;;
        *)
            echo "Error: Unknown argument: $1" >&2
            exit 1
            ;;
    esac
done

# --- Validate required arguments ---
if [ -z "$ISSUE_REF" ]; then
    echo "Error: --issue is required" >&2
    exit 1
fi
if [ -z "$PLAN_FILE" ]; then
    echo "Error: --plan is required" >&2
    exit 1
fi
if [ -z "$WORKTREE_PATH" ]; then
    echo "Error: --worktree is required" >&2
    exit 1
fi

# --- Validate inputs exist ---
if [ ! -f "$PLAN_FILE" ]; then
    echo "Error: Plan file not found: $PLAN_FILE" >&2
    exit 1
fi
if [ ! -d "$WORKTREE_PATH" ]; then
    echo "Error: Worktree directory not found: $WORKTREE_PATH" >&2
    exit 1
fi

# --- Validate tools ---
if ! command -v yq >/dev/null 2>&1; then
    echo "Error: yq is required but not found in PATH" >&2
    exit 1
fi

# --- JSON helpers ---
json_escape() {
    # Use jq for correct JSON string escaping (handles all control chars 0x00-0x1f)
    if command -v jq >/dev/null 2>&1; then
        printf '%s' "$1" | jq -Rs '.'  | sed 's/^"//; s/"$//'
    else
        # Fallback: manual escaping including control characters 0x01-0x1f
        printf '%s' "$1" \
            | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g; s/\r/\\r/g; s/\f/\\f/g' \
            | sed 's/\x08/\\b/g' \
            | sed 's/[\x01-\x07]//g; s/[\x0e-\x1f]//g' \
            | sed ':a;N;$!ba;s/\n/\\n/g'
    fi
}

# --- Temp file management ---
DIFF_STDERR_TMP=$(mktemp)
cleanup() { rm -f "$DIFF_STDERR_TMP"; }
trap cleanup EXIT

# --- Verify issue exists in plan ---
export ISSUE_REF
ISSUE_EXISTS=$(yq -r \
    '.epic.milestones[].issues[] | select(.ref == env(ISSUE_REF)) | .ref' \
    "$PLAN_FILE" 2>/dev/null) || true

if [ -z "$ISSUE_EXISTS" ]; then
    echo "Error: Issue ref '$ISSUE_REF' not found in plan file: $PLAN_FILE" >&2
    exit 1
fi

# --- Resolve the issue's depends_on list ---
# Search across all milestones for the issue with matching ref
DEPENDS_ON=$(yq -r \
    '.epic.milestones[].issues[] | select(.ref == env(ISSUE_REF)) | .depends_on // [] | .[]' \
    "$PLAN_FILE" 2>/dev/null) || true

if [ -z "$DEPENDS_ON" ]; then
    # No dependencies — nothing to check, output clean result
    printf '{"issue":"%s","dependencies":[],"all_satisfied":true,"summary":"No dependencies declared"}\n' \
        "$(json_escape "$ISSUE_REF")"
    exit 0
fi

# --- Check each dependency ---
ALL_SATISFIED=true
DEP_RESULTS=""
DEP_COUNT=0

while IFS= read -r dep_ref; do
    [ -n "$dep_ref" ] || continue

    # Look up the dependency issue's files array across all milestones
    export dep_ref
    DEP_FILES=$(yq -r \
        '.epic.milestones[].issues[] | select(.ref == env(dep_ref)) | .files // [] | .[]' \
        "$PLAN_FILE" 2>/dev/null) || true

    if [ -z "$DEP_FILES" ]; then
        # Dependency has no files listed — can't verify, treat as warning
        escaped_dep=$(json_escape "$dep_ref")
        entry=$(printf '{"dep_ref":"%s","status":"unknown","reason":"no files declared in plan","files":[]}' \
            "$escaped_dep")

        if [ "$DEP_COUNT" -gt 0 ]; then
            DEP_RESULTS="${DEP_RESULTS},${entry}"
        else
            DEP_RESULTS="${entry}"
        fi
        DEP_COUNT=$((DEP_COUNT + 1))
        continue
    fi

    # Check each file from the dependency
    FILE_RESULTS=""
    FILE_COUNT=0
    DEP_SATISFIED=true
    MISSING_FILES=""
    UNCHANGED_FILES=""
    ERROR_FILES=""

    while IFS= read -r dep_file; do
        [ -n "$dep_file" ] || continue

        worktree_file="${WORKTREE_PATH}/${dep_file}"
        file_status="satisfied"
        file_reason=""
        file_diff=""

        if [ ! -f "$worktree_file" ]; then
            file_status="missing"
            file_reason="file does not exist in worktree"
            DEP_SATISFIED=false
            MISSING_FILES="${MISSING_FILES:+${MISSING_FILES}, }${dep_file}"
        else
            # Check if the file differs from origin/main
            # Use git diff inside the worktree to compare against origin/main
            set +e
            diff_output=$(git -C "$WORKTREE_PATH" diff "origin/main" -- "$dep_file" 2>"$DIFF_STDERR_TMP")
            diff_exit=$?
            diff_err_text=$(cat "$DIFF_STDERR_TMP")
            set -e

            if [ $diff_exit -ne 0 ] && [ -z "$diff_output" ]; then
                # Distinguish known-benign errors (no remote/unknown revision) from unexpected failures
                if echo "$diff_err_text" | grep -qiE "(unknown revision|no such remote|bad default revision|not a git repository|could not access)"; then
                    # Benign: origin/main not available — fall back to file-exists check
                    file_status="satisfied"
                    file_reason="file exists (origin/main comparison unavailable)"
                else
                    # Unexpected git diff failure — surface the error, don't silently pass
                    file_status="error"
                    file_reason="git diff failed (exit $diff_exit): ${diff_err_text}"
                    DEP_SATISFIED=false
                    ERROR_FILES="${ERROR_FILES:+${ERROR_FILES}, }${dep_file}"
                fi
                if [ "$VERBOSE" = "true" ] && [ -n "$diff_err_text" ]; then
                    file_diff="stderr: ${diff_err_text}"
                fi
            elif [ -z "$diff_output" ]; then
                # Empty diff could mean file is identical OR file is untracked (new).
                # Check if the file exists in origin/main via git ls-tree.
                set +e
                ls_tree_output=$(git -C "$WORKTREE_PATH" ls-tree "origin/main" -- "$dep_file" 2>/dev/null)
                set -e
                if [ -z "$ls_tree_output" ]; then
                    # File not in origin/main but exists locally — it's a new file
                    file_status="satisfied"
                    file_reason="new file not present in origin/main"
                else
                    file_status="unchanged"
                    file_reason="file exists but is identical to origin/main"
                    DEP_SATISFIED=false
                    UNCHANGED_FILES="${UNCHANGED_FILES:+${UNCHANGED_FILES}, }${dep_file}"
                fi
            else
                if [ $diff_exit -ne 0 ]; then
                    # Non-zero exit with partial output — treat as error, not satisfied
                    file_status="error"
                    file_reason="git diff failed (exit $diff_exit) with partial output: ${diff_err_text}"
                    DEP_SATISFIED=false
                    ERROR_FILES="${ERROR_FILES:+${ERROR_FILES}, }${dep_file}"
                    if [ "$VERBOSE" = "true" ]; then
                        file_diff="partial output: ${diff_output}; stderr: ${diff_err_text}"
                    fi
                else
                    file_status="satisfied"
                    file_reason="file differs from origin/main"
                    if [ "$VERBOSE" = "true" ]; then
                        file_diff="$diff_output"
                    fi
                fi
            fi
        fi

        escaped_file=$(json_escape "$dep_file")
        escaped_reason=$(json_escape "$file_reason")

        file_entry=$(printf '{"file":"%s","status":"%s","reason":"%s"}' \
            "$escaped_file" "$file_status" "$escaped_reason")

        if [ "$VERBOSE" = "true" ] && [ -n "$file_diff" ]; then
            escaped_diff=$(json_escape "$file_diff")
            file_entry=$(printf '{"file":"%s","status":"%s","reason":"%s","diff":"%s"}' \
                "$escaped_file" "$file_status" "$escaped_reason" "$escaped_diff")
        fi

        if [ "$FILE_COUNT" -gt 0 ]; then
            FILE_RESULTS="${FILE_RESULTS},${file_entry}"
        else
            FILE_RESULTS="${file_entry}"
        fi
        FILE_COUNT=$((FILE_COUNT + 1))
    done <<< "$DEP_FILES"

    # Build dependency result
    escaped_dep=$(json_escape "$dep_ref")
    if [ "$DEP_SATISFIED" = "true" ]; then
        dep_status="satisfied"
        dep_reason="all dependency files present and modified"
    else
        dep_status="missing"
        dep_reason=""
        ALL_SATISFIED=false
        if [ -n "$MISSING_FILES" ]; then
            dep_reason="missing files: ${MISSING_FILES}"
        fi
        if [ -n "$UNCHANGED_FILES" ]; then
            if [ -n "$dep_reason" ]; then
                dep_reason="${dep_reason}; unchanged files: ${UNCHANGED_FILES}"
            else
                dep_reason="unchanged files: ${UNCHANGED_FILES}"
            fi
        fi
        if [ -n "$ERROR_FILES" ]; then
            if [ -n "$dep_reason" ]; then
                dep_reason="${dep_reason}; error files: ${ERROR_FILES}"
            else
                dep_reason="error files: ${ERROR_FILES}"
            fi
        fi
    fi
    escaped_reason=$(json_escape "$dep_reason")

    entry=$(printf '{"dep_ref":"%s","status":"%s","reason":"%s","files":[%s]}' \
        "$escaped_dep" "$dep_status" "$escaped_reason" "$FILE_RESULTS")

    if [ "$DEP_COUNT" -gt 0 ]; then
        DEP_RESULTS="${DEP_RESULTS},${entry}"
    else
        DEP_RESULTS="${entry}"
    fi
    DEP_COUNT=$((DEP_COUNT + 1))
done <<< "$DEPENDS_ON"

# --- Build and output final JSON ---
escaped_issue=$(json_escape "$ISSUE_REF")
if [ "$ALL_SATISFIED" = "true" ]; then
    all_satisfied_json="true"
    summary="All $DEP_COUNT dependencies satisfied"
else
    all_satisfied_json="false"
    summary="Some dependencies have missing or unchanged files"
fi
escaped_summary=$(json_escape "$summary")

printf '{"issue":"%s","dependencies":[%s],"all_satisfied":%s,"summary":"%s"}\n' \
    "$escaped_issue" "$DEP_RESULTS" "$all_satisfied_json" "$escaped_summary"

if [ "$ALL_SATISFIED" = "true" ]; then
    exit 0
else
    exit 1
fi
