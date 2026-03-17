---
name: ratchet:tighten
description: Tighten the ratchet — analyze all improvement signals and sharpen the system
---

# /ratchet:tighten — Tighten the Ratchet

Single entrypoint for improving the Ratchet system. Gathers all improvement signals — debate history, CI failures, PR feedback, discoveries, escalation rulings, retro findings — triages them by priority, and applies fixes: sharpened agent prompts, new guards, workflow config changes.

This replaces three formerly separate skills (advise, retro, tighten) with one unified flow.

**You have full read/write access.** This skill directly edits pair definition files (generative.md, adversarial.md), workflow.yaml, and writes analysis summaries. Do NOT defer edits to the user — apply changes yourself using the Edit tool. Always get human approval via AskUserQuestion before writing changes.

## Usage
```
/ratchet:tighten                    # Auto-triage all signals, prioritize, improve
/ratchet:tighten [pair-name]        # Focus on a specific pair
/ratchet:tighten pr [number]        # Analyze a PR's CI results and review comments
```

## Prerequisites
- `.ratchet/` must exist with valid config
- At least some signal data should exist (debates, reviews, retros, escalations, CI results, or discoveries)

If `.ratchet/` does not exist, inform the user:
> "Ratchet is not initialized for this project. Run /ratchet:init to set up."

Then use `AskUserQuestion` with options: `"Initialize now (/ratchet:init) (Recommended)"`, `"Cancel"`.

## Foundational Principle — Guilty Until Proven Innocent

**New changes are GUILTY until proven innocent.** When analyzing CI failures on a PR branch, the default assumption is that the PR caused the failure. The burden of proof is on demonstrating the failure exists on master — not on assuming it is unrelated or a "flake."

When classifying findings:
- **Do NOT classify a failure as `noise` (CI flake)** unless you have evidence it passed on re-run OR the same failure exists on master. "It looks like a flake" is not evidence.
- **Do NOT skip a failure** because "it's probably unrelated." Prove it by checking master.
- **Verification command**: Before downgrading any CI failure severity, run:
  ```bash
  # Check if the same test fails on master
  gh run list --branch main --workflow "<workflow>" --limit 3 --json conclusion -q '.[].conclusion'
  # Or check the specific test on the base commit
  git log --oneline -1 origin/main  # get base SHA
  # If all recent main runs passed, the PR is guilty
  ```

## Execution Steps

### Step 1: Gather All Signals

Read every available improvement signal from the project:

**Debate artifacts:**
- `.ratchet/debates/*/meta.json` — debate outcomes, fast-path flags, round counts
- `.ratchet/reviews/<pair-name>/review-*.json` — post-debate performance reviews

**External feedback:**
- `.ratchet/retros/*.json` — retrospective findings with severity and recurrence

**Escalation patterns:**
- `.ratchet/escalations/*.json` — tiebreaker rulings and dispute patterns

**Discoveries:**
- `.ratchet/plan.yaml` → `epic.discoveries` — pending sidequests from watch, retro, or manual logging

**Configuration:**
- `.ratchet/workflow.yaml` — current pairs, guards, components
- `.ratchet/project.yaml` — project stack, validation commands
- `.ratchet/plan.yaml` — milestone progress, regression counts

**PR-specific signals (when `pr [number]` mode):**
```bash
gh pr view <number> --json state,reviews,comments,statusCheckRollup
gh pr checks <number>
```

If no signal data exists at all (no debates, no retros, no reviews, no escalations), inform the user:
> "Not enough data to tighten. Run a few milestones first, then come back."

Then use `AskUserQuestion` with options: `"Start a debate (/ratchet:run) (Recommended)"`, `"Done for now"`.

### Step 2: Mode-Specific Signal Gathering

#### Mode: Auto-triage (no arguments or `[pair-name]`)

All signals from Step 1 are already gathered. If `[pair-name]` is specified, filter to signals relevant to that pair only.

#### Mode: PR Analysis (`tighten pr [number]`)

1. **Gather PR signals:**
   ```bash
   gh pr view <number> --json state,reviews,comments,statusCheckRollup
   gh pr checks <number>
   ```

2. **Identify failures and feedback:**
   - CI check failures (lint, tests, security scans, build)
   - Review comments from humans (requested changes, concerns raised)
   - Merge conflicts or blocked status

3. **Map failures to Ratchet gaps:**
   For each failure or piece of feedback, determine:
   - **Was this in scope of a debate pair?** Check file paths against pair scopes
   - **Did the adversarial agent have the right validation commands?** Check if the failing CI command is in the adversarial's prompt
   - **Is there a guard for this?** Check if a guard exists for this type of check
   - **Was this a phase gap?** Should this have been caught in a specific phase?

### Step 3: Triage and Prioritize

Spawn the **analyst** agent using the generative model from `workflow.yaml` (`models.generative`, default `opus`). Agent configuration:
- `model`: value of `workflow.yaml` → `models.generative` (or `opus` if unset)
- `tools`: Read, Grep, Glob, Bash, Write, Edit, AskUserQuestion

The analyst has Write/Edit access because it directly edits pair definition files, workflow.yaml, and writes summaries. It MUST use AskUserQuestion to get human approval before any file modification.

Task prompt:

```
Analyze all improvement signals for this Ratchet project and produce a prioritized action plan.

Current configuration: [contents of workflow.yaml]
Project context: [contents of project.yaml]
Milestone progress: [contents of plan.yaml]

Signal data:
  Debate metadata: [summary of debates/*/meta.json — counts, verdicts, rounds]
  Performance reviews: [contents of reviews/**/*.json]
  Retro findings: [contents of retros/*.json]
  Escalation rulings: [contents of escalations/*.json]
  Pending discoveries: [epic.discoveries with status == "pending"]
  [If PR mode: PR analysis for #<number>: [CI failures, review comments, merge status]]

PRINCIPLE — Guilty Until Proven Innocent:
  New changes are GUILTY until proven innocent. When reviewing CI failures
  and retro findings, check whether agents are dismissing failures without
  proof. A healthy workflow holds PRs accountable for their test failures.

Analyze these dimensions:

1. **PR/CI gaps** (if PR mode) — map each failure to a Ratchet gap:
   - Missing validation command in adversarial prompt
   - Missing guard at a phase boundary
   - Missing pair for an uncovered quality dimension
   - Phase gap (caught too late in the pipeline)

2. **Pair effectiveness** — which pairs add the most/least value?
   - Fast-path rate: pairs that always TRIVIAL_ACCEPT may be redundant
   - Escalation rate: pairs that always escalate may need splitting
   - Round trends: are debates converging faster or slower?

3. **Retro findings by severity** — process critical first:
   - Present distribution: "[N] critical, [N] major, [N] minor, [N] noise"
   - Flag recurring findings (those with `related_findings`) as structural
   - Cross-retro recurrence: if same gap found 2+ times, auto-escalate severity

4. **Escalation patterns** — settled law injection:
   - If a dispute type has 3+ rulings in the same direction, inject as "settled law"
     in the adversarial prompt to stop re-litigating settled disputes

5. **Scope coverage gaps** — files modified that fall outside all pair scopes

6. **Guard recommendations** — missing guards, timing adjustments, overly strict guards

7. **Workflow preset recommendations** — should any component switch presets?

8. **Guilty-until-proven-innocent compliance** — are agents dismissing test
   failures as "flaky" or "pre-existing" without evidence?

Produce a PRIORITIZED list of actionable improvements, grouped by type:
  A. Pair prompt changes (add knowledge, sharpen adversarial, settle law)
  B. New/modified guards
  C. Workflow config changes (scope, timing, presets, max_rounds)
  D. New pairs needed
  E. Pairs to disable/remove

For each improvement, include:
  - What to change (specific file and content)
  - Why (evidence from which signal)
  - Priority (critical / high / medium / low)
  - Type (prompt-tweak / structural / config-change)
```

**Error handling**: If the analyst agent fails or returns no recommendations:
> "Analysis could not be completed. This may be due to insufficient data or an internal error."

Then use `AskUserQuestion` with options: `"Try again"`, `"View raw data (/ratchet:score)"`, `"Done for now"`.

### Step 4: Present Assessment and Apply

Present the analyst's findings via `AskUserQuestion`:

```
Tighten Assessment
==================

[If PR mode: "PR #[number]: [N] gaps found"]

[Analyst's prioritized improvements — grouped by type]

Overall health: [healthy / needs attention / at risk]
Signal sources: [N] debates, [N] retros, [N] escalations, [N] discoveries
```

Options:
- `"Apply improvements (Recommended)"` — walk through each improvement interactively
- `"Apply all automatically"` — apply all changes without individual confirmation
- `"Export as report"` — save to `.ratchet/reports/tighten-<timestamp>.md` (creates `.ratchet/reports/` directory if it doesn't exist: `mkdir -p .ratchet/reports`)
- `"Done for now"`

#### Applying improvements

For "Apply improvements" — walk through each improvement by priority:

For each improvement, use `AskUserQuestion` to confirm:
- Question: "[priority] [improvement detail]. Apply this change?"
- Options: `"Apply"`, `"Skip"`, `"Modify"`, `"Stop applying"`

Types of changes the analyst applies:

- **Missing validation command** → Edit `.ratchet/pairs/<name>/adversarial.md` to add the command
- **Missing guard** → Add to `guards` array in `.ratchet/workflow.yaml`
- **Missing pair** → Suggest running `/ratchet:pair` for the uncovered dimension
- **Phase gap** → Reassign a pair to an earlier phase
- **Settled law injection** → Add settled patterns to adversarial prompts
- **Disable/remove pair** → Set `enabled: false` or remove from workflow.yaml
- **Split pair** → Create two narrower pairs from one broad pair
- **Adjust guard timing** → Move guard from post-debate to pre-debate (or vice versa)
- **Change workflow preset** → Switch a component from tdd to traditional (or vice versa)
- **Adjust max_rounds** → Increase for contentious pairs, decrease for fast-path pairs
- **Sharpen prompts** → Add knowledge about commonly missed issues, remove false-positive patterns

For "Apply all automatically" — apply all changes without individual confirmation, then show a summary of what changed.

### Step 5: Store Results

#### 5a. Retro findings (PR/CI mode)

When improvements originated from PR analysis, store findings for future reference:

Write to `.ratchet/retros/<timestamp>.json`:
```json
{
  "timestamp": "<ISO timestamp>",
  "source": "pr",
  "source_ref": "<PR number>",
  "findings": [
    {
      "type": "missing_validation|missing_guard|missing_pair|phase_gap",
      "description": "what was missed",
      "evidence": "CI output or review comment",
      "fix_applied": "what was changed, or null if skipped",
      "severity": "critical|major|minor|noise",
      "related_findings": ["<timestamp>:<index>"]
    }
  ]
}
```

**Cross-retro recurrence check**: Before storing, scan existing `.ratchet/retros/*.json` for findings with the same `type` and a similar `description`. If 2+ prior matches exist:
- Auto-escalate severity one level (noise -> minor -> major -> critical; critical stays critical)
- Populate `related_findings` with references to the matching prior findings
- Present: "This is the Nth time this gap was found. Escalating from [old severity] to [new severity]."

#### 5b. Create sidequests for skipped findings

For any finding with severity `major` or `critical` where `fix_applied` is null (user chose to skip), add a discovery to `epic.discoveries` in `.ratchet/plan.yaml` so it surfaces as actionable work in future runs:
```bash
if [ -f .ratchet/plan.yaml ]; then
  yq eval -i ".epic.discoveries += [{
    \"ref\": \"discovery-tighten-$(date +%s)\",
    \"title\": \"Address tighten finding: $description\",
    \"description\": \"Tighten finding (skipped): $description. Evidence: $evidence\",
    \"source\": \"tighten-$timestamp\",
    \"created_at\": \"$(date -Iseconds)\",
    \"severity\": \"$severity\",
    \"retro_type\": \"skipped-finding\",
    \"status\": \"pending\",
    \"issue_ref\": null,
    \"affected_scope\": null
  }]" .ratchet/plan.yaml
fi
```

#### 5c. Analyst summary

Write an analyst summary to `.ratchet/reviews/<pair-name>/analyst-summary.md` (per pair that was tightened) or `.ratchet/reports/tighten-<timestamp>.md` (for full assessment):
- Date of tightening
- Signal sources analyzed
- Changes made and rationale
- Patterns identified
- Recommendations for next tightening

### Step 6: Report

```
Tighten complete:

  Signals analyzed: [N] debates, [N] retros, [N] escalations, [N] discoveries
  [If PR mode: "PR #[number]: [N] gaps found"]
  Improvements found: [N] total
  Applied: [N]
  Skipped: [N]

  Changes:
    [list of changes made, one per line]

  [If report exported: "Report saved to .ratchet/reports/tighten-<timestamp>.md"]
  [If retro stored: "Findings saved to .ratchet/retros/<timestamp>.json"]
```

Then use `AskUserQuestion`:
- Options:
  - `"Run next debate (/ratchet:run) (Recommended)"`
  - `"View quality metrics (/ratchet:score)"`
  - `"Tighten another pair"` — if other pairs exist
  - `"Done for now"`
