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

    mkdir -p "$hooks_dir"

    # Remove existing ratchet block if present
    remove_git_hook "$target_dir"

    local block
    block=$(cat <<HOOKEOF
# BEGIN RATCHET
if [ -n "\${CLAUDE_CODE:-}" ]; then
  bash "$scripts_dir/check-consensus.sh"
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

    chmod +x "$hook_file"
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

do_install() {
    local target="$1"
    local skip_hooks="$2"
    local commands_dir="$target/commands/ratchet"
    local scripts_dir="$target/ratchet-scripts"

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

    # Copy commands (skills -> commands)
    mkdir -p "$commands_dir" || die "Failed to create commands directory: $commands_dir"
    for skill_dir in "$SCRIPT_DIR"/skills/*/; do
        local skill_name
        skill_name="$(basename "$skill_dir")"
        local skill_file="$skill_dir/SKILL.md"
        [ -f "$skill_file" ] || continue
        cp "$skill_file" "$commands_dir/$skill_name.md" || die "Failed to copy skill: $skill_name"
    done
    echo "  Installed commands to $commands_dir/"

    # Copy agents alongside commands
    if [ -d "$SCRIPT_DIR/agents" ] && ls "$SCRIPT_DIR"/agents/*.md >/dev/null 2>&1; then
        mkdir -p "$commands_dir/agents"
        cp "$SCRIPT_DIR"/agents/*.md "$commands_dir/agents/"
        echo "  Installed agents"
    fi

    # Copy scripts
    if [ -d "$SCRIPT_DIR/scripts" ] && ls "$SCRIPT_DIR"/scripts/*.sh >/dev/null 2>&1; then
        mkdir -p "$scripts_dir" || die "Failed to create scripts directory: $scripts_dir"
        cp "$SCRIPT_DIR"/scripts/*.sh "$scripts_dir/"
        chmod +x "$scripts_dir"/*.sh
        echo "  Installed scripts to $scripts_dir/"
    fi

    # Copy progress adapter scripts
    if [ -d "$SCRIPT_DIR/scripts/progress" ]; then
        for adapter_dir in "$SCRIPT_DIR"/scripts/progress/*/; do
            local adapter_name
            adapter_name="$(basename "$adapter_dir")"
            mkdir -p "$scripts_dir/progress/$adapter_name"
            cp "$adapter_dir"/*.sh "$scripts_dir/progress/$adapter_name/"
            chmod +x "$scripts_dir/progress/$adapter_name"/*.sh
        done
        echo "  Installed progress adapters"
    fi

    # Copy schemas
    local schemas_dir="$target/ratchet-schemas"
    if [ -d "$SCRIPT_DIR/schemas" ] && ls "$SCRIPT_DIR"/schemas/*.json >/dev/null 2>&1; then
        if [ -d "$schemas_dir" ]; then
            chmod -R u+w "$schemas_dir" 2>/dev/null || true
            rm -rf "$schemas_dir"
        fi
        mkdir -p "$schemas_dir"
        cp "$SCRIPT_DIR"/schemas/*.json "$schemas_dir/"
        echo "  Installed schemas to $schemas_dir/"
    fi

    # Copy statusline scripts
    if [ -d "$SCRIPT_DIR/statusline" ] && ls "$SCRIPT_DIR"/statusline/*.sh >/dev/null 2>&1; then
        for statusline_file in "$SCRIPT_DIR"/statusline/*.sh; do
            local basename_file
            basename_file="$(basename "$statusline_file")"
            cp "$statusline_file" "$target/$basename_file"
            chmod +x "$target/$basename_file"
        done
        echo "  Installed statusline scripts to $target/"
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
    for statusline_pattern in "$target"/statusline-*.sh; do
        if [ -f "$statusline_pattern" ]; then
            chmod u+w "$statusline_pattern" 2>/dev/null || true
            rm -f "$statusline_pattern"
        fi
    done
    if ls "$target"/statusline-*.sh >/dev/null 2>&1; then
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
