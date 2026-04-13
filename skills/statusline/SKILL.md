---
name: ratchet:statusline
description: Install the Ratchet statusline into Claude Code settings
---

# /ratchet:statusline — Install Statusline

Configure Claude Code to use Ratchet statusline — shows epic progress, milestone/issue counts, discoveries, and blocked issues in your terminal.

## Usage
```
/ratchet:statusline          # Install the statusline (local settings)
/ratchet:statusline --global # Install to global settings (~/.claude/)
/ratchet:statusline --remove # Remove it and restore default
```

## Execution

### Install

1. **Find statusline script.** Check local, global, then fetch from GitHub:
   ```bash
   if [ -f .claude/statusline-ratchet.sh ]; then
     STATUSLINE_PATH=".claude/statusline-ratchet.sh"
   elif [ -f "$HOME/.claude/statusline-ratchet.sh" ]; then
     STATUSLINE_PATH="$HOME/.claude/statusline-ratchet.sh"
   else
     echo "NOT FOUND LOCALLY"
   fi
   ```
   **If not found locally**, fetch from GitHub and install to global path:
   ```bash
   mkdir -p "$HOME/.claude"
   curl -fsSL https://raw.githubusercontent.com/netbrain/ratchet/refs/heads/main/statusline/statusline-ratchet.sh \
     -o "$HOME/.claude/statusline-ratchet.sh"
   chmod +x "$HOME/.claude/statusline-ratchet.sh"
   STATUSLINE_PATH="$HOME/.claude/statusline-ratchet.sh"
   ```
   **If curl fails** — STOP. Do NOT create the script yourself. Tell user: "Could not fetch statusline script. Install Ratchet first: `nix run github:netbrain/ratchet -- --global` or `./install.sh --local`" Then stop.

2. **Update settings.** Default to **local** (`.claude/settings.json`). Use `~/.claude/settings.json` only if `--global` specified. If file doesn't exist, create with just statusline key. If exists, add/update only `statusline` key — preserve everything else.
   ```json
   {
     "statusline": "<STATUSLINE_PATH from step 1>"
   }
   ```
   Use Edit tool to update existing file, or Write to create new.

3. **Confirm:** "Ratchet statusline installed. Restart Claude Code to see it."

### Remove (`--remove`)

1. Read `.claude/settings.json` (check local first, then global)
2. Remove `statusline` key — preserve everything else
3. Confirm: "Statusline removed. Restart Claude Code to restore default."
