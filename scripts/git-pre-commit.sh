#!/usr/bin/env bash
# Git pre-commit hook for ratchet — only runs when Claude Code is committing.
# Manual git commits pass through without checks.
set -euo pipefail

if [ -z "${CLAUDE_CODE:-}" ]; then
    exit 0
fi

# Delegate to consensus check
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec bash "$SCRIPT_DIR/check-consensus.sh"
