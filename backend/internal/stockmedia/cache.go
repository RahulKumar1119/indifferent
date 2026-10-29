package stockmedia

import (
	"sort"
	"sync"
	"time"
)

// Cache stores search results. Only successful searches are stored; errors
// never are.
//
// MemoryCache is the default. It lives inside one process, so on Lambda it is
// shared only by invocations that reuse a warm container. That falls short of
// Pixabay's rule that requests be cached for 24 hours across the whole
// service, which needs a store shared by every container; implement this
// interface over such a store for that (tracked in #194).
type Cache interface {
	Get(key string) (SearchResult, bool)
	Set(key string, res SearchResult, ttl time.Duration)
}

// DefaultCacheEntries bounds MemoryCache.
const DefaultCacheEntries = 512

type cacheEntry struct {
	res     SearchResult
	expires time.Time
}

// MemoryCache is a concurrency-safe in-process Cache with a size bound. When
// full it drops expired entries first and then the ones closest to expiring.
type MemoryCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	max     int
	now     func() time.Time
}

// NewMemoryCache returns a cache holding at most max entries (DefaultCacheEntries
// when max <= 0).
func NewMemoryCache(max int) *MemoryCache {
	if max <= 0 {
		max = DefaultCacheEntries
	}
	return &MemoryCache{entries: make(map[string]cacheEntry), max: max, now: time.Now}
}

// Get implements Cache. It returns a copy: callers may reorder or edit the
// result without changing what is cached.
func (c *MemoryCache) Get(key string) (SearchResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return SearchResult{}, false
	}
	if !c.now().Before(e.expires) {
		delete(c.entries, key)
		return SearchResult{}, false
	}
	return cloneResult(e.res), true
}

// Set implements Cache. A ttl of zero or less stores nothing.
func (c *MemoryCache) Set(key string, res SearchResult, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.max {
		c.evictLocked()
	}
	c.entries[key] = cacheEntry{res: cloneResult(res), expires: c.now().Add(ttl)}
}

// Len reports how many entries are stored, expired ones included.
func (c *MemoryCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func (c *MemoryCache) evictLocked() {
	now := c.now()
	for k, e := range c.entries {
		if !now.Before(e.expires) {
			delete(c.entries, k)
		}
	}
	if len(c.entries) < c.max {
		return
	}
	type kv struct {
		key     string
		expires time.Time
	}
	all := make([]kv, 0, len(c.entries))
	for k, e := range c.entries {
		all = append(all, kv{k, e.expires})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].expires.Before(all[j].expires) })
	// Drop the oldest tenth, at least one, so a full cache does not evict on
	// every insert.
	n := max(1, len(all)/10)
	for _, e := range all[:n] {
		delete(c.entries, e.key)
	}
}

// cloneResult copies the parts of a result a caller might modify.
func cloneResult(r SearchResult) SearchResult {
	out := r
	out.Assets = append([]Asset(nil), r.Assets...)
	if r.Quota != nil {
		q := *r.Quota
		out.Quota = &q
	}
	if r.SkipReasons != nil {
		out.SkipReasons = make(map[string]int, len(r.SkipReasons))
		for k, v := range r.SkipReasons {
			out.SkipReasons[k] = v
		}
	}
	return out
}
