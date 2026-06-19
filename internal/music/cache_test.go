package music

import (
	"testing"
	"time"
)

func TestCacheSetGet(t *testing.T) {
	c := NewCache()

	// Miss
	if v := c.Get("key1"); v != nil {
		t.Errorf("expected nil, got %v", v)
	}

	// Set and hit
	c.Set("key1", "value1", time.Minute)
	if v := c.Get("key1"); v != "value1" {
		t.Errorf("expected 'value1', got %v", v)
	}
}

func TestCacheExpiry(t *testing.T) {
	c := NewCache()

	c.Set("key1", "value1", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	if v := c.Get("key1"); v != nil {
		t.Errorf("expected nil after expiry, got %v", v)
	}
}

func TestCacheGetOrLoad(t *testing.T) {
	c := NewCache()
	called := 0

	loader := func() (any, error) {
		called++
		return "loaded", nil
	}

	// First call: cache miss, loader called
	v, err := c.GetOrLoad("key1", time.Minute, loader)
	if err != nil {
		t.Fatal(err)
	}
	if v != "loaded" {
		t.Errorf("expected 'loaded', got %v", v)
	}
	if called != 1 {
		t.Errorf("expected loader called once, got %d", called)
	}

	// Second call: cache hit, loader not called
	v, err = c.GetOrLoad("key1", time.Minute, loader)
	if err != nil {
		t.Fatal(err)
	}
	if v != "loaded" {
		t.Errorf("expected 'loaded', got %v", v)
	}
	if called != 1 {
		t.Errorf("expected loader still called once, got %d", called)
	}
}

func TestCacheKeyHelpers(t *testing.T) {
	if k := AudioKey(12345); k != "audio:12345" {
		t.Errorf("expected 'audio:12345', got %q", k)
	}
	if k := LyricsKey(0); k != "lyrics:0" {
		t.Errorf("expected 'lyrics:0', got %q", k)
	}
	if k := AudioKey(9999999999); k != "audio:9999999999" {
		t.Errorf("expected 'audio:9999999999', got %q", k)
	}
}

func TestCacheConcurrent(t *testing.T) {
	c := NewCache()
	done := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		go func(n int) {
			key := "key"
			c.Set(key, n, time.Minute)
			_ = c.Get(key)
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}
