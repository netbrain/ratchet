---
name: ratchet:statusline
description: Install the Ratchet statusline into Claude Code settings
---

# /ratchet:statusline — Install Statusline

Configure Claude Code to use the Ratchet statusline, which shows epic progress, milestone/issue counts, discoveries, and blocked issues in your terminal.

## Usage
```
/ratchet:statusline          # Install the statusline
/ratchet:statusline --remove # Remove it and restore default
```

## Execution

### Install

1. Verify the statusline script exists:
   ```bash
   test -f .claude/statusline-ratchet.sh || echo "missing"
   ```
   If missing: "Statusline script not found. Run the Ratchet installer first."

2. Read the current project settings from `.claude/settings.json` (create if it doesn't exist).

3. Set the `statusline` field:
   ```json
   {
     "statusline": ".claude/statusline-ratchet.sh"
   }
   ```
   Preserve all other existing settings — only add/update the `statusline` key.

4. Confirm: "Ratchet statusline installed. Restart Claude Code to see it."

### Remove (`--remove`)

1. Read `.claude/settings.json`
2. Remove the `statusline` key
3. Confirm: "Statusline removed. Restart Claude Code to restore default."
