# Mode Q: Quick-fix (`--quick "<description>"`)

If `--quick` is set, skip `plan.yaml` entirely. Fast path for small, well-understood fixes that don't need epic/milestone/issue management or adversarial review. **Checked first** — before all other modes. If `--quick` is present, no other mode flags are evaluated.

**Flag interactions:**
- `--quick --dry-run`: Print detected component, scope, guards that would run, then stop. No agent spawned, no changes made.
- `--quick --auto-pr`: Create branch and PR after commit (see step 5).
- `--quick --unsupervised`: If component auto-detection fails, halt with error. If guard fails, halt with `failed` (no retry).
- `--quick --no-cache`: No effect — Mode Q has no file-hash cache.

**1. Parse description:** The freeform `<description>` argument is the task. Should describe what to do and which files are involved:
```
/ratchet:run --quick "Fix off-by-one in src/parser.ts validateToken loop"
/ratchet:run --quick "Add missing error handling to scripts/deploy.sh"
```

**2. Auto-detect component from file paths:** Extract file paths from the description (tokens matching known file extensions or path separators):
```bash
# Extract candidate paths from the description
PATHS=$(echo "$DESCRIPTION" | grep -oE '[a-zA-Z0-9_./-]+\.[a-zA-Z]{1,10}' | sort -u)

# Match each path against component scope globs in workflow.yaml
for path in $PATHS; do
  for component in $(yq eval '.components[].name' .ratchet/workflow.yaml); do
    scope=$(yq eval ".components[] | select(.name == \"$component\") | .scope" .ratchet/workflow.yaml)
    # Check if path matches the component's scope glob
    # Use bash globbing or a dedicated match utility
  done
done
```

If no file paths detected or no component matches, use `AskUserQuestion`: "Could not auto-detect component from the description. Which component?" Options: one per component in `workflow.yaml`, plus `"Cancel"`. In unsupervised mode with no component match: halt with error — quick-fix requires detectable scope.

**3. Spawn one generative agent:** Spawn a single generative agent (using resolved `generative` model) with the description as prompt. Agent receives build-phase constraints (read, write, edit files within detected component's scope):

```
Quick-fix mode — single generative pass.

Task: <description>
Component: <detected-component>
Scope: <component scope glob>

Constraints:
  - You are in build-phase mode: read, write, edit files within scope.
  - No adversarial review — blocking guards are the quality gate.
  - Keep changes minimal and focused on the described task.
  - Do NOT modify files outside the component scope.

PRINCIPLE — Guilty Until Proven Innocent:
  If any test or guard fails after your changes, YOUR changes caused it
  unless you can prove otherwise on a clean checkout.
```

**Tool boundaries** (same as debate-mode generative agent — only difference: no adversarial review, no debate structure): Read, Grep, Glob, Bash, Write, Edit

**4. Run blocking guards:** After generative agent returns, run all blocking guards for detected component's phase (use `review` phase guards as default):

```bash
test -f .claude/ratchet-scripts/run-guards.sh \
  || { echo "Error: run-guards.sh not found. Run install.sh to restore Ratchet scripts." >&2; exit 1; }

bash .claude/ratchet-scripts/run-guards.sh quick-fix review <guard-name> "<guard-command>" true
```

- If any blocking guard fails: **Guilty until proven innocent** — the quick-fix caused it. Verify on clean master before dismissing.
  - Supervised: `AskUserQuestion`: "Guard '[name]' failed: [summary]." Options: `"Fix and re-run"`, `"Abort quick-fix"`, `"Override guard"`
  - Unsupervised: halt with status `failed` — quick-fix does not retry automatically.
- Advisory guards: log and continue.

**5. Commit and optionally create PR:** If all blocking guards pass, commit with a message derived from the description. If `--auto-pr` is also set, create a branch first:
```bash
# Create branch before committing (only with --auto-pr)
BRANCH="ratchet/quick-fix/$(echo "$DESCRIPTION" | tr ' ' '-' | tr '[:upper:]' '[:lower:]' | cut -c1-50)"
git checkout -b "$BRANCH"
```

Then commit (on new branch if `--auto-pr`, else current branch):
```bash
git add -A
git commit -m "<description>"
```

If `--auto-pr` is set, push and create PR:
```bash
git push -u origin "$BRANCH"
gh pr create --title "$DESCRIPTION" --body "Quick-fix via \`/ratchet:run --quick\`"
```

**6. Write execution log:**

```bash
EXEC_ID="quick-fix-$(date +%Y%m%dT%H%M%S)"
mkdir -p .ratchet/executions
cat > ".ratchet/executions/${EXEC_ID}.yaml" <<EOF
id: "${EXEC_ID}"
mode: quick-fix
component: <detected-component>
issue: null
started: "<timestamp>"
resolved: "<timestamp>"
guard_results: [<guard results>]
description: "<description>"
files_modified: [<files>]
EOF
```

**7. Skip epic/milestone/issue management:** Mode Q does not read or write `plan.yaml`. No milestone, issue, or phase tracking. No debate artifacts. Commit is local unless `--auto-pr` creates a branch and PR (see step 5).

> **Note on Step 1**: Mode Q still requires Step 1a (workspace resolution) to locate `workflow.yaml` for component auto-detection. Step 1b's `plan.yaml` reading is skipped entirely — Mode Q has no concept of milestones, issues, or phases.

**8. Output summary:**

```
Quick-fix complete:
  mode: quick-fix
  description: <description>
  component: <detected-component>
  files_modified: [<files>]
  guards: [<pass/fail summary>]
  execution_log: .ratchet/executions/<EXEC_ID>.yaml
```

Then stop — do not continue to any other step. Mode Q is a terminal path.
