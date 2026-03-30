---
name: ratchet:update
description: Update Ratchet framework to the latest version
---

# /ratchet:update — Framework Update

Update the Ratchet framework installation to the latest version.

## Step 1: Detect Current Installation

Check which installation exists:
- **Global**: `~/.claude/commands/ratchet/` exists
- **Local**: `.claude/commands/ratchet/` exists

If both exist, use `AskUserQuestion`:
- Question: "Both global and local Ratchet installations detected. Which should I update?"
- Options: `"Global (~/.claude/)"`, `"Local (.claude/)"`, `"Both"`

If neither exists: "Ratchet is not installed. Run `/ratchet:init` or `bash install.sh` from the Ratchet repo."

Record the scope: `global`, `local`, or `both`.

## Step 2: Choose Update Method

Use `AskUserQuestion`:
- Question: "How should I fetch the latest Ratchet?"
- Options:
  - `"Nix flake (Recommended)"` — uses `nix run github:netbrain/ratchet`
  - `"Git clone + install.sh"` — clones repo to a temp directory
  - `"Local source"` — use an existing checkout (auto-detected or user-provided path)

### Method detection hints (use to pre-select or reorder options):

1. If `nix` is on PATH → recommend Nix flake (fastest, no clone needed)
2. If CWD contains `install.sh` and `skills/` → offer "Local source" first
3. If `RATCHET_SOURCE` env var is set → offer "Local source" with that path

## Step 3: Run Update

Execute uninstall + reinstall for each scope.

### Method A: Nix flake

```bash
# For local:
nix run github:netbrain/ratchet --refresh -- --local --uninstall 2>&1
nix run github:netbrain/ratchet --refresh -- --local 2>&1

# For global:
nix run github:netbrain/ratchet --refresh -- --global --uninstall 2>&1
nix run github:netbrain/ratchet --refresh -- --global 2>&1
```

`--refresh` forces Nix to fetch the latest commit from GitHub (otherwise it uses the cached flake).

### Method B: Git clone + install.sh

```bash
TMPDIR=$(mktemp -d)
git clone --depth 1 https://github.com/netbrain/ratchet "$TMPDIR/ratchet" 2>&1

# For local:
bash "$TMPDIR/ratchet/install.sh" --local --uninstall 2>&1
bash "$TMPDIR/ratchet/install.sh" --local 2>&1

# For global:
bash "$TMPDIR/ratchet/install.sh" --global --uninstall 2>&1
bash "$TMPDIR/ratchet/install.sh" --global 2>&1

rm -rf "$TMPDIR"
```

### Method C: Local source

Locate install.sh:
1. CWD contains `install.sh` + `skills/` → use CWD
2. `RATCHET_SOURCE` env var → use that path
3. Otherwise, use `AskUserQuestion` (freeform): "Path to Ratchet source directory?"

```bash
cd /path/to/ratchet/source

# For local:
bash install.sh --local --uninstall 2>&1
bash install.sh --local 2>&1

# For global:
bash install.sh --global --uninstall 2>&1
bash install.sh --global 2>&1
```

### For scope "both"

Run the appropriate method twice — once with `--global`, once with `--local`.

## Step 4: Verify and Report

After install completes, verify the installation:

```bash
# Count installed commands
ls .claude/commands/ratchet/*.md 2>/dev/null | wc -l   # local
ls ~/.claude/commands/ratchet/*.md 2>/dev/null | wc -l  # global
```

Present summary:

```
Ratchet updated successfully!

  Scope: [Global/Local/Both]
  Method: [Nix flake / Git clone / Local source]
  Commands: [N] slash commands installed
  Scripts: [list key scripts: git-pre-commit, publish-debate-hook, etc.]

  Your project data (.ratchet/) was preserved.

  IMPORTANT: Restart Claude Code to pick up the new commands and hooks.
  Type /exit or press Ctrl+C, then start a new session.
```

After presenting the summary, always remind the user to restart. Slash commands and hooks are loaded at session start — the current session will continue using the old versions until restarted.

## What Gets Updated

**Updated:** Slash commands, runtime scripts, agent definitions, schemas, statusline, hooks config.

**Preserved:** `.ratchet/` directory (workflow.yaml, project.yaml, plan.yaml, pairs, debates, scores, escalations).
