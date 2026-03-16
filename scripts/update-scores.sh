#!/usr/bin/env bash
# Append a score entry to scores.jsonl after a debate resolves.
# Usage: update-scores.sh <debate-id>

set -euo pipefail

# JSON parsing uses grep/sed for simple key lookups — no external deps (jq, python3, node).
# CAVEAT: If JSON structures become nested or complex, migrate to jq or similar.

# Extract a top-level string value from a JSON file: json_get <file> <key>
json_get() {
    sed -n 's/.*"'"$2"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$1" | head -1
}

# Extract a top-level numeric value from a JSON file: json_get_num <file> <key>
json_get_num() {
    sed -n 's/.*"'"$2"'"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$1" | head -1
}

RATCHET_DIR=".ratchet"
SCORES_FILE="$RATCHET_DIR/scores/scores.jsonl"

if [ $# -lt 1 ]; then
    echo "Usage: update-scores.sh <debate-id>"
    exit 1
fi

DEBATE_ID="$1"
META_FILE="$RATCHET_DIR/debates/$DEBATE_ID/meta.json"

if [ ! -f "$META_FILE" ]; then
    echo "Error: Debate $DEBATE_ID not found"
    exit 1
fi

# Ensure scores directory exists
mkdir -p "$(dirname "$SCORES_FILE")"

# Extract fields from meta.json
meta_id=$(json_get "$META_FILE" "id")
meta_pair=$(json_get "$META_FILE" "pair")
meta_milestone=$(json_get "$META_FILE" "milestone")
meta_rounds=$(json_get_num "$META_FILE" "rounds")
meta_status=$(json_get "$META_FILE" "status")
meta_fast_path=$(sed -n 's/.*"fast_path"[[:space:]]*:[[:space:]]*\(true\|false\).*/\1/p' "$META_FILE" | head -1)

# Validate required fields
if [ -z "$meta_id" ] || [ -z "$meta_pair" ] || [ -z "$meta_rounds" ]; then
    echo "Error: Missing required fields (id, pair, rounds) in $META_FILE" >&2
    exit 1
fi

# Count issues from adversarial round files
rounds_dir="$RATCHET_DIR/debates/$DEBATE_ID/rounds"
issues_found=0

if [ -d "$rounds_dir" ]; then
    for f in "$rounds_dir"/round-*-adversarial.md; do
        [ -f "$f" ] || continue
        count=$(grep -c '"severity":' "$f" 2>/dev/null || true)
        issues_found=$((issues_found + count))
    done
fi

# Determine issues resolved based on status
if [ "$meta_status" = "consensus" ] || [ "$meta_status" = "resolved" ]; then
    issues_resolved=$issues_found
else
    issues_resolved=0
fi

# Determine escalated flag
if [ "$meta_status" = "consensus" ]; then escalated="false"; else escalated="true"; fi

# Handle null milestone
if [ -n "$meta_milestone" ]; then
    milestone_json="\"$meta_milestone\""
else
    milestone_json="null"
fi

timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S+00:00")

# Append score as JSONL using atomic write pattern
# Write to temp file first, then append atomically to prevent partial writes on crash
tmp_score=$(mktemp "${SCORES_FILE}.XXXXXX")
trap 'rm -f "$tmp_score"' EXIT

printf '{"timestamp":"%s","debate_id":"%s","pair":"%s","milestone":%s,"rounds_to_consensus":%s,"escalated":%s,"issues_found":%s,"issues_resolved":%s,"fast_path":%s}\n' \
    "$timestamp" "$meta_id" "$meta_pair" "$milestone_json" "$meta_rounds" "$escalated" "$issues_found" "$issues_resolved" "${meta_fast_path:-false}" \
    > "$tmp_score"

# Atomic append: cat the complete line to the scores file
cat "$tmp_score" >> "$SCORES_FILE"

echo "Score recorded for $meta_id: $issues_found found, $issues_resolved resolved"
