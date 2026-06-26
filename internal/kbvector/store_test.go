package kbvector

import (
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float32
		b    []float32
		want float64
	}{
		{"identical", []float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},
		{"orthogonal", []float32{1, 0, 0}, []float32{0, 1, 0}, 0.0},
		{"opposite", []float32{1, 0, 0}, []float32{-1, 0, 0}, -1.0},
		{"45 degrees", []float32{1, 1}, []float32{1, 0}, 0.7071}, // ~0.707
		{"empty", []float32{}, []float32{}, 0},
		{"mismatch", []float32{1, 0}, []float32{1}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if tt.name == "45 degrees" {
				if got < 0.70 || got > 0.71 {
					t.Errorf("cosineSimilarity(%v, %v) = %f, want ~0.707", tt.a, tt.b, got)
				}
			} else if got != tt.want {
				t.Errorf("cosineSimilarity(%v, %v) = %f, want %f", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/vectors.json"

	// Create store with entries
	s1 := NewStore(path)
	entry := VectorEntry{
		Path:    "/abs/path/doc.md",
		RelPath: "issues/doc.md",
		Title:   "Test Doc",
		Summary: "A test document",
		Vector:  []float32{0.1, 0.2, 0.3, 0.4},
	}
	s1.entries = append(s1.entries, entry)
	s1.pathIndex[entry.Path] = 0

	if err := s1.save(); err != nil {
		t.Fatal(err)
	}

	// Load from disk
	s2 := NewStore(path)
	if len(s2.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(s2.entries))
	}
	got := s2.entries[0]
	if got.Title != "Test Doc" {
		t.Errorf("title = %s, want 'Test Doc'", got.Title)
	}
	if len(got.Vector) != 4 {
		t.Errorf("vector len = %d, want 4", len(got.Vector))
	}
	if got.Vector[0] != 0.1 {
		t.Errorf("vector[0] = %f, want 0.1", got.Vector[0])
	}
}

func TestStoreSearch(t *testing.T) {
	s := NewStore(t.TempDir() + "/vectors.json")

	// Add 3 entries with known vectors
	entries := []VectorEntry{
		{RelPath: "a.md", Title: "A", Vector: []float32{1, 0, 0}},
		{RelPath: "b.md", Title: "B", Vector: []float32{0, 1, 0}},
		{RelPath: "c.md", Title: "C", Vector: []float32{0.9, 0.1, 0}}, // close to A
	}
	for _, e := range entries {
		s.pathIndex[e.Path] = len(s.entries)
		s.entries = append(s.entries, e)
	}

	// Query close to A
	results := s.Search([]float32{1, 0, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// a.md should be first (exact match), c.md second (close)
	if results[0].RelPath != "a.md" {
		t.Errorf("top result = %s, want 'a.md'", results[0].RelPath)
	}
	if results[1].RelPath != "c.md" {
		t.Errorf("second result = %s, want 'c.md'", results[1].RelPath)
	}
}

func TestEmbeddingClientNotAvailable(t *testing.T) {
	c := NewEmbeddingClient("", "", "")
	if c.Available() {
		t.Error("client with empty key should not be available")
	}
}

func TestEmbeddingClientAvailable(t *testing.T) {
	c := NewEmbeddingClient("test-key", "", "")
	if !c.Available() {
		t.Error("client with key should be available")
	}
}

func TestStoreStaleEntryRemoval(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/vectors.json"
	s := NewStore(path)

	// Add 3 entries
	entries := []VectorEntry{
		{Path: "/abs/a.md", RelPath: "a.md", Title: "A", Vector: []float32{1, 0}},
		{Path: "/abs/b.md", RelPath: "b.md", Title: "B", Vector: []float32{0, 1}},
		{Path: "/abs/c.md", RelPath: "c.md", Title: "C", Vector: []float32{1, 1}},
	}
	for _, e := range entries {
		s.pathIndex[e.Path] = len(s.entries)
		s.entries = append(s.entries, e)
	}

	// Simulate "b.md" deleted: remove it via removeEntryLocked
	s.mu.Lock()
	s.removeEntryLocked("/abs/b.md")
	s.mu.Unlock()

	if s.Len() != 2 {
		t.Fatalf("expected 2 entries after removal, got %d", s.Len())
	}
	// b.md should no longer be in pathIndex
	s.mu.Lock()
	if _, ok := s.pathIndex["/abs/b.md"]; ok {
		t.Error("b.md should have been removed from pathIndex")
	}
	// Remaining entries should be findable
	if idx, ok := s.pathIndex["/abs/a.md"]; !ok || s.entries[idx].Title != "A" {
		t.Error("a.md not correctly indexed after removal")
	}
	if idx, ok := s.pathIndex["/abs/c.md"]; !ok || s.entries[idx].Title != "C" {
		t.Error("c.md not correctly indexed after removal")
	}
	s.mu.Unlock()
}
