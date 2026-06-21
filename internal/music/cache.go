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

// Set stores a value in the cache with the given TTL.
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	c.data[key] = entry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// Cache key helpers.
const (
	TTLAudio  = 24 * time.Hour
	TTLLyrics = 24 * time.Hour
)

func AudioKey(songID int64) string  { return "audio:" + itoa(songID) }
func LyricsKey(songID int64) string { return "lyrics:" + itoa(songID) }

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
