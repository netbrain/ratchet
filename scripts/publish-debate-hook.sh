#!/usr/bin/env bash
# PostToolUse hook — auto-publish debate round files as GitHub issue comments.
# Triggered by Claude Code after every Write tool call.
# Reads JSON from stdin (tool_input.file_path), checks if the written file
# is a debate artifact, and publishes it via the configured progress adapter.
#
# Zero-trust: the debate-runner agent does not need to know about publishing.
# Silent failure: publish errors are logged but never surface to the agent.
set -euo pipefail

# --- Parse stdin for tool input ---
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null) || true

if [ -z "$FILE_PATH" ]; then
    exit 0
fi

# --- Match debate artifact paths ---
# Patterns: .ratchet/debates/*/rounds/round-*.md  (round files)
#           .ratchet/debates/*/verdict.json        (verdict files)
DEBATE_DIR=""
ARTIFACT_TYPE=""

case "$FILE_PATH" in
    */.ratchet/debates/*/rounds/round-*-generative.md)
        ARTIFACT_TYPE="generative"
        DEBATE_DIR="${FILE_PATH%%/rounds/round-*}"
        ;;
    */.ratchet/debates/*/rounds/round-*-adversarial.md)
        ARTIFACT_TYPE="adversarial"
        DEBATE_DIR="${FILE_PATH%%/rounds/round-*}"
        ;;
    */.ratchet/debates/*/meta.json)
        # Final meta.json write — check if debate just completed for summary publish
        ARTIFACT_TYPE="meta"
        DEBATE_DIR=$(dirname "$FILE_PATH")
        ;;
    *)
        # Not a debate artifact — nothing to do
        exit 0
        ;;
esac

# --- Read debate metadata ---
META_FILE="$DEBATE_DIR/meta.json"
if [ ! -f "$META_FILE" ]; then
    exit 0
fi

# Extract publish config from meta.json
# The debate-runner stores these from the orchestrator's context
PAIR_NAME=$(jq -r '.pair // empty' "$META_FILE" 2>/dev/null) || true
PHASE=$(jq -r '.phase // empty' "$META_FILE" 2>/dev/null) || true
STATUS=$(jq -r '.status // empty' "$META_FILE" 2>/dev/null) || true

# --- Locate workflow.yaml to read publish config ---
# Walk up from the debate directory to find .ratchet/workflow.yaml
RATCHET_DIR="${DEBATE_DIR%%/.ratchet/debates/*}/.ratchet"
PROJECT_DIR=$(dirname "$RATCHET_DIR")
WORKFLOW_FILE="$RATCHET_DIR/workflow.yaml"

if [ ! -f "$WORKFLOW_FILE" ]; then
    echo "Warning: publish-debate-hook: workflow.yaml not found at $WORKFLOW_FILE — debate publishing disabled" >&2
    exit 0
fi

# Read publish config from workflow.yaml
# Requires yq — fall back with warning if unavailable
if ! command -v yq >/dev/null 2>&1; then
    echo "Warning: publish-debate-hook: yq not found — debate publishing disabled (install yq to enable)" >&2
    exit 0
fi

ADAPTER=$(yq eval '.progress.adapter // "none"' "$WORKFLOW_FILE" 2>/dev/null) || true
PUBLISH_MODE=$(yq eval '.progress.publish_debates // "false"' "$WORKFLOW_FILE" 2>/dev/null) || true

# Only publish when explicitly configured — "per-round" or "summary"
case "$PUBLISH_MODE" in
    per-round|summary) ;;  # proceed
    *) exit 0 ;;           # false, null, empty, or any unexpected value
esac

if [ "$ADAPTER" = "none" ] || [ -z "$ADAPTER" ]; then
    exit 0
fi

# --- Resolve progress_ref (GitHub issue number to post to) ---
# Priority: meta.json progress_ref > meta.json issue (if numeric) > plan.yaml github_issue on milestone
PROGRESS_REF=$(jq -r '.progress_ref // empty' "$META_FILE" 2>/dev/null) || true

# Fallback: check issue field in meta.json — only if it looks like a GitHub issue number
if [ -z "$PROGRESS_REF" ]; then
    META_ISSUE=$(jq -r '.issue // empty' "$META_FILE" 2>/dev/null) || true
    # Accept bare numbers ("164"), hash-prefixed ("#164"), but not plan refs ("issue-4-3")
    case "$META_ISSUE" in
        [0-9]*) PROGRESS_REF="$META_ISSUE" ;;
        '#'[0-9]*) PROGRESS_REF="${META_ISSUE#\#}" ;;
    esac
fi

# Fallback: look up github_issue from milestone in plan.yaml
if [ -z "$PROGRESS_REF" ]; then
    PLAN_YAML="$RATCHET_DIR/plan.yaml"
    if [ -f "$PLAN_YAML" ] && command -v yq >/dev/null 2>&1; then
        # Get the milestone field from meta.json, then look up github_issue on that milestone
        META_MILESTONE=$(jq -r '.milestone // empty' "$META_FILE" 2>/dev/null) || true
        if [ -n "$META_MILESTONE" ]; then
            GH_ISSUE=$(yq eval "
                .epic.milestones[] | select(.id == \"$META_MILESTONE\" or .id == $META_MILESTONE) | .github_issue // null
            " "$PLAN_YAML" 2>/dev/null) || true
            if [ -n "$GH_ISSUE" ] && [ "$GH_ISSUE" != "null" ]; then
                PROGRESS_REF="$GH_ISSUE"
            fi
        fi
    fi
fi

if [ -z "$PROGRESS_REF" ]; then
    # No target to publish to — silently skip
    exit 0
fi

# --- Locate add-comment script ---
# Check both installed (.claude/ratchet-scripts/) and source (scripts/progress/) paths
ADD_COMMENT=""
for candidate in \
    "$PROJECT_DIR/.claude/ratchet-scripts/progress/$ADAPTER/add-comment.sh" \
    "$PROJECT_DIR/scripts/progress/$ADAPTER/add-comment.sh"; do
    if [ -f "$candidate" ]; then
        ADD_COMMENT="$candidate"
        break
    fi
done

if [ -z "$ADD_COMMENT" ]; then
    exit 0
fi

# --- Build and post comment ---
ROUND_NAME=$(basename "$FILE_PATH" .md 2>/dev/null || true)
ROUND_CONTENT=""
if [ -f "$FILE_PATH" ]; then
    ROUND_CONTENT=$(cat "$FILE_PATH")
fi

case "$ARTIFACT_TYPE" in
    generative|adversarial)
        # Per-round publish
        if [ "$PUBLISH_MODE" != "per-round" ]; then
            exit 0
        fi

        COMMENT="### Debate: ${PAIR_NAME} — ${ROUND_NAME}
**Phase:** ${PHASE} | **Issue:** ${PROGRESS_REF}
<details><summary>Round output</summary>

${ROUND_CONTENT}
</details>"

        bash "$ADD_COMMENT" "$PROGRESS_REF" "$COMMENT" >/dev/null 2>&1 || true
        ;;

    meta)
        # Summary publish — only when debate reaches terminal state
        if [ "$PUBLISH_MODE" != "summary" ]; then
            exit 0
        fi

        # Only publish on terminal states
        case "$STATUS" in
            consensus|resolved) ;;
            *) exit 0 ;;
        esac

        VERDICT=$(jq -r '.verdict // "unknown"' "$META_FILE" 2>/dev/null) || true
        ROUNDS=$(jq -r '.rounds // 0' "$META_FILE" 2>/dev/null) || true

        # Build summary from all round files
        SUMMARY_BODY=""
        for round_file in "$DEBATE_DIR"/rounds/round-*.md; do
            [ -f "$round_file" ] || continue
            ROUND_LABEL=$(basename "$round_file" .md)
            SUMMARY_BODY="${SUMMARY_BODY}
<details><summary>${ROUND_LABEL}</summary>

$(cat "$round_file")
</details>"
        done

        COMMENT="### Debate Summary: ${PAIR_NAME} (${ROUNDS} rounds — ${VERDICT})
**Phase:** ${PHASE} | **Issue:** ${PROGRESS_REF}
${SUMMARY_BODY}"

        bash "$ADD_COMMENT" "$PROGRESS_REF" "$COMMENT" >/dev/null 2>&1 || true
        ;;
esac

exit 0
