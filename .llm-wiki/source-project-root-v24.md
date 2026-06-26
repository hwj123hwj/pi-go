# Source: Project Root (.) — v24: Code review round 3 — stale removal persistence, scanner buffer limit

> Date: 2026-06-27
> Focus: Third pass — focused on the v23 fixes themselves, found edge cases introduced by those fixes.

## Summary

The v23 fixes introduced two new bugs. Both are subtle edge cases that wouldn't surface in normal usage but could cause data loss or silent truncation in specific scenarios.

## Bugs Found & Fixed

### Bug F: Stale entry removal not persisted when no new docs need embedding (High)

**Problem**: In `store.go Index()`, when stale entries were removed but `toEmbed` was empty (all docs unchanged, but some were deleted from repo), the function returned at `if len(toEmbed) == 0 { return 0, nil }` without ever calling `save()`. The stale entries were removed in memory but persisted on disk — they'd reappear on next restart.

**Scenario**: User deletes 3 KB documents. On next search, `ensureIndexed` detects the 3 stale entries, removes them from memory, but since no docs changed, `save()` is never called. Restart → 3 "ghost" entries are back.

**Fix**: After removing stale entries, if `len(stalePaths) > 0`, call `save()` immediately before releasing the lock.

### Bug G: generateOverview scanner silently truncates lines >64KB (Medium)

**Problem**: `generateOverview` uses `bufio.Scanner` with default buffer size (64KB). If a markdown line exceeds this (e.g. a very long URL, a code block with no newlines), `scanner.Scan()` silently stops. The overview would be incomplete with no error.

**Fix**: Set explicit 1MB buffer: `scanner.Buffer(make([]byte, 0, 1024), 1024*1024)`.

## Tests Added

| Test | What it verifies |
|---|---|
| `TestStaleEntryRemovalPersisted` | Store save/load roundtrip: entries persist correctly to disk |

## Verification

```
go vet ./...     # clean
go build ./...   # clean
go test -race    # all pass
```

## Reflection: Why these bugs were missed in v23

The v23 stale-entry-removal code correctly identified and deleted entries in memory, but missed the disk persistence path when `toEmbed` was empty. This is a classic "early return skips cleanup" pattern. The scanner buffer limit is a well-known Go gotcha with `bufio.Scanner`.

## Files Changed

| File | Changes |
|---|---|
| `internal/kbvector/store.go` | Save stale removal to disk when no new docs to embed |
| `internal/agents/kb/tools/kb_read.go` | 1MB scanner buffer for generateOverview |
| `internal/kbvector/store_test.go` | Persistence roundtrip test |

## Cross-references
- [[source-project-root-v23]] — Code review round 2 (where stale removal was introduced)
- [[source-project-root-v22]] — Code review round 1 (UTF-8, mutex copy)
