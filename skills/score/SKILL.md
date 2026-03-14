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

Read `.ratchet/scores/scores.jsonl`. If the file does not exist or is empty, inform the user:
> "No quality scores recorded yet. Scores are generated after debates complete. Run /ratchet:run to start your first debate."

Then use `AskUserQuestion` with options: `"Start a debate (/ratchet:run)"`, `"Done for now"`.

If data exists, each line is a JSON object with:
```json
{
  "timestamp": "ISO date",
  "debate_id": "id",
  "pair": "pair-name",
  "milestone": "milestone id or null",
  "rounds_to_consensus": N,
  "escalated": bool,
  "issues_found": N,
  "issues_resolved": N,
  "fast_path": bool
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
- **Fast-path rate**: % of debates with `fast_path: true` (TRIVIAL_ACCEPT)
- **Trend**: compare last 5 debates to previous 5 — improving, stable, or degrading

### Step 3: Present

```
Ratchet Quality Scores
═══════════════════════

Pair: api-contracts
  Debates: 12 | Consensus rate: 83% | Avg rounds: 1.8
  Issues: 24 found, 22 resolved (92%)
  Fast-path rate: 25% | Trend: ↑ improving (fewer rounds, higher resolution)

Pair: db-performance
  Debates: 8 | Consensus rate: 62% | Avg rounds: 2.4
  Issues: 15 found, 11 resolved (73%)
  Fast-path rate: 0% | Trend: → stable

Overall:
  Total debates: 20 | Overall consensus: 75%
  Fast-path rate: 15% | Quality trajectory: ↑ improving
```

If a specific pair is requested, show more detail including recent debate summaries.

### Step 4: Next Steps

After presenting metrics, use `AskUserQuestion` to guide the user:
- Options (adapt based on context):
  - "Run next debate (/ratchet:run)" — if milestones remain
  - "Tighten agents (/ratchet:tighten)" — if enough review data exists
  - "View a specific debate" — for drill-down
  - "Done for now"
