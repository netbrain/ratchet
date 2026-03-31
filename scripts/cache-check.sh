#!/usr/bin/env bash
# Check if scoped files have changed since last consensus.
# Usage: cache-check.sh <pair-name> <scope-glob>
# Scope-glob supports comma-separated patterns (e.g., "scripts/*.sh,install.sh")
# Exit 0 = unchanged (skip debate), Exit 1 = changed (debate needed)
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

PAIR_NAME="${1:?Usage: cache-check.sh <pair-name> <scope-glob>}"
SCOPE_GLOB="${2:?Usage: cache-check.sh <pair-name> <scope-glob>}"
CACHE_FILE=".ratchet/cache.json"

# No cache file = always debate
if [ ! -f "$CACHE_FILE" ]; then
    exit 1
fi

# Collect files matching comma-separated scope globs
# Note: find -path's * already matches across directories (unlike shell globbing),
# so ** is redundant. We collapse **/ and ** to * for compatibility.
matched_files=""
IFS=',' read -ra globs <<< "$SCOPE_GLOB"
for glob in "${globs[@]}"; do
    glob="$(echo "$glob" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//;s|\*\*/|*|g;s|\*\*|*|g')"
    # Validate glob pattern: must contain only safe characters for find -path
    # Reject empty patterns or patterns with characters that could cause issues
    if [ -z "$glob" ]; then
        echo "Warning: empty glob pattern skipped" >&2
        continue
    fi
    if [[ "$glob" =~ [[:cntrl:]] ]] || [[ "$glob" == *$'\n'* ]]; then
        echo "Warning: glob pattern contains control characters, skipped: $glob" >&2
        continue
    fi
    matches=$(find . -path "./$glob" -type f 2>/dev/null || true)
    if [ -n "$matches" ]; then
        matched_files="${matched_files:+${matched_files}
}${matches}"
    fi
done
matched_files=$(echo "$matched_files" | sort -u)

# No files matched = debate (greenfield)
if [ -z "$matched_files" ]; then
    exit 1
fi

current_hash=$(echo "$matched_files" | while IFS= read -r f; do cat "$f"; done 2>/dev/null | sha256)

# Compare against cached hash
# cache.json structure: { "pair-name": { "hash": "...", ... }, ... }
# We look for the pair name key, then extract the hash value from within its block.
cached_hash=$(sed -n '/"'"$PAIR_NAME"'"/,/}/s/.*"hash"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$CACHE_FILE" 2>/dev/null | head -1)

if [ "$current_hash" = "$cached_hash" ]; then
    exit 0
else
    exit 1
fi
