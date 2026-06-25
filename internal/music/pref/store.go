package pref

// ────────────────────────────────────────────────────────────────────────────
//  Preference store: persistent user listening history + aggregated profile.
//
//  DESIGN PRINCIPLE: Storage and injection are decoupled.
//  - Storage grows linearly but is capped at maxHistory records (ring buffer).
//  - Injection (Summary) is ALWAYS a fixed-size string (~40 tokens) regardless
//    of how many songs the user has played — it never inflates the system prompt.
//
//  The store is thread-safe (single mutex protects the in-memory state + file I/O).
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

const maxHistory = 500 // ring buffer cap; oldest entries are evicted

// PlayRecord is a single play event.
type PlayRecord struct {
	SongID   string    `json:"song_id"` // Composite ID: "bilibili:BV1xx"
	Name     string    `json:"name"`
	Artist   string    `json:"artist"`
	Source   string    `json:"source"` // "bilibili", "netease"
	PlayedAt time.Time `json:"played_at"`
}

// aggregates holds derived statistics from the play history.
// Recomputed on each Record() call — cheap since history is capped at 500.
type aggregates struct {
	TotalPlays   int            `json:"total_plays"`
	ArtistCounts map[string]int `json:"artist_counts"`
	SourceCounts map[string]int `json:"source_counts"`
	TopSongs     map[string]int `json:"top_songs"` // song_name → play count
}

// Store is a persistent, thread-safe user music preference store.
type Store struct {
	mu            sync.Mutex
	filePath      string
	history       []PlayRecord
	agg           aggregates
	profileSyncer profileSyncer // optional: syncs to unified profile
}

// NewStore creates a store backed by the given file path.
// If the file exists, it is loaded; otherwise an empty store is created.
func NewStore(filePath string) *Store {
	s := &Store{
		filePath: filePath,
		agg: aggregates{
			ArtistCounts: make(map[string]int),
			SourceCounts: make(map[string]int),
			TopSongs:     make(map[string]int),
		},
	}
	s.load()
	return s
}

// Record logs a play event, updates aggregates, and persists to disk.
// This is called by PlayTool after a successful play.
func (s *Store) Record(songID, name, artist, source string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := PlayRecord{
		SongID:   songID,
		Name:     name,
		Artist:   artist,
		Source:   source,
		PlayedAt: time.Now(),
	}

	s.history = append(s.history, rec)
	// Ring buffer: evict oldest if over cap
	if len(s.history) > maxHistory {
		s.history = s.history[len(s.history)-maxHistory:]
	}

	s.recomputeAggregates()
	_ = s.save()

	// Sync to unified profile (if connected)
	s.syncToProfileUnlocked()
}

// syncToProfileUnlocked exports music stats to the unified profile without locking.
// Called internally from Record() which already holds the lock.
//
// Uses RecordBatch for a SINGLE disk write instead of 16 separate writes.
func (s *Store) syncToProfileUnlocked() {
	if s.profileSyncer == nil {
		return
	}

	items := make(map[string]string, 16)
	items["total_plays"] = fmt.Sprintf("%d", s.agg.TotalPlays)

	// Only sync top 15 artists — enough for the profile's top-5 summary,
	// and avoids flooding the profile store with hundreds of low-count entries.
	topArtists := topNWithCounts(s.agg.ArtistCounts, 15)
	for _, a := range topArtists {
		items["artist:"+a.key] = fmt.Sprintf("%d", a.count)
	}

	s.profileSyncer.RecordBatch("music", "music-agent", items)
}

// Summary returns a compact, FIXED-SIZE string for system prompt injection.
// This is Tier 1: always injected, ~40 tokens, never grows with history size.
//
// Example output:
//
//	## 用户听歌偏好
//	累计播放 142 首。常听歌手：周杰伦、林俊杰、陈奕迅、邓紫棋、Taylor Swift
func (s *Store) Summary() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.agg.TotalPlays == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## 用户听歌偏好\n")
	b.WriteString(fmt.Sprintf("累计播放 %d 首。", s.agg.TotalPlays))

	// Top 5 artists — fixed count, never grows
	topArtists := topN(s.agg.ArtistCounts, 5)
	if len(topArtists) > 0 {
		b.WriteString("常听歌手：")
		b.WriteString(strings.Join(topArtists, "、"))
	}

	return b.String()
}

// HistoryDetail returns recent play history and stats for the music_history tool.
// This is Tier 2/3: only fetched on-demand via tool call, never auto-injected.
func (s *Store) HistoryDetail(limit int) (recent []PlayRecord, topArtists, topSongs []string, totalPlays int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}
	// Return most recent first
	recent = make([]PlayRecord, limit)
	for i := 0; i < limit; i++ {
		recent[i] = s.history[len(s.history)-1-i]
	}

	topArtists = topN(s.agg.ArtistCounts, 5)
	topSongs = topN(s.agg.TopSongs, 5)
	totalPlays = s.agg.TotalPlays
	return
}

// profileSyncer is the interface to the unified profile store.
// Using an interface keeps the pref package free from importing profile.
type profileSyncer interface {
	Record(category, key, value, source string)
	RecordBatch(category, source string, items map[string]string)
}

// SetProfileSyncer attaches a unified profile syncer.
// After this is set, every Record() call will also sync to the profile.
func (s *Store) SetProfileSyncer(ps profileSyncer) {
	s.mu.Lock()
	s.profileSyncer = ps
	s.mu.Unlock()
	// Do an initial sync of existing data
	s.syncToProfile()
}

// syncToProfile exports aggregated music stats to a unified profile store.
// Thread-safe version for external callers.
func (s *Store) syncToProfile() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncToProfileUnlocked()
}

// ── Internal: aggregation + persistence ────────────────────────────────────

// recomputeAggregates rebuilds all derived stats from the history slice.
// Called after every mutation (Record). O(n) where n ≤ 500.
func (s *Store) recomputeAggregates() {
	s.agg = aggregates{
		TotalPlays:   len(s.history),
		ArtistCounts: make(map[string]int),
		SourceCounts: make(map[string]int),
		TopSongs:     make(map[string]int),
	}
	for _, r := range s.history {
		if r.Artist != "" {
			s.agg.ArtistCounts[r.Artist]++
		}
		if r.Source != "" {
			s.agg.SourceCounts[r.Source]++
		}
		if r.Name != "" {
			s.agg.TopSongs[r.Name]++
		}
	}
}

// topN returns the top N keys from a frequency map, sorted by count desc.
func topN(counts map[string]int, n int) []string {
	pairs := topNWithCounts(counts, n)
	result := make([]string, len(pairs))
	for i, p := range pairs {
		result[i] = p.key
	}
	return result
}

// kvPair is a key-count pair used by topNWithCounts.
type kvPair struct {
	key   string
	count int
}

// topNWithCounts returns the top N key-count pairs, sorted by count desc.
func topNWithCounts(counts map[string]int, n int) []kvPair {
	pairs := make([]kvPair, 0, len(counts))
	for k, v := range counts {
		pairs = append(pairs, kvPair{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].key < pairs[j].key // tie-break alphabetical for determinism
	})
	if len(pairs) > n {
		pairs = pairs[:n]
	}
	return pairs
}

// ── Persistence ─────────────────────────────────────────────────────────────

type diskFormat struct {
	History []PlayRecord `json:"history"`
}

func (s *Store) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return // file doesn't exist yet — fine
	}
	var df diskFormat
	if err := json.Unmarshal(data, &df); err != nil {
		return // corrupt — start fresh
	}
	s.history = df.History
	s.recomputeAggregates()
}

func (s *Store) save() error {
	df := diskFormat{History: s.history}
	data, err := json.MarshalIndent(df, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write: write to temp file, then rename. Prevents partial
	// writes from corrupting the history if the process crashes mid-write.
	dir := filepath.Dir(s.filePath)
	if dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.filePath)
}
