---
name: ratchet:retro
description: Retrospective — learn from PR feedback, CI failures, and production issues to improve agents and guards
---

# /ratchet:retro — Retrospective

Feed external signals back into Ratchet. When a PR fails CI, gets review comments, or a production issue occurs, retro analyzes what Ratchet missed and tightens the system to catch it next time.

This is how Ratchet learns from the real world — not just from its own debates.

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

**You have full read/write access.** Unlike the run orchestrator (which is read-only), the retro skill MUST edit files directly — updating adversarial prompts, adding guards to workflow.yaml, and modifying test files. Do NOT defer edits to the user. When the user selects "Apply all fixes" or "Apply fixes", make the changes yourself using the Edit tool.

## Usage
```
/ratchet:retro                     # Interactive — ask what happened
/ratchet:retro pr [number]         # Analyze a specific PR's CI results and review comments
/ratchet:retro monitor [number]    # Watch a PR's checks until they complete, then analyze
```

## Execution Steps

### Mode: Interactive (no arguments)

Use `AskUserQuestion`:
- Question: "What should I learn from?"
- Options:
  - `"A pull request (CI failures or review comments)"`
  - `"A production incident or bug report"`
  - `"Something the debates should have caught but didn't"`
  - `"Cancel"`

Based on the answer, gather context (PR number, incident description, etc.) via follow-up `AskUserQuestion` calls.

### Mode: PR Analysis (`retro pr [number]`)

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

4. **Propose improvements:**
   Present findings using `AskUserQuestion`:

   ```
   PR Analysis: [title]

   Findings:
     [CI check] failed — [summary]
       → Gap: adversarial agent for [pair] doesn't know about [command]
       → Fix: add validation command to adversarial prompt

     [Reviewer] requested changes — [summary]
       → Gap: no pair covers [quality dimension] for [files]
       → Fix: add new guard or pair

     [check] passed but Ratchet didn't run it
       → Gap: no guard for [check] at [phase] boundary
       → Fix: add blocking guard

   Proposed actions:
   ```

   Options:
   - `"Apply all fixes (Recommended)"` — update adversarial prompts, add guards, add pairs
   - `"Review fixes one by one"`
   - `"Just log — I'll fix manually"`
   - `"Cancel"`

5. **Apply fixes:**
   - **Missing validation command** → Edit `.ratchet/pairs/<name>/adversarial.md` to add the command
   - **Missing guard** → Add to `guards` array in `.ratchet/workflow.yaml`
   - **Missing pair** → Suggest running `/ratchet:pair` for the uncovered dimension
   - **Phase gap** → Suggest reassigning a pair to an earlier phase

6. **Classify severity and check recurrence:**

   For each finding, assign a severity level:
   - `critical`: build-breaking, security vulnerability, data loss
   - `major`: test failure, functional regression, missing validation
   - `minor`: lint/style, formatting, convention deviation
   - `noise`: CI flake — **requires evidence**: must have passed on re-run OR same failure confirmed on master. Never classify as noise based on assumption alone (guilty until proven innocent)

   **Cross-retro recurrence check**: Before storing, scan existing `.ratchet/retros/*.json` for findings with the same `type` and a similar `description`. If 2+ prior matches exist:
   - Auto-escalate severity one level (noise → minor → major → critical; critical stays critical)
   - Populate `related_findings` with references to the matching prior findings
   - Present: "This is the Nth time this gap was found. Escalating from [old severity] to [new severity]."

7. **Store retro results:**
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

   This history helps `/ratchet:tighten` understand systemic patterns — e.g., "the adversarial keeps missing lint issues" vs. "one-off CI flake." Severity and recurrence data give tighten a priority queue.

8. **Create sidequests for skipped findings:**

   For any finding with severity `major` or `critical` where `fix_applied` is null (user chose to skip), add a discovery to `epic.discoveries` in `.ratchet/plan.yaml` so it surfaces as actionable work in future `/ratchet:run` and `/ratchet:status` views:
   ```bash
   # For each skipped major/critical finding:
   if [ -f .ratchet/plan.yaml ]; then
     yq eval -i ".epic.discoveries += [{
       \"ref\": \"discovery-retro-$(date +%s)\",
       \"title\": \"Address retro finding: $description\",
       \"description\": \"Retro finding (skipped): $description. Evidence: $evidence\",
       \"source\": \"retro-$timestamp\",
       \"created_at\": \"$(date -Iseconds)\",
       \"severity\": \"$severity\",
       \"retro_type\": \"skipped-finding\",
       \"status\": \"pending\",
       \"issue_ref\": null,
       \"affected_scope\": null
     }]" .ratchet/plan.yaml
   fi
   ```

### Mode: Monitor (`retro monitor [number]`)

Watch a PR's checks and analyze when they complete.

1. **Start monitoring:**
   ```bash
   # Timeout after 30 minutes to prevent indefinite blocking
   timeout 1800 gh pr checks <number> --watch
   ```
   This blocks until all checks complete (pass or fail) or the timeout is reached.

   **If timeout is reached** (exit code 124):
   > "PR checks did not complete within 30 minutes. The PR may have long-running checks or be stuck."

   Use `AskUserQuestion`:
   - Options: `"Extend timeout (30 more minutes)"`, `"Analyze current check status"`, `"Cancel"`
   - If "Analyze current check status": run `gh pr checks <number>` (non-blocking) and analyze whatever results are available

2. **On completion:**
   - If all checks passed: report success, no retro needed
     ```
     PR [number]: All checks passed ✓
     No gaps detected — Ratchet's debates and guards covered everything.
     ```
   - If any checks failed: automatically run the PR analysis (same as `retro pr [number]`)

3. **After analysis**, use `AskUserQuestion`:
   - Options:
     - `"Apply fixes and re-run failed debates (Recommended)"` — fix gaps, then `/ratchet:run` the affected pairs
     - `"Apply fixes only"` — update agents/guards but don't re-run
     - `"Just log"`
     - `"Done for now"`

### After Any Retro — Report

```
Retro complete for [source]:

  Gaps found: [N]
  Fixes applied:
    ✓ Added [command] to [pair] adversarial agent
    ✓ Added guard [name] at [phase] phase (blocking)
    ⊘ Skipped: [description] (user chose to skip)

  Retro saved to .ratchet/retros/[timestamp].json
```

Then use `AskUserQuestion`:
- Options:
  - `"Re-run affected debates (/ratchet:run)"`
  - `"View quality metrics (/ratchet:score)"`
  - `"Tighten agents (/ratchet:tighten)"`
  - `"Done for now"`
