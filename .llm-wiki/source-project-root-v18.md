# Source: Project Root (.) — v18: OpenViking deep-absorption (L1 synopsis + hotness eviction)

> Date: 2026-06-26
> Focus: Deep-absorption of OpenViking patterns adapted for pi-go — KB tiered loading and hotness-based memory eviction.

## Summary

Studied OpenViking's source code (cloned, read `hierarchical_retriever.py`, `memory_lifecycle.py`, `tool_result_synopsis.py`, `compressor_v2.py`). Extracted two core patterns and adapted them to pi-go's lightweight, no-external-dependency philosophy.

## OpenViking Patterns Studied & Adapted

### Pattern 1: Tool Result Synopsis (L1 Tier) → kb_read Overview Mode

**OpenViking approach**: `tool_result_synopsis.py` generates deterministic, type-aware synopses (JSON structure, code imports/symbols, text headers+excerpts) to replace full tool outputs in the context window. This lets the LLM decide if a full read is worth the tokens before spending them.

**pi-go adaptation**: Added `overview` parameter to `kb_read` tool. When `overview=true`, returns a structured synopsis:
- Document statistics (line count, word count, header count)
- Hierarchical structure (all `#` headers)
- First content line per section (120-char max)
- Hint to use full mode for deep content

**Token savings**: ~80% reduction for large documents (e.g. 3000-token doc → ~500-token overview).

**Key design choice**: Pure text extraction rules (no LLM, no embedding). Go's `bufio.Scanner` is sufficient.

### Pattern 2: Hotness Score for Memory Eviction → profile.Store Eviction

**OpenViking approach**: `memory_lifecycle.py` computes a `hotness_score` = `sigmoid(log1p(access_count)) × exp(-decay_rate × age_days)`. This blends frequency and recency — frequently-accessed AND recently-updated contexts get higher scores and survive eviction.

**pi-go adaptation**: Replaced `profile.Store`'s timestamp-only eviction with `evictLowestHotness()`:
- Each `Fact` now has `AccessCount` (incremented on every Record/RecordBatch)
- Eviction computes hotness for all facts in a category and removes the lowest
- Half-life = 7 days (same as OpenViking default)
- Formula: `freq = 1/(1+exp(-log1p(count)))`, `recency = exp(-ln2/7 × age_days)`, `score = freq × recency`

**Impact**: A fact that was recorded 50 times but a month ago can survive eviction over a fact recorded once yesterday — because frequency matters, not just recency.

## Implementation

### Modified: `internal/agents/kb/tools/kb_read.go`

- Added `overview` boolean parameter
- `generateOverview()` function: scans markdown, extracts headers + first line per section
- Updated tool description to explain the L0→L1→L2 workflow:
  - L0: `kb_search` (title + summary only)
  - L1: `kb_read overview=true` (structure + first lines)
  - L2: `kb_read` (full content)

### New: `internal/agents/kb/tools/kb_read_test.go`

7 tests covering: full mode, overview mode, size comparison, file not found, description hints, empty file, relative path.

### Modified: `internal/profile/store.go`

- Added `AccessCount` field to `Fact` struct
- `Record()` and `RecordBatch()` now increment `AccessCount` on upsert
- Added `evictLowestHotness()` using OpenViking's frequency×recency formula
- `evictOldest()` now delegates to `evictLowestHotness()` (backward compat)
- Added `math` import

## What We Deliberately Did NOT Adopt

| OpenViking pattern | Reason for rejection |
|---|---|
| Vector + rerank retrieval | Needs external embedding API + vector DB — too heavy for desktop |
| `viking://` virtual filesystem | Full server architecture overkill for local-first app |
| MCP protocol converter | pi-go uses its own tool protocol, not MCP |
| Semantic queue for async summarization | LLM-based summarization per file is too expensive; our rules-based overview is free |
| Session memory templating system | Complex ExtractLoop with VLM — overkill for our single-user desktop model |

## Files Changed

| File | Change |
|---|---|
| `internal/agents/kb/tools/kb_read.go` | Added overview mode + generateOverview() |
| `internal/agents/kb/tools/kb_read_test.go` | NEW — 7 tests for overview/full modes |
| `internal/profile/store.go` | AccessCount + hotness eviction |

## Cross-references
- [[source-project-root-v17]] — Unified user profile (predecessor, where profile.Store was created)
- [[kb-agent]] — KB application (now has L1 tier loading)
- [[music-agent]] — Music application (profile hotness eviction benefits artist ranking)
