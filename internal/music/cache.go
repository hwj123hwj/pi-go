package music

import (
	"sync"
	"time"
)

// Cache is a simple in-memory TTL cache for music data (audio URLs, lyrics).
type Cache struct {
	mu   sync.RWMutex
	data map[string]entry
}

type entry struct {
	value     any
	expiresAt time.Time
}

// NewCache creates a new Cache instance.
func NewCache() *Cache {
	return &Cache{
		data: make(map[string]entry),
	}
}

// Get retrieves a value from the cache. Returns nil if missing or expired.
func (c *Cache) Get(key string) any {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(e.expiresAt) {
		return nil
	}
	return e.value
}

// Delete removes a key from the cache (no-op if not present).
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.data, key)
	c.mu.Unlock()
}

// Set stores a value in the cache with the given TTL.
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	c.data[key] = entry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	// Opportunistic cleanup: purge expired entries every 128 inserts to bound memory.
	if len(c.data) > 128 && len(c.data)%128 == 0 {
		now := time.Now()
		for k, e := range c.data {
			if now.After(e.expiresAt) {
				delete(c.data, k)
			}
		}
	}
	c.mu.Unlock()
}

// Cache key helpers.
const (
	TTLAudio  = 24 * time.Hour
	TTLLyrics = 24 * time.Hour
)

// AudioKey returns a cache key for an audio URL, with source prefix.
// Example: AudioKey("netease", "12345") → "audio:netease:12345"
func AudioKey(source, rawID string) string { return "audio:" + source + ":" + rawID }

// LyricsKey returns a cache key for lyrics, with source prefix.
func LyricsKey(source, rawID string) string { return "lyrics:" + source + ":" + rawID }
