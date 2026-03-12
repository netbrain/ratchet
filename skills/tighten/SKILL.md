---
name: tighten
description: Tighten the ratchet — analyst reviews agent performance and sharpens pair definitions
user-invocable: true
---

# /ratchet:tighten — Tighten the Ratchet

Tighten agent pairs based on accumulated debate performance. The analyst reviews what agents missed, where they wasted effort, and rewrites their prompts to be sharper.

## Usage
```
/ratchet:tighten              # Tighten all pairs with sufficient review data
/ratchet:tighten [pair-name]  # Tighten a specific pair
```

## Execution Steps

### Step 1: Check Review Data

Read `.ratchet/reviews/<pair-name>/` for each target pair. If fewer than 3 reviews exist, inform the user that more debate data is needed before meaningful tightening can occur.

### Step 2: Launch Analyst

Spawn the **analyst** agent with the following task:

```
Review the performance data for agent pair [pair-name] and propose improvements.

Current agent definitions:
  Generative: [contents of .ratchet/pairs/<name>/generative.md]
  Adversarial: [contents of .ratchet/pairs/<name>/adversarial.md]

Performance reviews: [contents of all review files]

Project context: [contents of .ratchet/project.yaml]

Your task:
1. Analyze patterns across all reviews:
   - What issues are repeatedly missed?
   - Where is effort wasted on non-issues?
   - What blind spots exist?
   - What strengths should be preserved?

2. Propose specific prompt improvements:
   - Add knowledge about commonly missed issues
   - Remove focus areas that consistently produce false positives
   - Sharpen the adversarial's test strategy
   - Improve the generative's fix patterns

3. Present proposed changes as a diff:
   - Show what would change in each agent's definition
   - Explain the rationale for each change
   - Flag whether this is incremental tuning or a significant rework

4. Wait for human approval before writing any changes

5. If approved, update the agent definitions and write an analyst summary:
   .ratchet/reviews/<pair-name>/analyst-summary.md
   containing:
   - Date of tightening
   - Changes made and rationale
   - Patterns identified
   - Recommendations for next tightening
```

### Step 3: Report

```
Ratchet tightened for [pair-name]:
  Changes: [summary of what changed]
  Rationale: [why]
  Next tightening: after [N] more debates

[If no changes needed:]
Pair [pair-name] is already sharp — no changes recommended.
Review data shows consistent effectiveness with no new patterns.
```
