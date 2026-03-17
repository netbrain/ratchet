---
name: ratchet:tighten
description: Tighten the ratchet — analyst reviews agent performance and sharpens pair definitions
---

# /ratchet:tighten — Tighten the Ratchet

Tighten agent pairs based on accumulated debate performance. The analyst reviews what agents missed, where they wasted effort, and rewrites their prompts to be sharper.

**You have full read/write access.** This skill directly edits pair definition files (generative.md, adversarial.md) and writes review summaries. Do NOT defer edits to the user — apply changes yourself using the Edit tool.

## Usage
```
/ratchet:tighten              # Tighten all pairs with sufficient review data
/ratchet:tighten [pair-name]  # Tighten a specific pair
```

## Execution Steps

### Step 1: Check Review Data

Read `.ratchet/reviews/<pair-name>/` for each target pair. Look for `review-*.json` files (produced by `/ratchet:run` Step 9).

If the reviews directory doesn't exist or has no review files (0 reviews), inform the user:
> "No review data found for [pair-name]. Reviews are generated after debates complete. Run /ratchet:run to produce debate data first."

Then use `AskUserQuestion` with options: `"Start a debate (/ratchet:run) (Recommended)"`, `"Done for now"`.

Do NOT offer to proceed with 0 reviews — the analyst would have no data to work with.

If 1-2 reviews exist, inform the user that limited data exists but more is needed for meaningful tightening:
- Use `AskUserQuestion` with options: `"Proceed with limited data (results may be shallow)"`, `"Run more debates first (/ratchet:run) (Recommended)"`, `"Done for now"`

If 3+ reviews exist, proceed directly to Step 2.

### Step 2: Launch Analyst

Spawn the **analyst** agent using the generative model from `workflow.yaml` (`models.generative`, default `opus`). Agent configuration:
- `subagent_type`: analyst
- `model`: value of `workflow.yaml` → `models.generative` (or `opus` if unset)
- `tools`: Read, Grep, Glob, Bash, Write, Edit, AskUserQuestion

The analyst has Write/Edit access because it directly edits pair definition files (generative.md, adversarial.md). This is intentional — the analyst IS the generative role for tightening.

Task prompt:

```
Review the performance data for agent pair [pair-name] and propose improvements.

Current agent definitions:
  Generative: [contents of .ratchet/pairs/<name>/generative.md]
  Adversarial: [contents of .ratchet/pairs/<name>/adversarial.md]

Performance reviews: [contents of all review files]

Retrospective findings: [contents of .ratchet/retros/*.json, if any]
(These are gaps found AFTER debates — CI failures, PR review feedback, production issues
that Ratchet's debates did not catch. These are high-signal for improvement.)

Project context: [contents of .ratchet/project.yaml]

Escalation rulings: [contents of .ratchet/escalations/*.json, if any]
(These are tiebreaker verdicts from prior escalations. If a dispute type has 3+ rulings
in the same direction, inject it as "settled law" in the adversarial prompt so the pair
stops re-litigating settled disputes.)

PRINCIPLE — Guilty Until Proven Innocent:
  New changes are GUILTY until proven innocent. When reviewing CI failures
  and retro findings, assume the PR caused the failure unless there is
  definitive evidence the failure exists on master. Agents should be
  tightened to internalize this principle — they must fix failures, not
  dismiss them as "pre-existing" or "flaky" without proof.

Your task:
1. Analyze patterns across all reviews AND retro findings:
   - What issues are repeatedly missed by debates?
   - What did CI or human reviewers catch that Ratchet didn't?
   - Where is effort wasted on non-issues?
   - What blind spots exist?
   - What strengths should be preserved?
   - Are there missing guards that should be added?
   - Are agents dismissing test failures without proving them pre-existing? (guilty-until-proven-innocent violation)

1b. Process retro findings by severity (critical first, skip noise unless asked):
   - Present severity distribution: "[N] critical, [N] major, [N] minor, [N] noise (skipped)"
   - Flag recurring findings (those with `related_findings`) as needing structural fixes, not just prompt tweaks

1c. Consume escalation rulings from `.ratchet/escalations/*.json`:
   - If a dispute type has 3+ rulings in the same direction, inject as "settled law" in the adversarial prompt
   - This prevents the pair from re-litigating disputes that have been consistently resolved the same way

2. Propose specific improvements:
   - Add knowledge about commonly missed issues to agent prompts
   - Add missing validation commands to adversarial agents
   - Remove focus areas that consistently produce false positives
   - Sharpen the adversarial's test strategy
   - Improve the generative's fix patterns
   - Propose new guards for checks that CI catches but Ratchet doesn't

3. Present proposed changes using `AskUserQuestion` for approval:
   - Show what would change in each agent's definition in the question text
   - Explain the rationale for each change
   - Flag whether this is incremental tuning or a significant rework
   - Options: "Approve all changes (Recommended)", "Approve with modifications", "Reject changes"

4. If "Approve with modifications", use follow-up `AskUserQuestion` calls to refine. Wait for explicit human approval before writing any changes.

5. If approved, update the agent definitions and write an analyst summary:
   .ratchet/reviews/<pair-name>/analyst-summary.md
   (Note: This file is intentionally .md, not .json, because it is a human-readable narrative
   of what changed and why. The review-*.json files are machine-generated metrics; the analyst
   summary is prose for human consumption.)
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

After reporting, use `AskUserQuestion` to guide the user:
- Options:
  - "Tighten another pair" — if other pairs have review data
  - "Run next debate (/ratchet:run)"
  - "View quality metrics (/ratchet:score)"
  - "Done for now"
