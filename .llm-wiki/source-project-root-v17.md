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
4. **Every agent's system prompt** gets the same `profile.Summary()` — so coding agent knows you like 周杰伦, music agent knows you use Go

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
| Coding | `application.go` + `prompt/builder.go` | Added `Profile` field; injects `Summary()` into system prompt |
| Music | `application.go` + `prompt/prompt.go` | Uses unified profile (falls back to pref-only if profile is nil) |
| KB | `application.go` + `prompt/prompt.go` | Added `Profile` field; injects `Summary()` into system prompt |

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

## Cross-references
- [[source-project-root-v16]] — Music preference store (predecessor)
- [[music-agent]] — Music application (now syncs to unified profile)
- [[kb-agent]] — KB application (now receives profile summary)
- [[coding-application]] — Coding application (now receives profile summary)
- [[personal-assistant-roadmap]] — Memory layer design (this is a step toward it)
