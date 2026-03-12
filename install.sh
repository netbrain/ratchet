#!/usr/bin/env bash
# Ratchet installer — copies plugin files into Claude Code's discovery directories
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_NAME="ratchet"

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

json_merge_enabled_plugin() {
    local settings_file="$1"
    python3 -c "
import json, os, sys

path = sys.argv[1]
data = {}
if os.path.isfile(path):
    with open(path) as f:
        data = json.load(f)

plugins = data.setdefault('enabledPlugins', {})
plugins['${PLUGIN_NAME}@local'] = True

with open(path, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" "$settings_file"
}

json_remove_enabled_plugin() {
    local settings_file="$1"
    [ -f "$settings_file" ] || return 0
    python3 -c "
import json, sys

path = sys.argv[1]
with open(path) as f:
    data = json.load(f)

plugins = data.get('enabledPlugins', {})
plugins.pop('${PLUGIN_NAME}@local', None)
if not plugins:
    data.pop('enabledPlugins', None)

with open(path, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" "$settings_file"
}

install_git_hook() {
    local target_dir="$1"
    local plugin_dir="$2"

    # Only for local installs in a git repo
    local git_dir
    git_dir="$(git -C "$target_dir" rev-parse --git-dir 2>/dev/null)" || return 0

    local hooks_dir="$git_dir/hooks"
    local hook_file="$hooks_dir/pre-commit"

    mkdir -p "$hooks_dir"

    # Remove existing ratchet block if present
    remove_git_hook "$target_dir"

    local hook_script_path="$plugin_dir/scripts/check-consensus.sh"

    local block
    block=$(cat <<HOOKEOF
# BEGIN RATCHET
if [ -n "\${CLAUDE_CODE:-}" ]; then
  bash "$hook_script_path"
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

    # Remove the RATCHET block
    local tmp_file
    tmp_file="$(mktemp)"
    sed '/^# BEGIN RATCHET$/,/^# END RATCHET$/d' "$hook_file" > "$tmp_file"

    # If the file is now empty (just a shebang or whitespace), remove it
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
    local plugin_dir="$target/plugins/$PLUGIN_NAME"

    echo "Installing ratchet to $plugin_dir ..."

    # Clean previous install
    rm -rf "$plugin_dir"
    mkdir -p "$plugin_dir"

    # Copy plugin manifest
    mkdir -p "$plugin_dir/.claude-plugin"
    cp "$SCRIPT_DIR/.claude-plugin/plugin.json" "$plugin_dir/.claude-plugin/"

    # Copy agents
    if [ -d "$SCRIPT_DIR/agents" ]; then
        mkdir -p "$plugin_dir/agents"
        cp "$SCRIPT_DIR"/agents/*.md "$plugin_dir/agents/"
    fi

    # Copy skills
    if [ -d "$SCRIPT_DIR/skills" ]; then
        cp -r "$SCRIPT_DIR/skills" "$plugin_dir/skills"
    fi

    # Copy hooks
    if [ -d "$SCRIPT_DIR/hooks" ]; then
        mkdir -p "$plugin_dir/hooks"
        cp "$SCRIPT_DIR"/hooks/hooks.json "$plugin_dir/hooks/"
    fi

    # Copy scripts
    if [ -d "$SCRIPT_DIR/scripts" ]; then
        mkdir -p "$plugin_dir/scripts"
        cp "$SCRIPT_DIR"/scripts/*.sh "$plugin_dir/scripts/"
        chmod +x "$plugin_dir"/scripts/*.sh
    fi

    # Register in settings.json
    local settings_file="$target/settings.json"
    json_merge_enabled_plugin "$settings_file"
    echo "  Registered in $settings_file"

    # Git pre-commit hook (local installs only)
    if [ "$skip_hooks" = "false" ] && [ "$target" != "$HOME/.claude" ]; then
        install_git_hook "$(pwd)" "$plugin_dir"
    fi

    echo ""
    echo "Done! Ratchet installed."
    echo "  Start claude (no --plugin-dir needed) and verify with /help"
}

do_uninstall() {
    local target="$1"
    local plugin_dir="$target/plugins/$PLUGIN_NAME"

    echo "Uninstalling ratchet from $plugin_dir ..."

    if [ -d "$plugin_dir" ]; then
        rm -rf "$plugin_dir"
        echo "  Removed plugin files"
    else
        echo "  No plugin files found (already clean)"
    fi

    # Remove from settings.json
    local settings_file="$target/settings.json"
    if [ -f "$settings_file" ]; then
        json_remove_enabled_plugin "$settings_file"
        echo "  Removed from $settings_file"
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
        # Validate we're in a project (has .git, package.json, or similar)
        if [ ! -d ".git" ] && [ ! -f "package.json" ] && [ ! -f "Makefile" ] && [ ! -f "go.mod" ] && [ ! -f "pyproject.toml" ] && [ ! -f "Cargo.toml" ]; then
            die "Current directory doesn't look like a project root. Run from your project directory."
        fi
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
