#!/usr/bin/env bash
# test-install.sh — End-to-end tests for install.sh install/uninstall flows
# Uses temp directories exclusively — non-destructive to the real system
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
INSTALL_SCRIPT="$PROJECT_ROOT/install.sh"

PASS=0
FAIL=0
TESTS_RUN=0

# --- Helpers ---

CLEANUP_DIRS=()

setup_tempdir() {
    local tmp
    tmp="$(mktemp -d)"
    CLEANUP_DIRS+=("$tmp")
    echo "$tmp"
}

cleanup_all() {
    for dir in "${CLEANUP_DIRS[@]}"; do
        if [ -n "$dir" ] && [ -d "$dir" ]; then
            chmod -R u+w "$dir" 2>/dev/null || true
            rm -rf "$dir"
        fi
    done
    CLEANUP_DIRS=()
}

trap cleanup_all EXIT

assert_dir_exists() {
    local dir="$1"
    local msg="${2:-Directory $dir should exist}"
    if [ -d "$dir" ]; then
        pass "$msg"
    else
        fail "$msg (directory not found: $dir)"
    fi
}

assert_dir_not_exists() {
    local dir="$1"
    local msg="${2:-Directory $dir should not exist}"
    if [ ! -d "$dir" ]; then
        pass "$msg"
    else
        fail "$msg (directory still exists: $dir)"
    fi
}

assert_file_exists() {
    local file="$1"
    local msg="${2:-File $file should exist}"
    if [ -f "$file" ]; then
        pass "$msg"
    else
        fail "$msg (file not found: $file)"
    fi
}

assert_file_not_exists() {
    local file="$1"
    local msg="${2:-File $file should not exist}"
    if [ ! -f "$file" ]; then
        pass "$msg"
    else
        fail "$msg (file still exists: $file)"
    fi
}

assert_file_contains() {
    local file="$1"
    local pattern="$2"
    local msg="${3:-File $file should contain pattern: $pattern}"
    if grep -q "$pattern" "$file" 2>/dev/null; then
        pass "$msg"
    else
        fail "$msg (pattern not found in $file)"
    fi
}

assert_file_not_contains() {
    local file="$1"
    local pattern="$2"
    local msg="${3:-File $file should not contain pattern: $pattern}"
    if ! grep -q "$pattern" "$file" 2>/dev/null; then
        pass "$msg"
    else
        fail "$msg (pattern found in $file)"
    fi
}

assert_executable() {
    local file="$1"
    local msg="${2:-File $file should be executable}"
    if [ -x "$file" ]; then
        pass "$msg"
    else
        fail "$msg (file not executable: $file)"
    fi
}

pass() {
    PASS=$((PASS + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
    echo "  PASS: $1"
}

fail() {
    FAIL=$((FAIL + 1))
    TESTS_RUN=$((TESTS_RUN + 1))
    echo "  FAIL: $1" >&2
}

section() {
    echo ""
    echo "=== $1 ==="
}

# --- Tests ---

test_local_install() {
    section "Test: Local install (--local)"

    local tmp
    tmp="$(setup_tempdir)"


    # Create a fake project with a git repo
    mkdir -p "$tmp/project"
    git -C "$tmp/project" init --quiet

    # Run install in local mode
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --no-git-hooks) >/dev/null 2>&1

    local target="$tmp/project/.claude"

    # Verify commands directory created
    assert_dir_exists "$target/commands/ratchet" "Local: commands/ratchet directory created"

    # Verify at least one skill was copied
    local skill_count
    skill_count=$(find "$target/commands/ratchet" -maxdepth 1 -name "*.md" 2>/dev/null | wc -l)
    if [ "$skill_count" -gt 0 ]; then
        pass "Local: skill files copied ($skill_count found)"
    else
        fail "Local: no skill files found in commands/ratchet"
    fi

    # Verify scripts directory created
    assert_dir_exists "$target/ratchet-scripts" "Local: ratchet-scripts directory created"

    # Verify scripts are executable
    local script_files
    script_files=$(find "$target/ratchet-scripts" -maxdepth 1 -name "*.sh" 2>/dev/null)
    if [ -n "$script_files" ]; then
        local all_exec=true
        while IFS= read -r sf; do
            if [ ! -x "$sf" ]; then
                all_exec=false
                break
            fi
        done <<< "$script_files"
        if [ "$all_exec" = true ]; then
            pass "Local: scripts are executable"
        else
            fail "Local: some scripts are not executable"
        fi
    else
        fail "Local: no script files found"
    fi

    # Verify schemas directory
    assert_dir_exists "$target/ratchet-schemas" "Local: ratchet-schemas directory created"

    # Verify agents directory
    assert_dir_exists "$target/commands/ratchet/agents" "Local: agents directory created"
}

test_global_install() {
    section "Test: Global install (--global)"

    local tmp
    tmp="$(setup_tempdir)"


    # Override HOME to use temp directory
    local old_home="$HOME"
    export HOME="$tmp"

    mkdir -p "$tmp/.claude"

    (cd "$tmp" && bash "$INSTALL_SCRIPT" --global --no-git-hooks) >/dev/null 2>&1

    local target="$tmp/.claude"

    # Verify commands directory
    assert_dir_exists "$target/commands/ratchet" "Global: commands/ratchet directory created"

    # Verify scripts directory
    assert_dir_exists "$target/ratchet-scripts" "Global: ratchet-scripts directory created"

    # Verify schemas directory
    assert_dir_exists "$target/ratchet-schemas" "Global: ratchet-schemas directory created"

    # Verify skill files match source
    local source_count
    source_count=$(find "$PROJECT_ROOT/skills" -name "SKILL.md" 2>/dev/null | wc -l)
    local installed_count
    installed_count=$(find "$target/commands/ratchet" -maxdepth 1 -name "*.md" 2>/dev/null | wc -l)
    if [ "$source_count" -eq "$installed_count" ]; then
        pass "Global: all $source_count skills installed"
    else
        fail "Global: skill count mismatch (source=$source_count, installed=$installed_count)"
    fi

    export HOME="$old_home"
}

test_uninstall_local() {
    section "Test: Local uninstall"

    local tmp
    tmp="$(setup_tempdir)"


    mkdir -p "$tmp/project"
    git -C "$tmp/project" init --quiet

    # Install first
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --no-git-hooks) >/dev/null 2>&1

    local target="$tmp/project/.claude"

    # Verify install succeeded
    assert_dir_exists "$target/commands/ratchet" "Uninstall-local: install verified"

    # Now uninstall
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --uninstall) >/dev/null 2>&1

    # Verify commands removed
    assert_dir_not_exists "$target/commands/ratchet" "Uninstall-local: commands/ratchet removed"

    # Verify scripts removed
    assert_dir_not_exists "$target/ratchet-scripts" "Uninstall-local: ratchet-scripts removed"

    # Verify schemas removed
    assert_dir_not_exists "$target/ratchet-schemas" "Uninstall-local: ratchet-schemas removed"
}

test_uninstall_global() {
    section "Test: Global uninstall"

    local tmp
    tmp="$(setup_tempdir)"


    local old_home="$HOME"
    export HOME="$tmp"

    mkdir -p "$tmp/.claude"

    # Install first
    (cd "$tmp" && bash "$INSTALL_SCRIPT" --global --no-git-hooks) >/dev/null 2>&1

    local target="$tmp/.claude"

    assert_dir_exists "$target/commands/ratchet" "Uninstall-global: install verified"

    # Uninstall
    (cd "$tmp" && bash "$INSTALL_SCRIPT" --global --uninstall) >/dev/null 2>&1

    assert_dir_not_exists "$target/commands/ratchet" "Uninstall-global: commands removed"
    assert_dir_not_exists "$target/ratchet-scripts" "Uninstall-global: scripts removed"
    assert_dir_not_exists "$target/ratchet-schemas" "Uninstall-global: schemas removed"

    export HOME="$old_home"
}

test_git_hook_install() {
    section "Test: Git hook installation"

    local tmp
    tmp="$(setup_tempdir)"


    mkdir -p "$tmp/project"
    git -C "$tmp/project" init --quiet

    # Install with git hooks (local mode enables hooks by default)
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local) >/dev/null 2>&1

    local hook_file="$tmp/project/.git/hooks/pre-commit"

    assert_file_exists "$hook_file" "Git hook: pre-commit file created"
    assert_executable "$hook_file" "Git hook: pre-commit is executable"
    assert_file_contains "$hook_file" "BEGIN RATCHET" "Git hook: contains BEGIN RATCHET marker"
    assert_file_contains "$hook_file" "END RATCHET" "Git hook: contains END RATCHET marker"
    assert_file_contains "$hook_file" "git-pre-commit.sh" "Git hook: references git-pre-commit.sh"
}

test_git_hook_uninstall() {
    section "Test: Git hook removal on uninstall"

    local tmp
    tmp="$(setup_tempdir)"


    mkdir -p "$tmp/project"
    git -C "$tmp/project" init --quiet

    # Install with hooks
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local) >/dev/null 2>&1

    local hook_file="$tmp/project/.git/hooks/pre-commit"
    assert_file_exists "$hook_file" "Git hook uninstall: hook exists before uninstall"

    # Uninstall
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --uninstall) >/dev/null 2>&1

    # Hook file should be removed (since it only had ratchet content)
    assert_file_not_exists "$hook_file" "Git hook uninstall: hook file removed (was ratchet-only)"
}

test_git_hook_preserves_existing() {
    section "Test: Git hook preserves existing hooks"

    local tmp
    tmp="$(setup_tempdir)"


    mkdir -p "$tmp/project"
    git -C "$tmp/project" init --quiet

    # Create an existing pre-commit hook
    mkdir -p "$tmp/project/.git/hooks"
    cat > "$tmp/project/.git/hooks/pre-commit" <<'EXISTING'
#!/usr/bin/env bash
echo "existing hook"
EXISTING
    chmod +x "$tmp/project/.git/hooks/pre-commit"

    # Install ratchet (should append)
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local) >/dev/null 2>&1

    local hook_file="$tmp/project/.git/hooks/pre-commit"

    assert_file_contains "$hook_file" "existing hook" "Preserve hook: existing content preserved"
    assert_file_contains "$hook_file" "BEGIN RATCHET" "Preserve hook: ratchet block added"

    # Uninstall (should remove only ratchet block)
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --uninstall) >/dev/null 2>&1

    assert_file_exists "$hook_file" "Preserve hook: file still exists after uninstall"
    assert_file_contains "$hook_file" "existing hook" "Preserve hook: existing content preserved after uninstall"
    assert_file_not_contains "$hook_file" "BEGIN RATCHET" "Preserve hook: ratchet block removed"
}

test_no_git_hooks_flag() {
    section "Test: --no-git-hooks flag"

    local tmp
    tmp="$(setup_tempdir)"


    mkdir -p "$tmp/project"
    git -C "$tmp/project" init --quiet

    # Install with --no-git-hooks
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --no-git-hooks) >/dev/null 2>&1

    local hook_file="$tmp/project/.git/hooks/pre-commit"

    assert_file_not_exists "$hook_file" "No-git-hooks: pre-commit hook not created"
}

test_idempotent_install() {
    section "Test: Idempotent install (running twice)"

    local tmp
    tmp="$(setup_tempdir)"


    mkdir -p "$tmp/project"
    git -C "$tmp/project" init --quiet

    # Install twice
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --no-git-hooks) >/dev/null 2>&1
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --no-git-hooks) >/dev/null 2>&1

    local target="$tmp/project/.claude"

    # Should still have commands
    assert_dir_exists "$target/commands/ratchet" "Idempotent: commands directory still present"

    # Should not have duplicate files or errors
    local skill_count
    skill_count=$(find "$target/commands/ratchet" -maxdepth 1 -name "*.md" 2>/dev/null | wc -l)
    if [ "$skill_count" -gt 0 ]; then
        pass "Idempotent: skills present after double install ($skill_count)"
    else
        fail "Idempotent: no skills after double install"
    fi
}

test_progress_adapters_installed() {
    section "Test: Progress adapters installed"

    local tmp
    tmp="$(setup_tempdir)"


    mkdir -p "$tmp/project"
    git -C "$tmp/project" init --quiet

    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --no-git-hooks) >/dev/null 2>&1

    local target="$tmp/project/.claude"

    # Check that progress adapter directories exist
    assert_dir_exists "$target/ratchet-scripts/progress" "Progress: progress directory exists"

    # Check for at least one adapter
    local adapter_count
    adapter_count=$(find "$target/ratchet-scripts/progress" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)
    if [ "$adapter_count" -gt 0 ]; then
        pass "Progress: $adapter_count adapter(s) installed"
    else
        fail "Progress: no adapter directories found"
    fi
}

test_invalid_flag_exits_nonzero() {
    section "Test: Invalid flag exits non-zero"

    local tmp
    tmp="$(setup_tempdir)"


    local exit_code=0
    (cd "$tmp" && bash "$INSTALL_SCRIPT" --bogus) >/dev/null 2>&1 || exit_code=$?

    if [ "$exit_code" -ne 0 ]; then
        pass "Invalid flag: exits with non-zero ($exit_code)"
    else
        fail "Invalid flag: should exit non-zero but got 0"
    fi
}

test_statusline_scripts_installed() {
    section "Test: Statusline scripts installed"

    local tmp
    tmp="$(setup_tempdir)"


    mkdir -p "$tmp/project"
    git -C "$tmp/project" init --quiet

    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --no-git-hooks) >/dev/null 2>&1

    local target="$tmp/project/.claude"

    # Check if statusline scripts exist
    local statusline_count
    statusline_count=$(find "$target" -maxdepth 1 -name "statusline-*.sh" 2>/dev/null | wc -l)
    if [ "$statusline_count" -gt 0 ]; then
        pass "Statusline: $statusline_count script(s) installed"
        # Verify executable
        for sf in "$target"/statusline-*.sh; do
            assert_executable "$sf" "Statusline: $(basename "$sf") is executable"
        done
    else
        # Statusline is optional, so this is a soft check
        pass "Statusline: no statusline scripts in source (OK if statusline/ is empty)"
    fi
}

test_uninstall_removes_statusline() {
    section "Test: Uninstall removes statusline scripts"

    local tmp
    tmp="$(setup_tempdir)"


    mkdir -p "$tmp/project"
    git -C "$tmp/project" init --quiet

    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --no-git-hooks) >/dev/null 2>&1
    (cd "$tmp/project" && bash "$INSTALL_SCRIPT" --local --uninstall) >/dev/null 2>&1

    local target="$tmp/project/.claude"

    local statusline_count
    statusline_count=$(find "$target" -maxdepth 1 -name "statusline-*.sh" 2>/dev/null | wc -l)
    if [ "$statusline_count" -eq 0 ]; then
        pass "Uninstall statusline: no statusline scripts remain"
    else
        fail "Uninstall statusline: $statusline_count script(s) still present"
    fi
}

# --- Run all tests ---

main() {
    echo "=========================================="
    echo "  Ratchet install.sh Test Suite"
    echo "=========================================="

    test_local_install
    test_global_install
    test_uninstall_local
    test_uninstall_global
    test_git_hook_install
    test_git_hook_uninstall
    test_git_hook_preserves_existing
    test_no_git_hooks_flag
    test_idempotent_install
    test_progress_adapters_installed
    test_invalid_flag_exits_nonzero
    test_statusline_scripts_installed
    test_uninstall_removes_statusline

    echo ""
    echo "=========================================="
    echo "  Results: $PASS passed, $FAIL failed (of $TESTS_RUN)"
    echo "=========================================="

    if [ "$FAIL" -gt 0 ]; then
        exit 1
    fi
}

main "$@"
