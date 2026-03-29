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

# Gate: source file changes require an ACCEPT verdict from a debate.
# This prevents agent chain collapse — agents cannot commit code without
# having gone through the debate framework. Enforced by git, not agent compliance.
if [ -z "${RATCHET_SKIP_VERDICT_CHECK:-}" ]; then
    SOURCE_EXTS='\.go$|\.templ$|\.ts$|\.tsx$|\.js$|\.jsx$|\.py$|\.rs$'
    SKILL_AGENTS='skills/.*/SKILL\.md$|agents/.*\.md$'
    SCRIPTS='scripts/.*\.sh$|install\.sh$'
    SCHEMAS='schemas/.*\.json$'
    SOURCE_PATTERN="$SOURCE_EXTS|$SKILL_AGENTS|$SCRIPTS|$SCHEMAS"

    if git diff --cached --name-only | grep -qE "$SOURCE_PATTERN"; then
        # Source files are staged — require a debate verdict
        LATEST_VERDICT=$(ls -t .ratchet/debates/*/meta.json 2>/dev/null | head -1)
        VERDICT_OK=false

        if [ -n "$LATEST_VERDICT" ]; then
            # Extract verdict using sed (no jq dependency, same as check-consensus.sh)
            verdict_val=$(sed -n 's/.*"verdict"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$LATEST_VERDICT" | head -1)
            case "$verdict_val" in
                ACCEPT|CONDITIONAL_ACCEPT|TRIVIAL_ACCEPT)
                    VERDICT_OK=true
                    ;;
            esac
        fi

        if [ "$VERDICT_OK" != "true" ]; then
            echo "╔══════════════════════════════════════════════════════╗"
            echo "║  Ratchet: Source changes require a debate verdict    ║"
            echo "╚══════════════════════════════════════════════════════╝"
            echo ""
            echo "  Source files are staged but no recent ACCEPT verdict found."
            echo "  All code changes must flow through the debate framework:"
            echo "    orchestrator -> debate-runner -> generative + adversarial"
            echo ""
            echo "  Run /ratchet:run to start a debate, or set"
            echo "  RATCHET_SKIP_VERDICT_CHECK=1 to bypass (not recommended)."
            echo ""
            exit 1
        fi
    fi
fi

# Gate: verify all active debates have reached consensus
CONSENSUS_SCRIPT="$SCRIPT_DIR/check-consensus.sh"
if [ ! -f "$CONSENSUS_SCRIPT" ]; then
    echo "Error: check-consensus.sh not found at $CONSENSUS_SCRIPT" >&2
    exit 1
fi
exec bash "$CONSENSUS_SCRIPT"
