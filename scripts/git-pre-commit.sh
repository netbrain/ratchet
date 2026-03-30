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
        # Source files are staged — require a debate verdict newer than the branch point.
        # Without the timestamp check, old verdicts from prior milestones satisfy the
        # gate, letting chain-collapsed agents commit without running debates.

        # Determine when the current branch diverged from main (epoch seconds).
        # Falls back to 0 if on main or no merge-base found (any verdict passes).
        BRANCH_CREATED=0
        CURRENT_BRANCH=$(git branch --show-current 2>/dev/null || true)
        if [ -n "$CURRENT_BRANCH" ] && [ "$CURRENT_BRANCH" != "main" ] && [ "$CURRENT_BRANCH" != "master" ]; then
            MERGE_BASE=$(git merge-base HEAD main 2>/dev/null || git merge-base HEAD master 2>/dev/null || true)
            if [ -n "$MERGE_BASE" ]; then
                BRANCH_CREATED=$(git log -1 --format='%ct' "$MERGE_BASE" 2>/dev/null || echo "0")
            fi
        fi

        VERDICT_OK=false
        for meta_file in $(ls -t .ratchet/debates/*/meta.json 2>/dev/null); do
            [ -f "$meta_file" ] || continue

            # Extract verdict
            verdict_val=$(sed -n 's/.*"verdict"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$meta_file" | head -1)
            case "$verdict_val" in
                ACCEPT|CONDITIONAL_ACCEPT|TRIVIAL_ACCEPT) ;;
                *) continue ;;
            esac

            # Extract debate started timestamp and convert to epoch.
            # meta.json stores ISO 8601: "started": "2026-03-28T14:30:00Z"
            started_iso=$(sed -n 's/.*"started"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$meta_file" | head -1)
            if [ -n "$started_iso" ]; then
                started_epoch=$(date -d "$started_iso" +%s 2>/dev/null || echo "0")
            else
                started_epoch=0
            fi

            # Verdict must be from after the branch was created
            if [ "$started_epoch" -ge "$BRANCH_CREATED" ] 2>/dev/null; then
                VERDICT_OK=true
                break
            fi
        done

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
