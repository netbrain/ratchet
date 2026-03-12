#!/usr/bin/env bash
# Append a score entry to scores.jsonl after a debate resolves.
# Usage: update-scores.sh <debate-id>

set -euo pipefail

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

meta = json.load(open(sys.argv[1]))

# Count issues from adversarial round files
import glob, re
rounds_dir = sys.argv[1].replace('meta.json', 'rounds')
issues_found = 0
issues_resolved = 0

for f in sorted(glob.glob(f'{rounds_dir}/round-*-adversarial.md')):
    content = open(f).read()
    # Count findings by looking for severity markers
    issues_found += len(re.findall(r'\"severity\":', content))

# If consensus or accept verdict, assume all issues resolved
verdict = meta.get('verdict', {})
decision = verdict.get('decision', '') if verdict else ''
if decision in ('accept', '') and meta.get('status') == 'consensus':
    issues_resolved = issues_found
elif decision == 'modify':
    # Partial resolution
    issues_resolved = max(0, issues_found - len(verdict.get('required_changes', [])))
else:
    issues_resolved = 0

score = {
    'timestamp': datetime.now(timezone.utc).isoformat(),
    'debate_id': meta['id'],
    'pair': meta['pair'],
    'rounds_to_consensus': meta['rounds'],
    'escalated': meta['status'] == 'escalated' or (verdict and verdict.get('decided_by') != 'consensus'),
    'issues_found': issues_found,
    'issues_resolved': issues_resolved
}

with open(sys.argv[2], 'a') as f:
    f.write(json.dumps(score) + '\n')

print(f'Score recorded for {meta[\"id\"]}: {issues_found} found, {issues_resolved} resolved')
" "$META_FILE" "$SCORES_FILE"
