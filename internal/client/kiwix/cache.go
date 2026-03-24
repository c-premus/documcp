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
func (c *cache) get(key string) (any, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		defer c.mu.Unlock()
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
