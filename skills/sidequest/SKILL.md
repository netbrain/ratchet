---
name: ratchet:sidequest
description: Manually log discoveries and sidequests during active work
---

# /ratchet:sidequest — Log a Discovery

Manually log discoveries during active work — mid-debate, mid-phase, mid-milestone. This fills the gap between auto-detected discoveries (watch creates them for CI failures/merge conflicts, retro creates them for skipped findings) and manual tracking.

Lightweight and fast. No debates, no guards, just bookkeeping.

## Usage
```
/ratchet:sidequest            # Interactive — log a new discovery
```

## Prerequisites
- `.ratchet/` must exist
- `.ratchet/plan.yaml` is used if present but NOT required — the skill creates a minimal structure if the file or the `epic.discoveries` array is missing

## Execution Steps

### Step 1: Locate plan.yaml

Check for `.ratchet/plan.yaml`:

1. **File exists and is valid YAML**: Read it, ensure `epic.discoveries` array exists. If the array is missing, it will be created when appending.
2. **File exists but is not valid YAML**: Inform the user:
   > "plan.yaml exists but could not be parsed. Please fix the file or delete it to start fresh."
   Then use `AskUserQuestion` with options: `"Cancel"`, `"Delete plan.yaml and start fresh"`.
3. **File does not exist**: Create a minimal `plan.yaml`:
   ```yaml
   epic:
     discoveries: []
   ```
   Inform the user: "No plan.yaml found. Created a minimal one for discovery tracking."

### Step 2: Gather Discovery Details

Use `AskUserQuestion` for each field. All prompts must be plain text (no markdown) since AskUserQuestion renders as a terminal selector.

#### 2a: Description

Use `AskUserQuestion`:
- Question: "What did you discover? (describe the finding)"
- Freeform text input

#### 2b: Category

Use `AskUserQuestion`:
- Question: "What category does this fall under?"
- Options:
  - `"bug — Something is broken or wrong"`
  - `"tech-debt — Shortcuts, duplication, or design smell"`
  - `"feature — New capability or enhancement idea"`
  - `"security — Vulnerability or hardening opportunity"`
  - `"performance — Bottleneck or optimization opportunity"`
  - `"other — Does not fit the above categories"`

Parse the selected option to extract the category key (the word before the dash).

#### 2c: Severity

Use `AskUserQuestion`:
- Question: "How severe is this?"
- Options:
  - `"critical — Blocks progress or poses immediate risk"`
  - `"major — Significant impact, should be addressed soon"`
  - `"minor — Low impact, address when convenient"`
  - `"info — Just noting for future reference"`

Parse the selected option to extract the severity key.

#### 2d: Relevant Pairs (optional)

Read `pairs` from `.ratchet/workflow.yaml` (if it exists). If pairs are configured:

Use `AskUserQuestion`:
- Question: "Which pair(s) would be relevant to this discovery? (select one, or skip)"
- Options: one option per pair name from workflow.yaml, plus `"Skip — no specific pair"`, plus `"Multiple — I will list them"`

If "Multiple" is selected, use a follow-up `AskUserQuestion` (freeform):
- Question: "List the relevant pair names, comma-separated"

If workflow.yaml does not exist or has no pairs, skip this step and set pairs to an empty list.

#### 2e: Context (auto-detect with override)

Attempt to auto-detect context from `plan.yaml`:

1. Read `epic.current_focus` if it exists — extract milestone and issue refs
2. If auto-detected, use `AskUserQuestion`:
   - Question: "Detected context: milestone [milestone-id], issue [issue-ref]. Use this context?"
   - Options:
     - `"Yes, use detected context"`
     - `"No, let me specify"`
     - `"No context — this is a general finding"`
3. If no auto-detection possible, or user chose to specify manually, use `AskUserQuestion`:
   - Question: "Which milestone is this related to? (enter milestone ID or leave blank)"
   - Freeform text input
   - Then: "Which issue is this related to? (enter issue ref or leave blank)"
   - Freeform text input
4. For debate context: read `debates/*/meta.json` to find any active debate. If one exists, include its ID. Otherwise set to null.

### Step 3: Generate and Append Discovery

Generate the discovery entry:

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
  \"created_at\": \"$created\"
}]" .ratchet/plan.yaml
```

If `epic.discoveries` does not exist yet, yq will create it as part of the append operation. Ensure the array is initialized first if needed:

```bash
yq eval -i '.epic.discoveries //= []' .ratchet/plan.yaml
```

### Step 4: Confirm

Display a summary to the user:

```
Discovery logged: discovery-manual-<timestamp>
  Category: <category> | Severity: <severity>
  Context: milestone <id>, issue <ref> (or "no specific context")
  Pairs: <pair-list> (or "none")
  Status: pending

This discovery will appear in /ratchet:status and be available as a sidequest in /ratchet:run.
```

Then use `AskUserQuestion`:
- Options:
  - `"Log another discovery"`
  - `"View status (/ratchet:status)"`
  - `"Done"`

If "Log another discovery" is selected, return to Step 2.

## Error Handling

- **yq not available**: Fall back to reading/writing plan.yaml with standard file tools (Read/Write). Parse YAML manually and append the discovery entry as text.
- **plan.yaml locked or read-only**: Inform user and suggest checking file permissions.
- **Invalid YAML after write**: Read back the file after writing and validate with `yq eval '.' .ratchet/plan.yaml`. If invalid, inform the user and offer to restore from the pre-edit state.

## Discovery Lifecycle

Discoveries logged by this skill enter the standard Ratchet discovery pipeline:

1. **Created here** with `status: "pending"` and `source: "manual"`
2. **Surfaced** by `/ratchet:status` in the Sidequests section
3. **Consumed** by `/ratchet:run` as Mode B sidequest work items
4. **Promoted** to full issues when addressed (sets `status: "promoted"` and `issue_ref`)
5. **Dismissed** if determined to be non-actionable (sets `status: "dismissed"`)

## See Also

- `/ratchet:status` — View pending discoveries
- `/ratchet:run` — Process discoveries as sidequest work items
- `/ratchet:watch` — Auto-detects discoveries from CI failures and merge conflicts
- `/ratchet:retro` — Creates discoveries from retrospective analysis
