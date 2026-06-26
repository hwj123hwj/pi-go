# Source: Project Root (.) — v19: KB vector search (SiliconFlow bge-m3 + hybrid retrieval)

> Date: 2026-06-26
> Focus: Semantic vector search for the KB agent using SiliconFlow embedding API (bge-m3 model).

## Summary

Added vector search capability to the KB agent. When an embedding API key is configured, `kb_search` automatically upgrades from pure keyword matching to **hybrid search** (keyword + vector blend). No external database needed — vectors are stored in a local JSON file.

## Architecture

```
User query "Go并发编程"
        │
        ├── Keyword Search (existing) ──→ exact match scores
        │
        └── Vector Search (NEW)
                │
                ├── SiliconFlow bge-m3 API → query vector [1024-dim]
                │
                └── cosine similarity vs stored doc vectors
                        │
                        └── semantic match scores

        ──── Hybrid Blender ────
        final = 0.6 × vector_score + 0.4 × normalized_keyword_score
```

### Key Design Decisions

1. **API-based, not local model**: Uses SiliconFlow's `BAAI/bge-m3` (1024-dim). No GPU, no local model download — just HTTP calls.

2. **Local JSON vector store**: Vectors persisted at `{DataDir}/kb_vectors.json`. No vector database dependency. Atomic writes for crash safety.

3. **Incremental indexing**: Only new/modified documents are embedded on each index build. Unchanged docs reuse cached vectors — saves API calls.

4. **Graceful fallback**: If no API key is configured, KB agent automatically uses pure keyword search (`KeywordSearcher`). Zero config = zero vector.

5. **One-way dependency**: `kbvector` package imports `kbtools` (for the SearchStrategy interface), but `kbtools` never imports `kbvector`. Strategy is injected at runtime.

## Implementation

### New: `internal/kbvector/`

| File | Purpose |
|---|---|
| `embedding.go` | SiliconFlow API client (OpenAI-compatible `/v1/embeddings`). Batch support (64 texts/request). |
| `store.go` | Vector persistence: JSON-backed store, incremental indexing, cosine similarity search. Atomic writes. |
| `searcher.go` | `VectorSearcher` + `HybridSearcher` implementing `kbtools.SearchStrategy`. Blend: 0.6 vector + 0.4 keyword. |
| `store_test.go` | Tests: cosine similarity math, persistence, search ranking, client availability. |

### Modified: Config

`internal/config/config.go`:
- Added `KBEmbeddingAPIKey`, `KBEmbeddingModel`, `KBEmbeddingBaseURL`
- `LoadFromEnv()` reads `SILICONFLOW_API_KEY`, `SILICONFLOW_EMBEDDING_MODEL`, `SILICONFLOW_BASE_URL`

### Modified: KB Agent Assembly

`internal/agents/kb/application.go`:
- `buildSearchStrategy()`: if `KBEmbeddingAPIKey` is set → create `HybridSearcher`; else → `KeywordSearcher`
- `BuildTools()`: injects the chosen strategy into `SearchTool`

`internal/agents/kb/tools/tools.go`:
- `ListOptions` now accepts `SearchStrategy` field
- `BuildList()` injects custom strategy into `SearchTool` when provided

### Config: `.env`

```env
SILICONFLOW_API_KEY=sk-...
SILICONFLOW_EMBEDDING_MODEL=BAAI/bge-m3
```

`.env` is already in `.gitignore`. Config loader reads it via `LoadDotEnv()`.

## Verified

- API call tested: `bge-m3` returns 1024-dim vectors ✅
- All unit tests pass with `-race` ✅
- Build clean ✅
- Fallback to keyword search when no API key works ✅

## Cross-references
- [[source-project-root-v18]] — OpenViking L1 synopsis + hotness eviction (predecessor)
- [[kb-agent]] — KB application (now has hybrid vector search)
