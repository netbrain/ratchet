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

**Execution logs:**
- `.ratchet/executions/*.yaml` — solo/promoted execution records: mode, guard results, promotion events, files modified, token estimates

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

Context: [contents of workflow.yaml, project.yaml, plan.yaml]
Signals: [debate meta.json summaries, reviews, retros, escalations, execution logs, pending discoveries]
[If PR mode: PR analysis for #<number>: CI failures, review comments, merge status]

PRINCIPLE: New changes are GUILTY until proven innocent. Check whether agents dismiss failures without proof.

Analyze these 10 dimensions:

1. **PR/CI gaps** (PR mode) — map failures to: missing validation cmd, missing guard, missing pair, phase gap
2. **Pair effectiveness** — fast-path rate (always TRIVIAL_ACCEPT → redundant), escalation rate (always escalate → split), round convergence trends
3. **Retro severity** — distribution (critical/major/minor/noise), flag recurring findings as structural, auto-escalate if same gap found 2+ times
4. **Escalation patterns** — if a dispute type has 3+ same-direction rulings, inject as settled law in adversarial prompt (see Settled Law Injection below)
5. **Scope coverage gaps** — files modified outside all pair scopes
6. **Guard recommendations** — missing guards, timing adjustments, overly strict guards
7. **Workflow preset recommendations** — should any component switch presets?
8. **Workflow mode effectiveness** — solo guard failure rate >30% → promote to debate; frequent solo→debate promotions → wrong default; compare token costs solo vs debate
9. **Over-engineered workflow** — >80% TRIVIAL_ACCEPT → demote to solo; no adversarial pushback in R1 → simplify pipeline; output specific workflow.yaml field changes
10. **Guilty-until-proven-innocent compliance** — agents dismissing failures without evidence?

11. **Token efficiency (caveman-compress)** — only when `caveman.enabled` is true in workflow.yaml:
   - Review pair definitions (`.ratchet/pairs/*/generative.md` and `adversarial.md`) for verbosity
   - Apply caveman compression to the markdown body below the YAML frontmatter `---` delimiter
   - Use the pair's corresponding role intensity (generative pairs use `caveman.intensity.generative`, adversarial pairs use `caveman.intensity.adversarial`) — if the role intensity is `off`, skip that file
   - **Preserve unchanged**: YAML frontmatter, all code blocks, file paths, command examples, JSON/YAML schemas, tool declarations, verdict keywords
   - **Compress**: narrative prose, explanatory text, rationale sections, verbose instructions
   - Before compressing, create backups: copy `generative.md` to `generative.original.md` (and same for adversarial)
   - Present the before/after diff to the user via `AskUserQuestion` before applying:
     - Question: "Caveman-compress pair definitions? Estimated token savings: ~[N]% per pair."
     - Options: `"Apply all (Recommended)"`, `"Review each pair individually"`, `"Skip compression"`
   - **Reversibility**: If the user later disables caveman (`caveman.enabled: false`), offer to restore originals:
     - Check for `.original.md` files alongside pair definitions
     - Question: "Caveman is disabled. Restore original (uncompressed) pair definitions?"
     - Options: `"Restore originals (Recommended)"`, `"Keep compressed versions"`, `"Skip"`

Output a PRIORITIZED list grouped by: A) Pair prompt changes, B) New/modified guards, C) Workflow config changes, D) New pairs, E) Pairs to disable/remove, F) Token efficiency (caveman-compress recommendations).
Per improvement: what to change (file + content), why (evidence), priority (critical/high/medium/low), type (prompt-tweak/structural/config-change).
```

**Settled Law Injection**: When 3+ escalation rulings agree on a dispute type, edit `.ratchet/pairs/<pair-name>/adversarial.md`:
- Find or create a `### Settled Law (Patterns from Prior Debates)` section (before the last `## ` heading)
- Append: `- [ ] **<dispute type>**: <N> rulings found <direction>. Treat as settled — do not re-litigate. Source: escalations/<ids>.json`
- Verify: `grep -c "Settled Law" .ratchet/pairs/<pair-name>/adversarial.md` returns 1

**Error handling**: If the analyst agent fails or returns no recommendations:
> "Analysis could not be completed. This may be due to insufficient data or an internal error."

Then use `AskUserQuestion` with options: `"Try again"`, `"View raw data (/ratchet:score)"`, `"Done for now"`.

### Step 3b: Adversarial Verification

The analyst's findings are **claims until verified**. Before presenting to the user, run each finding through adversarial fact-checking. This prevents false recommendations from wasting time or degrading the system.

Spawn a **verification agent** with the adversarial model (`models.adversarial`, default `sonnet`). Agent configuration:
- `model`: value of `workflow.yaml` → `models.adversarial` (or `sonnet` if unset)
- `tools`: Read, Grep, Glob, Bash
- `disallowedTools`: Write, Edit

The verification agent is **read-only** — it checks claims, it does not fix them.

Task prompt:

```
Adversarial fact-checker for tighten findings. For each finding, READ the actual files and RUN commands to verify — do not accept claims at face value (analyst may hallucinate).

Check per finding: Is the gap real? Is the evidence accurate? Is the fix correct? Is the severity justified?

Verdict per finding: CONFIRMED | DOWNGRADED (explain) | REJECTED (counter-evidence) | NEEDS_INFO (what's missing)

Findings: [analyst's improvements list]
Pair definitions: [paths to .ratchet/pairs/*/generative.md and adversarial.md]
Guards: [guards array from workflow.yaml]
```

**Processing results:** CONFIRMED → keep. DOWNGRADED → adjust severity, append note. REJECTED → remove, log to `.ratchet/reports/verification-rejected.log` (`mkdir -p .ratchet/reports`). NEEDS_INFO → keep as unverified.

Present summary: `"Verification: [N] checked — [N] confirmed, [N] downgraded, [N] rejected, [N] unverified"`. Only confirmed/downgraded/unverified proceed to Step 4.

**Error handling**: If verifier fails, treat all as NEEDS_INFO and proceed with caveat. If ALL rejected, present options: `"Re-run analysis"`, `"View rejected findings"`, `"Done for now"`. Do NOT proceed to Step 4 with empty list.

### Step 4: Present Assessment and Apply

Present the verified findings via `AskUserQuestion`:

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

For "Apply improvements" — walk through each by priority. Per improvement, `AskUserQuestion`: "[priority] [detail]. Apply?" → Options: `"Apply"`, `"Skip"`, `"Modify"`, `"Stop applying"`.

| Change type | Action |
|---|---|
| Missing validation cmd | Edit `adversarial.md` to add command |
| Missing guard | Add to `guards` in `workflow.yaml` |
| Missing pair | Suggest `/ratchet:pair` |
| Phase gap | Reassign pair to earlier phase |
| Settled law | Add patterns to adversarial prompts |
| Disable/remove pair | Set `enabled: false` or remove |
| Split pair | Create two narrower pairs |
| Adjust guard timing | Move between pre/post-debate |
| Change preset | Switch component workflow preset |
| Adjust max_rounds | Increase (contentious) / decrease (fast-path) |
| Sharpen prompts | Add missed-issue knowledge, remove false positives |

For "Apply all automatically" — apply all without confirmation, then show summary.

### Step 5: Store Results

#### 5a. Retro findings (PR/CI mode)

When improvements originated from PR analysis, write to `.ratchet/retros/<timestamp>.json` with fields: `timestamp`, `source` ("pr"), `source_ref` (PR number), and `findings[]` — each finding has `type` (missing_validation|missing_guard|missing_pair|phase_gap), `description`, `evidence`, `fix_applied` (or null), `severity` (critical|major|minor|noise), `related_findings[]`.

**Cross-retro recurrence**: Before storing, scan existing retros for same `type` + similar `description`. If 2+ prior matches: auto-escalate severity one level, populate `related_findings`, present "Nth occurrence — escalating from [old] to [new]."

#### 5b. Create sidequests for skipped findings

For any skipped finding with severity `major` or `critical` (`fix_applied` is null), add a discovery to `epic.discoveries` in `.ratchet/plan.yaml` via `yq eval -i`. Fields: `ref` (discovery-tighten-TIMESTAMP), `title`, `description` (include evidence), `source`, `created_at`, `severity`, `retro_type: "skipped-finding"`, `status: "pending"`.

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

### Step 7: Persist Changes

If any improvements were applied (changes to pair definitions, workflow.yaml, guards, etc.), ask how the user wants to persist them.

First, show what changed:
```bash
git diff --stat
```

Check the progress adapter to determine the default:
```bash
adapter=$(yq eval '.progress.adapter' .ratchet/workflow.yaml 2>/dev/null)
```

Then use `AskUserQuestion`:
- Question: "How would you like to persist these tighten changes?"
- Options (default depends on adapter):
  - `"Commit and create PR (Recommended)"` — **recommended when `adapter == "github-issues"`**. Commit on a new branch `ratchet/tighten-<timestamp>`, push, and open a PR
  - `"Commit (Recommended)"` — **recommended when no github adapter**. Stage and commit all tighten changes with message `"Tighten: [brief summary of changes]"`
  - `"Commit and push"` — commit, then push to the current branch
  - `"Don't persist yet"` — leave changes unstaged for manual review

Stage only `.ratchet/` files (pairs, workflow.yaml, reports, retros, scores.yaml, plan.yaml). Do NOT stage source code.

- **Commit**: `git add .ratchet/... && git commit -m "Tighten: [summary]"`
- **Commit and push**: same + `git push`
- **Commit and create PR**: branch `ratchet/tighten-<timestamp>`, commit, push, `gh pr create`

If no files changed (all skipped), skip this step.

### Step 8: Next Steps

Then use `AskUserQuestion`:
- Options:
  - `"Run next debate (/ratchet:run) (Recommended)"`
  - `"View quality metrics (/ratchet:score)"`
  - `"Tighten another pair"` — if other pairs exist
  - `"Done for now"`
