---
name: ratchet:sidequest
description: Manually log discoveries and sidequests during active work
---

# /ratchet:sidequest — Log a Discovery

Manually log discoveries during active work — mid-debate, mid-phase, mid-milestone. Fills the gap between auto-detected discoveries (watch creates them for CI failures/merge conflicts, tighten creates them for skipped findings) and manual tracking. Lightweight and fast — no debates, no guards, just bookkeeping.

## Usage
```
/ratchet:sidequest            # Interactive — log a new discovery
```

## Prerequisites
- `.ratchet/` must exist
- `.ratchet/plan.yaml` is used if present but NOT required — skill creates a minimal structure if file or `epic.discoveries` array is missing

## Execution Steps

### Step 1: Locate plan.yaml

Check for `.ratchet/plan.yaml`:

1. **File exists and is valid YAML**: Read it, ensure `epic.discoveries` array exists. If missing, it will be created when appending.
2. **File exists but not valid YAML**: Inform user: "plan.yaml exists but could not be parsed. Fix the file or delete it to start fresh." Then `AskUserQuestion` with options: `"Cancel"`, `"Delete plan.yaml and start fresh"`.
3. **File does not exist**: Create minimal `plan.yaml`:
   ```yaml
   epic:
     discoveries: []
   ```
   Inform user: "No plan.yaml found. Created minimal one for discovery tracking."

### Step 2: Gather Discovery Details

Use `AskUserQuestion` for each field. Prompts must be plain text (no markdown) since AskUserQuestion renders as terminal selector.

#### 2a: Description
- Question: "What did you discover? (describe the finding)" — freeform text input

#### 2b: Category
- Question: "What category does this fall under?"
- Options: `"bug — Something is broken or wrong"`, `"tech-debt — Shortcuts, duplication, or design smell"`, `"feature — New capability or enhancement idea"`, `"security — Vulnerability or hardening opportunity"`, `"performance — Bottleneck or optimization opportunity"`, `"other — Does not fit above categories"`

Parse selected option to extract category key (word before dash).

#### 2c: Severity
- Question: "How severe is this?"
- Options: `"critical — Blocks progress or poses immediate risk"`, `"major — Significant impact, address soon"`, `"minor — Low impact, address when convenient"`, `"info — Just noting for future reference"`

Parse selected option to extract severity key.

#### 2d: Relevant Pairs (optional)

Read `pairs` from `.ratchet/workflow.yaml` (if exists). If configured:
- Question: "Which pair(s) would be relevant to this discovery? (select one, or skip)"
- Options: one per pair name from workflow.yaml, plus `"Skip — no specific pair"`, plus `"Multiple — I will list them"`

If "Multiple" selected, follow-up freeform: "List relevant pair names, comma-separated". If workflow.yaml does not exist or has no pairs, skip and set pairs to empty list.

#### 2e: Context (auto-detect with override)

Attempt auto-detect from `plan.yaml`:
1. Read `epic.current_focus` if exists — extract milestone and issue refs
2. If auto-detected, `AskUserQuestion`:
   - Question: "Detected context: milestone [milestone-id], issue [issue-ref]. Use this context?"
   - Options: `"Yes, use detected context"`, `"No, let me specify"`, `"No context — general finding"`
3. If no auto-detection or user chose manual, ask: "Which milestone is this related to? (enter milestone ID or leave blank)" then "Which issue is this related to? (enter issue ref or leave blank)" — both freeform
4. For debate context: read `debates/*/meta.json` to find any active debate. If exists, include ID; otherwise null.

### Step 3: Generate and Append Discovery

Generate discovery entry:

```yaml
- ref: "discovery-manual-<unix-timestamp>"
  title: "<short summary of the discovery>"
  description: "<user's description>"
  category: "<bug|tech-debt|feature|security|performance|other>"
  severity: "<critical|major|minor|info>"
  source: "manual"
  context:
    milestone: <detected-or-specified-milestone-id or null>
    issue: "<detected-or-specified-issue-ref or null>"
    debate: "<active-debate-id or null>"
  pairs: []  # populated from step 2d
  status: "pending"
  issue_ref: null
  affected_scope: null
  retro_type: null
  created_at: "<ISO 8601 timestamp>"
```

Append to `epic.discoveries` in `.ratchet/plan.yaml` using yq:

```bash
timestamp=$(date +%s)
created=$(date -Iseconds)

yq eval -i ".epic.discoveries += [{
  \"ref\": \"discovery-manual-$timestamp\",
  \"title\": \"<short summary>\",
  \"description\": \"<description>\",
  \"category\": \"<category>\",
  \"severity\": \"<severity>\",
  \"source\": \"manual\",
  \"context\": {
    \"milestone\": <milestone or null>,
    \"issue\": <issue or null>,
    \"debate\": <debate or null>
  },
  \"pairs\": [<pair-list>],
  \"status\": \"pending\",
  \"issue_ref\": null,
  \"affected_scope\": null,
  \"retro_type\": null,
  \"created_at\": \"$created\"
}]" .ratchet/plan.yaml
```

If `epic.discoveries` does not exist yet, yq will create it. Ensure array initialized first if needed:
```bash
yq eval -i '.epic.discoveries //= []' .ratchet/plan.yaml
```

### Step 4: Confirm

Display summary:
```
Discovery logged: discovery-manual-<timestamp>
  Category: <category> | Severity: <severity>
  Context: milestone <id>, issue <ref> (or "no specific context")
  Pairs: <pair-list> (or "none")
  Status: pending

This discovery will appear in /ratchet:status and be available as a sidequest in /ratchet:run.
```

Then `AskUserQuestion`:
- Options: `"Log another discovery"`, `"View status (/ratchet:status)"`, `"Done"`

If "Log another discovery" selected, return to Step 2.

## Error Handling

- **yq not available**: Fall back to reading/writing plan.yaml with standard file tools (Read/Write). Parse YAML manually and append discovery entry as text.
- **plan.yaml locked or read-only**: Inform user and suggest checking file permissions.
- **Invalid YAML after write**: Read back file after writing and validate with `yq eval '.' .ratchet/plan.yaml`. If invalid, inform user and offer to restore from pre-edit state.

## Discovery Lifecycle

Discoveries enter standard Ratchet discovery pipeline:
1. **Created here** with `status: "pending"` and `source: "manual"`
2. **Surfaced** by `/ratchet:status` in Sidequests section (only `pending` shown)
3. **Consumed** by `/ratchet:run` as Mode B sidequest work items
4. **Processed** by `/ratchet:run` pipeline (sets `status: "done"` after resolved)
5. **Promoted** to full issues when addressed (sets `status: "promoted"` and `issue_ref`)
6. **Dismissed** if non-actionable (sets `status: "dismissed"`)

Valid status values: `pending` | `done` | `promoted` | `dismissed`

## See Also

- `/ratchet:status` — View pending discoveries
- `/ratchet:run` — Process discoveries as sidequest work items
- `/ratchet:watch` — Auto-detects discoveries from CI failures and merge conflicts
- `/ratchet:tighten` — Creates discoveries from retrospective analysis and PR feedback
