#!/usr/bin/env bash
# Git pre-commit hook for ratchet — only runs when Claude Code is committing.
# Manual git commits pass through without checks.

if [ -z "${CLAUDE_CODE:-}" ]; then
    exit 0
fi

# Delegate to consensus check
exec bash "$(dirname "$0")/check-consensus.sh"
