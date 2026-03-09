package confluence

import (
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	t.Parallel()
	c := newCache()
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
	if len(c.entries) != 0 {
		t.Fatal("expected empty entries")
	}
}

func TestCache_SetAndGet(t *testing.T) {
	t.Parallel()
	c := newCache()
	c.set("key1", "value1", time.Hour)

	got, ok := c.get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != "value1" {
		t.Errorf("got %v, want value1", got)
	}
}

func TestCache_GetMissing(t *testing.T) {
	t.Parallel()
	c := newCache()
	got, ok := c.get("nonexistent")
	if ok {
		t.Fatal("expected cache miss")
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestCache_GetExpired(t *testing.T) {
	t.Parallel()
	c := newCache()
	c.set("key1", "value1", time.Millisecond)

	// Wait for expiration.
	time.Sleep(5 * time.Millisecond)

	got, ok := c.get("key1")
	if ok {
		t.Fatal("expected cache miss for expired entry")
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}

	// Entry should be removed from map.
	c.mu.RLock()
	_, exists := c.entries["key1"]
	c.mu.RUnlock()
	if exists {
		t.Error("expired entry should be removed from map")
	}
}

func TestCache_Overwrite(t *testing.T) {
	t.Parallel()
	c := newCache()
	c.set("key", "first", time.Hour)
	c.set("key", "second", time.Hour)

	got, ok := c.get("key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != "second" {
		t.Errorf("got %v, want second", got)
	}
}

func TestCache_DifferentValueTypes(t *testing.T) {
	t.Parallel()
	c := newCache()

	c.set("int", 42, time.Hour)
	c.set("slice", []string{"a", "b"}, time.Hour)

	got, ok := c.get("int")
	if !ok || got != 42 {
		t.Errorf("int: got %v, want 42", got)
	}

	got2, ok := c.get("slice")
	if !ok {
		t.Fatal("expected cache hit for slice")
	}
	s, sOk := got2.([]string)
	if !sOk || len(s) != 2 {
		t.Errorf("slice: got %v, want [a b]", got2)
	}
}
