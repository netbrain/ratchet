---
name: ratchet:tighten
description: Tighten the ratchet — analyze all improvement signals and sharpen the system
---

# /ratchet:tighten — Tighten Ratchet

Single entrypoint for improving Ratchet. Gathers signals (debate history, CI failures, PR feedback, discoveries, escalations, retros), triages by priority, applies fixes: sharpened prompts, new guards, workflow config. Replaces former advise/retro/tighten skills.

**Full read/write.** Edits pair files (generative.md, adversarial.md), workflow.yaml, summaries. Use Edit tool directly — do NOT defer. Get human approval via AskUserQuestion before writing.

## Usage
```
/ratchet:tighten                    # Auto-triage all signals, prioritize, improve
/ratchet:tighten [pair-name]        # Focus on a specific pair
/ratchet:tighten pr [number]        # Analyze a PR's CI results and review comments
```

## Prerequisites
- `.ratchet/` exists with valid config
- Some signal data exists (debates/reviews/retros/escalations/CI/discoveries)

If `.ratchet/` missing:
> "Ratchet is not initialized for this project. Run /ratchet:init to set up."

`AskUserQuestion`: `"Initialize now (/ratchet:init) (Recommended)"`, `"Cancel"`.

## Foundational Principle — Guilty Until Proven Innocent

**New changes are GUILTY until proven innocent.** On PR CI failures, default: PR caused it. Burden of proof: show failure exists on master. Never assume "unrelated" or "flake."

Classifying findings:
- **Never classify as `noise` (CI flake)** without evidence (passed on re-run OR same failure on master). "Looks like a flake" is not evidence.
- **Never skip a failure** as "probably unrelated." Prove via master check.
- **Verification command** before downgrading severity:
  ```bash
  # Check if the same test fails on master
  gh run list --branch main --workflow "<workflow>" --limit 3 --json conclusion -q '.[].conclusion'
  # Or check the specific test on the base commit
  git log --oneline -1 origin/main  # get base SHA
  # If all recent main runs passed, the PR is guilty
  ```

## Execution Steps

### Step 1: Gather All Signals

Read all signals:

- **Debates:** `.ratchet/debates/*/meta.json` (outcomes, fast-path, rounds), `.ratchet/reviews/<pair-name>/review-*.json` (post-debate reviews)
- **Retros:** `.ratchet/retros/*.json` (findings, severity, recurrence)
- **Escalations:** `.ratchet/escalations/*.json` (tiebreaker rulings, disputes)
- **Executions:** `.ratchet/executions/*.yaml` (solo/promoted: mode, guards, promotions, files, tokens)
- **Discoveries:** `.ratchet/plan.yaml` → `epic.discoveries` (pending sidequests)
- **Config:** `.ratchet/workflow.yaml` (pairs/guards/components), `.ratchet/project.yaml` (stack, validation cmds), `.ratchet/plan.yaml` (milestones, regressions)

**PR-specific (`pr [number]` mode):**
```bash
gh pr view <number> --json state,reviews,comments,statusCheckRollup
gh pr checks <number>
```

If no signals exist:
> "Not enough data to tighten. Run a few milestones first, then come back."

`AskUserQuestion`: `"Start a debate (/ratchet:run) (Recommended)"`, `"Done for now"`.

### Step 2: Mode-Specific Gathering

#### Auto-triage (no args or `[pair-name]`)

Use Step 1 signals. If `[pair-name]` given, filter to that pair.

#### PR Analysis (`tighten pr [number]`)

1. **Gather PR signals:**
   ```bash
   gh pr view <number> --json state,reviews,comments,statusCheckRollup
   gh pr checks <number>
   ```

2. **Identify failures/feedback:** CI failures (lint/tests/security/build), human review comments, merge conflicts, blocked status.

3. **Map failures to Ratchet gaps** — per item:
   - **In scope of debate pair?** Match file paths to pair scopes
   - **Adversarial has validation cmds?** Check failing CI cmd in adversarial prompt
   - **Guard exists?** Check guards for this check type
   - **Phase gap?** Should earlier phase have caught it?

### Step 3: Triage and Prioritize

Spawn **analyst** agent with generative model (`workflow.yaml` → `models.generative`, default `opus`):
- `model`: `models.generative` (or `opus`)
- `tools`: Read, Grep, Glob, Bash, Write, Edit, AskUserQuestion

Analyst edits pair files, workflow.yaml, summaries. MUST use AskUserQuestion before any file modification.

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
   - Read compression rules from `caveman/compress-rules.md` in the repo root
   - Review pair definitions (`.ratchet/pairs/*/generative.md` and `adversarial.md`) for verbosity
   - Apply the compress rules to the markdown body below the YAML frontmatter `---` delimiter
   - Use the pair's corresponding role intensity (generative pairs use `caveman.intensity.generative`, adversarial pairs use `caveman.intensity.adversarial`) — if the role intensity is `off`, skip that file
   - Follow the **Remove**, **Preserve EXACTLY**, **Preserve Structure**, and **Compress** sections from `caveman/compress-rules.md`
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

**Settled Law Injection**: 3+ escalation rulings agree on a dispute type → edit `.ratchet/pairs/<pair-name>/adversarial.md`:
- Find/create `### Settled Law (Patterns from Prior Debates)` (before last `## ` heading)
- Append: `- [ ] **<dispute type>**: <N> rulings found <direction>. Treat as settled — do not re-litigate. Source: escalations/<ids>.json`
- Verify: `grep -c "Settled Law" .ratchet/pairs/<pair-name>/adversarial.md` returns 1

**Error handling**: If analyst fails or returns nothing:
> "Analysis could not be completed. This may be due to insufficient data or an internal error."

`AskUserQuestion`: `"Try again"`, `"View raw data (/ratchet:score)"`, `"Done for now"`.

### Step 3b: Adversarial Verification

Findings are **claims until verified**. Run through adversarial fact-checking before user. Prevents false recommendations.

Spawn **verification agent** with adversarial model (`models.adversarial`, default `sonnet`):
- `model`: `models.adversarial` (or `sonnet`)
- `tools`: Read, Grep, Glob, Bash
- `disallowedTools`: Write, Edit

Read-only — checks, never fixes.

Task prompt:

```
Adversarial fact-checker for tighten findings. For each finding, READ the actual files and RUN commands to verify — do not accept claims at face value (analyst may hallucinate).

Check per finding: Is the gap real? Is the evidence accurate? Is the fix correct? Is the severity justified?

Verdict per finding: CONFIRMED | DOWNGRADED (explain) | REJECTED (counter-evidence) | NEEDS_INFO (what's missing)

Findings: [analyst's improvements list]
Pair definitions: [paths to .ratchet/pairs/*/generative.md and adversarial.md]
Guards: [guards array from workflow.yaml]
```

**Processing:** CONFIRMED → keep. DOWNGRADED → adjust severity + note. REJECTED → drop, log to `.ratchet/reports/verification-rejected.log` (`mkdir -p .ratchet/reports`). NEEDS_INFO → keep as unverified.

Summary: `"Verification: [N] checked — [N] confirmed, [N] downgraded, [N] rejected, [N] unverified"`. Only confirmed/downgraded/unverified proceed.

**Error handling**: Verifier fails → all NEEDS_INFO, proceed with caveat. ALL rejected → `"Re-run analysis"`, `"View rejected findings"`, `"Done for now"`. Never proceed with empty list.

### Step 4: Present and Apply

`AskUserQuestion`:

```
Tighten Assessment
==================

[If PR mode: "PR #[number]: [N] gaps found"]

[Analyst's prioritized improvements — grouped by type]

Overall health: [healthy / needs attention / at risk]
Signal sources: [N] debates, [N] retros, [N] escalations, [N] discoveries
```

Options:
- `"Apply improvements (Recommended)"` — walk each interactively
- `"Apply all automatically"` — no per-item confirmation
- `"Export as report"` — save to `.ratchet/reports/tighten-<timestamp>.md` (`mkdir -p .ratchet/reports`)
- `"Done for now"`

#### Applying improvements

Walk by priority. Per item, `AskUserQuestion`: "[priority] [detail]. Apply?" → `"Apply"`, `"Skip"`, `"Modify"`, `"Stop applying"`.

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

"Apply all automatically" — apply all, then summary.

### Step 5: Store Results

#### 5a. Retro findings (PR/CI mode)

PR-sourced improvements → write `.ratchet/retros/<timestamp>.json` with: `timestamp`, `source` ("pr"), `source_ref` (PR number), `findings[]` — each: `type` (missing_validation|missing_guard|missing_pair|phase_gap), `description`, `evidence`, `fix_applied` (or null), `severity` (critical|major|minor|noise), `related_findings[]`.

**Cross-retro recurrence**: Before storing, scan retros for same `type` + similar `description`. 2+ matches → auto-escalate severity one level, populate `related_findings`, present "Nth occurrence — escalating from [old] to [new]."

#### 5b. Sidequests for skipped findings

Skipped findings with severity `major`/`critical` (`fix_applied` null) → add discovery to `epic.discoveries` in `.ratchet/plan.yaml` via `yq eval -i`. Required: `ref` (discovery-tighten-TIMESTAMP), `title`, `description` (with evidence), `category` (missing_validation/missing_guard → "tech-debt", missing_pair → "feature", phase_gap → "tech-debt"), `severity` (critical|major|minor|info), `source` ("tighten"), `status: "pending"`, `retro_type: "skipped-finding"`, `created_at` (ISO 8601). Optional: `context`, `pairs`, `affected_scope`, `issue_ref`.

#### 5c. Analyst summary

Write to `.ratchet/reviews/<pair-name>/analyst-summary.md` (per pair) or `.ratchet/reports/tighten-<timestamp>.md` (full): date, signal sources, changes + rationale, patterns, next-tightening recommendations.

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

If improvements applied (pair files, workflow.yaml, guards), ask how to persist.

Show diff:
```bash
git diff --stat
```

Check adapter for default:
```bash
adapter=$(yq eval '.progress.adapter' .ratchet/workflow.yaml 2>/dev/null)
```

`AskUserQuestion` — "How to persist these tighten changes?":
- `"Commit and create PR (Recommended)"` — **default when `adapter == "github-issues"`**. Branch `ratchet/tighten-<timestamp>`, commit, push, open PR
- `"Commit (Recommended)"` — **default with no github adapter**. Stage + commit `"Tighten: [brief summary]"`
- `"Commit and push"` — commit + push current branch
- `"Don't persist yet"` — leave unstaged

Stage only `.ratchet/` files (pairs, workflow.yaml, reports, retros, scores.yaml, plan.yaml). NEVER stage source code.

- **Commit**: `git add .ratchet/... && git commit -m "Tighten: [summary]"`
- **Commit and push**: same + `git push`
- **Commit and PR**: branch `ratchet/tighten-<timestamp>`, commit, push, `gh pr create`

Nothing changed → skip.

### Step 8: Next Steps

`AskUserQuestion`:
- `"Run next debate (/ratchet:run) (Recommended)"`
- `"View quality metrics (/ratchet:score)"`
- `"Tighten another pair"` — if other pairs exist
- `"Done for now"`
