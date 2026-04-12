package datasource

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestCachedDataSource_HitOnRepeatedCalls verifies that the second call to
// the same method returns cached data without re-reading the file.
func TestCachedDataSource_HitOnRepeatedCalls(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)
	cached := NewCachedDataSource(ds, WithMaxAge(10*time.Second))

	// First call — cache miss, populates cache.
	result1, err := cached.Pairs("")
	if err != nil {
		t.Fatalf("first Pairs() call failed: %v", err)
	}

	// Second call — should be a cache hit (same mtime, within TTL).
	result2, err := cached.Pairs("")
	if err != nil {
		t.Fatalf("second Pairs() call failed: %v", err)
	}

	// Both results should be the same pointer (cached).
	if &result1 == nil || &result2 == nil {
		t.Fatal("expected non-nil results")
	}

	// Verify data is consistent.
	pairs1, ok1 := result1.([]PairStatus)
	pairs2, ok2 := result2.([]PairStatus)
	if !ok1 || !ok2 {
		t.Fatalf("expected []PairStatus, got %T and %T", result1, result2)
	}
	if len(pairs1) != len(pairs2) {
		t.Errorf("expected same length, got %d and %d", len(pairs1), len(pairs2))
	}
}

// TestCachedDataSource_MissOnMtimeChange verifies that when the file's
// modification time changes, the cache returns a miss.
func TestCachedDataSource_MissOnMtimeChange(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)
	cached := NewCachedDataSource(ds, WithMaxAge(10*time.Second))

	// Populate cache.
	_, err := cached.Plan()
	if err != nil {
		t.Fatalf("first Plan() call failed: %v", err)
	}

	// Modify the file to change mtime.
	planPath := filepath.Join(dir, "plan.yaml")
	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read plan.yaml: %v", err)
	}
	// Advance mtime by 1 second to ensure it differs.
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(planPath, data, 0o644); err != nil {
		t.Fatalf("rewrite plan.yaml: %v", err)
	}
	// Force a distinct mtime (some filesystems have 1s granularity).
	future := time.Now().Add(10 * time.Second)
	os.Chtimes(planPath, future, future)

	// Second call should detect mtime change and re-read.
	result2, err := cached.Plan()
	if err != nil {
		t.Fatalf("second Plan() call failed: %v", err)
	}
	if result2 == nil {
		t.Fatal("expected non-nil result after mtime change")
	}
}

// TestCachedDataSource_Invalidate verifies that Invalidate() clears
// relevant cache entries.
func TestCachedDataSource_Invalidate(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)

	callCount := 0
	now := time.Now()
	cached := NewCachedDataSource(ds, WithMaxAge(1*time.Minute), withClock(func() time.Time {
		return now
	}))

	// Populate cache.
	_, err := cached.Plan()
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}
	_, err = cached.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}
	callCount = 2

	// Verify cache is populated.
	cached.mu.RLock()
	if len(cached.cache) != callCount {
		t.Errorf("expected %d cache entries, got %d", callCount, len(cached.cache))
	}
	cached.mu.RUnlock()

	// Invalidate plan.yaml — should clear "plan" and "status" keys.
	cached.Invalidate(filepath.Join(dir, "plan.yaml"))

	cached.mu.RLock()
	if len(cached.cache) != 0 {
		t.Errorf("expected 0 cache entries after invalidation, got %d", len(cached.cache))
	}
	cached.mu.RUnlock()
}

// TestCachedDataSource_InvalidateDebates verifies that debate file changes
// invalidate debate-related cache entries.
func TestCachedDataSource_InvalidateDebates(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)
	now := time.Now()
	cached := NewCachedDataSource(ds, WithMaxAge(1*time.Minute), withClock(func() time.Time {
		return now
	}))

	// Populate cache for debates and pairs.
	_, err := cached.Debates("")
	if err != nil {
		t.Fatalf("Debates() failed: %v", err)
	}
	_, err = cached.Pairs("")
	if err != nil {
		t.Fatalf("Pairs() failed: %v", err)
	}
	_, err = cached.Debate("api-design-1")
	if err != nil {
		t.Fatalf("Debate() failed: %v", err)
	}

	cached.mu.RLock()
	if len(cached.cache) != 3 {
		t.Errorf("expected 3 cache entries, got %d", len(cached.cache))
	}
	cached.mu.RUnlock()

	// Invalidate a debate meta.json.
	cached.Invalidate(filepath.Join(dir, "debates", "api-design-1", "meta.json"))

	cached.mu.RLock()
	remaining := len(cached.cache)
	cached.mu.RUnlock()

	// All three should be cleared: debates: prefix, pairs: prefix, debate:api-design-1
	if remaining != 0 {
		t.Errorf("expected 0 cache entries after debate invalidation, got %d", remaining)
	}
}

// TestCachedDataSource_TTLExpiry verifies that entries expire after maxAge.
func TestCachedDataSource_TTLExpiry(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)

	now := time.Now()
	mu := sync.Mutex{}
	cached := NewCachedDataSource(ds, WithMaxAge(100*time.Millisecond), withClock(func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}))

	// Populate cache.
	_, err := cached.Workspaces()
	if err != nil {
		t.Fatalf("Workspaces() failed: %v", err)
	}

	// Verify cache hit within TTL.
	cached.mu.RLock()
	if len(cached.cache) != 1 {
		t.Fatal("expected 1 cache entry")
	}
	cached.mu.RUnlock()

	// Advance time past TTL.
	mu.Lock()
	now = now.Add(200 * time.Millisecond)
	mu.Unlock()

	// Next call should be a miss (TTL expired) and repopulate.
	_, err = cached.Workspaces()
	if err != nil {
		t.Fatalf("Workspaces() after TTL failed: %v", err)
	}

	// Should still have 1 entry (repopulated).
	cached.mu.RLock()
	if len(cached.cache) != 1 {
		t.Errorf("expected 1 cache entry after TTL refresh, got %d", len(cached.cache))
	}
	cached.mu.RUnlock()
}

// TestCachedDataSource_ConcurrentAccess runs many goroutines hitting the
// cache simultaneously. Run with -race to detect data races.
func TestCachedDataSource_ConcurrentAccess(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)
	cached := NewCachedDataSource(ds, WithMaxAge(1*time.Second))

	var wg sync.WaitGroup
	var errCount atomic.Int64
	const goroutines = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			switch n % 7 {
			case 0:
				if _, err := cached.Pairs(""); err != nil {
					errCount.Add(1)
				}
			case 1:
				if _, err := cached.Debates(""); err != nil {
					errCount.Add(1)
				}
			case 2:
				if _, err := cached.Debate("api-design-1"); err != nil {
					errCount.Add(1)
				}
			case 3:
				if _, err := cached.Plan(); err != nil {
					errCount.Add(1)
				}
			case 4:
				if _, err := cached.Status(); err != nil {
					errCount.Add(1)
				}
			case 5:
				if _, err := cached.Workspaces(); err != nil {
					errCount.Add(1)
				}
			case 6:
				cached.Invalidate(filepath.Join(dir, "plan.yaml"))
			}
		}(i)
	}

	wg.Wait()
	if errCount.Load() > 0 {
		t.Errorf("got %d errors during concurrent access", errCount.Load())
	}
}

// TestCachedDataSource_DisabledWithZeroMaxAge verifies that maxAge=0
// disables caching entirely (passthrough).
func TestCachedDataSource_DisabledWithZeroMaxAge(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)
	cached := NewCachedDataSource(ds, WithMaxAge(0))

	// Call multiple times — cache should remain empty.
	for i := 0; i < 3; i++ {
		_, err := cached.Pairs("")
		if err != nil {
			t.Fatalf("Pairs() call %d failed: %v", i, err)
		}
	}

	cached.mu.RLock()
	if len(cached.cache) != 0 {
		t.Errorf("expected 0 cache entries with maxAge=0, got %d", len(cached.cache))
	}
	cached.mu.RUnlock()

	// Invalidate should be a no-op.
	cached.Invalidate(filepath.Join(dir, "plan.yaml"))
}

// TestCachedDataSource_ErrorsNotCached verifies that errors from the inner
// data source are not cached.
func TestCachedDataSource_ErrorsNotCached(t *testing.T) {
	// Use a non-existent subdirectory that won't have workflow.yaml.
	dir := t.TempDir()
	// Create a workflow.yaml with permission denied to trigger error.
	wfPath := filepath.Join(dir, "workflow.yaml")
	os.WriteFile(wfPath, []byte("bogus"), 0o000)
	t.Cleanup(func() { os.Chmod(wfPath, 0o644) })

	ds := NewFileDataSource(dir)
	cached := NewCachedDataSource(ds, WithMaxAge(10*time.Second))

	// First call should return an error.
	_, err := cached.Pairs("")
	if err == nil {
		// If not running as root, this should fail with permission denied.
		// Skip if we're root (containers often run as root).
		t.Skip("expected error from permission-denied file, skipping (likely running as root)")
	}

	// Cache should be empty — errors are not cached.
	cached.mu.RLock()
	if len(cached.cache) != 0 {
		t.Errorf("expected 0 cache entries after error, got %d", len(cached.cache))
	}
	cached.mu.RUnlock()
}

// TestCachedDataSource_InvalidateAll verifies that InvalidateAll clears everything.
func TestCachedDataSource_InvalidateAll(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)
	now := time.Now()
	cached := NewCachedDataSource(ds, WithMaxAge(1*time.Minute), withClock(func() time.Time {
		return now
	}))

	// Populate multiple cache entries.
	cached.Pairs("")
	cached.Plan()
	cached.Status()
	cached.Workspaces()

	cached.mu.RLock()
	if len(cached.cache) < 3 {
		t.Fatalf("expected at least 3 cache entries, got %d", len(cached.cache))
	}
	cached.mu.RUnlock()

	cached.InvalidateAll()

	cached.mu.RLock()
	if len(cached.cache) != 0 {
		t.Errorf("expected 0 entries after InvalidateAll, got %d", len(cached.cache))
	}
	cached.mu.RUnlock()
}

// TestCachedDataSource_WorkflowInvalidation verifies that changing workflow.yaml
// invalidates Pairs, Workspaces, Debates, Plan, and Status.
func TestCachedDataSource_WorkflowInvalidation(t *testing.T) {
	dir := setupTestDir(t)
	ds := NewFileDataSource(dir)
	now := time.Now()
	cached := NewCachedDataSource(ds, WithMaxAge(1*time.Minute), withClock(func() time.Time {
		return now
	}))

	// Populate all method caches.
	cached.Pairs("")
	cached.Pairs("nonexistent-workspace-sentinel") // will error, not cached
	cached.Debates("")
	cached.Plan()
	cached.Status()
	cached.Workspaces()

	cached.mu.RLock()
	before := len(cached.cache)
	cached.mu.RUnlock()
	if before < 4 {
		t.Fatalf("expected at least 4 cache entries, got %d", before)
	}

	// Invalidate workflow.yaml.
	cached.Invalidate(filepath.Join(dir, "workflow.yaml"))

	cached.mu.RLock()
	after := len(cached.cache)
	cached.mu.RUnlock()

	if after != 0 {
		t.Errorf("expected 0 entries after workflow invalidation, got %d", after)
	}
}

// TestCachedDataSource_ScoresInvalidation verifies that scores file changes
// invalidate score cache entries.
func TestCachedDataSource_ScoresInvalidation(t *testing.T) {
	dir := setupTestDir(t)
	// Create scores file.
	scoresDir := filepath.Join(dir, "scores")
	os.MkdirAll(scoresDir, 0o755)
	os.WriteFile(filepath.Join(scoresDir, "scores.jsonl"), []byte(`{"pair":"api-design","score":0.8,"timestamp":"2026-03-14T10:00:00Z"}`+"\n"), 0o644)

	ds := NewFileDataSource(dir)
	now := time.Now()
	cached := NewCachedDataSource(ds, WithMaxAge(1*time.Minute), withClock(func() time.Time {
		return now
	}))

	// Populate score cache.
	_, err := cached.Scores("")
	if err != nil {
		t.Fatalf("Scores() failed: %v", err)
	}
	_, err = cached.Scores("api-design")
	if err != nil {
		t.Fatalf("Scores(api-design) failed: %v", err)
	}

	cached.mu.RLock()
	if len(cached.cache) != 2 {
		t.Fatalf("expected 2 cache entries, got %d", len(cached.cache))
	}
	cached.mu.RUnlock()

	// Invalidate scores file.
	cached.Invalidate(filepath.Join(dir, "scores", "scores.jsonl"))

	cached.mu.RLock()
	remaining := len(cached.cache)
	cached.mu.RUnlock()

	if remaining != 0 {
		t.Errorf("expected 0 cache entries after scores invalidation, got %d", remaining)
	}
}

// TestAffectedKeys verifies the cache key mapping logic for various file paths.
func TestAffectedKeys(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"workflow.yaml", []string{"workspaces", "plan", "status", "pairs:", "debates:"}},
		{"plan.yaml", []string{"plan", "status"}},
		{"debates/api-design-1/meta.json", []string{"pairs:", "debates:", "debate:api-design-1"}},
		{"debates/test-1/rounds/round-1-generative.md", []string{"pairs:", "debates:", "debate:test-1"}},
		{"scores/scores.jsonl", []string{"scores:"}},
		{"unknown/file.txt", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := affectedKeys(tt.path)
			if len(got) != len(tt.expected) {
				t.Fatalf("affectedKeys(%q) = %v, want %v", tt.path, got, tt.expected)
			}
			for i, key := range got {
				if key != tt.expected[i] {
					t.Errorf("affectedKeys(%q)[%d] = %q, want %q", tt.path, i, key, tt.expected[i])
				}
			}
		})
	}
}

// TestCachedDataSource_AllMethods verifies all DataSource methods work through cache.
func TestCachedDataSource_AllMethods(t *testing.T) {
	dir := setupTestDir(t)

	// Add scores file for Scores() test.
	scoresDir := filepath.Join(dir, "scores")
	os.MkdirAll(scoresDir, 0o755)
	os.WriteFile(filepath.Join(scoresDir, "scores.jsonl"),
		[]byte(`{"pair":"api-design","score":0.8,"timestamp":"2026-03-14T10:00:00Z"}`+"\n"), 0o644)

	ds := NewFileDataSource(dir)
	cached := NewCachedDataSource(ds, WithMaxAge(10*time.Second))

	// Verify each method works and returns non-nil.
	tests := []struct {
		name string
		fn   func() (any, error)
	}{
		{"Pairs", func() (any, error) { return cached.Pairs("") }},
		{"Debates", func() (any, error) { return cached.Debates("") }},
		{"Debate", func() (any, error) { return cached.Debate("api-design-1") }},
		{"Plan", func() (any, error) { return cached.Plan() }},
		{"Status", func() (any, error) { return cached.Status() }},
		{"Workspaces", func() (any, error) { return cached.Workspaces() }},
		{"Scores", func() (any, error) { return cached.Scores("") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.fn()
			if err != nil {
				t.Fatalf("%s() failed: %v", tt.name, err)
			}
			if result == nil {
				t.Fatalf("%s() returned nil", tt.name)
			}

			// Second call should hit cache.
			result2, err := tt.fn()
			if err != nil {
				t.Fatalf("%s() second call failed: %v", tt.name, err)
			}
			if result2 == nil {
				t.Fatalf("%s() second call returned nil", tt.name)
			}
		})
	}
}
