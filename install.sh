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

    local tmp_file
    tmp_file="$(mktemp)"
    sed '/^# BEGIN RATCHET$/,/^# END RATCHET$/d' "$hook_file" > "$tmp_file"

    local content
    content=$(grep -v '^#!/' "$tmp_file" | grep -v '^[[:space:]]*$' || true)
    if [ -z "$content" ]; then
        rm -f "$hook_file" "$tmp_file"
    else
        mv "$tmp_file" "$hook_file"
        chmod +x "$hook_file"
    fi
}

do_install() {
    local target="$1"
    local skip_hooks="$2"
    local commands_dir="$target/commands/ratchet"
    local scripts_dir="$target/ratchet-scripts"

    echo "Installing ratchet to $target ..."

    # Clean previous install (both old plugin-style and new commands-style)
    # chmod first in case files came from nix store (read-only)
    [ -d "$target/plugins/ratchet" ] && chmod -R u+w "$target/plugins/ratchet" 2>/dev/null; rm -rf "$target/plugins/ratchet"
    [ -d "$commands_dir" ] && chmod -R u+w "$commands_dir" 2>/dev/null; rm -rf "$commands_dir"
    [ -d "$scripts_dir" ] && chmod -R u+w "$scripts_dir" 2>/dev/null; rm -rf "$scripts_dir"

    # Copy commands (skills -> commands)
    mkdir -p "$commands_dir"
    for skill_dir in "$SCRIPT_DIR"/skills/*/; do
        local skill_name
        skill_name="$(basename "$skill_dir")"
        local skill_file="$skill_dir/SKILL.md"
        [ -f "$skill_file" ] || continue
        cp "$skill_file" "$commands_dir/$skill_name.md"
    done
    echo "  Installed commands to $commands_dir/"

    # Copy agents alongside commands
    if [ -d "$SCRIPT_DIR/agents" ]; then
        mkdir -p "$commands_dir/agents"
        cp "$SCRIPT_DIR"/agents/*.md "$commands_dir/agents/"
        echo "  Installed agents"
    fi

    # Copy scripts
    if [ -d "$SCRIPT_DIR/scripts" ]; then
        mkdir -p "$scripts_dir"
        cp "$SCRIPT_DIR"/scripts/*.sh "$scripts_dir/"
        chmod +x "$scripts_dir"/*.sh
        echo "  Installed scripts to $scripts_dir/"
    fi

    # Git pre-commit hook (local installs only)
    if [ "$skip_hooks" = "false" ] && [ "$target" != "$HOME/.claude" ]; then
        install_git_hook "$(pwd)" "$scripts_dir"
    fi

    echo ""
    echo "Done! Ratchet installed."
    echo "  Start claude and verify with /help — look for /ratchet:init"
}

do_uninstall() {
    local target="$1"
    local commands_dir="$target/commands/ratchet"
    local scripts_dir="$target/ratchet-scripts"

    echo "Uninstalling ratchet from $target ..."

    # Remove commands
    if [ -d "$commands_dir" ]; then
        chmod -R u+w "$commands_dir" 2>/dev/null; rm -rf "$commands_dir"
        echo "  Removed commands"
    fi

    # Remove scripts
    if [ -d "$scripts_dir" ]; then
        chmod -R u+w "$scripts_dir" 2>/dev/null; rm -rf "$scripts_dir"
        echo "  Removed scripts"
    fi

    # Remove old plugin-style install if present
    if [ -d "$target/plugins/ratchet" ]; then
        chmod -R u+w "$target/plugins/ratchet" 2>/dev/null; rm -rf "$target/plugins/ratchet"
        echo "  Removed legacy plugin files"
    fi

    # Clean enabledPlugins from settings.json if present (legacy cleanup)
    local settings_file="$target/settings.json"
    if [ -f "$settings_file" ] && grep -q "ratchet@local" "$settings_file" 2>/dev/null; then
        python3 -c "
import json, sys
path = sys.argv[1]
with open(path) as f:
    data = json.load(f)
plugins = data.get('enabledPlugins', {})
plugins.pop('ratchet@local', None)
if not plugins:
    data.pop('enabledPlugins', None)
with open(path, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" "$settings_file"
        echo "  Cleaned settings.json"
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
        mkdir -p "$TARGET"
        ;;
    local)
        TARGET=".claude"
        mkdir -p "$TARGET"
        ;;
esac

# Execute
if [ "$UNINSTALL" = true ]; then
    do_uninstall "$TARGET"
else
    do_install "$TARGET" "$NO_GIT_HOOKS"
fi
