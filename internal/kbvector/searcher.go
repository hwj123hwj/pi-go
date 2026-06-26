package kbvector

// ────────────────────────────────────────────────────────────────────────────
//  VectorSearcher and HybridSearcher implement the kbtools.SearchStrategy
//  interface. This file provides the bridge between kbvector (pure vector
//  search) and kbtools (the KB agent's tool layer).
//
//  IMPORTANT: This file imports kbtools, but kbtools does NOT import kbvector.
//  The dependency flows one way: kbvector → kbtools.
//  The SearchStrategy is injected at runtime via NewSearchToolWithStrategy().
// ────────────────────────────────────────────────────────────────────────────

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	kbtools "github.com/hwj123hwj/pi-go/internal/agents/kb/tools"
)

// VectorSearcher implements SearchStrategy using vector embeddings.
type VectorSearcher struct {
	client    *EmbeddingClient
	store     *Store
	mu        sync.Mutex
	lastBuild time.Time
}

// NewVectorSearcher creates a vector-backed searcher.
func NewVectorSearcher(client *EmbeddingClient, store *Store) *VectorSearcher {
	return &VectorSearcher{
		client: client,
		store:  store,
	}
}

func (v *VectorSearcher) Name() string { return "vector" }

// Search ensures the index is built, then does cosine similarity search.
func (v *VectorSearcher) Search(entries []kbtools.Entry, q kbtools.SearchQuery) []kbtools.SearchResult {
	// Build index if not done yet or if entries changed
	v.ensureIndexed(entries)

	if v.store.Len() == 0 {
		return nil
	}

	// Embed the query
	queryVec, err := v.client.EmbedOne(context.Background(), q.Query)
	if err != nil {
		// Fallback to no results rather than crash
		return nil
	}

	// Vector search
	vecResults := v.store.Search(queryVec, q.Limit)

	// Apply filters (tag/category) and convert types
	var results []kbtools.SearchResult
	for _, vr := range vecResults {
		// Find the matching entry to apply filters
		var entry kbtools.Entry
		for _, e := range entries {
			if e.RelPath == vr.RelPath {
				entry = e
				break
			}
		}
		// Apply tag/category filters
		if q.Category != "" && !strings.EqualFold(entry.Category, q.Category) {
			continue
		}
		if q.Tag != "" && !entryHasTag(entry, q.Tag) {
			continue
		}
		results = append(results, kbtools.SearchResult{
			Entry: entry,
			Score: vr.Score,
		})
	}

	return results
}

// ensureIndexed builds the vector index if it's empty or stale.
func (v *VectorSearcher) ensureIndexed(entries []kbtools.Entry) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Rebuild if store is empty or if it was built >30 seconds ago with new entries
	if v.store.Len() > 0 && time.Since(v.lastBuild) < 30*time.Second {
		// Check if entry count matches
		if v.store.Len() == len(entries) {
			return
		}
	}

	// Convert to IndexDoc
	docs := make([]IndexDoc, len(entries))
	for i, e := range entries {
		docs[i] = IndexDoc{
			Path:     e.Path,
			RelPath:  e.RelPath,
			Title:    e.Title,
			Summary:  e.Summary,
			Modified: e.Modified,
		}
	}

	// Index (only new/modified docs are embedded — saves API calls)
	count, err := v.store.Index(context.Background(), v.client, docs)
	if err != nil {
		slog.Warn("vector index build failed", "error", err, "docs", len(docs))
		return
	}
	v.lastBuild = time.Now()
	if count > 0 {
		slog.Info("vector index built", "embedded", count, "total", v.store.Len())
	}
}

// ── HybridSearcher: keyword + vector blend ──────────────────────────────────

// HybridSearcher blends keyword and vector search results.
// Inspired by OpenViking's approach of combining dense + sparse retrieval.
type HybridSearcher struct {
	vector *VectorSearcher // pointer — copying a mutex by value is a Go bug
}

// NewHybridSearcher creates a hybrid searcher that blends keyword and vector.
func NewHybridSearcher(client *EmbeddingClient, store *Store) *HybridSearcher {
	return &HybridSearcher{
		vector: NewVectorSearcher(client, store),
	}
}

func (h *HybridSearcher) Name() string { return "hybrid" }

func (h *HybridSearcher) Search(entries []kbtools.Entry, q kbtools.SearchQuery) []kbtools.SearchResult {
	// ── Step 1: Get keyword results ──
	keywordResults := kbtools.KeywordSearcher{}.Search(entries, q)

	// If no query (e.g. listing by tag), just return keyword
	if q.Query == "" {
		return keywordResults
	}

	// ── Step 2: Get vector results ──
	vecResults := h.vector.Search(entries, q)

	// ── Step 3: Blend scores ──
	// Normalize both to [0, 1] range, then combine with weights.
	// Vector weight = 0.6, Keyword weight = 0.4
	const vecWeight = 0.6
	const kwWeight = 0.4

	// Build a map of relPath → blended score
	type blendedEntry struct {
		entry    kbtools.Entry
		vecScore float64
		kwScore  float64
	}
	blended := make(map[string]blendedEntry)
	maxKwScore := 1.0

	for _, r := range keywordResults {
		blended[r.Entry.RelPath] = blendedEntry{entry: r.Entry, kwScore: r.Score}
		if r.Score > maxKwScore {
			maxKwScore = r.Score
		}
	}

	for _, r := range vecResults {
		b, ok := blended[r.Entry.RelPath]
		if !ok {
			b = blendedEntry{entry: r.Entry}
		}
		b.vecScore = r.Score
		blended[r.Entry.RelPath] = b
	}

	// Compute final scores
	results := make([]kbtools.SearchResult, 0, len(blended))
	for _, b := range blended {
		normalizedKw := b.kwScore / maxKwScore
		finalScore := vecWeight*b.vecScore + kwWeight*normalizedKw
		results = append(results, kbtools.SearchResult{
			Entry: b.entry,
			Score: finalScore,
		})
	}

	// Sort by final score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply limit
	if q.Limit > 0 && len(results) > q.Limit {
		results = results[:q.Limit]
	}

	return results
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func entryHasTag(entry kbtools.Entry, tag string) bool {
	for _, t := range entry.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}
