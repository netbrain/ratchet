#!/usr/bin/env bash
# test-debate-flow.sh — End-to-end tests for the debate lifecycle
# Exercises: debate directory creation, round file generation, meta.json structure,
# consensus detection via check-consensus.sh, and cache updates via cache-update.sh.
# Uses temp directories exclusively — non-destructive.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PASS=0
FAIL=0
TESTS_RUN=0

CLEANUP_DIRS=()

# --- Helpers ---

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

assert_file_exists() {
    local file="$1"
    local msg="${2:-File $file should exist}"
    if [ -f "$file" ]; then
        pass "$msg"
    else
        fail "$msg (file not found: $file)"
    fi
}

assert_dir_exists() {
    local dir="$1"
    local msg="${2:-Directory $dir should exist}"
    if [ -d "$dir" ]; then
        pass "$msg"
    else
        fail "$msg (directory not found: $dir)"
    fi
}

assert_file_contains() {
    local file="$1"
    local pattern="$2"
    local msg="${3:-File $file should contain: $pattern}"
    if grep -q "$pattern" "$file" 2>/dev/null; then
        pass "$msg"
    else
        fail "$msg (pattern not found)"
    fi
}

assert_json_field() {
    local file="$1"
    local key="$2"
    local expected="$3"
    local msg="${4:-JSON field $key should be $expected}"
    local actual
    actual=$(sed -n 's/.*"'"$key"'"[[:space:]]*:[[:space:]]*"\{0,1\}\([^",}]*\)"\{0,1\}.*/\1/p' "$file" | head -1)
    if [ "$actual" = "$expected" ]; then
        pass "$msg"
    else
        fail "$msg (got: '$actual')"
    fi
}

# Create a minimal .ratchet project in a temp directory
setup_test_project() {
    local tmp="$1"

    mkdir -p "$tmp/.ratchet/debates"
    mkdir -p "$tmp/.ratchet/pairs/test-pair"

    # Create minimal pair definitions
    cat > "$tmp/.ratchet/pairs/test-pair/generative.md" <<'EOF'
# Test Pair — Generative
You are the generative agent for test-pair.
EOF
    cat > "$tmp/.ratchet/pairs/test-pair/adversarial.md" <<'EOF'
# Test Pair — Adversarial
You are the adversarial agent for test-pair.
EOF

    # Copy scripts for testing
    mkdir -p "$tmp/scripts"
    cp "$PROJECT_ROOT/scripts/check-consensus.sh" "$tmp/scripts/" 2>/dev/null || true
    cp "$PROJECT_ROOT/scripts/cache-check.sh" "$tmp/scripts/" 2>/dev/null || true
    cp "$PROJECT_ROOT/scripts/cache-update.sh" "$tmp/scripts/" 2>/dev/null || true
    cp "$PROJECT_ROOT/scripts/run-guards.sh" "$tmp/scripts/" 2>/dev/null || true
}

# Create a debate directory with meta.json and round files
create_test_debate() {
    local base_dir="$1"
    local debate_id="$2"
    local status="$3"
    local verdict="${4:-null}"
    local rounds="${5:-1}"

    local debate_dir="$base_dir/.ratchet/debates/$debate_id"
    mkdir -p "$debate_dir/rounds"

    # Write meta.json
    local verdict_json
    if [ "$verdict" = "null" ]; then
        verdict_json="null"
    else
        verdict_json="\"$verdict\""
    fi

    cat > "$debate_dir/meta.json" <<META_EOF
{
  "id": "$debate_id",
  "pair": "test-pair",
  "phase": "review",
  "milestone": "test-milestone",
  "issue": "test-1",
  "files": ["test.sh"],
  "status": "$status",
  "rounds": $rounds,
  "max_rounds": 3,
  "started": "2026-03-17T00:00:00Z",
  "resolved": null,
  "verdict": $verdict_json,
  "fast_path": false,
  "decided_by": null
}
META_EOF

    # Create round files
    for i in $(seq 1 "$rounds"); do
        cat > "$debate_dir/rounds/round-${i}-generative.md" <<ROUND_EOF
# Round $i — Generative
Test generative output for round $i.
ROUND_EOF
        cat > "$debate_dir/rounds/round-${i}-adversarial.md" <<ROUND_EOF
# Round $i — Adversarial
Test adversarial output for round $i.
$(if [ "$i" -eq "$rounds" ] && [ "$verdict" != "null" ]; then echo "**$verdict**"; fi)
ROUND_EOF
    done

    echo "$debate_dir"
}

# --- Tests ---

test_debate_directory_creation() {
    section "Test: Debate directory structure"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    local debate_id="test-pair-20260317T000000"
    local debate_dir
    debate_dir=$(create_test_debate "$tmp" "$debate_id" "consensus" "ACCEPT" 2)

    # Verify directory structure
    assert_dir_exists "$debate_dir" "Debate dir exists"
    assert_dir_exists "$debate_dir/rounds" "Rounds subdirectory exists"
    assert_file_exists "$debate_dir/meta.json" "meta.json exists"
}

test_meta_json_structure() {
    section "Test: meta.json structure and fields"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    local debate_id="test-pair-20260317T000001"
    local debate_dir
    debate_dir=$(create_test_debate "$tmp" "$debate_id" "consensus" "ACCEPT" 2)

    # Verify required fields
    assert_json_field "$debate_dir/meta.json" "id" "$debate_id" "meta.json: id field correct"
    assert_json_field "$debate_dir/meta.json" "pair" "test-pair" "meta.json: pair field correct"
    assert_json_field "$debate_dir/meta.json" "phase" "review" "meta.json: phase field correct"
    assert_json_field "$debate_dir/meta.json" "status" "consensus" "meta.json: status field correct"
    assert_json_field "$debate_dir/meta.json" "verdict" "ACCEPT" "meta.json: verdict field correct"
    assert_json_field "$debate_dir/meta.json" "max_rounds" "3" "meta.json: max_rounds field correct"

    # Verify rounds count
    assert_json_field "$debate_dir/meta.json" "rounds" "2" "meta.json: rounds field is 2"

    # Verify files array present
    assert_file_contains "$debate_dir/meta.json" '"files"' "meta.json: files array present"

    # Verify started timestamp format (ISO 8601)
    assert_file_contains "$debate_dir/meta.json" '"started": "20[0-9][0-9]-' "meta.json: started has ISO timestamp"
}

test_round_file_generation() {
    section "Test: Round file naming and content"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    local debate_id="test-pair-20260317T000002"
    local debate_dir
    debate_dir=$(create_test_debate "$tmp" "$debate_id" "consensus" "ACCEPT" 3)

    # Verify round files exist for each round
    for i in 1 2 3; do
        assert_file_exists "$debate_dir/rounds/round-${i}-generative.md" "Round $i: generative file exists"
        assert_file_exists "$debate_dir/rounds/round-${i}-adversarial.md" "Round $i: adversarial file exists"
    done

    # Verify naming convention
    local gen_count
    gen_count=$(find "$debate_dir/rounds" -name "round-*-generative.md" | wc -l)
    local adv_count
    adv_count=$(find "$debate_dir/rounds" -name "round-*-adversarial.md" | wc -l)

    if [ "$gen_count" -eq 3 ]; then
        pass "Round files: 3 generative round files"
    else
        fail "Round files: expected 3 generative files, got $gen_count"
    fi

    if [ "$adv_count" -eq 3 ]; then
        pass "Round files: 3 adversarial round files"
    else
        fail "Round files: expected 3 adversarial files, got $adv_count"
    fi

    # Verify last adversarial round contains verdict
    assert_file_contains "$debate_dir/rounds/round-3-adversarial.md" "ACCEPT" \
        "Round 3 adversarial: contains verdict keyword"
}

test_consensus_detection_allows_commit() {
    section "Test: check-consensus.sh allows commit when debate is resolved"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    create_test_debate "$tmp" "test-pair-20260317T000003" "consensus" "ACCEPT" 1 >/dev/null

    # Run check-consensus from the project directory
    local exit_code=0
    (cd "$tmp" && bash scripts/check-consensus.sh) >/dev/null 2>&1 || exit_code=$?

    if [ "$exit_code" -eq 0 ]; then
        pass "Consensus: resolved debate allows commit (exit 0)"
    else
        fail "Consensus: resolved debate should allow commit but got exit $exit_code"
    fi
}

test_consensus_detection_blocks_initiated() {
    section "Test: check-consensus.sh blocks commit when debate is initiated"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    create_test_debate "$tmp" "test-pair-20260317T000004" "initiated" "null" 0 >/dev/null

    local exit_code=0
    (cd "$tmp" && bash scripts/check-consensus.sh) >/dev/null 2>&1 || exit_code=$?

    if [ "$exit_code" -ne 0 ]; then
        pass "Consensus: initiated debate blocks commit (exit $exit_code)"
    else
        fail "Consensus: initiated debate should block commit but got exit 0"
    fi
}

test_consensus_detection_blocks_escalated() {
    section "Test: check-consensus.sh blocks commit when debate is escalated (no verdict)"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    create_test_debate "$tmp" "test-pair-20260317T000005" "escalated" "null" 2 >/dev/null

    local exit_code=0
    (cd "$tmp" && bash scripts/check-consensus.sh) >/dev/null 2>&1 || exit_code=$?

    if [ "$exit_code" -ne 0 ]; then
        pass "Consensus: escalated debate (no verdict) blocks commit (exit $exit_code)"
    else
        fail "Consensus: escalated debate should block commit but got exit 0"
    fi
}

test_consensus_allows_no_ratchet_dir() {
    section "Test: check-consensus.sh allows commit when no .ratchet directory"

    local tmp
    tmp="$(setup_tempdir)"

    local exit_code=0
    (cd "$tmp" && bash "$PROJECT_ROOT/scripts/check-consensus.sh") >/dev/null 2>&1 || exit_code=$?

    if [ "$exit_code" -eq 0 ]; then
        pass "Consensus: no .ratchet directory allows commit (exit 0)"
    else
        fail "Consensus: no .ratchet directory should allow commit but got exit $exit_code"
    fi
}

test_cache_update_creates_cache_file() {
    section "Test: cache-update.sh creates cache.json"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    # Create a test file to hash
    echo "test content" > "$tmp/test.sh"

    # Run cache update
    (cd "$tmp" && bash scripts/cache-update.sh "test-pair" "test.sh" "test-debate-id") >/dev/null 2>&1

    assert_file_exists "$tmp/.ratchet/cache.json" "Cache: cache.json created"
    assert_file_contains "$tmp/.ratchet/cache.json" '"test-pair"' "Cache: contains pair name"
    assert_file_contains "$tmp/.ratchet/cache.json" '"hash"' "Cache: contains hash field"
    assert_file_contains "$tmp/.ratchet/cache.json" '"test-debate-id"' "Cache: contains debate ID"
}

test_cache_check_detects_unchanged() {
    section "Test: cache-check.sh detects unchanged files"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    # Create a test file
    echo "stable content" > "$tmp/test.sh"

    # Update cache
    (cd "$tmp" && bash scripts/cache-update.sh "test-pair" "test.sh") >/dev/null 2>&1

    # Check should return 0 (unchanged)
    local exit_code=0
    (cd "$tmp" && bash scripts/cache-check.sh "test-pair" "test.sh") >/dev/null 2>&1 || exit_code=$?

    if [ "$exit_code" -eq 0 ]; then
        pass "Cache check: unchanged files return exit 0"
    else
        fail "Cache check: unchanged files should return 0 but got $exit_code"
    fi
}

test_cache_check_detects_changed() {
    section "Test: cache-check.sh detects changed files"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    # Create and cache a file
    echo "original content" > "$tmp/test.sh"
    (cd "$tmp" && bash scripts/cache-update.sh "test-pair" "test.sh") >/dev/null 2>&1

    # Modify the file
    echo "modified content" > "$tmp/test.sh"

    # Check should return 1 (changed)
    local exit_code=0
    (cd "$tmp" && bash scripts/cache-check.sh "test-pair" "test.sh") >/dev/null 2>&1 || exit_code=$?

    if [ "$exit_code" -eq 1 ]; then
        pass "Cache check: changed files return exit 1"
    else
        fail "Cache check: changed files should return 1 but got $exit_code"
    fi
}

test_cache_check_no_cache_triggers_debate() {
    section "Test: cache-check.sh returns 1 when no cache exists"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    echo "content" > "$tmp/test.sh"

    local exit_code=0
    (cd "$tmp" && bash scripts/cache-check.sh "test-pair" "test.sh") >/dev/null 2>&1 || exit_code=$?

    if [ "$exit_code" -eq 1 ]; then
        pass "Cache check: no cache file returns exit 1 (triggers debate)"
    else
        fail "Cache check: no cache file should return 1 but got $exit_code"
    fi
}

test_guard_execution() {
    section "Test: run-guards.sh executes and records results"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    # Run a passing guard
    (cd "$tmp" && bash scripts/run-guards.sh "test-ms" "review" "echo-guard" "echo hello" "true") >/dev/null 2>&1

    assert_file_exists "$tmp/.ratchet/guards/test-ms/review/echo-guard.json" \
        "Guard: result JSON created"
    assert_file_contains "$tmp/.ratchet/guards/test-ms/review/echo-guard.json" '"exit_code": 0' \
        "Guard: exit_code is 0 for passing guard"
    assert_file_contains "$tmp/.ratchet/guards/test-ms/review/echo-guard.json" '"guard": "echo-guard"' \
        "Guard: guard name recorded"
}

test_guard_blocking_failure() {
    section "Test: run-guards.sh blocking guard failure"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    # Run a failing blocking guard
    local exit_code=0
    (cd "$tmp" && bash scripts/run-guards.sh "test-ms" "review" "fail-guard" "false" "true") >/dev/null 2>&1 || exit_code=$?

    if [ "$exit_code" -ne 0 ]; then
        pass "Guard: blocking failure exits non-zero"
    else
        fail "Guard: blocking failure should exit non-zero but got 0"
    fi

    assert_file_exists "$tmp/.ratchet/guards/test-ms/review/fail-guard.json" \
        "Guard: result JSON created even on failure"
}

test_guard_nonblocking_failure() {
    section "Test: run-guards.sh non-blocking guard failure"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    # Run a failing non-blocking guard
    local exit_code=0
    (cd "$tmp" && bash scripts/run-guards.sh "test-ms" "review" "advisory-guard" "false" "false") >/dev/null 2>&1 || exit_code=$?

    if [ "$exit_code" -eq 0 ]; then
        pass "Guard: non-blocking failure exits 0"
    else
        fail "Guard: non-blocking failure should exit 0 but got $exit_code"
    fi
}

test_multiple_debates_consensus() {
    section "Test: Multiple debates — mixed statuses"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    # One resolved, one initiated
    create_test_debate "$tmp" "pair-a-resolved" "consensus" "ACCEPT" 1 >/dev/null
    create_test_debate "$tmp" "pair-b-initiated" "initiated" "null" 0 >/dev/null

    local exit_code=0
    (cd "$tmp" && bash scripts/check-consensus.sh) >/dev/null 2>&1 || exit_code=$?

    if [ "$exit_code" -ne 0 ]; then
        pass "Multiple debates: blocks when any debate is initiated"
    else
        fail "Multiple debates: should block when any debate is initiated"
    fi
}

test_debate_id_format() {
    section "Test: Debate ID format convention"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    # Verify debate ID follows pair-name-timestamp pattern
    local debate_id="test-pair-20260317T120000"
    local debate_dir
    debate_dir=$(create_test_debate "$tmp" "$debate_id" "consensus" "ACCEPT" 1)

    # Check that the directory name matches the ID in meta.json
    local dir_name
    dir_name=$(basename "$debate_dir")
    assert_json_field "$debate_dir/meta.json" "id" "$dir_name" "Debate ID: matches directory name"

    # Verify ID contains pair name
    if echo "$debate_id" | grep -q "test-pair"; then
        pass "Debate ID: contains pair name"
    else
        fail "Debate ID: should contain pair name"
    fi

    # Verify ID contains timestamp pattern
    if echo "$debate_id" | grep -qE '[0-9]{8}T[0-9]{6}'; then
        pass "Debate ID: contains timestamp (YYYYMMDDTHHMMSS)"
    else
        fail "Debate ID: should contain timestamp pattern"
    fi
}

test_verdict_types() {
    section "Test: All verdict types in meta.json"

    local tmp
    tmp="$(setup_tempdir)"
    setup_test_project "$tmp"

    # Test each verdict type
    for verdict in ACCEPT CONDITIONAL_ACCEPT TRIVIAL_ACCEPT REJECT; do
        local status="consensus"
        if [ "$verdict" = "REJECT" ]; then status="escalated"; fi

        local debate_id="test-pair-verdict-${verdict}"
        create_test_debate "$tmp" "$debate_id" "$status" "$verdict" 1 >/dev/null

        assert_json_field "$tmp/.ratchet/debates/$debate_id/meta.json" "verdict" "$verdict" \
            "Verdict type: $verdict recorded correctly"
    done
}

# --- Run all tests ---

main() {
    echo "=========================================="
    echo "  Ratchet Debate Flow Test Suite"
    echo "=========================================="

    test_debate_directory_creation
    test_meta_json_structure
    test_round_file_generation
    test_consensus_detection_allows_commit
    test_consensus_detection_blocks_initiated
    test_consensus_detection_blocks_escalated
    test_consensus_allows_no_ratchet_dir
    test_cache_update_creates_cache_file
    test_cache_check_detects_unchanged
    test_cache_check_detects_changed
    test_cache_check_no_cache_triggers_debate
    test_guard_execution
    test_guard_blocking_failure
    test_guard_nonblocking_failure
    test_multiple_debates_consensus
    test_debate_id_format
    test_verdict_types

    echo ""
    echo "=========================================="
    echo "  Results: $PASS passed, $FAIL failed (of $TESTS_RUN)"
    echo "=========================================="

    if [ "$FAIL" -gt 0 ]; then
        exit 1
    fi
}

main "$@"
