---
type: concept
date: 2026-06-10
tags: [compaction, context, summary, memory]
---

# Context Compaction

> LLM-driven summarization of conversation history to prevent context window overflow.

## How It Works

1. **Split** — Full message history is split at `KeepRecentTokens` boundary:
   - `historyPart` — Older messages (to summarize)
   - `recentPart` — Recent messages (to keep intact)
2. **Summarize** — `SummarizeFunc` (any LLM call) generates a summary of the history part
3. **Persist** — A `compaction` entry is appended to the [[session-persistence]] store
4. **Replace** — The history part is replaced by the summary in subsequent contexts

## Configuration

```go
type Settings struct {
    KeepRecentTokens int  // How many recent tokens to preserve
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

- `EventCompacted` — Emitted on successful compaction
- `EventCompactionFailed` — Emitted on failure

## Related

- [[agent-core]] — Agent holds CompactionSettings and SummarizeFunc
- [[session-persistence]] — Compaction entries stored in session JSONL