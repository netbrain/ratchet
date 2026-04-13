#!/usr/bin/env bash
# Ratchet installer — copies commands and scripts into Claude Code's discovery directories
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# --- Helpers ---

usage() {
    cat <<EOF
Usage: $(basename "$0") [--global|--local] [--uninstall] [--no-git-hooks]

  --global       Install to ~/.claude/ (available in all projects)
  --local        Install to .claude/ in current project
  (no flag)      Interactive prompt
  --uninstall    Remove ratchet from the chosen scope
  --no-git-hooks Skip git pre-commit hook installation
EOF
    exit 1
}

die() { echo "Error: $*" >&2; exit 1; }

install_git_hook() {
    local target_dir="$1"
    local scripts_dir="$2"

    local git_dir
    git_dir="$(git -C "$target_dir" rev-parse --git-dir 2>/dev/null)" || return 0

    local hooks_dir="$git_dir/hooks"
    local hook_file="$hooks_dir/pre-commit"

    mkdir -p "$hooks_dir" || { echo "Error: Failed to create hooks directory: $hooks_dir" >&2; return 1; }

    # Remove existing ratchet block if present
    remove_git_hook "$target_dir"

    local block
    block=$(cat <<HOOKEOF
# BEGIN RATCHET
if [ -n "\${CLAUDE_CODE:-}" ] && [ -f "$scripts_dir/git-pre-commit.sh" ]; then
  bash "$scripts_dir/git-pre-commit.sh" || exit \$?
fi
# END RATCHET
HOOKEOF
)

    if [ -f "$hook_file" ]; then
        # Ensure trailing newline so BEGIN marker starts on its own line
        echo "" >> "$hook_file"
        echo "$block" >> "$hook_file"
    else
        cat > "$hook_file" <<SHEBANG
#!/usr/bin/env bash
$block
SHEBANG
    fi

    chmod +x "$hook_file" || { echo "Error: Failed to set hook permissions: $hook_file" >&2; return 1; }
    echo "  Installed git pre-commit hook"
}

remove_git_hook() {
    local target_dir="$1"

    local git_dir
    git_dir="$(git -C "$target_dir" rev-parse --git-dir 2>/dev/null)" || return 0

    local hook_file="$git_dir/hooks/pre-commit"
    [ -f "$hook_file" ] || return 0

    # Verify both markers exist before attempting removal
    if ! grep -q '^# BEGIN RATCHET$' "$hook_file" 2>/dev/null; then
        return 0
    fi
    if ! grep -q '^# END RATCHET$' "$hook_file" 2>/dev/null; then
        echo "Warning: Found BEGIN RATCHET marker but no END RATCHET in $hook_file — skipping removal to avoid data loss" >&2
        return 0
    fi

    local tmp_file
    tmp_file="$(mktemp)"
    # Use a subshell-scoped cleanup: clean up tmp_file immediately after use
    # Avoid setting EXIT trap inside function — EXIT fires at script exit, not function return,
    # so a function-level trap would overwrite any outer trap and reference a stale variable.
    sed '/^# BEGIN RATCHET$/,/^# END RATCHET$/d' "$hook_file" > "$tmp_file"

    local content
    content=$(grep -v '^#!/' "$tmp_file" | grep -v '^[[:space:]]*$' || true)
    if [ -z "$content" ]; then
        rm -f "$hook_file" "$tmp_file"
    else
        mv "$tmp_file" "$hook_file"
        chmod +x "$hook_file"
    fi
    rm -f "$tmp_file" 2>/dev/null || true
}

setup_gitignore() {
    # Add Ratchet gitignore block (exclude runtime state, whitelist
    # institutional memory), then safely untrack any files that are now
    # ignored but were previously committed.
    #
    # Philosophy: low-churn, high-value artifacts that agents need on a
    # fresh clone (escalations/, retros/, reviews/, scores.yaml, and
    # debates/*/meta.json) are whitelisted. High-churn runtime state
    # (worktrees/, locks/, archive/, guards/, reports/, progress/,
    # issues/, plan.yaml, and debate transcripts) stays excluded.
    #
    # CRITICAL ORDER: gitignore entry MUST be written before git rm --cached,
    # otherwise git rm --cached removes the file from tracking without a safety
    # net and a subsequent commit could re-add it.
    local project_dir="$1"

    # Only operate inside a git repo
    git -C "$project_dir" rev-parse --git-dir >/dev/null 2>&1 || return 0

    local gitignore="$project_dir/.gitignore"

    local marker="# Ratchet runtime state (auto-added by install.sh)"

    # Check if marker is already present (idempotent)
    if grep -qF "$marker" "$gitignore" 2>/dev/null; then
        return 0
    fi

    # Step 1 — Write gitignore entries FIRST (safety net before any untracking).
    # The block uses the "exclude all, then whitelist" pattern so low-churn
    # institutional memory survives fresh clones while noisy runtime state
    # is kept out of the repo.
    {
        echo ""
        echo "$marker"
        echo "# Exclude all runtime state, then whitelist institutional memory."
        echo ".ratchet/*"
        echo "!.ratchet/workflow.yaml"
        echo "!.ratchet/project.yaml"
        echo "!.ratchet/pairs/"
        echo "# Tiebreaker rulings — precedents scanned by skills/run/SKILL.md Step 5d"
        echo "!.ratchet/escalations/"
        echo "# EMA quality metrics — needed for /ratchet:score trends"
        echo "!.ratchet/scores.yaml"
        echo "# Retrospective findings and agent performance reviews (Tighten reads these)"
        echo "!.ratchet/retros/"
        echo "!.ratchet/reviews/"
        echo "# plan.yaml is runtime state when progress.adapter=github-issues (default)."
        echo "# Uncomment below if you use progress.adapter=markdown or none:"
        echo "# !.ratchet/plan.yaml"
        echo ""
        echo "# Debates: exclude transcripts (round-*.md) but keep structured metadata."
        echo "# The debates/ dir and its subdirs must stay visible so meta.json can"
        echo "# be whitelisted (git cannot re-include a file whose parent is excluded)."
        echo "!.ratchet/debates/"
        echo "!.ratchet/debates/*/"
        echo ".ratchet/debates/*/*"
        echo "!.ratchet/debates/*/meta.json"
        echo ""
        echo "# Runtime state that stays fully excluded"
        echo ".ratchet/worktrees/"
        echo ".ratchet/locks/"
        echo ".ratchet/archive/"
        echo ".ratchet/guards/"
        echo ".ratchet/reports/"
        echo ".ratchet/progress/"
        echo ".ratchet/issues/"
    } >> "$gitignore" || { echo "Error: Failed to write to $gitignore" >&2; return 1; }

    echo "  Updated .gitignore with Ratchet runtime state entries"

    # Step 2 — Untrack any files that are now ignored but were previously committed.
    # Run AFTER the gitignore is updated so the patterns are active. Only untrack
    # entries that remain excluded; do NOT touch whitelisted institutional memory.
    local tracked_runtime=()
    while IFS= read -r tracked_file; do
        [ -n "$tracked_file" ] && tracked_runtime+=("$tracked_file")
    done < <(git -C "$project_dir" ls-files -- \
        ".ratchet/plan.yaml" \
        ".ratchet/worktrees/" \
        ".ratchet/locks/" \
        ".ratchet/archive/" \
        ".ratchet/guards/" \
        ".ratchet/reports/" \
        ".ratchet/progress/" \
        ".ratchet/issues/" \
        2>/dev/null)

    # Also untrack debate transcripts (everything under debates/*/ except meta.json)
    while IFS= read -r tracked_file; do
        [ -n "$tracked_file" ] || continue
        case "$tracked_file" in
            *"/meta.json") ;;  # keep metadata
            *) tracked_runtime+=("$tracked_file") ;;
        esac
    done < <(git -C "$project_dir" ls-files -- ".ratchet/debates/" 2>/dev/null)

    if [ "${#tracked_runtime[@]}" -gt 0 ]; then
        git -C "$project_dir" rm --cached -- "${tracked_runtime[@]}" 2>/dev/null || \
            echo "  Warning: Could not untrack some runtime files — commit manually with 'git rm --cached <file>'" >&2
        echo "  Untracked ${#tracked_runtime[@]} runtime file(s) from git index"
    fi
}

install_publish_hook() {
    local target="$1"
    local scripts_dir="$2"
    local settings_file="$target/settings.json"
    local hook_script="$scripts_dir/publish-debate-hook.sh"

    # Only install if publish-debate-hook.sh was copied to scripts dir
    if [ ! -f "$hook_script" ]; then
        return 0
    fi

    # Resolve to absolute path so the hook works in worktree-isolated agents.
    # Worktrees are clean git checkouts — relative paths resolve against the
    # worktree root, which may not have .claude/ratchet-scripts/.
    # Using an absolute path ensures the hook script is always found.
    local abs_hook_script
    abs_hook_script="$(cd "$(dirname "$hook_script")" && pwd)/$(basename "$hook_script")"

    # Build the hook entry we want
    local hook_json
    hook_json=$(cat <<HOOKJSON
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write",
        "hooks": [
          {
            "type": "command",
            "command": "bash $abs_hook_script"
          }
        ]
      }
    ]
  }
}
HOOKJSON
)

    if [ ! -f "$settings_file" ]; then
        # No settings file — create one with the hook
        echo "$hook_json" > "$settings_file" || { echo "Error: Failed to write $settings_file" >&2; return 1; }
    else
        # Settings file exists — merge hooks if not already present
        if grep -q "publish-debate-hook" "$settings_file" 2>/dev/null; then
            return 0  # Already installed
        fi

        if command -v jq >/dev/null 2>&1; then
            # Use jq to merge
            local tmp_file
            tmp_file="$(mktemp)"
            jq --argjson new "$hook_json" '
                .hooks.PostToolUse = (.hooks.PostToolUse // []) + $new.hooks.PostToolUse
            ' "$settings_file" > "$tmp_file" 2>/dev/null && mv "$tmp_file" "$settings_file"
            rm -f "$tmp_file" 2>/dev/null || true
        else
            echo "  Warning: jq not found, skipping settings.json hook merge. Add PostToolUse hook manually." >&2
            return 0
        fi
    fi

    # Track settings.json in git so worktree-isolated agents inherit hooks.
    # Worktrees are clean git checkouts — untracked files don't appear in them.
    if [ "$target" != "$HOME/.claude" ]; then
        git add "$settings_file" 2>/dev/null || true
    fi

    echo "  Installed PostToolUse hook for debate publishing"
}

install_auto_watch_hook() {
    local target="$1"
    local scripts_dir="$2"
    local settings_file="$target/settings.json"
    local hook_script="$scripts_dir/auto-watch-hook.sh"

    # Only install if auto-watch-hook.sh was copied to scripts dir
    if [ ! -f "$hook_script" ]; then
        return 0
    fi

    # Resolve to absolute path so the hook works in worktree-isolated agents.
    local abs_hook_script
    abs_hook_script="$(cd "$(dirname "$hook_script")" && pwd)/$(basename "$hook_script")"

    # Build the hook entry — fires on Bash tool use to detect gh pr create
    local hook_json
    hook_json=$(cat <<HOOKJSON
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "bash $abs_hook_script"
          }
        ]
      }
    ]
  }
}
HOOKJSON
)

    if [ ! -f "$settings_file" ]; then
        # No settings file — create one with the hook
        echo "$hook_json" > "$settings_file" || { echo "Error: Failed to write $settings_file" >&2; return 1; }
    else
        # Settings file exists — merge hooks if not already present
        if grep -q "auto-watch-hook" "$settings_file" 2>/dev/null; then
            return 0  # Already installed
        fi

        if command -v jq >/dev/null 2>&1; then
            # Use jq to merge
            local tmp_file
            tmp_file="$(mktemp)"
            jq --argjson new "$hook_json" '
                .hooks.PostToolUse = (.hooks.PostToolUse // []) + $new.hooks.PostToolUse
            ' "$settings_file" > "$tmp_file" 2>/dev/null && mv "$tmp_file" "$settings_file"
            rm -f "$tmp_file" 2>/dev/null || true
        else
            echo "  Warning: jq not found, skipping settings.json hook merge. Add PostToolUse hook manually." >&2
            return 0
        fi
    fi

    # Track settings.json in git so worktree-isolated agents inherit hooks.
    if [ "$target" != "$HOME/.claude" ]; then
        git add "$settings_file" 2>/dev/null || true
    fi

    echo "  Installed PostToolUse hook for auto-watch on PR creation"
}

do_install() {
    local target="$1"
    local skip_hooks="$2"
    local commands_dir="$target/commands/ratchet"
    local scripts_dir="$target/ratchet-scripts"

    # Use symlinks for local installs (source files are in the same repo)
    # Use copies for global installs (source files won't be at a relative path)
    local use_symlinks=false
    if [ "$target" = ".claude" ] && [ "$SCRIPT_DIR" = "$(pwd)" ]; then
        use_symlinks=true
    fi

    echo "Installing ratchet to $target ..."

    # Clean previous install (both old plugin-style and new commands-style)
    # chmod first in case files came from nix store (read-only)
    if [ -d "$target/plugins/ratchet" ]; then
        chmod -R u+w "$target/plugins/ratchet" 2>/dev/null || true
        rm -rf "$target/plugins/ratchet"
        rmdir "$target/plugins" 2>/dev/null || true
    fi
    if [ -d "$commands_dir" ]; then
        chmod -R u+w "$commands_dir" 2>/dev/null || true
        rm -rf "$commands_dir"
    fi
    if [ -d "$scripts_dir" ]; then
        chmod -R u+w "$scripts_dir" 2>/dev/null || true
        rm -rf "$scripts_dir"
    fi

    # Install commands (skills -> commands)
    mkdir -p "$commands_dir" || die "Failed to create commands directory: $commands_dir"
    for skill_dir in "$SCRIPT_DIR"/skills/*/; do
        local skill_name
        skill_name="$(basename "$skill_dir")"
        local skill_file="$skill_dir/SKILL.md"
        [ -f "$skill_file" ] || continue
        if [ "$use_symlinks" = true ]; then
            ln -sf "../../../skills/$skill_name/SKILL.md" "$commands_dir/$skill_name.md" || die "Failed to link skill: $skill_name"
        else
            cp "$skill_file" "$commands_dir/$skill_name.md" || die "Failed to copy skill: $skill_name"
        fi
    done
    echo "  Installed commands to $commands_dir/"

    # Install top-level command aliases (shortcuts outside the ratchet/ subdirectory)
    # These create e.g. .claude/commands/rr.md -> ratchet/run.md
    local aliases_dir="$target/commands"
    local -a alias_pairs=(
        "rr:run"
        "rrs:status"
        "rrt:tighten"
    )
    for alias_entry in "${alias_pairs[@]}"; do
        local alias_name="${alias_entry%%:*}"
        local skill_target="${alias_entry#*:}"
        local alias_file="$aliases_dir/$alias_name.md"
        local target_file="$commands_dir/$skill_target.md"
        # Only create alias if the target skill was installed
        if [ -f "$target_file" ] || [ -L "$target_file" ]; then
            if [ "$use_symlinks" = true ]; then
                ln -sf "ratchet/$skill_target.md" "$alias_file" || die "Failed to link alias: $alias_name"
            else
                cp "$target_file" "$alias_file" || die "Failed to copy alias: $alias_name"
            fi
        fi
    done
    echo "  Installed command aliases (rr, rrs, rrt)"

    # Install agents alongside commands
    if [ -d "$SCRIPT_DIR/agents" ] && compgen -G "$SCRIPT_DIR/agents/*.md" >/dev/null 2>&1; then
        mkdir -p "$commands_dir/agents" || die "Failed to create agents directory: $commands_dir/agents"
        for agent_file in "$SCRIPT_DIR"/agents/*.md; do
            local agent_name
            agent_name="$(basename "$agent_file")"
            if [ "$use_symlinks" = true ]; then
                ln -sf "../../../../agents/$agent_name" "$commands_dir/agents/$agent_name" || die "Failed to link agent: $agent_name"
            else
                cp "$agent_file" "$commands_dir/agents/$agent_name" || die "Failed to copy agent: $agent_name"
            fi
        done
        echo "  Installed agents"
    fi

    # Copy scripts
    if [ -d "$SCRIPT_DIR/scripts" ] && compgen -G "$SCRIPT_DIR/scripts/*.sh" >/dev/null 2>&1; then
        mkdir -p "$scripts_dir" || die "Failed to create scripts directory: $scripts_dir"
        cp "$SCRIPT_DIR"/scripts/*.sh "$scripts_dir/" || die "Failed to copy script files"
        chmod +x "$scripts_dir"/*.sh || die "Failed to set script permissions"
        echo "  Installed scripts to $scripts_dir/"
    fi

    # Copy progress adapter scripts
    if [ -d "$SCRIPT_DIR/scripts/progress" ]; then
        for adapter_dir in "$SCRIPT_DIR"/scripts/progress/*/; do
            local adapter_name
            adapter_name="$(basename "$adapter_dir")"
            mkdir -p "$scripts_dir/progress/$adapter_name" || die "Failed to create progress adapter directory: $scripts_dir/progress/$adapter_name"
            cp "$adapter_dir"/*.sh "$scripts_dir/progress/$adapter_name/" || die "Failed to copy progress adapter: $adapter_name"
            chmod +x "$scripts_dir/progress/$adapter_name"/*.sh || die "Failed to set permissions for progress adapter: $adapter_name"
        done
        echo "  Installed progress adapters"
    fi

    # Copy schemas
    local schemas_dir="$target/ratchet-schemas"
    if [ -d "$SCRIPT_DIR/schemas" ] && compgen -G "$SCRIPT_DIR/schemas/*.json" >/dev/null 2>&1; then
        if [ -d "$schemas_dir" ]; then
            chmod -R u+w "$schemas_dir" 2>/dev/null || true
            rm -rf "$schemas_dir"
        fi
        mkdir -p "$schemas_dir" || die "Failed to create schemas directory: $schemas_dir"
        cp "$SCRIPT_DIR"/schemas/*.json "$schemas_dir/" || die "Failed to copy schema files"
        echo "  Installed schemas to $schemas_dir/"
    fi

    # Copy statusline scripts
    if [ -d "$SCRIPT_DIR/statusline" ] && compgen -G "$SCRIPT_DIR/statusline/*.sh" >/dev/null 2>&1; then
        for statusline_file in "$SCRIPT_DIR"/statusline/*.sh; do
            local basename_file
            basename_file="$(basename "$statusline_file")"
            cp "$statusline_file" "$target/$basename_file" || die "Failed to copy statusline script: $basename_file"
            chmod +x "$target/$basename_file" || die "Failed to set permissions for: $basename_file"
        done
        echo "  Installed statusline scripts to $target/"
    fi

    # Install PostToolUse hook for debate publish automation
    install_publish_hook "$target" "$scripts_dir"

    # Install PostToolUse hook for auto-watch on PR creation
    install_auto_watch_hook "$target" "$scripts_dir"

    # Gitignore entries + safe untracking (local installs only — global installs
    # target ~/.claude, not the project repo, so there is no project .gitignore)
    if [ "$target" != "$HOME/.claude" ]; then
        setup_gitignore "$(pwd)"
    fi

    # Git pre-commit hook (local installs only)
    if [ "$skip_hooks" = "false" ] && [ "$target" != "$HOME/.claude" ]; then
        install_git_hook "$(pwd)" "$scripts_dir"
    fi

    echo ""
    echo "Done! Ratchet installed."
    echo "  Start claude and verify with /help — look for /ratchet:init"
    echo ""
    echo "Optional: Configure Ratchet statusline in Claude Code settings:"
    echo "  statusline: $target/statusline-ratchet.sh"
}

do_uninstall() {
    local target="$1"
    local commands_dir="$target/commands/ratchet"
    local scripts_dir="$target/ratchet-scripts"

    echo "Uninstalling ratchet from $target ..."

    # Remove command aliases (must happen before rmdir on commands/)
    local -a alias_names=("rr" "rrs" "rrt")
    for alias_name in "${alias_names[@]}"; do
        local alias_file="$target/commands/$alias_name.md"
        if [ -f "$alias_file" ] || [ -L "$alias_file" ]; then
            rm -f "$alias_file"
        fi
    done
    echo "  Removed command aliases"

    # Remove commands
    if [ -d "$commands_dir" ]; then
        chmod -R u+w "$commands_dir" 2>/dev/null || true
        rm -rf "$commands_dir"
        rmdir "$target/commands" 2>/dev/null || true
        echo "  Removed commands"
    fi

    # Remove scripts
    if [ -d "$scripts_dir" ]; then
        chmod -R u+w "$scripts_dir" 2>/dev/null || true
        rm -rf "$scripts_dir"
        echo "  Removed scripts"
    fi

    # Remove schemas
    local schemas_dir="$target/ratchet-schemas"
    if [ -d "$schemas_dir" ]; then
        chmod -R u+w "$schemas_dir" 2>/dev/null || true
        rm -rf "$schemas_dir"
        echo "  Removed schemas"
    fi

    # Remove statusline scripts
    local removed_statusline=false
    for statusline_pattern in "$target"/statusline-*.sh; do
        if [ -f "$statusline_pattern" ]; then
            chmod u+w "$statusline_pattern" 2>/dev/null || true
            rm -f "$statusline_pattern"
            removed_statusline=true
        fi
    done
    if [ "$removed_statusline" = "true" ]; then
        echo "  Removed statusline scripts"
    fi

    # Remove old plugin-style install if present
    if [ -d "$target/plugins/ratchet" ]; then
        chmod -R u+w "$target/plugins/ratchet" 2>/dev/null || true
        rm -rf "$target/plugins/ratchet"
        rmdir "$target/plugins" 2>/dev/null || true
        echo "  Removed legacy plugin files"
    fi

    # Clean enabledPlugins from settings.json if present (legacy cleanup)
    local settings_file="$target/settings.json"
    if [ -f "$settings_file" ] && grep -q "ratchet@local" "$settings_file" 2>/dev/null; then
        if ! command -v python3 >/dev/null 2>&1; then
            echo "  Warning: python3 not found, skipping settings.json cleanup" >&2
        else
            if python3 -c "
import json, sys
path = sys.argv[1]
try:
    with open(path) as f:
        data = json.load(f)
except json.JSONDecodeError:
    print(f'  Warning: {path} contains malformed JSON, skipping cleanup', file=sys.stderr)
    sys.exit(1)
plugins = data.get('enabledPlugins', {})
plugins.pop('ratchet@local', None)
if not plugins:
    data.pop('enabledPlugins', None)
with open(path, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" "$settings_file"; then
                echo "  Cleaned settings.json"
            # else: warning already printed by Python, continue silently
            fi
        fi
    fi

    # Remove auto-watch PostToolUse hook from settings.json
    if [ -f "$target/settings.json" ] && grep -q "auto-watch-hook" "$target/settings.json" 2>/dev/null; then
        if command -v jq >/dev/null 2>&1; then
            local tmp_file
            tmp_file="$(mktemp)"
            jq '
                if .hooks.PostToolUse then
                    .hooks.PostToolUse |= [.[] | select(.hooks | all(.command | test("auto-watch-hook") | not))]
                    | if (.hooks.PostToolUse | length) == 0 then del(.hooks.PostToolUse) else . end
                    | if (.hooks | length) == 0 then del(.hooks) else . end
                else . end
            ' "$target/settings.json" > "$tmp_file" 2>/dev/null && mv "$tmp_file" "$target/settings.json"
            rm -f "$tmp_file" 2>/dev/null || true
            echo "  Cleaned auto-watch PostToolUse hook from settings.json"
        fi
    fi

    # Remove PostToolUse hook from settings.json
    if [ -f "$target/settings.json" ] && grep -q "publish-debate-hook" "$target/settings.json" 2>/dev/null; then
        if command -v jq >/dev/null 2>&1; then
            local tmp_file
            tmp_file="$(mktemp)"
            jq '
                if .hooks.PostToolUse then
                    .hooks.PostToolUse |= [.[] | select(.hooks | all(.command | test("publish-debate-hook") | not))]
                    | if (.hooks.PostToolUse | length) == 0 then del(.hooks.PostToolUse) else . end
                    | if (.hooks | length) == 0 then del(.hooks) else . end
                else . end
            ' "$target/settings.json" > "$tmp_file" 2>/dev/null && mv "$tmp_file" "$target/settings.json"
            rm -f "$tmp_file" 2>/dev/null || true
            echo "  Cleaned PostToolUse hook from settings.json"
        fi
    fi

    # Remove git hook block
    if [ "$target" != "$HOME/.claude" ]; then
        remove_git_hook "$(pwd)"
        echo "  Cleaned git pre-commit hook"
    fi

    echo ""
    echo "Done! Ratchet uninstalled."
    echo "  Note: .ratchet/ project data was NOT removed."
}

# --- Parse args ---

MODE=""
UNINSTALL=false
NO_GIT_HOOKS=false

while [ $# -gt 0 ]; do
    case "$1" in
        --global) MODE="global" ;;
        --local)  MODE="local" ;;
        --uninstall) UNINSTALL=true ;;
        --no-git-hooks) NO_GIT_HOOKS=true ;;
        -h|--help) usage ;;
        *) die "Unknown option: $1" ;;
    esac
    shift
done

# Interactive mode selection if not specified
if [ -z "$MODE" ]; then
    echo "Where should ratchet be installed?"
    echo ""
    echo "  1) Global  (~/.claude/ — available in all projects)"
    echo "  2) Local   (.claude/ — this project only)"
    echo ""
    read -rp "Choose [1/2]: " choice
    case "$choice" in
        1) MODE="global" ;;
        2) MODE="local" ;;
        *) die "Invalid choice" ;;
    esac
fi

# Resolve target directory
case "$MODE" in
    global)
        TARGET="$HOME/.claude"
        mkdir -p "$TARGET" || die "Failed to create global install directory: $TARGET"
        ;;
    local)
        TARGET=".claude"
        mkdir -p "$TARGET" || die "Failed to create local install directory: $TARGET"
        ;;
esac

# Execute
if [ "$UNINSTALL" = true ]; then
    do_uninstall "$TARGET"
else
    do_install "$TARGET" "$NO_GIT_HOOKS"
fi
