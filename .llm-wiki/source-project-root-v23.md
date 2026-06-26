# Source: Project Root (.) — v23: Code review round 2 — stale vectors, batch eviction, phantom results, double-synopsis

> Date: 2026-06-27
> Focus: Second pass of code review on v17–v22, finding deeper logic bugs missed in the first pass.

## Summary

After the first code review (v22), I did a second pass focusing on edge cases and logic correctness. Found and fixed 5 additional bugs, 3 of which are high severity (data correctness issues).

## Bugs Found & Fixed

### Bug A: generateOverview UTF-8 truncation (High) — kb_read.go

**Problem**: `generateOverview()` used `firstLine[:120]` to truncate section first-sentences. This byte-slices Chinese text (3 bytes/rune), producing invalid UTF-8. This bug existed since v18 and was missed in v22's first pass because it was in a different code path than the truncation functions.

**Fix**: Extracted a shared `truncateRunes()` helper and used it in both places (mid-document and end-of-document section extraction).

### Bug B: RecordBatch eviction only removes 1 item when N over limit (High) — store.go

**Problem**: `RecordBatch()` called `evictLowestHotness()` which removes exactly 1 item. If a batch insert added 5 items over the limit, only 1 was evicted, leaving the category 4 items over capacity. Over time this causes unbounded growth.

**Fix**: Created `evictToLimit()` which loops `evictLowestHotness()` until `len(cat) <= limit`. Both `Record()` and `RecordBatch()` now call `evictToLimit()`.

### Bug C: Vector store never removes stale entries (High) — store.go

**Problem**: When a KB document was deleted or renamed, the old vector entry remained in the store forever. Searches would return hits for documents that no longer exist.

**Fix**: `Index()` now builds a `currentPaths` set from the input docs, detects entries whose path is not in the set, and removes them via `removeEntryLocked()`. This runs on every index build.

### Bug D: VectorSearcher phantom results (High) — searcher.go

**Problem**: When a vector store entry didn't match any entry in the current `entries` list (e.g. stale entry), the code used a zero-value `kbtools.Entry{}`. This phantom entry would be returned in results with empty Title/Category/Tags, potentially confusing the LLM.

**Fix**: Added a `found` flag — if the vector result doesn't match any current entry, it's skipped entirely.

### Bug E: SynopsisAfterHook double-synopsis (Medium) — synopsis.go

**Problem**: `kb_read` with `overview=true` already returns a synopsized output. If that output happened to be >4000 chars, the SynopsisAfterHook would generate a synopsis *of the synopsis*, producing a confusing double-indirection.

**Fix**: Added `synopsisSkipMarker = "[输出概览]"` check — if Content already contains this marker, the hook skips processing.

## Tests Added

| Test | What it verifies |
|---|---|
| `TestStoreStaleEntryRemoval` | Vector store removes deleted entries correctly, path index stays consistent |
| `TestRecordBatchEvictsMultiple` | Batch insert evicts down to limit even when many items over |
| `TestSynopsisAfterHook_NoDoubleSynopsis` | Hook skips content already containing synopsis marker |

## Verification

```
go vet ./...     # clean
go build ./...   # clean
go test -race    # all pass
```

## Files Changed

| File | Changes |
|---|---|
| `internal/agents/kb/tools/kb_read.go` | UTF-8 safe truncation in generateOverview via truncateRunes() |
| `internal/profile/store.go` | evictToLimit() loop for batch inserts |
| `internal/kbvector/store.go` | Stale entry removal in Index(), removeEntryLocked() helper |
| `internal/kbvector/searcher.go` | Skip phantom vector results (found flag) |
| `internal/agent/synopsis.go` | Double-synopsis prevention via marker check |

## Cross-references
- [[source-project-root-v22]] — First code review pass (UTF-8, mutex copy, error logging)
- [[source-project-root-v18]] — KB L1 overview mode (generateOverview bug origin)
- [[source-project-root-v19]] — KB vector search (stale entry + phantom result bugs)
