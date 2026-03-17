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

Read debate artifacts directly from the source of truth — no derived score files needed.

**Workspace scoping**: In a multi-workspace setup, `/ratchet:score` with no arguments shows scores for the current workspace. Use `/ratchet:score --all` to aggregate across all workspaces, or `/ratchet:score [workspace]` for a specific workspace.

**Error handling for debate metadata**: When reading meta.json files, skip any that are malformed:
```bash
for f in .ratchet/debates/*/meta.json; do
  jq empty "$f" 2>/dev/null || { echo "Warning: Skipping malformed $f" >&2; continue; }
  # process valid file
done
```

**Debate metadata**: Read all `.ratchet/debates/*/meta.json` files. Each contains:
```json
{
  "id": "debate-id",
  "pair": "pair-name",
  "phase": "review",
  "milestone": "milestone-id or null",
  "issue": "issue-ref or null",
  "files": ["file1", "file2"],
  "status": "consensus|resolved|escalated",
  "rounds": 3,
  "max_rounds": 3,
  "started": "ISO timestamp",
  "resolved": "ISO timestamp",
  "verdict": "ACCEPT|CONDITIONAL_ACCEPT|TRIVIAL_ACCEPT|REJECT|REGRESS",
  "fast_path": false
}
```

**Review data**: Read all `.ratchet/reviews/<pair-name>/review-*.json` files. Each contains effectiveness scores and missed issues from both agents.

If no debate directories exist, inform the user:
> "No debates found. Run /ratchet:run to start your first debate."

Then use `AskUserQuestion` with options: `"Start a debate (/ratchet:run) (Recommended)"`, `"Done for now"`.

### Step 2: Compute Metrics

Per pair, calculate from meta.json files:
- **Total debates**: count of meta.json files for this pair
- **Consensus rate**: % with status "consensus" or "resolved" (not "escalated")
- **Avg rounds to consensus**: mean of `rounds` field (excluding escalated)
- **Verdict breakdown**: count of ACCEPT, CONDITIONAL_ACCEPT, TRIVIAL_ACCEPT
- **Fast-path rate**: % of debates with `fast_path: true` (TRIVIAL_ACCEPT)
- **Trend**: compare last 5 debates to previous 5 by avg rounds — improving, stable, or degrading. If fewer than 10 total debates exist, show `Trend: — (insufficient data for last-5-vs-previous-5)` and instead describe the trajectory narratively.

From review files, calculate:
- **Avg effectiveness**: mean of self_assessment.effectiveness and partner_assessment.effectiveness across all reviews
- **Gen vs Adv split**: separate averages for generative and adversarial effectiveness
- **Top missed issues**: most commonly flagged patterns from missed_issues arrays

### Step 3: Present

```
Ratchet Quality Scores
═══════════════════════

Pair: [pair-name]
  Debates: [N] | Consensus rate: [N]% | Avg rounds: [N.N]
  Verdicts: [N] ACCEPT, [N] CONDITIONAL_ACCEPT, [N] TRIVIAL_ACCEPT
  Fast-path rate: [N]% | Trend: [↑ improving | → stable | ↓ degrading | — insufficient data]
  Effectiveness: gen [N.N] / adv [N.N]

[...repeat for each pair...]

Overall:
  Total debates: [N] | Overall consensus: [N]%
  Avg rounds: [N.N] | Fast-path rate: [N]%
  Quality trajectory: [↑ improving | → stable | ↓ degrading]
```

If a specific pair is requested, show more detail including:
- Recent debate summaries (last 5)
- Top missed issues from reviews
- Suggestions from reviews

### Step 4: Next Steps

After presenting metrics, use `AskUserQuestion` to guide the user:
- Options (adapt based on context):
  - "Run next debate (/ratchet:run)" — if milestones remain
  - "Tighten agents (/ratchet:tighten)" — if enough review data exists
  - "View a specific debate" — for drill-down
  - "Done for now"
