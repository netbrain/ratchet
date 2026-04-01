#!/usr/bin/env bash
# Manage .ratchet/watch-state.json — track seen comment IDs and clean stale entries.
#
# Usage:
#   watch-state.sh mark-seen <pr-number> <comment-id>    # Record a comment as seen
#   watch-state.sh is-seen <pr-number> <comment-id>      # Check if seen (exit 0=yes, 1=no)
#   watch-state.sh list-seen <pr-number>                  # List all seen comment IDs for a PR
#   watch-state.sh cleanup [--max-age-days N]             # Remove entries older than N days (default: 30)
#   watch-state.sh reset <pr-number>                      # Remove all state for a specific PR
#   watch-state.sh reset-all                              # Remove all watch state
#
# State file format (.ratchet/watch-state.json):
# {
#   "version": 1,
#   "prs": {
#     "42": {
#       "seen_comments": {
#         "123456": "2026-04-01T12:00:00+00:00",
#         "123457": "2026-04-01T12:05:00+00:00"
#       }
#     }
#   }
# }

set -euo pipefail

RATCHET_DIR=".ratchet"
STATE_FILE="$RATCHET_DIR/watch-state.json"

# --- Dependency check ---
if ! command -v jq >/dev/null 2>&1; then
    echo "Error: jq is required but not found in PATH" >&2
    exit 1
fi

# --- Helpers ---

# Ensure state file exists with valid structure
ensure_state_file() {
    if [ ! -d "$RATCHET_DIR" ]; then
        echo "Error: $RATCHET_DIR directory not found. Run /ratchet:init first." >&2
        exit 1
    fi

    if [ ! -f "$STATE_FILE" ]; then
        printf '{"version":1,"prs":{}}\n' > "$STATE_FILE"
    fi

    # Validate JSON
    if ! jq empty "$STATE_FILE" 2>/dev/null; then
        echo "Error: $STATE_FILE contains invalid JSON. Remove or fix it manually." >&2
        exit 1
    fi
}

# Atomic write: write to temp file, then mv
atomic_write() {
    local content="$1"
    local tmp
    tmp=$(mktemp "${STATE_FILE}.XXXXXX")
    trap 'rm -f "$tmp"' EXIT

    printf '%s\n' "$content" > "$tmp"

    # Validate the output before committing
    if ! jq empty "$tmp" 2>/dev/null; then
        echo "Error: Generated invalid JSON — aborting write to $STATE_FILE" >&2
        rm -f "$tmp"
        exit 1
    fi

    mv "$tmp" "$STATE_FILE"
    # Reset trap since file was moved successfully
    trap - EXIT
}

# Validate PR number is numeric
validate_pr_num() {
    local pr_num="$1"
    if ! printf '%s' "$pr_num" | grep -qE '^[0-9]+$'; then
        echo "Error: Invalid PR number: $pr_num (must be numeric)" >&2
        exit 1
    fi
}

# Validate comment ID is numeric
validate_comment_id() {
    local comment_id="$1"
    if ! printf '%s' "$comment_id" | grep -qE '^[0-9]+$'; then
        echo "Error: Invalid comment ID: $comment_id (must be numeric)" >&2
        exit 1
    fi
}

# --- Commands ---

cmd_mark_seen() {
    local pr_num="${1:?Usage: watch-state.sh mark-seen <pr-number> <comment-id>}"
    local comment_id="${2:?Usage: watch-state.sh mark-seen <pr-number> <comment-id>}"

    validate_pr_num "$pr_num"
    validate_comment_id "$comment_id"
    ensure_state_file

    local timestamp
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S+00:00")

    local updated
    updated=$(jq \
        --arg pr "$pr_num" \
        --arg cid "$comment_id" \
        --arg ts "$timestamp" \
        '.prs[$pr] //= {"seen_comments": {}} |
         .prs[$pr].seen_comments[$cid] = $ts' \
        "$STATE_FILE")

    atomic_write "$updated"
    echo "Marked comment $comment_id as seen for PR #$pr_num"
}

cmd_is_seen() {
    local pr_num="${1:?Usage: watch-state.sh is-seen <pr-number> <comment-id>}"
    local comment_id="${2:?Usage: watch-state.sh is-seen <pr-number> <comment-id>}"

    validate_pr_num "$pr_num"
    validate_comment_id "$comment_id"
    ensure_state_file

    local result
    result=$(jq -r \
        --arg pr "$pr_num" \
        --arg cid "$comment_id" \
        '.prs[$pr].seen_comments[$cid] // empty' \
        "$STATE_FILE")

    if [ -n "$result" ]; then
        exit 0
    else
        exit 1
    fi
}

cmd_list_seen() {
    local pr_num="${1:?Usage: watch-state.sh list-seen <pr-number>}"

    validate_pr_num "$pr_num"
    ensure_state_file

    jq -r \
        --arg pr "$pr_num" \
        '.prs[$pr].seen_comments // {} | to_entries[] | "\(.key)\t\(.value)"' \
        "$STATE_FILE"
}

cmd_cleanup() {
    local max_age_days=30

    # Parse optional --max-age-days flag
    while [ $# -gt 0 ]; do
        case "$1" in
            --max-age-days)
                if [ -z "${2:-}" ] || ! printf '%s' "$2" | grep -qE '^[0-9]+$'; then
                    echo "Error: --max-age-days requires a numeric argument" >&2
                    exit 1
                fi
                max_age_days="$2"
                shift 2
                ;;
            *)
                echo "Error: Unknown argument: $1" >&2
                echo "Usage: watch-state.sh cleanup [--max-age-days N]" >&2
                exit 1
                ;;
        esac
    done

    ensure_state_file

    local cutoff_epoch
    cutoff_epoch=$(date -u -d "-${max_age_days} days" +%s 2>/dev/null || \
                   date -u -v "-${max_age_days}d" +%s 2>/dev/null || {
        echo "Error: Failed to compute cutoff date. Neither GNU nor BSD date available." >&2
        exit 1
    })

    local removed_count=0
    local updated

    # Use jq to filter out entries older than cutoff
    # jq doesn't have native date parsing, so we use a two-pass approach:
    # 1. Export all entries with timestamps
    # 2. Filter in bash and rebuild

    local temp_clean
    temp_clean=$(mktemp)
    trap 'rm -f "$temp_clean"' EXIT

    # Get all PR/comment entries as tab-separated: pr_num \t comment_id \t timestamp
    jq -r '.prs | to_entries[] | .key as $pr |
           .value.seen_comments // {} | to_entries[] |
           "\($pr)\t\(.key)\t\(.value)"' "$STATE_FILE" > "$temp_clean"

    local stale_entries=""
    while IFS=$'\t' read -r pr_num comment_id timestamp; do
        # Parse timestamp to epoch
        local entry_epoch
        entry_epoch=$(date -u -d "$timestamp" +%s 2>/dev/null || \
                     date -u -j -f "%Y-%m-%dT%H:%M:%S" "${timestamp%%+*}" +%s 2>/dev/null || echo "0")

        if [ "$entry_epoch" -lt "$cutoff_epoch" ]; then
            stale_entries="$stale_entries $pr_num:$comment_id"
            removed_count=$((removed_count + 1))
        fi
    done < "$temp_clean"
    rm -f "$temp_clean"
    trap - EXIT

    if [ "$removed_count" -eq 0 ]; then
        echo "No stale entries found (max age: ${max_age_days} days)"
        return 0
    fi

    # Remove stale entries using jq
    updated=$(cat "$STATE_FILE")
    for entry in $stale_entries; do
        local pr_num="${entry%%:*}"
        local comment_id="${entry##*:}"
        updated=$(printf '%s' "$updated" | jq \
            --arg pr "$pr_num" \
            --arg cid "$comment_id" \
            'del(.prs[$pr].seen_comments[$cid])')
    done

    # Clean up empty PR entries
    updated=$(printf '%s' "$updated" | jq \
        '.prs |= with_entries(select(.value.seen_comments | length > 0))')

    atomic_write "$updated"
    echo "Removed $removed_count stale entries (older than ${max_age_days} days)"
}

cmd_reset() {
    local pr_num="${1:?Usage: watch-state.sh reset <pr-number>}"

    validate_pr_num "$pr_num"
    ensure_state_file

    local updated
    updated=$(jq --arg pr "$pr_num" 'del(.prs[$pr])' "$STATE_FILE")

    atomic_write "$updated"
    echo "Reset watch state for PR #$pr_num"
}

cmd_reset_all() {
    ensure_state_file

    atomic_write '{"version":1,"prs":{}}'
    echo "Reset all watch state"
}

# --- Usage ---

usage() {
    echo "Usage: watch-state.sh <command> [args...]" >&2
    echo "" >&2
    echo "Commands:" >&2
    echo "  mark-seen <pr-number> <comment-id>    Record a comment as seen" >&2
    echo "  is-seen <pr-number> <comment-id>      Check if seen (exit 0=yes, 1=no)" >&2
    echo "  list-seen <pr-number>                  List seen comment IDs for a PR" >&2
    echo "  cleanup [--max-age-days N]             Remove entries older than N days (default: 30)" >&2
    echo "  reset <pr-number>                      Remove all state for a PR" >&2
    echo "  reset-all                              Remove all watch state" >&2
    exit 1
}

# --- Dispatch ---

COMMAND="${1:-}"
shift || true

case "$COMMAND" in
    mark-seen)   cmd_mark_seen "$@" ;;
    is-seen)     cmd_is_seen "$@" ;;
    list-seen)   cmd_list_seen "$@" ;;
    cleanup)     cmd_cleanup "$@" ;;
    reset)       cmd_reset "$@" ;;
    reset-all)   cmd_reset_all ;;
    -h|--help)   usage ;;
    "")          usage ;;
    *)
        echo "Error: Unknown command: $COMMAND" >&2
        usage
        ;;
esac
