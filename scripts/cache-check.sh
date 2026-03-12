#!/usr/bin/env bash
# Check if scoped files have changed since last consensus.
# Usage: cache-check.sh <pair-name> <scope-glob>
# Exit 0 = unchanged (skip debate), Exit 1 = changed (debate needed)
set -euo pipefail

PAIR_NAME="${1:?Usage: cache-check.sh <pair-name> <scope-glob>}"
SCOPE_GLOB="${2:?Usage: cache-check.sh <pair-name> <scope-glob>}"
CACHE_FILE=".ratchet/cache.json"

# No cache file = always debate
if [ ! -f "$CACHE_FILE" ]; then
    exit 1
fi

# Compute current hash of scoped files
current_hash=$(find . -path "./$SCOPE_GLOB" -type f 2>/dev/null | sort | xargs cat 2>/dev/null | sha256sum | awk '{print $1}')

# Empty hash = no files matched = debate (greenfield)
if [ -z "$current_hash" ] || [ "$current_hash" = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" ]; then
    exit 1
fi

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
