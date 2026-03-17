---
name: ratchet:pair
description: Add a new agent pair to an existing Ratchet configuration
---

# /ratchet:pair — Add a New Agent Pair

Add a new generative-adversarial agent pair to an existing Ratchet configuration.

## Usage
```
/ratchet:pair [name]
```

If `[name]` is provided, use it as the pair name. Otherwise, the analyst will suggest a name based on the discussion.

## Prerequisites
- `.ratchet/` must exist (run `/ratchet:init` first)
- `.ratchet/project.yaml` must exist
- `.ratchet/workflow.yaml` must exist

If prerequisites are not met, inform the user and suggest `/ratchet:init`.

## Execution Steps

### Step 1: Load Project Context

Read `.ratchet/project.yaml` and `.ratchet/workflow.yaml` to understand:
- Current tech stack and architecture
- Existing pairs (to avoid overlap)
- Testing capabilities

### Step 2: Launch Analyst Agent

Spawn the **analyst** agent using the generative model from `workflow.yaml` (`models.generative`, default `opus`). Agent configuration:
- `subagent_type`: analyst
- `model`: value of `workflow.yaml` → `models.generative` (or `opus` if unset)
- `tools`: Read, Grep, Glob, Bash, Write, Edit, AskUserQuestion

Task prompt:

```
A new agent pair is being added to this Ratchet-configured project.

Project profile: [contents of .ratchet/project.yaml]
Existing pairs: [list from workflow.yaml]
Requested pair name: [name if provided, otherwise "to be determined"]

Your task:
1. If no name was provided, use `AskUserQuestion` to ask the human what quality dimension they want to cover
   - Options: suggest 3-4 dimensions based on project profile, plus "Other" for custom input
2. Use `AskUserQuestion` to discuss the scope and focus with the human
   - Options: suggest file glob patterns based on project structure
3. Review the codebase areas relevant to this concern
4. Generate the pair:
   - .ratchet/pairs/<name>/generative.md — builder agent with project-specific knowledge
   - .ratchet/pairs/<name>/adversarial.md — critic agent with testing commands baked in
5. Present the pair definition to the human for approval using `AskUserQuestion`
   - Options: "Approve (Recommended)", "Modify scope", "Modify agents", "Start over"
6. On approval, write the agent files and update `.ratchet/workflow.yaml` to register the new pair with component and phase fields.

Follow the same agent generation conventions as init:
- Generative: tools: Read, Grep, Glob, Bash, Write, Edit
- Adversarial: tools: Read, Grep, Glob, Bash, disallowedTools: Write, Edit
- Include project-specific knowledge in prompts
- Define tight file scope globs
- Encode the guilty-until-proven-innocent principle: test failures on a PR branch are caused by the PR unless definitively proven otherwise. Generative agents must fix failures, not dismiss them. Adversarial agents must reject dismissals lacking evidence.
```

### Step 3: Verify & Report

Verify the new pair was created and registered:
```bash
# Verify agent files exist
test -f .ratchet/pairs/<name>/generative.md || { echo "Error: generative.md not created for pair '<name>'" >&2; exit 1; }
test -f .ratchet/pairs/<name>/adversarial.md || { echo "Error: adversarial.md not created for pair '<name>'" >&2; exit 1; }

# Verify registered in workflow.yaml
yq eval '.pairs[] | select(.name == "<name>")' .ratchet/workflow.yaml | grep -q 'name:' \
  || { echo "Error: pair '<name>' not registered in workflow.yaml" >&2; exit 1; }
```

If the analyst agent fails (returns an error or produces empty output), inform the user:
> "Pair generation failed. This may be due to insufficient project context or an invalid pair name."

Then use `AskUserQuestion` with options: `"Try again"`, `"Try with a different name"`, `"Cancel"`.

Report:
```
New pair added: [name]
  Scope: [file glob]
  Quality dimension: [what it checks]
  Generative: .ratchet/pairs/[name]/generative.md
  Adversarial: .ratchet/pairs/[name]/adversarial.md

Run /ratchet:run [name] to test the new pair.
```

After reporting, use `AskUserQuestion` to guide the user:
- Options:
  - "Run debate for [name] (/ratchet:run [name]) (Recommended)" — test the new pair immediately
  - "Add another pair (/ratchet:pair)"
  - "Done for now"
