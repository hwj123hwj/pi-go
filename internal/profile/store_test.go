package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return NewStore(filepath.Join(dir, "user_profile.json"))
}

func TestRecordAndSummary(t *testing.T) {
	s := tempStore(t)

	if got := s.Summary(); got != "" {
		t.Fatalf("empty store should return empty summary, got: %q", got)
	}

	s.Record(CategoryCoding, "language", "Go", "coding-agent")
	s.Record(CategoryCoding, "os", "macOS", "coding-agent")
	s.Record(CategoryGeneral, "location", "北京", "general-agent")

	summary := s.Summary()
	if !strings.Contains(summary, "用户画像") {
		t.Errorf("summary should have header, got: %s", summary)
	}
	if !strings.Contains(summary, "Go") {
		t.Errorf("summary should contain Go, got: %s", summary)
	}
	if !strings.Contains(summary, "macOS") {
		t.Errorf("summary should contain macOS, got: %s", summary)
	}
	if !strings.Contains(summary, "北京") {
		t.Errorf("summary should contain 北京, got: %s", summary)
	}
}

func TestMusicSummary(t *testing.T) {
	s := tempStore(t)

	s.Record(CategoryMusic, "total_plays", "142", "music-agent")
	s.Record(CategoryMusic, "artist:周杰伦", "50", "music-agent")
	s.Record(CategoryMusic, "artist:林俊杰", "30", "music-agent")
	s.Record(CategoryMusic, "artist:陈奕迅", "20", "music-agent")

	summary := s.Summary()
	if !strings.Contains(summary, "累计 142 首") {
		t.Errorf("summary should contain total plays, got: %s", summary)
	}
	if !strings.Contains(summary, "周杰伦") {
		t.Errorf("summary should contain 周杰伦, got: %s", summary)
	}
	// Top artist should come before lower-ranked
	zhouIdx := strings.Index(summary, "周杰伦")
	linIdx := strings.Index(summary, "林俊杰")
	if zhouIdx == -1 || linIdx == -1 {
		t.Fatalf("missing artists in summary: %s", summary)
	}
	if zhouIdx > linIdx {
		t.Errorf("周杰伦 (50 plays) should come before 林俊杰 (30), got: %s", summary)
	}
}

func TestFixedSizeTop5Artists(t *testing.T) {
	s := tempStore(t)

	artists := []string{"A", "B", "C", "D", "E", "F", "G"}
	for i, a := range artists {
		count := len(artists) - i // A=7, B=6, ... so A is top
		s.Record(CategoryMusic, "artist:"+a, intToStr(count), "music-agent")
	}

	summary := s.Summary()
	// Should only contain top 5: A, B, C, D, E
	for _, expected := range []string{"A", "B", "C", "D", "E"} {
		if !strings.Contains(summary, expected) {
			t.Errorf("summary should contain %s, got: %s", expected, summary)
		}
	}
	// Should NOT contain F or G (ranked too low)
	for _, unexpected := range []string{"F", "G"} {
		// Check that F/G don't appear as artist names (they might appear in other words)
		// Since our artists are single chars, be careful. Just check exact "、F" or "F、" patterns
		if strings.Contains(summary, "、"+unexpected) || strings.Contains(summary, unexpected+"、") || strings.HasSuffix(summary, unexpected) {
			t.Errorf("summary should NOT contain artist %s (outside top 5), got: %s", unexpected, summary)
		}
	}
}

func TestUpdateExistingFact(t *testing.T) {
	s := tempStore(t)

	s.Record(CategoryCoding, "language", "Go", "coding-agent")
	s.Record(CategoryCoding, "language", "Python", "coding-agent") // overwrite

	facts := s.GetFacts(CategoryCoding)
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].Value != "Python" {
		t.Errorf("expected value 'Python' (last write wins), got '%s'", facts[0].Value)
	}
}

func TestCategoryEviction(t *testing.T) {
	s := tempStore(t)

	for i := 0; i < maxPerCategory+5; i++ {
		s.Record(CategoryCoding, "key-"+intToStr(i), "val", "test")
	}

	facts := s.GetFacts(CategoryCoding)
	if len(facts) != maxPerCategory {
		t.Errorf("expected %d facts after eviction, got %d", maxPerCategory, len(facts))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "user_profile.json")

	s1 := NewStore(path)
	s1.Record(CategoryCoding, "language", "Go", "coding-agent")
	s1.Record(CategoryMusic, "artist:周杰伦", "42", "music-agent")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("profile file should exist after Record()")
	}

	s2 := NewStore(path)
	summary := s2.Summary()
	if !strings.Contains(summary, "Go") {
		t.Errorf("loaded store should contain 'Go', got: %s", summary)
	}
	if !strings.Contains(summary, "周杰伦") {
		t.Errorf("loaded store should contain '周杰伦', got: %s", summary)
	}
}

func TestRemoveFact(t *testing.T) {
	s := tempStore(t)

	s.Record(CategoryCoding, "language", "Go", "coding-agent")
	s.Record(CategoryCoding, "os", "macOS", "coding-agent")

	s.Remove(CategoryCoding, "language")

	facts := s.GetFacts(CategoryCoding)
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact after remove, got %d", len(facts))
	}
	if facts[0].Key != "os" {
		t.Errorf("expected remaining key 'os', got '%s'", facts[0].Key)
	}
}

func TestEmptySummary(t *testing.T) {
	s := tempStore(t)

	// No facts at all
	if got := s.Summary(); got != "" {
		t.Errorf("empty store summary should be empty, got: %q", got)
	}

	// Category exists but no facts
	s.mu.Lock()
	s.facts[CategoryCoding] = make(map[string]Fact)
	s.mu.Unlock()

	if got := s.Summary(); got != "" {
		t.Errorf("store with empty categories should return empty summary, got: %q", got)
	}
}

func TestCorruptFileRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "user_profile.json")

	_ = os.WriteFile(path, []byte("{not valid json"), 0o644)

	s := NewStore(path)
	if got := s.Summary(); got != "" {
		t.Errorf("corrupt file should result in empty summary, got: %q", got)
	}
}

func TestRecordBatch(t *testing.T) {
	s := tempStore(t)

	s.RecordBatch(CategoryCoding, "coding-agent", map[string]string{
		"language": "Go",
		"editor":   "Neovim",
		"shell":    "zsh",
	})

	facts := s.GetFacts(CategoryCoding)
	if len(facts) != 3 {
		t.Fatalf("expected 3 facts after batch, got %d", len(facts))
	}

	summary := s.Summary()
	for _, expected := range []string{"Go", "Neovim", "zsh"} {
		if !strings.Contains(summary, expected) {
			t.Errorf("summary should contain %s, got: %s", expected, summary)
		}
	}
}

func TestCrossCategorySummary(t *testing.T) {
	s := tempStore(t)

	s.Record(CategoryCoding, "language", "Go", "coding-agent")
	s.Record(CategoryMusic, "total_plays", "50", "music-agent")
	s.Record(CategoryMusic, "artist:周杰伦", "30", "music-agent")
	s.Record(CategoryGeneral, "timezone", "UTC+8", "general-agent")

	summary := s.Summary()

	// Should have all three category lines
	if !strings.Contains(summary, "开发：") {
		t.Errorf("summary should have coding section, got: %s", summary)
	}
	if !strings.Contains(summary, "音乐：") {
		t.Errorf("summary should have music section, got: %s", summary)
	}
	if !strings.Contains(summary, "通用：") {
		t.Errorf("summary should have general section, got: %s", summary)
	}
}

// intToStr is a simple int→string without importing strconv (keeps test deps minimal).
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var sign string
	if n < 0 {
		sign = "-"
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return sign + string(digits)
}

func TestConcurrentAccess(t *testing.T) {
	s := tempStore(t)
	done := make(chan struct{})

	// Concurrent writers
	for i := 0; i < 4; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				s.Record(CategoryCoding, "key-"+intToStr(id*100+j), "val", "test")
			}
			done <- struct{}{}
		}(i)
	}

	// Concurrent readers (Summary calls)
	for i := 0; i < 4; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				_ = s.Summary()
			}
			done <- struct{}{}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 8; i++ {
		<-done
	}

	// Should not panic, should have correct fact count (capped at maxPerCategory)
	facts := s.GetFacts(CategoryCoding)
	if len(facts) > maxPerCategory {
		t.Errorf("expected at most %d facts, got %d", maxPerCategory, len(facts))
	}
}
