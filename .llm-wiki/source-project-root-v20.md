# Source: Project Root (.) — v20: Session memory extraction (OpenViking ExtractLoop adaptation)

> Date: 2026-06-26
> Focus: Automatic user fact extraction from conversations, adapted from OpenViking's SessionCompressorV2.

## Summary

Implemented session-based memory extraction. After each conversation, the system scans recent messages and uses the LLM to extract persistent user facts (coding preferences, general info, music taste). Extracted facts are written to the unified `profile.Store`, becoming available to all agents.

This is the adaptation of OpenViking's `extract_long_term_memories()` — but dramatically simplified for pi-go's single-user desktop model.

## OpenViking vs pi-go Design Comparison

| Aspect | OpenViking | pi-go |
|---|---|---|
| Extraction engine | ExtractLoop with VLM + memory templating | Single LLM call with JSON output |
| Memory types | trajectory, experience, skill, identity, soul | user facts only (coding, music, general) |
| Trigger | Session commit (explicit API call) | Post-conversation (async goroutine) |
| Concurrency | Distributed locks (AGFS) | Single mutex (desktop = single user) |
| Storage | VikingFS virtual filesystem | Local JSON (profile.Store) |
| Cost control | Multi-phase extraction with isolation | Truncate to 4K chars, 15s timeout, skip tool messages |

## Architecture

```
Conversation ends
       │
       ▼
AgentSession.Prompt/PromptStream returns
       │
       ▼ (caller: server/ws handler)
SessionExtractor.ExtractAsync(recentMessages)
       │
       ▼ (goroutine, non-blocking)
1. Filter: skip tool messages, truncate each msg to 500 chars
2. Build transcript (max 4K chars total)
3. LLM call: "extract user facts as JSON"
4. Parse JSON response
5. profile.Record() each fact
```

## Implementation

### New: `internal/profile/extractor.go`

- `SessionExtractor` struct with `LLMCaller` interface
- `ExtractFromMessages()` — scans messages, calls LLM, records facts
- `ExtractAsync()` — non-blocking wrapper (goroutine + timeout)
- `extractJSON()` — robust JSON extraction from LLM responses (handles markdown wrapping)
- `LLMCaller` interface: `CallSimple(ctx, system, user) (string, error)` — decouples from Provider

### New: `internal/profile/extractor_test.go`

8 tests: JSON extraction, truncation, extraction with facts, no-facts case, tool-skip, nil LLM safety, invalid category filtering.

### Key Design Decisions

1. **Only extract USER facts, not agent execution patterns**: OpenViking extracts trajectories and experiences (how the agent solved problems). We don't need that — pi-go is a personal assistant, not a multi-agent system.

2. **Non-blocking**: `ExtractAsync()` runs in a goroutine. The user never waits for extraction.

3. **LLMCaller interface**: The extractor doesn't import `ai/providers`. Any system that can make a simple text completion call can provide facts. This keeps the profile package dependency-free.

4. **JSON output format**: Instead of OpenViking's complex memory templating system, we use a simple JSON schema: `{"facts": [{"category": "coding", "key": "language", "value": "Go"}]}`.

## How It Connects to the Runtime

The `SessionExtractor` is designed to be called from:
- `AgentSession.Close()` — when a session closes, extract from recent history
- Post-turn hook — after each user turn, schedule extraction (debounced)

Currently, the `LLMCaller` interface is defined but not yet wired to a concrete provider. The wiring will happen in the App layer when integrating with the server. The extractor itself is fully functional and tested.

## Cross-references
- [[source-project-root-v17]] — Unified profile store (where facts are stored)
- [[source-project-root-v19]] — KB vector search (SiliconFlow bge-m3)
- [[source-project-root-v18]] — Hotness eviction (facts scored by frequency × recency)
