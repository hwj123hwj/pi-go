---
type: concept
date: 2026-06-20
tags: [compaction, context, optimization, micro-compact]
---

# MicroCompact

> A lightweight, LLM-free compaction strategy that clears old tool results from conversation history to save tokens. Part of the two-tier compaction system in [[context-compaction]].

## Motivation

Full context compaction (calling an LLM to summarize) is expensive and slow. MicroCompact provides a cheap first pass: when tool results accumulate and push context usage past a moderate threshold, old tool results are replaced with placeholders — no LLM call needed.

## Two-Tier Compaction Strategy

```
contextTokens / contextWindow
│
├─ > 60% (MicroCompactRatio)  → MicroCompact (clear old tool results, no LLM)
│
└─ > ~90% (ReserveTokens)     → Full Compact (LLM summarization)
```

MicroCompact fires **before** full compaction, at a lower threshold. This delays the need for expensive LLM summarization.

## Configuration

```go
type Settings struct {
    // ... existing fields ...

    // MicroCompact
    MicroCompactRatio float64 // Default 0.6 — trigger when context > 60% of window
    MicroKeepRecent   int     // Default 5 — keep the N most recent tool results intact
}
```

## Behavior

When `ShouldMicroCompact()` returns true:

1. Walk message history from oldest to newest
2. For tool result messages older than the last `MicroKeepRecent` results:
   - Replace `Content` with a short placeholder (e.g., `"[tool result cleared to save tokens]"`)
   - Preserve `ToolCallID`, `ToolName`, `IsError` metadata
3. Emit `EventMicroCompacted{ClearedResults, TokensBefore}`

This is safe because:
- Recent tool results (last 5) are preserved for ongoing context
- Older results rarely matter for the current conversation
- The LLM can still see that a tool was called and what its name was

## Integration Point

MicroCompact runs inside `Agent.maybeCompact()` in `internal/agent/loop.go`:

```go
// Two-tier: MicroCompact first (low threshold), then Full Compact (high threshold)
if compaction.ShouldMicroCompact(contextTokens, contextWindow, a.compactionSettings) {
    newHistory, cleared := compaction.MicroCompact(history, a.compactionSettings.MicroKeepRecent)
    a.emit(ctx, EventMicroCompacted{ClearedResults: cleared, TokensBefore: contextTokens})
}
```

## Events

| Event | Field | Description |
|-------|-------|-------------|
| `EventMicroCompacted` | `ClearedResults int` | Number of tool results cleared |
| | `TokensBefore int` | Token count before clearing |

Stream event type: `micro_compacted` with `cleared_count` field.

## PreCompressHook

The [[tool-lifecycle-hooks|PreCompressHook]] fires **before** both MicroCompact and Full Compact, allowing extensions to observe context pressure.

## Related

- [[context-compaction]] — Parent compaction system; MicroCompact is the first tier
- [[agent-core]] — Agent runs `maybeCompact()` after each turn
- [[tool-lifecycle-hooks]] — PreCompressHook observes pre-compaction state
