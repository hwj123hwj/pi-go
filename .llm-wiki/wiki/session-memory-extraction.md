---
type: concept
date: 2026-06-27
tags: [memory, extraction, openviking, async, llm, profile]
related: [unified-profile, personal-assistant-roadmap, agent-core, tool-lifecycle-hooks]
---

# Session Memory Extraction

> An OpenViking ExtractLoop adaptation that runs after each agent turn to extract
> user facts from the conversation and persist them to the [[unified-profile]].

## Overview

Implemented in v20, this feature gives agents a "memory" — after each conversation turn, an async (non-blocking) LLM call analyzes the exchange and extracts structured facts about the user (preferences, habits, environment). These facts are written to the `profile.Store`, where they become available to all agents via the fixed-size summary.

## Design

### Key Principles

1. **Non-blocking**: Extraction runs in a goroutine; the agent turn completes without waiting
2. **LLM-based**: Uses structured prompt to extract facts, not regex heuristics
3. **Scoped**: Only extracts facts about the *user*, not about code/projects
4. **Dedup-safe**: Uses `Record()` which upserts by category+key (incrementing access_count)

### Adaptation from OpenViking

| OpenViking | pi-go |
|-----------|-------|
| `ExtractLoop` background worker | Post-turn goroutine in session ext |
| VikingFS storage | `profile.Store` (local JSON) |
| Complex 8-subtype memory model | Simplified: coding/music/general categories |
| Vector-based recall | Fixed-size summary injection |

## Extraction Flow

```
AgentSession.Turn() completes
  └─ session_ext.OnTurnEnd()
       └─ go extractor.Extract(turn)
            ├─ Build extraction prompt (system + user messages from turn)
            ├─ LLM call (non-blocking, fire-and-forget)
            ├─ Parse JSON response → []ExtractedFact
            └─ profile.Store.RecordBatch(category, source, items)
```

### Extraction Prompt

The LLM is asked to extract user facts in JSON format:

```json
[
  {"category": "coding", "key": "language", "value": "Go"},
  {"category": "music", "key": "artist:周杰伦", "value": "playcount:42"},
  {"category": "general", "key": "location", "value": "北京"}
]
```

## Truncation Safety

Extracted fact values are rune-safe truncated to prevent UTF-8 byte splitting:

```go
func truncate(s string, maxLen int) string {
    runes := []rune(s)
    if len(runes) <= maxLen {
        return s
    }
    return string(runes[:maxLen]) + "..."
}
```

## Non-Blocking Guarantee

The extraction goroutine:
- Has its own `context.Background()` (not tied to the turn's context)
- Recovers from panics internally (won't crash the session)
- Errors are logged via `slog.Warn` but never surfaced to the user

## Code Locations

| File | Responsibility |
|------|----------------|
| `internal/profile/extractor.go` | LLM prompt, fact parsing, rune-safe truncation |
| `internal/agents/*/session_ext.go` | Per-agent hook that triggers extraction |

## Source

- [[source-project-root-v20]] — Initial implementation
- [[source-project-root-v22]] — UTF-8 truncation fix

## Related

- [[unified-profile]] — Where extracted facts are stored
- [[personal-assistant-roadmap]] — P1 memory layer (now implemented)
- [[tool-lifecycle-hooks]] — Extraction triggers on turn end
