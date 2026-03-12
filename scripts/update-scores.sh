#!/usr/bin/env bash
# Append a score entry to scores.jsonl after a debate resolves.
# Usage: update-scores.sh <debate-id>

set -euo pipefail

command -v python3 >/dev/null 2>&1 || { echo "Error: python3 is required but not found" >&2; exit 1; }

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

# Extract fields and append score
python3 -c "
import json, sys
from datetime import datetime, timezone

try:
    with open(sys.argv[1]) as f:
        meta = json.load(f)
except json.JSONDecodeError as e:
    print(f'Error: Malformed JSON in {sys.argv[1]}: {e}', file=sys.stderr)
    sys.exit(1)

# Validate required fields
for key in ('id', 'pair', 'rounds'):
    if key not in meta:
        print(f'Error: Missing required field \"{key}\" in {sys.argv[1]}', file=sys.stderr)
        sys.exit(1)

# Count issues from adversarial round files
import glob, os, re
rounds_dir = os.path.join(os.path.dirname(sys.argv[1]), 'rounds')
issues_found = 0
issues_resolved = 0

for f in sorted(glob.glob(f'{rounds_dir}/round-*-adversarial.md')):
    content = open(f).read()
    # Count findings by looking for severity markers
    issues_found += len(re.findall(r'\"severity\":', content))

# If consensus or accept verdict, assume all issues resolved
verdict = meta.get('verdict') or {}
decision = verdict.get('decision', '')
if decision in ('accept', '') and meta.get('status') in ('consensus', 'resolved'):
    issues_resolved = issues_found
elif decision in ('modify', 'conditional_accept'):
    # Partial resolution
    issues_resolved = max(0, issues_found - len(verdict.get('required_changes', [])))
else:
    issues_resolved = 0

score = {
    'timestamp': datetime.now(timezone.utc).isoformat(),
    'debate_id': meta['id'],
    'pair': meta['pair'],
    'milestone': meta.get('milestone', None),
    'rounds_to_consensus': meta['rounds'],
    'escalated': meta.get('status') not in ('consensus',),
    'issues_found': issues_found,
    'issues_resolved': issues_resolved
}

with open(sys.argv[2], 'a') as f:
    f.write(json.dumps(score) + '\n')

print(f'Score recorded for {meta[\"id\"]}: {issues_found} found, {issues_resolved} resolved')
" "$META_FILE" "$SCORES_FILE"
