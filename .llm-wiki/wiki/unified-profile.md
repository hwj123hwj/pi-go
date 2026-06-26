---
type: concept
date: 2026-06-27
tags: [profile, memory, user-profile, personal-assistant, openviking, cross-agent]
related:
  [
    [personal-assistant-roadmap],
    [kb-agent],
    [music-agent],
    [session-memory-extraction],
    [config-system],
    [desktop-app],
    [server-websocket],
  ]
---

# Unified User Profile

> A cross-agent, persistent user profile store that acts as a "condensed second brain".
> Any agent (coding, music, kb) can record facts about the user, and every agent sees
> the same fixed-size summary in their system prompt.

## Overview

The unified profile was adapted from [OpenViking's](https://github.com/volcengine/OpenViking) context philosophy and implemented in v17–v18. It solves the "agent doesn't remember the user" problem without requiring an external vector database.

### Design Principles (from OpenViking)

1. **Storage/injection decoupled** — facts accumulate unboundedly; summary is always fixed-size
2. **Category-based organization** — coding, music, general
3. **Last-write-wins per key** — agents update facts about the same topic
4. **Max items per category** — prevents unbounded growth

## Data Model

```go
type Fact struct {
    Key         string    // Unique within category, e.g. "language", "artist:周杰伦"
    Value       string    // e.g. "Go", "playcount:42"
    Source      string    // Which agent recorded this, e.g. "music-agent"
    Updated     time.Time
    AccessCount int       // How many times this fact was read/updated
}

type Store struct {
    mu       sync.Mutex
    filePath string
    facts    map[string]map[string]Fact // category → key → Fact
}
```

### Categories

| Category | Constant | Max Items | Who Sees It |
|----------|----------|-----------|-------------|
| `coding` | `CategoryCoding` | 10 | (not yet injected into coding agent) |
| `music` | `CategoryMusic` | 20 | Music agent (via `SummaryForCategories`) |
| `general` | `CategoryGeneral` | 10 | KB agent + Music agent |

## Hotness-Based Eviction

When a category exceeds its limit, the fact with the lowest **hotness score** is evicted.

### Formula

```
hotness = frequency × recency

frequency = 1 / (1 + e^(-log1p(access_count)))      // sigmoid
recency   = e^(-decayRate × ageDays)                  // exponential decay

decayRate = ln(2) / halfLifeDays     // halfLifeDays = 7
```

- Frequency: bounded [0, 1) via sigmoid of log(access_count)
- Recency: exponential decay with 7-day half-life

### Batch Handling

`RecordBatch` calls `evictToLimit()` which loops `evictLowestHotness()` until within limit — handles batch inserts where multiple items may push over the limit.

## Fixed-Size Summary

The `Summary()` method produces a **compact string** (~80 tokens) regardless of stored fact count:

```
## 用户画像
- 开发：language: Go、os: macOS、editor: VS Code
- 音乐：累计 142 首，常听：周杰伦、林俊杰、陈奕迅
- 通用：location: 北京、language: 中文
```

### Per-Agent Visibility

Each agent only sees categories relevant to its domain:

| Agent | Method | Categories |
|-------|--------|------------|
| KB agent | `Summary()` | coding + music + general |
| Music agent | `SummaryForCategories(music, general)` | music + general |
| Coding agent | (not injected — relies on `.llm-wiki` + project context) | — |

## REST API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/profile` | GET | All categories + facts + summary string |
| `/profile` | DELETE | Remove a specific fact by category+key |

### GET /profile Response

```json
{
  "categories": [
    {
      "name": "coding",
      "label": "开发",
      "count": 5,
      "facts": [
        {
          "key": "language",
          "value": "Go",
          "source": "coding-agent",
          "updated": "2026-06-27T10:00:00Z",
          "access_count": 3
        }
      ]
    }
  ],
  "summary": "## 用户画像\n- 开发：language: Go...",
  "total_facts": 12
}
```

## Desktop Visualization

The [[desktop-app]] includes a **Profile Panel** (`ProfilePanel.tsx`) in the right sidebar that:
- Shows all facts grouped by expandable categories
- Renders the exact agent-injected summary (Markdown)
- Displays per-fact metadata: source agent, recency, access count
- Supports hover-reveal deletion of individual facts

## Persistence

- **Atomic writes**: writes to `.tmp` file, then `os.Rename` (prevents corruption on crash)
- **File location**: `{DataDir}/user_profile.json`
- **Format**: JSON with `facts` map

## Code Locations

| File | Responsibility |
|------|----------------|
| `internal/profile/store.go` | `Store`, `Record`, `RecordBatch`, `Summary`, `AllFacts`, eviction |
| `internal/profile/extractor.go` | LLM-based fact extraction from conversations |
| `internal/server/profile_handler.go` | REST API (`GET /profile`, `DELETE /profile`) |
| `internal/app/app.go` | `App.Profile()` getter |
| `cmd/pi-agent/main.go` | Creates and injects the profile store |
| `desktop/src/components/workspace/ProfilePanel.tsx` | Desktop UI |

## Relationship to Session Memory Extraction

Facts are populated by [[session-memory-extraction]] — an async, non-blocking LLM call that runs after each agent turn to extract user facts and write them to this store.

## Source

- [[source-project-root-v17]] — Initial implementation
- [[source-project-root-v25]] — REST API + desktop panel

## Related

- [[personal-assistant-roadmap]] — P1 memory layer (now implemented)
- [[session-memory-extraction]] — How facts get into the profile
- [[kb-agent]] — KB agent injects full profile into system prompt
- [[music-agent]] — Music agent injects music + general categories
- [[server-websocket]] — `/profile` REST endpoints
