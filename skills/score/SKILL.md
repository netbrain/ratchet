---
name: ratchet:score
description: View quality metrics and trends per pair
---

# /ratchet:score — Quality Metrics

View quality metrics and trends across all pairs or a specific pair.

## Usage
```
/ratchet:score              # Show all pairs' metrics
/ratchet:score [pair-name]  # Show metrics for a specific pair
```

## Execution Steps

### Step 1: Load Data

Read `.ratchet/scores/scores.jsonl` — each line is a JSON object with:
```json
{
  "timestamp": "ISO date",
  "debate_id": "id",
  "pair": "pair-name",
  "rounds_to_consensus": N,
  "escalated": bool,
  "issues_found": N,
  "issues_resolved": N
}
```

### Step 2: Compute Metrics

Per pair, calculate:
- **Total debates**: count
- **Consensus rate**: % of debates reaching consensus without escalation
- **Avg rounds to consensus**: mean rounds (excluding escalated)
- **Issues found**: total across all debates
- **Issues resolved**: total resolved
- **Resolution rate**: resolved / found
- **Trend**: compare last 5 debates to previous 5 — improving, stable, or degrading

### Step 3: Present

```
Ratchet Quality Scores
═══════════════════════

Pair: api-contracts
  Debates: 12 | Consensus rate: 83% | Avg rounds: 1.8
  Issues: 24 found, 22 resolved (92%)
  Trend: ↑ improving (fewer rounds, higher resolution)

Pair: db-performance
  Debates: 8 | Consensus rate: 62% | Avg rounds: 2.4
  Issues: 15 found, 11 resolved (73%)
  Trend: → stable

Overall:
  Total debates: 20 | Overall consensus: 75%
  Quality trajectory: ↑ improving
```

If a specific pair is requested, show more detail including recent debate summaries.
