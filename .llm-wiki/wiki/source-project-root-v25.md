---
type: source
source_path: "."
date: 2026-06-27
tags: [profile, vector-search, memory-extraction, synopsis, desktop-ui, rest-api, openviking]
related:
  [
    [unified-profile],
    [kb-vector-search],
    [session-memory-extraction],
    [tool-output-synopsis],
    [desktop-app],
    [server-websocket],
    [kb-agent],
    [personal-assistant-roadmap],
    [config-system],
  ]
---

# Source: Project Root (.) — v25 Ingest

> Covers all changes from v17 through v25: unified user profile, KB vector search,
> session memory extraction, tool output auto-synopsis, KB L1 overview mode, desktop
> profile panel, and three rounds of code review fixes.

## Key Takeaways

1. **Unified user profile** ([[unified-profile]]) — A cross-agent, persistent user profile store that acts as a "condensed second brain". Any agent records facts; every agent sees the same fixed-size summary. Uses hotness-based eviction (frequency × recency).

2. **KB vector search** ([[kb-vector-search]]) — Hybrid keyword + vector retrieval using SiliconFlow bge-m3 embeddings. Local JSON vector store with cosine similarity. Pluggable `SearchStrategy` interface.

3. **Session memory extraction** ([[session-memory-extraction]]) — OpenViking ExtractLoop adaptation: after each turn, an async LLM call extracts user facts from the conversation and writes them to the profile store.

4. **Tool output auto-synopsis** ([[tool-output-synopsis]]) — An `AfterToolCallHook` that replaces large tool outputs (>4000 chars) with a deterministic structural synopsis, preserving full text in `UserFacing`.

5. **KB L1 overview mode** — `kb_read overview=true` returns document structure (headers + first sentences + word count) instead of full content, ~10-20% of original size.

6. **Desktop profile panel** — New right-sidebar panel (`ProfilePanel.tsx`) that visualizes all profile facts grouped by category, shows the agent-injected summary, and supports per-fact deletion.

7. **Profile REST API** — `GET /profile` returns all facts + summary; `DELETE /profile` removes individual facts.

8. **Three code review rounds** (v22–v24) — Fixed UTF-8 byte truncation, mutex copy-by-value, stale vector persistence, batch eviction, phantom search results, double-synopsis prevention, and scanner buffer limits.

## Important Entities & Concepts

- **`profile.Store`** (`internal/profile/store.go`) — Thread-safe user profile with category-based facts, hotness eviction, atomic file writes
- **`kbvector.Store`** (`internal/kbvector/store.go`) — Local JSON vector store with cosine similarity search
- **`kbvector.HybridSearcher`** (`internal/kbvector/searcher.go`) — Blends keyword (0.4 weight) + vector (0.6 weight) results
- **`profile.Extractor`** (`internal/profile/extractor.go`) — LLM-based fact extraction from conversation turns
- **`SynopsisAfterHook`** (`internal/agent/synopsis.go`) — After-hook that synopsizes large tool outputs
- **`generateOverview`** (`internal/agents/kb/tools/kb_read.go`) — L1 document structure extraction

## Notable Claims & Data Points

- Profile summary is always ~80 tokens regardless of stored fact count
- Max items per category: 10 (coding/general), 20 (music)
- Hotness formula: `frequency(logistic(access_count)) × recency(exp(-decay × age_days))`
- Synopsis threshold: 4000 chars; synopsis skip marker: `[输出概览]`
- Vector search uses `title + title + summary` as embedding text (title weighted 2x)
- KB scanner buffer: 1MB (was 64KB default — silent truncation bug fixed in v24)

## Cross-references

- [[source-project-root-v17]] — Original unified profile implementation
- [[source-project-root-v18]] — KB L1 overview + hotness eviction
- [[source-project-root-v19]] — KB vector search implementation
- [[source-project-root-v20]] — Session memory extraction
- [[source-project-root-v21]] — Tool output auto-synopsis
- [[source-project-root-v22]] — Code review round 1 (UTF-8, mutex, logging)
- [[source-project-root-v23]] — Code review round 2 (stale vectors, eviction, phantoms)
- [[source-project-root-v24]] — Code review round 3 (persistence, scanner buffer)
- [[source-project-root-v25]] — Desktop profile panel

## Contradictions with Existing Wiki

- **[[personal-assistant-roadmap]]** lists memory layer as "P1 (planned)" — it is now **implemented** (v17–v20). Needs update.
- **[[kb-agent]]** Search Strategy section says "Future strategies (e.g., VectorSearcher)" — vector search is now implemented (v19). Needs update.
- **[[server-websocket]]** REST API table does not list `/profile` endpoints — needs update.
- **[[desktop-app]]** right sidebar rail lists 5 icons — now has 6 (profile added). Needs update.
- **[[config-system]]** does not document `KBEmbeddingAPIKey` / `KBEmbeddingBaseURL` / `KBEmbeddingModel` config fields — needs update.
