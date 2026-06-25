# Source: Project Root (.) — v17: Unified user profile (cross-agent second brain)

> Date: 2026-06-26
> Focus: Cross-agent personalization via unified profile store, inspired by OpenViking's context philosophy.

## Summary

Created a unified `profile.Store` that serves as a "condensed second brain" shared across all agents. Any agent can record facts about the user, and every agent sees the same fixed-size summary in their system prompt. Inspired by OpenViking's L0/L1/L2 tiered context model and session-based memory extraction.

**Core insight**: The unified user profile IS the condensed second brain — it's a cross-domain, fixed-size projection of what the KB agent's deep knowledge base and the music agent's listening history have learned about the user.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  profile.Store (unified)                      │
│              {DataDir}/user_profile.json                      │
│                                                               │
│  coding: { language: "Go", os: "macOS", editor: "vim" }     │
│  music:  { total_plays: 142, artist:周杰伦: 50, ... }        │
│  general:{ location: "北京", timezone: "UTC+8" }             │
│                                                               │
│  Summary() → fixed ~80 tokens, NEVER grows:                   │
│  ## 用户画像                                                  │
│  - 开发：Go、macOS、vim                                       │
│  - 音乐：累计 142 首，常听：周杰伦、林俊杰、陈奕迅            │
│  - 通用：北京、UTC+8                                          │
└──────────┬──────────────────┬──────────────┬─────────────────┘
           │                  │              │
    ┌──────▼──────┐   ┌──────▼──────┐ ┌────▼──────┐
    │ CodingAgent │   │ MusicAgent  │ │ KB Agent  │
    │ + Profile   │   │ + PrefStore │ │ + Profile │
    │   injects   │   │   syncs →   │ │   injects │
    └─────────────┘   └─────────────┘ └───────────┘
```

### Data Flow

1. **Music agent** plays a song → `pref.Store.Record()` → `pref.Store` syncs top artists + total plays to `profile.Store`
2. **Coding agent** (future: LLM extraction) records "user prefers Go" → `profile.Store.Record("coding", "language", "Go")`
3. **KB agent** (future: LLM extraction) records "user in 北京" → `profile.Store.Record("general", "location", "北京")`
4. **Each agent sees only relevant categories** via `SummaryForCategories()`:
   - **Coding agent**: NO profile injection (relies on `.llm-wiki/` + project context — coding is domain-specific, no cross-domain memory needed)
   - **Music agent**: sees `music` + `general` only (doesn't need to know you use Go)
   - **KB agent**: sees ALL categories (it's the "second brain" that benefits from the full picture)

## OpenViking Concepts Adopted

| OpenViking | pi-go Implementation |
|---|---|
| L0/L1/L2 tiered loading | Tier 1 = `Summary()` (~80 tokens, always); Tier 2/3 = tool calls (on-demand) |
| Context self-iteration | Music prefs auto-sync to profile; future: session-based fact extraction |
| Unified context management | One `profile.Store` across all agents, not per-agent silos |
| Fixed-size context delivery | Summary never grows — capped at top 5 artists, top 10 facts per category |

## Implementation

### New: `internal/profile/store.go`

Thread-safe JSON-backed unified profile:
- Categories: `coding`, `music`, `general`
- `Record(category, key, value, source)` — upserts a fact
- `RecordBatch(category, source, items)` — batch upsert
- `Summary()` — returns fixed-size prompt string (~80 tokens)
- Max 10 facts per category, oldest evicted (LRU-like)
- Music category has special formatting: artist facts aggregated, top 5 shown
- Auto-loads from `{DataDir}/user_profile.json`, auto-saves on each Record

### Modified: Music pref store sync

`internal/music/pref/store.go`:
- Added `SetProfileSyncer()` — connects music pref store to unified profile
- Every `Record()` now also calls `syncToProfileUnlocked()` → pushes artist counts + total plays
- Music pref store remains the source of truth for detailed history; profile gets the aggregate

### Modified: All three agents

| Agent | File | Change |
|---|---|---|
| Coding | `application.go` + `prompt/builder.go` | **No profile injection** — coding agent relies on `.llm-wiki/` + project context. Coding is domain-specific; injecting user music taste or personal facts adds noise without value. |
| Music | `application.go` + `prompt/prompt.go` | Uses `SummaryForCategories(music, general)` — only sees music + general prefs |
| KB | `application.go` + `prompt/prompt.go` | Uses full `Summary()` — sees all categories (it's the "second brain") |

### Modified: Entrypoints

`cmd/pi-agent/main.go` and `cmd/pi-music/main.go`:
- Create a single `profile.Store` instance in `{DataDir}`
- Pass it to all three agents (coding, music, kb)

## Why It's a "Condensed Second Brain"

The KB agent has deep knowledge (800+ documents, full-text search). The music agent has detailed play history (500 records, ring buffer). But the **unified profile** is the *condensation* — the ~80 token essence of "who is this user" that any agent can use without context bloat.

```
KB Agent (deep) ──────┐
                       │    ┌──────────────┐
Music Agent (deep) ────┼───▶│ profile.Store│───▶ ~80 token Summary
                       │    │ (condensed)  │     injected into ALL prompts
Coding Agent (deep) ───┘    └──────────────┘
```

## Test Coverage

`internal/profile/store_test.go`: 11 tests covering record+summary, music formatting, top-5 cap, fact updates, eviction, persistence, removal, batch, cross-category, empty/corrupt recovery.

## Files Changed

| File | Change |
|---|---|
| `internal/profile/store.go` | NEW — unified profile store |
| `internal/profile/store_test.go` | NEW — 11 tests |
| `internal/music/pref/store.go` | Added ProfileSyncer integration |
| `internal/agents/coding/application.go` | Added profile support |
| `internal/agents/coding/prompt/builder.go` | Inject profile summary |
| `internal/agents/music/application.go` | Uses unified profile |
| `internal/agents/music/prompt/prompt.go` | Prefer unified profile over pref-only |
| `internal/agents/kb/application.go` | Added profile support |
| `internal/agents/kb/prompt/prompt.go` | Inject profile summary |
| `cmd/pi-agent/main.go` | Create + wire unified profile |
| `cmd/pi-music/main.go` | Create + wire unified profile |

## Third Code Review: Persistence Robustness & Concurrency

### Bug 7: Non-atomic file writes (data corruption risk)
- **Problem**: Both `profile.Store.save()` and `pref.Store.save()` called `os.WriteFile()` directly. If the process crashed mid-write (e.g. OOM kill, power loss, Electron force-quit), the JSON file would be truncated/partial — corrupting all user profile/history data permanently.
- **Fix**: Switched both stores to atomic writes: write to `.tmp` file, then `os.Rename()` (atomic on POSIX systems). Crash leaves either the old or new file, never a partial write.

### Test gap: No concurrent access coverage for profile.Store
- **Problem**: `profile.Store` is accessed by multiple agent sessions concurrently (e.g. music agent writes while KB agent reads Summary). No test verified thread safety.
- **Fix**: Added `TestConcurrentAccess` — 4 concurrent writers + 4 concurrent readers, 400 operations total, verified with `-race` detector.

### Design audit: no direction drift
- Coding agent: still no profile injection ✅
- Music agent: still domain-isolated (music + general only) ✅
- KB agent: still full profile ✅
- All write paths: now atomic ✅
- Concurrency: verified clean under `-race` ✅

### Bug 4: 16× disk writes per song play (performance, round 2)
- **Problem**: Previous fix reduced from O(N) to top-15, but each artist still called `profile.Record()` individually → 16 disk writes per play. With frequent play, this is still excessive I/O.
- **Fix**: Replaced 16 `Record()` calls with a single `RecordBatch()` call. Now **1 disk write per play**.

### Bug 5: formatCategory loses semantic keys (clarity)
- **Problem**: For coding/general categories, the summary joined raw values: `- 开发：Go、macOS`. This is ambiguous — is "Go" a language? A tool? An OS?
- **Fix**: Changed to `key: value` format: `- 开发：language: Go、os: macOS`. The key provides context.

### Code smell 6: ProfileSyncer wrapper indirection (simplification)
- **Problem**: `ProfileSyncer` was a struct wrapping a `profileSyncer` interface, stored as `*ProfileSyncer`. The struct added zero value — it just forwarded calls.
- **Fix**: Removed the wrapper struct entirely. `Store.profileSyncer` is now directly the interface type. Cleaner, less indirection.
- Also extended the `profileSyncer` interface to include `RecordBatch()` so batch sync uses a single write.

### Bug 1: N+1 disk writes per music play (performance)
- **Problem**: `syncToProfileUnlocked()` iterated ALL artists in `ArtistCounts` map and called `profile.Record()` for each. Each `Record()` does a full JSON marshal + `os.WriteFile`. With 200 unique artists, that's **200 disk writes per song play**.
- **Fix**: Only sync TOP 15 artists (refactored to use `topNWithCounts`). 15 is enough for the profile's top-5 summary display, and limits writes to ~16 per play.

### Bug 2: Profile eviction corrupts artist ranking (correctness)
- **Problem**: `profile.Store` had `maxPerCategory=10` for all categories. Music had up to 200 artist facts synced, so eviction kicked in — but eviction was by oldest timestamp, not by play count. This meant the top-5 artists shown in the summary could be wrong: a high-count artist synced early could be evicted in favor of a low-count artist synced recently.
- **Fix**: Set `maxPerCategoryMusic=20` for music (enough headroom for top-15 sync + total_plays). Added `maxForCategory()` method for per-category cap resolution.

### Code smell 3: Dead code in coding agent
- **Problem**: `NewCodingApplicationWithProfile()` was a no-op wrapper (coding agent doesn't use profile). It was called from `main.go` creating confusion.
- **Fix**: Removed `NewCodingApplicationWithProfile()`. Coding agent uses plain `NewCodingApplication()`.

### Design audit: Domain isolation confirmed correct
- Coding agent: **no profile injection** — relies on `.llm-wiki/` + project context ✅
- Music agent: `SummaryForCategories(music, general)` — only sees relevant domains ✅
- KB agent: full `Summary()` — sees all domains (it's the second brain) ✅
- [[source-project-root-v16]] — Music preference store (predecessor)
- [[music-agent]] — Music application (now syncs to unified profile)
- [[kb-agent]] — KB application (now receives profile summary)
- [[coding-application]] — Coding application (now receives profile summary)
- [[personal-assistant-roadmap]] — Memory layer design (this is a step toward it)
