package kiwix

import (
	"sync"
	"time"
)

// cache is a simple in-memory key-value store with per-entry TTL expiration.
type cache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

// cacheEntry holds a cached value and its expiration time.
type cacheEntry struct {
	data      any
	expiresAt time.Time
}

// newCache creates an empty cache.
func newCache() *cache {
	return &cache{
		entries: make(map[string]cacheEntry),
	}
}

// get returns the cached value for key if it exists and has not expired.
// Uses a write lock for the entire operation to avoid a TOCTOU race between
// the expiry check and the delete — safe because this cache is low-contention.
func (c *cache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}

	return entry.data, true
}

// set stores a value in the cache with the given TTL.
func (c *cache) set(key string, data any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
}
