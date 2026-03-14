---
name: ratchet:advise
description: On-demand workflow health check — analyst reviews debates, scores, retros, and escalations to recommend improvements
---

# /ratchet:advise — Workflow Health Check

On-demand assessment of your Ratchet workflow's health. The analyst reviews accumulated data to identify what's working, what's not, and what to change.

## Usage
```
/ratchet:advise              # Full health check across all pairs
/ratchet:advise [pair-name]  # Focus assessment on a specific pair
```

## Prerequisites
- `.ratchet/` must exist with valid config
- At least some debate history should exist (scores, reviews, retros, or escalations)

If no data exists, inform the user:
> "Not enough data for a health assessment. Run a few milestones first, then come back."

Then use `AskUserQuestion` with options: `"Start a debate (/ratchet:run) (Recommended)"`, `"Done for now"`.

## Execution Steps

### Step 1: Gather Data

Read all available signals:
- `.ratchet/scores/scores.jsonl` — debate metrics per pair
- `.ratchet/reviews/` — performance reviews from debates
- `.ratchet/retros/*.json` — retrospective findings with severity and recurrence
- `.ratchet/escalations/*.json` — orchestrator rulings and dispute patterns
- `.ratchet/debates/*/meta.json` — debate outcomes, fast-path flags, round counts
- `.ratchet/workflow.yaml` — current configuration (pairs, guards, components)
- `.ratchet/plan.yaml` — milestone progress and regression counts

### Step 2: Spawn Analyst

Spawn the **analyst** agent with the following task:

```
Perform a workflow health assessment for this Ratchet project.

Current configuration: [contents of workflow.yaml]
Milestone progress: [contents of plan.yaml]
Score data: [contents of scores.jsonl]
Reviews: [summary of review data]
Retro findings: [contents of retros/*.json]
Escalation rulings: [contents of escalations/*.json]

Analyze the following dimensions (see "Ongoing Workflow Health Monitoring" in your instructions):
1. Pair effectiveness rankings — which pairs add the most/least value?
2. Scope coverage gaps — are there unreviewed files or components?
3. Guard recommendations — missing guards, overly strict guards, timing adjustments
4. Workflow preset recommendations — should any component switch presets?
5. Round trends — are debates converging faster or slower?
6. Fast-path analysis — are any pairs redundant (always TRIVIAL_ACCEPT)?
7. Escalation analysis — are any pairs too contentious (always escalate)?
8. Regression analysis — are regressions concentrated on specific phase transitions?
9. Severity distribution from retros — are critical/major findings being addressed?
10. Settled law coverage — are settled escalation patterns being injected into adversarials?

Present your findings as a prioritized list of actionable recommendations.
```

### Step 3: Present Assessment

Present the analyst's findings via `AskUserQuestion`:

```
Workflow Health Assessment
══════════════════════════

[Analyst's prioritized recommendations — 5-10 bullet points]

Overall health: [healthy / needs attention / at risk]
```

Options:
- `"Apply recommendations (Recommended)"` — walk through each recommendation interactively
- `"Apply all automatically"` — apply all changes without individual confirmation
- `"Export as report"` — save to `.ratchet/reports/health-<timestamp>.md`
- `"Done for now"`

### Step 4: Apply Recommendations (if chosen)

For each recommendation, use `AskUserQuestion` to confirm:
- Question: "[recommendation detail]. Apply this change?"
- Options: `"Apply"`, `"Skip"`, `"Modify"`, `"Stop applying"`

Types of changes the analyst may recommend:
- **Disable or remove a pair** — if it's consistently redundant (always fast-paths with no value)
- **Split a pair** — if it always escalates, break its scope into narrower concerns
- **Add a guard** — if retros show a recurring gap that can be automated
- **Adjust guard timing** — move a guard from post-debate to pre-debate (or vice versa)
- **Change workflow preset** — switch a component from tdd to traditional (or vice versa)
- **Adjust max_rounds** — increase for contentious pairs, decrease for fast-path pairs
- **Inject settled law** — add settled escalation patterns to adversarial prompts
- **Tighten a pair** — suggest running `/ratchet:tighten` for a specific pair

### Step 5: Report

```
Health check complete:
  Recommendations: [N] total
  Applied: [N]
  Skipped: [N]

  [If report exported: "Report saved to .ratchet/reports/health-<timestamp>.md"]
```

Then use `AskUserQuestion`:
- Options:
  - `"Run next debate (/ratchet:run)"`
  - `"Tighten agents (/ratchet:tighten)"`
  - `"View quality metrics (/ratchet:score)"`
  - `"Done for now"`
