#!/usr/bin/env bash
# PostToolUse hook — auto-start /ratchet:watch loop when gh pr create succeeds.
# Triggered by Claude Code after every Bash tool call.
# Reads JSON from stdin (tool_input.command, tool_result.exit_code),
# detects successful `gh pr create` invocations, and starts the watch loop
# if it is not already running.
#
# Lock file: .ratchet/locks/watch.lock — prevents duplicate watch loops.
# Silent failure: errors are logged to stderr but never surface to the agent.
set -euo pipefail

# --- Parse stdin for tool input ---
INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null) || true
EXIT_CODE=$(echo "$INPUT" | jq -r '.tool_result.exit_code // empty' 2>/dev/null) || true

# Quick exit if not a Bash tool call with a command
if [ -z "$COMMAND" ]; then
    exit 0
fi

# --- Match gh pr create command ---
# Match both "gh pr create" and variants like "gh pr create --title ..."
# Case-insensitive not needed: gh CLI commands are lowercase.
case "$COMMAND" in
    *gh\ pr\ create*) ;;
    *) exit 0 ;;
esac

# --- Verify successful exit ---
if [ "$EXIT_CODE" != "0" ] && [ -n "$EXIT_CODE" ]; then
    exit 0
fi

# --- Locate project root ---
# Walk up from current directory to find .ratchet/
PROJECT_DIR=""
SEARCH_DIR="${PWD:-$(pwd)}"
while [ -n "$SEARCH_DIR" ] && [ "$SEARCH_DIR" != "/" ]; do
    if [ -d "$SEARCH_DIR/.ratchet" ]; then
        PROJECT_DIR="$SEARCH_DIR"
        break
    fi
    SEARCH_DIR=$(dirname "$SEARCH_DIR")
done

# Worktree fallback: try git worktree list
if [ -z "$PROJECT_DIR" ] && command -v git >/dev/null 2>&1; then
    MAIN_WORKTREE=$(git worktree list 2>/dev/null | head -1 | awk '{print $1}') || true
    if [ -n "$MAIN_WORKTREE" ] && [ -d "$MAIN_WORKTREE/.ratchet" ]; then
        PROJECT_DIR="$MAIN_WORKTREE"
    fi
fi

if [ -z "$PROJECT_DIR" ]; then
    echo "Warning: auto-watch-hook: .ratchet/ directory not found — skipping watch auto-start" >&2
    exit 0
fi

LOCK_DIR="$PROJECT_DIR/.ratchet/locks"
LOCK_FILE="$LOCK_DIR/watch.lock"

# --- Check if watch loop is already running ---
if [ -f "$LOCK_FILE" ]; then
    WATCH_PID=$(cat "$LOCK_FILE" 2>/dev/null) || true
    if [ -n "$WATCH_PID" ] && kill -0 "$WATCH_PID" 2>/dev/null; then
        # Watch loop is already running — nothing to do
        exit 0
    fi
    # Stale lock file — PID no longer running, remove it
    rm -f "$LOCK_FILE"
fi

# --- Create lock directory if needed ---
mkdir -p "$LOCK_DIR" || {
    echo "Error: auto-watch-hook: failed to create lock directory: $LOCK_DIR" >&2
    exit 0
}

# --- Signal that watch should be started ---
# Write a sentinel file that the agent/skill layer can detect.
# The hook itself cannot start /loop (that's a Claude Code session command,
# not a bash process). Instead, we write a request file that /ratchet:run
# or the session can pick up.
#
# The lock file serves dual purpose:
#   1. Prevents duplicate watch-start requests
#   2. Stores the timestamp of the request for staleness detection
echo "$$" > "$LOCK_FILE"
echo "watch_requested_at=$(date -Iseconds)" >> "$LOCK_FILE"
echo "trigger_command=$(echo "$COMMAND" | head -c 200)" >> "$LOCK_FILE"

echo "Info: auto-watch-hook: PR created successfully — watch loop start requested (lock: $LOCK_FILE)" >&2

exit 0
