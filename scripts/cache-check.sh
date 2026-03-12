#!/usr/bin/env bash
# Check if scoped files have changed since last consensus.
# Usage: cache-check.sh <pair-name> <scope-glob>
# Scope-glob supports comma-separated patterns (e.g., "scripts/*.sh,install.sh")
# Exit 0 = unchanged (skip debate), Exit 1 = changed (debate needed)
set -euo pipefail

command -v python3 >/dev/null 2>&1 || { echo "Error: python3 is required but not found" >&2; exit 1; }

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
matched_files=""
IFS=',' read -ra globs <<< "$SCOPE_GLOB"
for glob in "${globs[@]}"; do
    glob="$(echo "$glob" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
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
cached_hash=$(python3 -c "
import json, sys
try:
    with open(sys.argv[1]) as f:
        cache = json.load(f)
    print(cache.get(sys.argv[2], {}).get('hash', ''))
except (json.JSONDecodeError, FileNotFoundError):
    print('')
" "$CACHE_FILE" "$PAIR_NAME" 2>/dev/null)

if [ "$current_hash" = "$cached_hash" ]; then
    exit 0
else
    exit 1
fi
