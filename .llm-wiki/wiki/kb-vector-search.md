---
type: concept
date: 2026-06-27
tags: [vector-search, embeddings, kb, retrieval, siliconflow, bge-m3, hybrid-search]
related: [kb-agent, config-system, unified-profile]
---

# KB Vector Search

> Hybrid keyword + vector retrieval for the knowledge base agent, using SiliconFlow
> bge-m3 embeddings and a local JSON vector store.

## Overview

Implemented in v19, the KB vector search extends the `SearchStrategy` interface with a `HybridSearcher` that blends keyword matching and semantic similarity. This enables finding relevant KB entries even when the query doesn't contain exact keywords from the document.

## Architecture

```
HybridSearcher
├── VectorSearcher (0.6 weight)
│   ├── EmbeddingClient (SiliconFlow bge-m3)
│   ├── kbvector.Store (local JSON, cosine similarity)
│   └── ensureIndexed() — embeds new/changed docs, removes stale
│
└── KeywordSearcher (0.4 weight)
    └── Weighted full-text matching (title: 5x, tag: 3x, summary: 2x, path: 1x)
```

## Embedding

### Client

`EmbeddingClient` wraps the SiliconFlow (or compatible) embeddings API:

```go
func NewEmbeddingClient(apiKey, baseURL, model string) *EmbeddingClient
```

- **Model**: `bge-m3` (multilingual, 1024 dimensions)
- **Batch embedding**: `Embed(ctx, texts []string) ([][]float32, error)`
- **Single embedding**: `EmbedOne(ctx, text) ([]float32, error)`
- **Availability check**: `Available()` returns false if no API key configured

### Embedding Text Construction

Title is weighted 2× by duplication (common trick for keyword importance):

```go
texts[i] = doc.Title + " " + doc.Title + " " + doc.Summary
```

## Vector Store

`kbvector.Store` is a local JSON-backed vector store:

### Data Structure

```go
type VectorEntry struct {
    Path     string
    RelPath  string
    Title    string
    Summary  string
    Vector   []float32
    Modified time.Time
}
```

### Indexing Flow

1. **Detect changes**: Compare incoming docs to stored entries via `pathIndex` + `Modified` timestamp
2. **Remove stale entries**: Files deleted from repo are removed from store
3. **Embed new/changed**: Batch API call for `toEmbed` docs
4. **Persist**: Save to `{DataDir}/kb_vectors.json`

### Stale Entry Removal (v23–v24 fix)

- v23: Added stale detection (files in store but not in current repo scan)
- v23: Added `removeEntryLocked()` to maintain `pathIndex` consistency
- v24: Fixed persistence — stale removal is now saved to disk even when no new docs need embedding

### Cosine Similarity

Standard cosine similarity with zero-vector guards:

```go
func cosineSimilarity(a, b []float32) float64
```

## Hybrid Blending

Scores from both searchers are blended:

```go
const vecWeight = 0.6
const kwWeight = 0.4

blendedScore = vecScore * vecWeight + normalizedKwScore * kwWeight
```

Keyword scores are normalized by dividing by `maxKwScore` to bring them to [0, 1].

### Filtering

After blending, tag/category filters are applied by cross-referencing the current KB index (since the vector store may have entries not in the current index).

## Configuration

| Config Field | Env Var | Description |
|---|---|---|
| `KBEmbeddingAPIKey` | `PI_GO_KB_EMBEDDING_API_KEY` | SiliconFlow API key |
| `KBEmbeddingBaseURL` | `PI_GO_KB_EMBEDDING_BASE_URL` | API base URL |
| `KBEmbeddingModel` | `PI_GO_KB_EMBEDDING_MODEL` | Model name (default: `bge-m3`) |

When `KBEmbeddingAPIKey` is empty, falls back to pure `KeywordSearcher`.

## Code Locations

| File | Responsibility |
|------|----------------|
| `internal/kbvector/store.go` | Vector store, indexing, cosine similarity, persistence |
| `internal/kbvector/searcher.go` | `VectorSearcher`, `HybridSearcher`, `ensureIndexed` |
| `internal/kbvector/client.go` | Embedding API client |
| `internal/agents/kb/application.go` | Search strategy selection |
| `internal/agents/kb/tools/kb_search.go` | Search tool (uses strategy) |

## Source

- [[source-project-root-v19]] — Initial implementation
- [[source-project-root-v23]] — Stale vector cleanup
- [[source-project-root-v24]] — Stale removal persistence fix

## Related

- [[kb-agent]] — Consumes vector search via `SearchStrategy` interface
- [[config-system]] — `KBEmbedding*` config fields
