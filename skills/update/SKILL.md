---
name: ratchet:update
description: Update Ratchet framework to the latest version with new features
---

# /ratchet:update — Framework Update

Update the Ratchet framework installation to get the latest features and improvements.

## What It Does

Updates your Ratchet installation by:
1. Detecting current installation location (global or local)
2. Running uninstall + reinstall to get the latest framework version
3. Preserving your project data (`.ratchet/` directory remains untouched)
4. Installing new framework features (statusline, monitoring, etc.)
5. Optionally configuring Claude Code settings for new features

## Prerequisites

- Existing Ratchet installation (`~/.claude/commands/ratchet/` or `.claude/commands/ratchet/`)
- Access to the Ratchet source repository (for latest version)

## Execution Steps

### Step 1: Detect Current Installation

Check which installation exists:
- **Global**: `~/.claude/commands/ratchet/` exists
- **Local**: `.claude/commands/ratchet/` exists

If both exist, ask the user which one to update using `AskUserQuestion`:
- Question: "Both global and local Ratchet installations detected. Which should I update?"
- Options: `"Global (~/.claude/)"`, `"Local (.claude/)"`, `"Both"`

If neither exists, inform the user that Ratchet is not installed and suggest running `bash install.sh` from the Ratchet repository.

### Step 2: Locate Install Script

The update requires access to the Ratchet source repository's `install.sh` script. Check common locations:
1. If CWD contains `install.sh` and `skills/` directory → use CWD
2. If `.ratchet/` exists in CWD, search parent directories for the Ratchet repo
3. If `RATCHET_SOURCE` environment variable is set → use that path
4. Otherwise, ask the user for the path using `AskUserQuestion`:
   - Question: "Where is the Ratchet source repository? (Need path to install.sh)"
   - Options: `"Current directory"`, `"Let me specify the path"`

If the user chooses "Let me specify the path", ask a follow-up question to get the path.

### Step 3: Run Update

Execute the update based on the installation type:

**For global installation:**
```bash
cd /path/to/ratchet/source
bash install.sh --global --uninstall
bash install.sh --global
```

**For local installation:**
```bash
cd /path/to/ratchet/source
bash install.sh --local --uninstall
bash install.sh --local
```

**For both:**
Run both sequences above.

The install script will:
- Remove old framework files (commands, scripts, schemas, statusline)
- Copy latest versions from the source repository
- Preserve git hooks unless `--no-git-hooks` was originally used
- Install new features (statusline, monitoring capabilities, etc.)

### Step 4: Verify New Features

After installation completes, check what new features are available:

1. **Statusline support**: Check if `statusline-ratchet.sh` was installed
   - Global: `~/.claude/statusline-ratchet.sh`
   - Local: `.claude/statusline-ratchet.sh`

2. **Loop monitoring**: Check if `/ratchet:watch` command is available by verifying `skills/watch/SKILL.md` exists in the installed commands

3. **New slash commands**: List any new commands that weren't in the previous version

### Step 5: Configure Optional Features

Use `AskUserQuestion` to offer configuration of new features:

**Question**: "Ratchet updated successfully! New features available. Would you like to configure them?"

**For statusline (if installed)**:
- Check if Claude Code settings already has a statusline configured
- If not, offer to configure it:
  - Options: `"Configure Ratchet statusline (Recommended)"`, `"Skip — I'll configure manually"`, `"Not now"`

If "Configure Ratchet statusline" selected, inform the user:
```
To enable the Ratchet statusline, add this to your Claude Code settings:

  statusline: ~/.claude/statusline-ratchet.sh  (for global)
  statusline: .claude/statusline-ratchet.sh    (for local)

The statusline will display epic progress when you're in a Ratchet workspace.
```

**For monitoring (if /ratchet:watch available)**:
- Inform the user about the new `/ratchet:watch` command for background monitoring
- Explain that it uses Claude Code's `/loop` feature to monitor PRs, CI status, and epic progress

### Step 6: Report Results

Present a summary of what was updated:

```
Ratchet framework updated successfully!

Updated installation: [Global/Local/Both]
Source: [path to ratchet source]

New features installed:
  ✓ Custom statusline support (statusline-ratchet.sh)
  ✓ Background monitoring (/ratchet:watch)
  ✓ [Any other new features detected]

Commands available: [count] slash commands
  /ratchet:init, /ratchet:run, /ratchet:watch, ...

Your project data in .ratchet/ was preserved.
```

## When to Use

Run `/ratchet:update` when:
- A new Ratchet version is released with features you want
- You want to get the latest improvements and bug fixes
- The framework suggests running an update (e.g., after git pull in the Ratchet repo)
- You want to ensure you have all the latest skills and monitoring capabilities

## What Gets Updated

**Updated:**
- Slash commands (skills) — new commands added, existing ones improved
- Runtime scripts (guards, progress adapters, cache management)
- Agent definitions (analyst, debate-runner, tiebreaker)
- JSON schemas (workflow validation)
- Statusline scripts
- Installation logic

**Preserved:**
- Your project data (`.ratchet/` directory)
- Workflow configuration (`.ratchet/workflow.yaml`)
- Project profile (`.ratchet/project.yaml`)
- Development roadmap (`.ratchet/plan.yaml`)
- Custom agent pair definitions (`.ratchet/pairs/`)
- Debate history, scores, retros, escalations
- Git hooks (re-installed with current configuration)

## Alternative: Manual Update

If you prefer manual control, you can update by running:

```bash
cd /path/to/ratchet/source
git pull origin main  # Get latest changes
bash install.sh --global --uninstall  # Remove old
bash install.sh --global              # Install new
```

The `/ratchet:update` skill automates this process and offers configuration assistance.

## See Also

- `install.sh` — The underlying installation script
- `/ratchet:init` — Initialize new Ratchet projects
- `/ratchet:watch` — Background monitoring (new feature)
