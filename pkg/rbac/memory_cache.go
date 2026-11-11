package rbac

import (
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	permissions []string
	expiresAt   time.Time
}

type memoryCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]cacheEntry
}

// NewMemoryCache creates a new in-memory role permission cache with the given TTL.
// If ttl <= 0, a default of 5 minutes is used.
func NewMemoryCache(ttl time.Duration) PermissionCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &memoryCache{
		ttl:     ttl,
		entries: make(map[string]cacheEntry),
	}
}

func normalizeRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func (c *memoryCache) Get(roleName string) ([]string, bool) {
	key := normalizeRole(roleName)
	if key == "" {
		return nil, false
	}

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		// Recheck in case another goroutine updated it
		if cur, ok := c.entries[key]; ok && time.Now().After(cur.expiresAt) {
			delete(c.entries, key)
		}
		c.mu.Unlock()
		return nil, false
	}

	// Return defensive copy
	cp := make([]string, len(entry.permissions))
	copy(cp, entry.permissions)
	return cp, true
}

func (c *memoryCache) Set(roleName string, permissions []string) {
	key := normalizeRole(roleName)
	if key == "" {
		return
	}

	cp := make([]string, len(permissions))
	copy(cp, permissions)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{
		permissions: cp,
		expiresAt:   time.Now().Add(c.ttl),
	}
}

func (c *memoryCache) Invalidate(roleName string) {
	key := normalizeRole(roleName)
	if key == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

func (c *memoryCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]cacheEntry)
}
