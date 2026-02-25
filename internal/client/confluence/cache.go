package confluence

import (
	"sync"
	"time"
)

// cacheEntry holds a cached value with its expiration time.
type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// cache is a simple in-memory TTL cache protected by a read-write mutex.
type cache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

// newCache creates an empty cache.
func newCache() *cache {
	return &cache{
		entries: make(map[string]cacheEntry),
	}
}

// get retrieves a value from the cache. It returns nil and false if the key
// is missing or expired.
func (c *cache) get(key string) (any, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}
	return entry.value, true
}

// set stores a value in the cache with the given TTL.
func (c *cache) set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	c.entries[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}
