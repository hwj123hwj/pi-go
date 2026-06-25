package profile

// ────────────────────────────────────────────────────────────────────────────
//  Unified User Profile Store
//
//  A cross-agent, persistent user profile that acts as a "condensed second
//  brain". Any agent (coding, music, kb) can record facts about the user,
//  and every agent sees the same fixed-size summary in their system prompt.
//
//  DESIGN PRINCIPLES (borrowed from OpenViking's context philosophy):
//  1. Storage/injection decoupled — facts accumulate, summary is fixed-size
//  2. Category-based organization — coding, music, general, etc.
//  3. Last-write-wins per key — agents can update facts about the same topic
//  4. Max items per category — prevents unbounded growth in any single domain
//
//  The summary is ALWAYS ~80 tokens regardless of how many facts are stored:
//
//	## 用户画像
//	- 开发：偏好 Go、用 macOS、zsh 终端
//	- 音乐：常听周杰伦、林俊杰（累计 142 首）
//	- 通用：在北京，用中文交流
// ────────────────────────────────────────────────────────────────────────────

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// CategoryCoding  holds development preferences (languages, tools, OS, etc.)
	CategoryCoding = "coding"
	// CategoryMusic holds listening preferences (top artists, genres, etc.)
	CategoryMusic = "music"
	// CategoryGeneral holds general facts (location, language, timezone, etc.)
	CategoryGeneral = "general"

	maxPerCategory = 10 // max facts per category; oldest evicted
)

// Fact is a single piece of information about the user.
type Fact struct {
	Key      string    `json:"key"`      // Unique within category, e.g. "language", "artist:周杰伦"
	Value    string    `json:"value"`    // e.g. "Go", "playcount:42"
	Source   string    `json:"source"`   // Which agent recorded this, e.g. "music-agent"
	Updated  time.Time `json:"updated"`
}

// Store is a persistent, thread-safe user profile.
type Store struct {
	mu       sync.Mutex
	filePath string
	// facts is organized as category → key → Fact
	facts map[string]map[string]Fact
}

// NewStore creates a profile store backed by the given file path.
func NewStore(filePath string) *Store {
	s := &Store{
		filePath: filePath,
		facts:    make(map[string]map[string]Fact),
	}
	s.load()
	return s
}

// Record upserts a fact into the profile.
// If a fact with the same category+key exists, it is updated.
func (s *Store) Record(category, key, value, source string) {
	if category == "" || key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cat := s.getOrCreateCategory(category)
	cat[key] = Fact{
		Key:     key,
		Value:   value,
		Source:  source,
		Updated: time.Now(),
	}

	// Evict oldest if over limit
	if len(cat) > maxPerCategory {
		s.evictOldest(category)
	}

	_ = s.save()
}

// RecordBatch records multiple facts atomically.
func (s *Store) RecordBatch(category, source string, items map[string]string) {
	if category == "" || len(items) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cat := s.getOrCreateCategory(category)
	now := time.Now()
	for key, value := range items {
		cat[key] = Fact{
			Key:     key,
			Value:   value,
			Source:  source,
			Updated: now,
		}
	}
	if len(cat) > maxPerCategory {
		s.evictOldest(category)
	}
	_ = s.save()
}

// Remove deletes a fact from the profile.
func (s *Store) Remove(category, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cat, ok := s.facts[category]; ok {
		delete(cat, key)
		_ = s.save()
	}
}

// GetFacts returns all facts in a category.
func (s *Store) GetFacts(category string) []Fact {
	s.mu.Lock()
	defer s.mu.Unlock()
	cat, ok := s.facts[category]
	if !ok {
		return nil
	}
	result := make([]Fact, 0, len(cat))
	for _, f := range cat {
		result = append(result, f)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Updated.After(result[j].Updated)
	})
	return result
}

// Summary returns a compact, FIXED-SIZE string for system prompt injection.
// This is the "condensed second brain" — always ~80 tokens, never grows.
func (s *Store) Summary() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.facts) == 0 || s.allEmpty() {
		return ""
	}

	var sections []string

	// Coding section
	if coding := s.formatCategory(CategoryCoding, "开发"); coding != "" {
		sections = append(sections, coding)
	}
	// Music section
	if music := s.formatCategory(CategoryMusic, "音乐"); music != "" {
		sections = append(sections, music)
	}
	// General section
	if general := s.formatCategory(CategoryGeneral, "通用"); general != "" {
		sections = append(sections, general)
	}

	if len(sections) == 0 {
		return ""
	}

	return "## 用户画像\n" + strings.Join(sections, "\n")
}

// ── Internal helpers ────────────────────────────────────────────────────────

func (s *Store) getOrCreateCategory(category string) map[string]Fact {
	cat, ok := s.facts[category]
	if !ok {
		cat = make(map[string]Fact)
		s.facts[category] = cat
	}
	return cat
}

func (s *Store) evictOldest(category string) {
	cat := s.facts[category]
	if len(cat) <= maxPerCategory {
		return
	}
	// Find and remove the oldest entries
	type kv struct {
		key string
		t   time.Time
	}
	entries := make([]kv, 0, len(cat))
	for k, f := range cat {
		entries = append(entries, kv{k, f.Updated})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].t.Before(entries[j].t)
	})
	// Remove oldest until we're at capacity
	toRemove := len(entries) - maxPerCategory
	for i := 0; i < toRemove; i++ {
		delete(cat, entries[i].key)
	}
}

func (s *Store) allEmpty() bool {
	for _, cat := range s.facts {
		if len(cat) > 0 {
			return false
		}
	}
	return true
}

// formatCategory formats a category's facts into a single summary line.
// Music artist facts (key prefix "artist:") get special formatting.
func (s *Store) formatCategory(category, label string) string {
	cat, ok := s.facts[category]
	if !ok || len(cat) == 0 {
		return ""
	}

	// For music category, aggregate artist facts specially
	if category == CategoryMusic {
		return s.formatMusicCategory(cat)
	}

	// Generic formatting: collect all values, join with "、"
	var facts []Fact
	for _, f := range cat {
		facts = append(facts, f)
	}
	sort.Slice(facts, func(i, j int) bool {
		return facts[i].Updated.After(facts[j].Updated)
	})

	values := make([]string, 0, len(facts))
	for _, f := range facts {
		if f.Value != "" {
			values = append(values, f.Value)
		}
	}
	if len(values) == 0 {
		return ""
	}

	return fmt.Sprintf("- %s：%s", label, strings.Join(values, "、"))
}

// formatMusicCategory formats music facts: top artists + total play count.
func (s *Store) formatMusicCategory(cat map[string]Fact) string {
	var parts []string

	// Total plays
	if total, ok := cat["total_plays"]; ok {
		parts = append(parts, fmt.Sprintf("累计 %s 首", total.Value))
	}

	// Top artists (key prefix "artist:")
	type artistEntry struct {
		name  string
		count int
	}
	var artists []artistEntry
	for k, f := range cat {
		if strings.HasPrefix(k, "artist:") {
			name := strings.TrimPrefix(k, "artist:")
			// Value is "playcount:N" or just a count
			count := 0
			if v, err := parseIntSafe(f.Value); err == nil {
				count = v
			}
			artists = append(artists, artistEntry{name, count})
		}
	}
	sort.Slice(artists, func(i, j int) bool {
		return artists[i].count > artists[j].count
	})

	// Top 5 artists
	topN := 5
	if len(artists) < topN {
		topN = len(artists)
	}
	if topN > 0 {
		names := make([]string, topN)
		for i := 0; i < topN; i++ {
			names[i] = artists[i].name
		}
		parts = append(parts, "常听："+strings.Join(names, "、"))
	}

	if len(parts) == 0 {
		return ""
	}
	return "- 音乐：" + strings.Join(parts, "，")
}

func parseIntSafe(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// ── Persistence ─────────────────────────────────────────────────────────────

type diskFormat struct {
	Facts map[string]map[string]Fact `json:"facts"`
}

func (s *Store) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return // file doesn't exist yet
	}
	var df diskFormat
	if err := json.Unmarshal(data, &df); err != nil {
		return // corrupt — start fresh
	}
	s.facts = df.Facts
	if s.facts == nil {
		s.facts = make(map[string]map[string]Fact)
	}
}

func (s *Store) save() error {
	df := diskFormat{Facts: s.facts}
	data, err := json.MarshalIndent(df, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.filePath); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	return os.WriteFile(s.filePath, data, 0o644)
}
