---
name: ratchet:migrate
description: Migrate v1 config.yaml to v2 workflow.yaml
---

# /ratchet:migrate — Migrate v1 → v2

Upgrade a Ratchet v1 project (using `config.yaml`) to v2 (using `workflow.yaml`).

## Usage
```
/ratchet:migrate              # Interactive migration with component detection
/ratchet:migrate --dry-run    # Show what would change without writing files
```

## Prerequisites
- `.ratchet/` must exist
- `.ratchet/config.yaml` must exist (v1 format)
- `.ratchet/workflow.yaml` must NOT exist (already migrated)

If `workflow.yaml` already exists, inform the user:
> "This project already uses workflow.yaml (v2). No migration needed."

Then use `AskUserQuestion` with options: `"View current workflow config"`, `"Done for now"`.

If `config.yaml` does not exist, inform the user:
> "No config.yaml found. Run /ratchet:init to set up a new v2 project."

Then use `AskUserQuestion` with options: `"Initialize now (/ratchet:init) (Recommended)"`, `"Cancel"`.

## Execution Steps

### Step 1: Read v1 Config

Read `.ratchet/config.yaml` and `.ratchet/project.yaml`. Extract:
- `max_rounds`
- `escalation`
- All pairs (name, scope, enabled)
- Stack info from project.yaml (languages, architecture, testing)

### Step 2: Detect Components

Analyze the existing pairs' scopes and project structure to propose logical components:

1. Group pairs by scope overlap — pairs covering similar directories likely belong to the same component
2. Scan the project directory structure for natural boundaries (e.g., `internal/`, `cmd/`, `web/`, `src/components/`, `src/api/`)
3. Read project.yaml architecture section for hints

For each proposed component, determine a workflow preset:
- **tdd**: if the project has test infrastructure and the component has test-related pairs
- **traditional**: if the component has build/implementation pairs but no explicit test-first approach
- **review-only**: if pairs only do quality review on existing code

### Step 3: Propose Migration

Present the migration plan using `AskUserQuestion`:

Question text (build from analysis):
```
Migration plan: config.yaml → workflow.yaml

Current v1 config:
  max_rounds: [N]
  escalation: [policy]
  pairs: [N] pairs

Proposed v2 workflow:
  Components:
    [name] — scope: [glob] — workflow: [preset]
    ...

  Pairs (with phase assignments):
    [name] — component: [comp] — phase: [phase] — scope: [glob]
    ...

  Guards (inferred from project.yaml testing spec):
    [name] — command: [cmd] — phase: [phase] — blocking: [yes/no]
    ...

  Progress: none (configure later with /ratchet:init)

[If --dry-run: "Dry run — no files will be written."]
```

Options: `"Approve migration (Recommended)"`, `"Modify components"`, `"Modify phase assignments"`, `"Cancel"`

If "Modify components" or "Modify phase assignments": use follow-up `AskUserQuestion` calls to refine.

### Step 4: Infer Guards

From `.ratchet/project.yaml` testing spec, propose guards:

- Layer 1 (static analysis) → guard at `build` phase, blocking
- Layer 7 (security gate) → guard at `harden` phase, blocking
- Layer 2 (unit tests) → guard at `build` phase, blocking
- Other layers → advisory guards at appropriate phases

Only propose guards for testing layers that have commands configured (status != "not_applicable").

### Step 5: Write workflow.yaml

On approval, write `.ratchet/workflow.yaml`:

```yaml
version: 2
max_rounds: 3
escalation: human

progress:
  adapter: none

components:
  - name: backend
    scope: "internal/**/*.go"
    workflow: tdd

pairs:
  - name: api-contracts
    component: backend
    phase: review
    scope: "internal/handler/**/*.go"
    enabled: true

guards:
  - name: lint
    command: "golangci-lint run ./..."
    phase: build
    blocking: true
    components: [backend]
```

### Step 6: Update plan.yaml

If `.ratchet/plan.yaml` exists, add `phase_status` tracking to each milestone:

```yaml
epic:
  milestones:
    - id: 1
      name: "milestone name"
      phase_status:
        plan: done
        test: done
        build: done
        review: done
        harden: pending
      # ... existing fields preserved
```

For milestones with `status: done`, set all phases to `done`.
For milestones with `status: in_progress`, set phases up to `review` as `done` (since v1 only had review).
For milestones with `status: pending`, set all phases to `pending`.

### Step 7: Preserve v1 Config

Do NOT delete `config.yaml`. Rename it to `config.yaml.v1` as a backup:

```bash
mv .ratchet/config.yaml .ratchet/config.yaml.v1
```

### Step 8: Validate

Read the newly written `workflow.yaml` and verify:
- `version` is 2
- All pairs from v1 are present
- Each pair has a valid `phase` (defaulting to `review`)
- Components reference valid scopes
- Guards reference valid phases and components

### Step 9: Report

```
Migration complete: config.yaml → workflow.yaml

  Components: [N] detected
  Pairs: [N] migrated (all assigned phase: review by default)
  Guards: [N] inferred from testing spec
  Progress: none (configure with /ratchet:init)

  Backup: .ratchet/config.yaml.v1

All existing debates, reviews, and scores are preserved.
```

After reporting, use `AskUserQuestion` to guide the user:
- Options:
  - "Run next debate with v2 workflow (/ratchet:run) (Recommended)"
  - "View workflow config"
  - "Done for now"
