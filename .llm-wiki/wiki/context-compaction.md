---
type: concept
date: 2026-06-10
tags: [compaction, context, summary, memory]
---

# Context Compaction

> LLM-driven summarization of conversation history to prevent context window overflow.

## Two-Tier Compaction Strategy

Pi-Go uses a two-tier approach to manage context window pressure:

### Tier 1: [[micro-compact|MicroCompact]] (LLM-free)
- **Threshold**: context > 60% of window (`MicroCompactRatio`)
- **Action**: Replace old tool result content with placeholders
- **Cost**: Zero LLM calls
- **Keeps**: Last N tool results intact (`MicroKeepRecent`, default 5)

### Tier 2: Full Compact (LLM-driven)
- **Threshold**: context > ~90% of window (`ReserveTokens`)
- **Action**: LLM summarizes old history, keeps recent messages
- **Cost**: One LLM call

MicroCompact fires first, delaying the need for expensive full compaction.

## Full Compact Flow

1. **Split** — Full message history is split at `KeepRecentTokens` boundary:
   - `historyPart` — Older messages (to summarize)
   - `recentPart` — Recent messages (to keep intact)
2. **Summarize** — `SummarizeFunc` (any LLM call) generates a summary of the history part
3. **Persist** — A `compaction` entry is appended to the [[session-persistence]] store
4. **Replace** — The history part is replaced by the summary in subsequent contexts

## Configuration

```go
type Settings struct {
    Enabled          bool    // Default: true
    ReserveTokens    int     // Default: 16384 — tokens reserved for summary prompt
    KeepRecentTokens int     // Default: 20000 — recent tokens to preserve intact

    // MicroCompact (Tier 1)
    MicroCompactRatio float64 // Default: 0.6 — trigger MicroCompact at 60% of window
    MicroKeepRecent   int     // Default: 5 — keep last N tool results intact
}
```

## Manual Compaction

`Agent.CompactNow()` — Explicit trigger via:
- API endpoint `POST /sessions/{id}/compact`
- Slash command `/compact`

## Components

| Component | File | Purpose |
|-----------|------|---------|
| `compaction.Settings` | `internal/compaction/compaction.go` | Configuration |
| `compaction.SplitMessages()` | `internal/compaction/compaction.go` | Split at token boundary |
| `compaction.Compact()` | `internal/compaction/compaction.go` | Generate summary |
| `compaction.SummarizeFunc` | `internal/compaction/summary.go` | The LLM call signature |
| `Agent.CompactNow()` | `internal/agent/agent.go` | Entry point |

## Events

- `EventMicroCompacted` — Emitted after MicroCompact (Tier 1): `{ClearedResults, TokensBefore}`
- `EventCompacted` — Emitted on successful full compaction (Tier 2)
- `EventCompactionFailed` — Emitted on failure

## Related

- [[micro-compact]] — Tier 1 compaction (LLM-free, clears old tool results)
- [[agent-core]] — Agent holds CompactionSettings and SummarizeFunc; runs `maybeCompact()` after each turn
- [[tool-lifecycle-hooks]] — PreCompressHook fires before both tiers
- [[session-persistence]] — Compaction entries stored in session JSONL