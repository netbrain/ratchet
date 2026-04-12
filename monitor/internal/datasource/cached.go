package datasource

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// cacheEntry holds a cached API response alongside its validity metadata.
type cacheEntry struct {
	data    any
	modTime time.Time // mtime of the primary file at fetch time
	fetched time.Time // wall-clock time when entry was created
}

// CachedDataSource wraps a FileDataSource with an in-memory cache.
// It implements handler.DataSource and is safe for concurrent use.
//
// Cache entries are keyed by a composite string of method name + arguments.
// Entries are invalidated when:
//   - The primary file's mtime has changed (checked on read)
//   - The configurable maxAge TTL has expired
//   - Invalidate() is called (by the pipeline on file-system events)
//
// Errors are never cached — only successful results.
// When maxAge is 0, caching is disabled (passthrough).
type CachedDataSource struct {
	inner  *FileDataSource
	mu     sync.RWMutex
	cache  map[string]*cacheEntry
	maxAge time.Duration
	now    func() time.Time // injectable clock for testing
}

// CacheOption configures the CachedDataSource.
type CacheOption func(*CachedDataSource)

// WithMaxAge sets the maximum age for cache entries.
// A zero value disables caching entirely (every call passes through).
func WithMaxAge(d time.Duration) CacheOption {
	return func(c *CachedDataSource) {
		c.maxAge = d
	}
}

// withClock overrides the time source (for testing).
func withClock(fn func() time.Time) CacheOption {
	return func(c *CachedDataSource) {
		c.now = fn
	}
}

// NewCachedDataSource wraps ds with an in-memory cache layer.
// Default maxAge is 5 seconds.
func NewCachedDataSource(ds *FileDataSource, opts ...CacheOption) *CachedDataSource {
	c := &CachedDataSource{
		inner:  ds,
		cache:  make(map[string]*cacheEntry),
		maxAge: 5 * time.Second,
		now:    time.Now,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Invalidate removes cache entries that depend on the given file path.
// It is called by the pipeline when a file-system event is observed.
func (c *CachedDataSource) Invalidate(path string) {
	if c.maxAge == 0 {
		return
	}

	// Determine which cache keys are affected by this file path.
	rel := path
	if c.inner.rootDir != "" {
		if r, err := filepath.Rel(c.inner.rootDir, path); err == nil {
			rel = r
		}
	}
	// Normalize to forward slashes for matching.
	rel = strings.ReplaceAll(rel, "\\", "/")

	keys := affectedKeys(rel)
	if len(keys) == 0 {
		return
	}
	c.invalidateKeys(keys)
}

// InvalidateAll clears the entire cache.
func (c *CachedDataSource) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*cacheEntry)
	slog.Debug("cache invalidated (all)")
}

// affectedKeys returns cache keys that should be invalidated when the given
// relative path changes.
func affectedKeys(relPath string) []string {
	var keys []string

	switch {
	case relPath == "workflow.yaml" || strings.HasSuffix(relPath, "/workflow.yaml"):
		// workflow.yaml affects Pairs, Workspaces, Debates, Plan (max_regressions)
		keys = append(keys, "workspaces")
		keys = append(keys, "plan")
		keys = append(keys, "status")
		// Pairs and Debates are keyed with workspace suffix — flush all with prefix
		keys = append(keys, "pairs:")
		keys = append(keys, "debates:")

	case relPath == "plan.yaml" || strings.HasSuffix(relPath, "/plan.yaml"):
		keys = append(keys, "plan")
		keys = append(keys, "status")

	case strings.HasPrefix(relPath, "debates/"):
		// Any debate change affects debates list, individual debate, and pairs
		// (which reads debate metadata for pair status).
		keys = append(keys, "pairs:")
		keys = append(keys, "debates:")
		// If it's a specific debate's meta.json, also invalidate the detail key.
		parts := strings.Split(relPath, "/")
		if len(parts) >= 2 {
			keys = append(keys, "debate:"+parts[1])
		}

	case strings.HasPrefix(relPath, "scores/"):
		keys = append(keys, "scores:")
	}

	return keys
}

// get retrieves a cached value if it exists, is not expired, and the primary
// file's mtime has not changed. Returns (nil, false) on miss.
func (c *CachedDataSource) get(key, primaryFile string) (any, bool) {
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()

	if !ok {
		slog.Debug("cache miss", "key", key, "reason", "not_found")
		return nil, false
	}

	// TTL check.
	if c.now().Sub(entry.fetched) > c.maxAge {
		slog.Debug("cache miss", "key", key, "reason", "ttl_expired")
		c.mu.Lock()
		delete(c.cache, key)
		c.mu.Unlock()
		return nil, false
	}

	// Mtime check (if a primary file is specified).
	if primaryFile != "" {
		info, err := os.Stat(primaryFile)
		if err != nil {
			// File gone or unreadable — treat as miss.
			slog.Debug("cache miss", "key", key, "reason", "stat_failed", "error", err)
			c.mu.Lock()
			delete(c.cache, key)
			c.mu.Unlock()
			return nil, false
		}
		if !info.ModTime().Equal(entry.modTime) {
			slog.Debug("cache miss", "key", key, "reason", "mtime_changed")
			c.mu.Lock()
			delete(c.cache, key)
			c.mu.Unlock()
			return nil, false
		}
	}

	slog.Debug("cache hit", "key", key)
	return entry.data, true
}

// put stores a value in the cache, recording the primary file's current mtime.
func (c *CachedDataSource) put(key, primaryFile string, data any) {
	var modTime time.Time
	if primaryFile != "" {
		if info, err := os.Stat(primaryFile); err == nil {
			modTime = info.ModTime()
		}
	}

	c.mu.Lock()
	c.cache[key] = &cacheEntry{
		data:    data,
		modTime: modTime,
		fetched: c.now(),
	}
	c.mu.Unlock()
}

// invalidatePrefix removes all cache entries whose key starts with the given prefix.
// Caller must hold c.mu write lock.
func (c *CachedDataSource) invalidatePrefix(prefix string) {
	for k := range c.cache {
		if strings.HasPrefix(k, prefix) {
			delete(c.cache, k)
		}
	}
}

// invalidateKeys removes cache entries matching the given keys.
// Keys ending with ":" are treated as prefix patterns.
func (c *CachedDataSource) invalidateKeys(keys []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range keys {
		// If the key ends with ":", it's a prefix pattern.
		if strings.HasSuffix(k, ":") {
			c.invalidatePrefix(k)
			slog.Debug("cache prefix invalidated", "prefix", k)
		} else {
			if _, ok := c.cache[k]; ok {
				delete(c.cache, k)
				slog.Debug("cache entry invalidated", "key", k)
			}
		}
	}
}

// primaryFile returns the absolute path to the "primary" file for mtime checking.
func (c *CachedDataSource) primaryFile(name string) string {
	return filepath.Join(c.inner.rootDir, name)
}

// --- handler.DataSource implementation ---

func (c *CachedDataSource) Pairs(workspace string) (any, error) {
	if c.maxAge == 0 {
		return c.inner.Pairs(workspace)
	}
	key := "pairs:" + workspace
	if data, ok := c.get(key, c.primaryFile("workflow.yaml")); ok {
		return data, nil
	}
	data, err := c.inner.Pairs(workspace)
	if err != nil {
		return nil, err
	}
	c.put(key, c.primaryFile("workflow.yaml"), data)
	return data, nil
}

func (c *CachedDataSource) Workspaces() (any, error) {
	if c.maxAge == 0 {
		return c.inner.Workspaces()
	}
	key := "workspaces"
	if data, ok := c.get(key, c.primaryFile("workflow.yaml")); ok {
		return data, nil
	}
	data, err := c.inner.Workspaces()
	if err != nil {
		return nil, err
	}
	c.put(key, c.primaryFile("workflow.yaml"), data)
	return data, nil
}

func (c *CachedDataSource) Debates(workspace string) (any, error) {
	if c.maxAge == 0 {
		return c.inner.Debates(workspace)
	}
	key := "debates:" + workspace
	// No single primary file — debates reads multiple files.
	// Rely on TTL + pipeline invalidation.
	if data, ok := c.get(key, ""); ok {
		return data, nil
	}
	data, err := c.inner.Debates(workspace)
	if err != nil {
		return nil, err
	}
	c.put(key, "", data)
	return data, nil
}

func (c *CachedDataSource) Debate(id string) (any, error) {
	if c.maxAge == 0 {
		return c.inner.Debate(id)
	}
	key := "debate:" + id
	primaryFile := filepath.Join(c.inner.rootDir, "debates", id, "meta.json")
	if data, ok := c.get(key, primaryFile); ok {
		return data, nil
	}
	data, err := c.inner.Debate(id)
	if err != nil {
		return nil, err
	}
	c.put(key, primaryFile, data)
	return data, nil
}

func (c *CachedDataSource) Plan() (any, error) {
	if c.maxAge == 0 {
		return c.inner.Plan()
	}
	key := "plan"
	if data, ok := c.get(key, c.primaryFile("plan.yaml")); ok {
		return data, nil
	}
	data, err := c.inner.Plan()
	if err != nil {
		return nil, err
	}
	c.put(key, c.primaryFile("plan.yaml"), data)
	return data, nil
}

func (c *CachedDataSource) Status() (any, error) {
	if c.maxAge == 0 {
		return c.inner.Status()
	}
	key := "status"
	if data, ok := c.get(key, c.primaryFile("plan.yaml")); ok {
		return data, nil
	}
	data, err := c.inner.Status()
	if err != nil {
		return nil, err
	}
	c.put(key, c.primaryFile("plan.yaml"), data)
	return data, nil
}

func (c *CachedDataSource) Scores(pair string) (any, error) {
	if c.maxAge == 0 {
		return c.inner.Scores(pair)
	}
	key := "scores:" + pair
	primaryFile := filepath.Join(c.inner.rootDir, "scores", "scores.jsonl")
	if data, ok := c.get(key, primaryFile); ok {
		return data, nil
	}
	data, err := c.inner.Scores(pair)
	if err != nil {
		return nil, err
	}
	c.put(key, primaryFile, data)
	return data, nil
}
