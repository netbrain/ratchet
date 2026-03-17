---
name: ratchet:statusline
description: Install the Ratchet statusline into Claude Code settings
---

# /ratchet:statusline — Install Statusline

Configure Claude Code to use the Ratchet statusline, which shows epic progress, milestone/issue counts, discoveries, and blocked issues in your terminal.

## Usage
```
/ratchet:statusline          # Install the statusline (local settings)
/ratchet:statusline --global # Install to global settings (~/.claude/)
/ratchet:statusline --remove # Remove it and restore default
```

## Execution

### Install

1. **Find the statusline script.** Check local, global, then fetch from GitHub:
   ```bash
   if [ -f .claude/statusline-ratchet.sh ]; then
     STATUSLINE_PATH=".claude/statusline-ratchet.sh"
   elif [ -f "$HOME/.claude/statusline-ratchet.sh" ]; then
     STATUSLINE_PATH="$HOME/.claude/statusline-ratchet.sh"
   else
     echo "NOT FOUND LOCALLY"
   fi
   ```

   **If not found locally**, fetch it from GitHub and install to the global path:
   ```bash
   mkdir -p "$HOME/.claude"
   curl -fsSL https://raw.githubusercontent.com/netbrain/ratchet/main/statusline-ratchet.sh \
     -o "$HOME/.claude/statusline-ratchet.sh"
   chmod +x "$HOME/.claude/statusline-ratchet.sh"
   STATUSLINE_PATH="$HOME/.claude/statusline-ratchet.sh"
   ```

   **If curl fails** — STOP. Do NOT create the script yourself. Tell the user:
   > "Could not fetch statusline script. Install Ratchet first: `nix run github:netbrain/ratchet -- --global` or `./install.sh --local`"

   Then stop. Do nothing else.

2. **Update settings.** Default to **local** (`.claude/settings.json`). Use `~/.claude/settings.json` only if `--global` is specified.

   If the settings file doesn't exist, create it with just the statusline key. If it exists, add/update only the `statusline` key — preserve everything else.

   ```json
   {
     "statusline": "<STATUSLINE_PATH from step 1>"
   }
   ```

   Use the Edit tool to update an existing file, or Write to create a new one.

3. **Confirm:** "Ratchet statusline installed. Restart Claude Code to see it."

### Remove (`--remove`)

1. Read `.claude/settings.json` (check local first, then global)
2. Remove the `statusline` key — preserve everything else
3. Confirm: "Statusline removed. Restart Claude Code to restore default."
