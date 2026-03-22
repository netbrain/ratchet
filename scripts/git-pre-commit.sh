#!/usr/bin/env bash
# Git pre-commit hook for ratchet — only runs when Claude Code is committing.
# Manual git commits pass through without checks.
set -euo pipefail

if [ -z "${CLAUDE_CODE:-}" ]; then
    exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Guard: block generated/runtime files from being committed
if [ -z "${RATCHET_ALLOW_GENERATED:-}" ]; then
    GENERATED_SCRIPT="$SCRIPT_DIR/check-generated-files.sh"
    if [ -f "$GENERATED_SCRIPT" ]; then
        bash "$GENERATED_SCRIPT" || exit 1
    fi
fi

# Gate: verify all active debates have reached consensus
CONSENSUS_SCRIPT="$SCRIPT_DIR/check-consensus.sh"
if [ ! -f "$CONSENSUS_SCRIPT" ]; then
    echo "Error: check-consensus.sh not found at $CONSENSUS_SCRIPT" >&2
    exit 1
fi
exec bash "$CONSENSUS_SCRIPT"
