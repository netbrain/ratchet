#!/usr/bin/env bash
# Update the file-hash cache after a debate reaches consensus.
# Usage: cache-update.sh <pair-name> <scope-glob> [debate-id]
# Scope-glob supports comma-separated patterns (e.g., "scripts/*.sh,install.sh")
set -euo pipefail

# JSON parsing uses grep/sed for simple key lookups — no external deps (jq, python3, node).
# CAVEAT: If JSON structures become nested or complex, migrate to jq or similar.

# Cross-platform sha256: macOS uses shasum, Linux uses sha256sum
if command -v sha256sum >/dev/null 2>&1; then
    sha256() { sha256sum | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
    sha256() { shasum -a 256 | awk '{print $1}'; }
else
    echo "Error: neither sha256sum nor shasum found" >&2; exit 1
fi

PAIR_NAME="${1:?Usage: cache-update.sh <pair-name> <scope-glob> [debate-id]}"
SCOPE_GLOB="${2:?Usage: cache-update.sh <pair-name> <scope-glob> [debate-id]}"
DEBATE_ID="${3:-}"
CACHE_FILE=".ratchet/cache.json"

# Collect files matching comma-separated scope globs
# Note: find -path's * already matches across directories (unlike shell globbing),
# so ** is redundant. We collapse **/ and ** to * for compatibility.
matched_files=""
IFS=',' read -ra globs <<< "$SCOPE_GLOB"
for glob in "${globs[@]}"; do
    glob="$(echo "$glob" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//;s|\*\*/|*|g;s|\*\*|*|g')"
    matches=$(find . -path "./$glob" -type f 2>/dev/null || true)
    if [ -n "$matches" ]; then
        matched_files="${matched_files:+${matched_files}
}${matches}"
    fi
done
matched_files=$(echo "$matched_files" | sort -u)

# No files matched = skip caching (avoids infinite debate loop with cache-check)
if [ -z "$matched_files" ]; then
    echo "Warning: No files matched scope '$SCOPE_GLOB' — skipping cache update" >&2
    exit 0
fi

current_hash=$(echo "$matched_files" | while IFS= read -r f; do cat "$f"; done 2>/dev/null | sha256)

# Ensure .ratchet directory exists
mkdir -p "$(dirname "$CACHE_FILE")" || { echo "Error: Failed to create cache directory" >&2; exit 1; }

# Update cache
# Strategy: build the new entry, then merge into existing cache file.
# We remove any existing block for this pair and append the new one.
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%S+00:00")
DEBATE_REF="${DEBATE_ID:-null}"
if [ "$DEBATE_REF" != "null" ]; then
    DEBATE_REF="\"$DEBATE_REF\""
fi

NEW_ENTRY=$(cat <<ENTRY_EOF
  "$PAIR_NAME": {
    "hash": "$current_hash",
    "debate_id": $DEBATE_REF,
    "timestamp": "$TIMESTAMP"
  }
ENTRY_EOF
)

# Use atomic write pattern: temp file + mv
# This prevents corruption if the process crashes or disk fills during write
tmp_cache=$(mktemp "${CACHE_FILE}.XXXXXX")
trap 'rm -f "$tmp_cache"' EXIT

if [ -f "$CACHE_FILE" ] && [ -s "$CACHE_FILE" ]; then
    # Remove existing entry for this pair (from key line to next key or closing brace)
    # Then remove the outer braces, trim, and rebuild
    existing=$(sed '1d;$d' "$CACHE_FILE" | sed -n '/^  "'"$PAIR_NAME"'"/,/^  }/!p' | sed '/^$/d')
    # Remove trailing comma from existing if present
    existing=$(echo "$existing" | sed '$ s/,$//')
    if [ -n "$existing" ]; then
        printf '{\n%s,\n%s\n}\n' "$existing" "$NEW_ENTRY" > "$tmp_cache"
    else
        printf '{\n%s\n}\n' "$NEW_ENTRY" > "$tmp_cache"
    fi
else
    printf '{\n%s\n}\n' "$NEW_ENTRY" > "$tmp_cache"
fi

# Atomic move: only replace the cache file if tmp write succeeded
mv "$tmp_cache" "$CACHE_FILE"
