package pref

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return NewStore(filepath.Join(dir, "music_pref.json"))
}

func TestRecordAndSummary(t *testing.T) {
	s := tempStore(t)

	// Initially empty
	got := s.Summary()
	if got != "" {
		t.Fatalf("empty store should return empty summary, got: %q", got)
	}

	// Record a few plays
	s.Record("bilibili:BV1", "晴天", "周杰伦", "bilibili")
	s.Record("bilibili:BV2", "七里香", "周杰伦", "bilibili")
	s.Record("netease:123", "江南", "林俊杰", "netease")

	summary := s.Summary()
	if !strings.Contains(summary, "3 首") {
		t.Errorf("summary should contain total plays '3 首', got: %s", summary)
	}
	if !strings.Contains(summary, "周杰伦") {
		t.Errorf("summary should contain top artist '周杰伦', got: %s", summary)
	}
	// 周杰伦 should be listed before 林俊杰 (2 plays vs 1)
	if !strings.Contains(summary, "周杰伦") || !strings.Contains(summary, "林俊杰") {
		t.Errorf("summary should contain both artists, got: %s", summary)
	}
}

func TestSummaryFixedSizeTop5(t *testing.T) {
	s := tempStore(t)

	// Record 10 different artists
	for i := 0; i < 10; i++ {
		artist := string(rune('A' + i)) // A, B, C, ...
		s.Record("netease:"+artist, "song-"+artist, artist, "netease")
	}

	summary := s.Summary()
	// Count occurrences of "、" (the artist separator)
	// With 5 artists, there should be 4 separators
	sepCount := strings.Count(summary, "、")
	if sepCount != 4 {
		t.Errorf("expected 4 separators for top 5 artists, got %d. Summary: %s", sepCount, summary)
	}
}

func TestRingBufferEviction(t *testing.T) {
	s := tempStore(t)

	// Fill beyond cap
	for i := 0; i < maxHistory+50; i++ {
		s.Record("netease:1", "same-song", "same-artist", "netease")
	}

	if len(s.history) != maxHistory {
		t.Errorf("history should be capped at %d, got %d", maxHistory, len(s.history))
	}
	// Total plays should equal cap (not 550)
	if s.agg.TotalPlays != maxHistory {
		t.Errorf("total plays should be %d, got %d", maxHistory, s.agg.TotalPlays)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "music_pref.json")

	// Write data
	s1 := NewStore(path)
	s1.Record("bilibili:BV1", "晴天", "周杰伦", "bilibili")
	s1.Record("netease:123", "江南", "林俊杰", "netease")

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("preference file should exist after Record()")
	}

	// Create a new store from the same file — should load existing data
	s2 := NewStore(path)
	summary := s2.Summary()
	if !strings.Contains(summary, "2 首") {
		t.Errorf("loaded store should have 2 plays, summary: %s", summary)
	}
	if !strings.Contains(summary, "周杰伦") {
		t.Errorf("loaded store should contain '周杰伦', summary: %s", summary)
	}
}

func TestHistoryDetail(t *testing.T) {
	s := tempStore(t)

	s.Record("netease:1", "song1", "artist1", "netease")
	s.Record("netease:2", "song2", "artist2", "netease")
	s.Record("netease:3", "song3", "artist1", "netease")

	recent, topArtists, topSongs, total := s.HistoryDetail(2)

	// Should return 2 most recent entries
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent records, got %d", len(recent))
	}
	// Most recent should be "song3"
	if recent[0].Name != "song3" {
		t.Errorf("expected most recent 'song3', got '%s'", recent[0].Name)
	}
	if recent[1].Name != "song2" {
		t.Errorf("expected second recent 'song2', got '%s'", recent[1].Name)
	}
	// Total should still reflect full history
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	// artist1 has 2 plays, should be top
	if len(topArtists) == 0 || topArtists[0] != "artist1" {
		t.Errorf("expected top artist 'artist1', got %v", topArtists)
	}
	if len(topSongs) == 0 || topSongs[0] != "song3" && topSongs[0] != "song1" && topSongs[0] != "song2" {
		// all songs have count 1, so tie-break alphabetical
	}
}

func TestEmptyHistoryDetail(t *testing.T) {
	s := tempStore(t)

	recent, topArtists, topSongs, total := s.HistoryDetail(10)

	if len(recent) != 0 {
		t.Errorf("expected 0 recent records, got %d", len(recent))
	}
	if len(topArtists) != 0 {
		t.Errorf("expected 0 top artists, got %d", len(topArtists))
	}
	if len(topSongs) != 0 {
		t.Errorf("expected 0 top songs, got %d", len(topSongs))
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
}

func TestRecordTimestamps(t *testing.T) {
	s := tempStore(t)

	before := time.Now()
	s.Record("netease:1", "song1", "artist1", "netease")
	after := time.Now()

	if len(s.history) != 1 {
		t.Fatal("expected 1 record")
	}
	ts := s.history[0].PlayedAt
	if ts.Before(before) || ts.After(after) {
		t.Errorf("timestamp %v not in expected range [%v, %v]", ts, before, after)
	}
}

func TestCorruptFileRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "music_pref.json")

	// Write corrupt JSON
	_ = os.WriteFile(path, []byte("{not valid json"), 0o644)

	// Should not panic, should start fresh
	s := NewStore(path)
	summary := s.Summary()
	if summary != "" {
		t.Errorf("corrupt file should result in empty summary, got: %q", summary)
	}
}
