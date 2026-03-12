#!/usr/bin/env bash
# Update the file-hash cache after a debate reaches consensus.
# Usage: cache-update.sh <pair-name> <scope-glob> [debate-id]
set -euo pipefail

PAIR_NAME="${1:?Usage: cache-update.sh <pair-name> <scope-glob> [debate-id]}"
SCOPE_GLOB="${2:?Usage: cache-update.sh <pair-name> <scope-glob> [debate-id]}"
DEBATE_ID="${3:-}"
CACHE_FILE=".ratchet/cache.json"

# Compute current hash of scoped files
current_hash=$(find . -path "./$SCOPE_GLOB" -type f 2>/dev/null | sort | xargs cat 2>/dev/null | sha256sum | awk '{print $1}')

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
