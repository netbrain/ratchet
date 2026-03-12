#!/usr/bin/env bash
# Update the file-hash cache after a debate reaches consensus.
# Usage: cache-update.sh <pair-name> <scope-glob> [debate-id]
# Scope-glob supports comma-separated patterns (e.g., "scripts/*.sh,install.sh")
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

PAIR_NAME="${1:?Usage: cache-update.sh <pair-name> <scope-glob> [debate-id]}"
SCOPE_GLOB="${2:?Usage: cache-update.sh <pair-name> <scope-glob> [debate-id]}"
DEBATE_ID="${3:-}"
CACHE_FILE=".ratchet/cache.json"

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

# No files matched = skip caching (avoids infinite debate loop with cache-check)
if [ -z "$matched_files" ]; then
    echo "Warning: No files matched scope '$SCOPE_GLOB' — skipping cache update" >&2
    exit 0
fi

current_hash=$(echo "$matched_files" | while IFS= read -r f; do cat "$f"; done 2>/dev/null | sha256)

# Ensure .ratchet directory exists
mkdir -p "$(dirname "$CACHE_FILE")"

# Update cache
python3 -c "
import json, os, sys
from datetime import datetime, timezone

cache_file = sys.argv[1]
pair_name = sys.argv[2]
file_hash = sys.argv[3]
debate_id = sys.argv[4] if len(sys.argv) > 4 and sys.argv[4] else None

cache = {}
if os.path.isfile(cache_file):
    try:
        with open(cache_file) as f:
            cache = json.load(f)
    except (json.JSONDecodeError, FileNotFoundError):
        cache = {}

cache[pair_name] = {
    'hash': file_hash,
    'debate_id': debate_id,
    'timestamp': datetime.now(timezone.utc).isoformat()
}

with open(cache_file, 'w') as f:
    json.dump(cache, f, indent=2)
    f.write('\n')
" "$CACHE_FILE" "$PAIR_NAME" "$current_hash" "$DEBATE_ID"
