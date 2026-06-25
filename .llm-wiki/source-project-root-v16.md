# Source: Project Root (.) — v16: Music personalization (preference store)

> Date: 2026-06-26
> Focus: User listening history persistence, fixed-size preference injection, personalized recommendations.

## Summary

Added a lightweight preference system to the music agent. The agent now remembers what the user listens to and uses this knowledge to personalize recommendations — without inflating the system prompt over time.

**Design principle: Storage and injection are decoupled.** Raw data grows (capped at 500 entries), but prompt injection is always a fixed-size string (~40 tokens) regardless of how much history exists.

## Architecture: Three-Tier Injection Model

```
┌──────────────────────────────────────────────────┐
│ Storage (disk JSON, ring buffer cap=500)         │
│ play_history[] + aggregates (artist/source/song)  │
│ Grows up to 500 entries, then ring buffer evicts │
└──────────────────┬───────────────────────────────┘
                   │
    ┌──────────────┼──────────────────┐
    ▼              ▼                  ▼
  Tier 1         Tier 2             Tier 3
  (system prompt) (tool result)     (tool result)
  ~40 tokens     ~100 tokens        ~200 tokens
  ALWAYS          on-demand          on-demand
```

- **Tier 1** (`Store.Summary()`): Injected into system prompt on every conversation. Fixed-size: total play count + top 5 artists. Never grows.
- **Tier 2/3** (`music_history` tool): Called by LLM when user asks "我常听什么" or "根据我的喜好推荐". Returns recent history + stats as tool output (enters conversation context, not system prompt, so it clears naturally).

## Implementation

### New: `internal/music/pref/store.go`

Thread-safe JSON-backed preference store:
- `Record(songID, name, artist, source)` — called by PlayTool after successful play
- `Summary() string` — returns fixed-size prompt string (Tier 1 injection)
- `HistoryDetail(limit)` — returns recent plays + top artists/songs (Tier 2/3 query)
- Ring buffer: max 500 records, oldest evicted
- Aggregates recomputed on each Record() — O(n) where n ≤ 500
- Auto-loads from `{DataDir}/music_pref.json` on startup, auto-saves on each Record
- Corrupt file → starts fresh (no panic)

### Modified: `internal/agents/music/`

| File | Change |
|---|---|
| `application.go` | Creates `pref.Store` in `DataDir`; passes to BuildTools and BuildPrompt |
| `prompt/prompt.go` | Injects `Pref.Summary()` between base prompt and tools section; added workflow entry for `music_history`; added interaction style hint to use preferences for personalization |
| `tools/tools.go` | Added `Pref` to `ListOptions`; added `NewHistoryTool`; updated `BaseToolNames()` |
| `tools/play.go` | Added `pref *pref.Store`; calls `pref.Record()` after successful play in `playByID()` |
| `tools/play_test.go` | Updated `NewPlayTool` signature to accept `nil` pref |

### New: `internal/agents/music/tools/history.go`

`music_history` tool — lets LLM query user's listening profile on demand. Returns: total plays, top 5 artists, top 5 songs, N most recent plays.

## Why Prompt Never Inflates

| Data | Storage Growth | Injection Growth |
|---|---|---|
| play_history | Linear, **capped at 500** (ring buffer) | Not injected |
| top artists | Map grows with 500-entry window | **Fixed top 5, always 5** |
| total_plays | One integer | One number |
| Recent plays | Stored in history | **Fixed recent N** |

Whether the user has played 10 songs or 10,000, the system prompt injection is always ~40 tokens.

## Test Coverage

`internal/music/pref/store_test.go`: 8 tests covering record+summary, fixed-size top 5, ring buffer eviction, JSON persistence + reload, history detail ordering, empty store, timestamps, and corrupt file recovery.

## Cross-references
- [[music-agent]] — Music application architecture
- [[source-project-root-v15]] — Music player deep-audit round 2 (audio proxy hardening)
- [[prompt-system]] — How system prompts are constructed
