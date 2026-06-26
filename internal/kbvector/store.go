package kbvector

// ────────────────────────────────────────────────────────────────────────────
//  Vector store: persists document embeddings to a local JSON file.
//
//  Design:
//  - Each KB document is embedded once (on first index build).
//  - Vectors are cached on disk, keyed by file path.
//  - Re-indexing only embeds files that changed (based on mod time).
//  - Cosine similarity is used for search (pure Go, no external deps).
// ────────────────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// VectorEntry stores one document's embedding + metadata.
type VectorEntry struct {
	Path      string    `json:"path"`
	RelPath   string    `json:"rel_path"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Vector    []float32 `json:"vector"`
	Modified  time.Time `json:"modified"`
}

// Store persists vector embeddings to disk.
type Store struct {
	mu       sync.Mutex
	filePath string
	entries  []VectorEntry
	// pathIndex for O(1) lookup
	pathIndex map[string]int
}

// NewStore creates a vector store backed by the given file path.
func NewStore(filePath string) *Store {
	s := &Store{
		filePath:  filePath,
		pathIndex: make(map[string]int),
	}
	s.load()
	return s
}

// Index builds or updates the vector index for the given documents.
// Only documents that are new or modified since last index are embedded
// (saving API calls).
type IndexDoc struct {
	Path     string
	RelPath  string
	Title    string
	Summary  string
	Modified time.Time
}

// Index embeds new/changed docs and saves to disk.
// Returns the number of docs actually embedded (API calls made).
func (s *Store) Index(ctx context.Context, client *EmbeddingClient, docs []IndexDoc) (int, error) {
	if !client.Available() {
		return 0, fmt.Errorf("embedding client not available")
	}

	s.mu.Lock()
	// Find docs that need embedding
	var toEmbed []IndexDoc
	for _, doc := range docs {
		if idx, ok := s.pathIndex[doc.Path]; ok {
			// Already indexed — check if modified
			if !doc.Modified.After(s.entries[idx].Modified) {
				continue // unchanged
			}
		}
		toEmbed = append(toEmbed, doc)
	}
	s.mu.Unlock()

	if len(toEmbed) == 0 {
		return 0, nil
	}

	// Build texts to embed: title + summary (title weighted by duplication)
	texts := make([]string, len(toEmbed))
	for i, doc := range toEmbed {
		// Weight title by repeating it (common trick for keyword importance)
		if doc.Title != "" {
			texts[i] = doc.Title + " " + doc.Title + " " + doc.Summary
		} else {
			texts[i] = doc.Summary
		}
	}

	// Call embedding API
	embeddings, err := client.Embed(ctx, texts)
	if err != nil {
		return 0, fmt.Errorf("failed to embed %d docs: %w", len(toEmbed), err)
	}

	if len(embeddings) != len(toEmbed) {
		return 0, fmt.Errorf("embedding count mismatch: got %d, want %d", len(embeddings), len(toEmbed))
	}

	// Update store
	s.mu.Lock()
	for i, doc := range toEmbed {
		entry := VectorEntry{
			Path:     doc.Path,
			RelPath:  doc.RelPath,
			Title:    doc.Title,
			Summary:  doc.Summary,
			Vector:   embeddings[i],
			Modified: doc.Modified,
		}
		if idx, ok := s.pathIndex[doc.Path]; ok {
			s.entries[idx] = entry // update existing
		} else {
			s.pathIndex[doc.Path] = len(s.entries)
			s.entries = append(s.entries, entry)
		}
	}
	err = s.save()
	s.mu.Unlock()

	return len(toEmbed), err
}

// Search performs cosine similarity search against the stored vectors.
func (s *Store) Search(queryVec []float32, limit int) []SearchResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.entries) == 0 {
		return nil
	}

	results := make([]SearchResult, 0, len(s.entries))
	for _, entry := range s.entries {
		score := cosineSimilarity(queryVec, entry.Vector)
		results = append(results, SearchResult{
			RelPath: entry.RelPath,
			Title:   entry.Title,
			Summary: entry.Summary,
			Score:   score,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// SearchResult is a single vector search hit.
type SearchResult struct {
	RelPath string
	Title   string
	Summary string
	Score   float64
}

// Len returns the number of indexed documents.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// ── Internal: similarity + persistence ──────────────────────────────────────

// cosineSimilarity computes the cosine similarity between two vectors.
// Pure Go, no external dependencies.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		ai := float64(a[i])
		bi := float64(b[i])
		dotProduct += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// ── Persistence ─────────────────────────────────────────────────────────────

type diskFormat struct {
	Entries []VectorEntry `json:"entries"`
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
	s.entries = df.Entries
	// Rebuild path index
	s.pathIndex = make(map[string]int, len(s.entries))
	for i, e := range s.entries {
		s.pathIndex[e.Path] = i
	}
}

func (s *Store) save() error {
	df := diskFormat{Entries: s.entries}
	data, err := json.MarshalIndent(df, "", "  ")
	if err != nil {
		return err
	}
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
