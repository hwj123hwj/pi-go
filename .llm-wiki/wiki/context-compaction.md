---
type: concept
date: 2026-06-22
tags: [compaction, context, summary, memory, cross-framework]
related: [[micro-compact]], [[agent-core]], [[tool-lifecycle-hooks]], [[session-persistence]], [[competitive-analysis]]
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
- Custom instructions: `/compact <text>` passes to `SummarizeFunc` as `customInstructions`

## Cross-Framework Comparison

Based on analysis of Pi (TypeScript), Claude Code, Codex CLI (Rust), and DeepV Code:

| Dimension | Pi (TS) | Claude Code | Codex CLI | DeepV Code | **pi-go** |
|-----------|---------|-------------|-----------|------------|-----------|
| Trigger | Manual + threshold + overflow | Manual + auto(2-step) | Manual + Pre-turn + Mid-turn | 3-tier progressive (70%/80%/90%) | Manual + auto threshold |
| Algorithm | LLM summary + incremental | LLM summary (fresh each time) | LLM handoff + user msg keep | MicroCompact + LLM + Emergency | MicroCompact + LLM summary |
| Summary format | Goal/Progress/Decisions/Next | Free format | summary_prefix + user msgs | XML analysis+summary+snapshot | Free format |
| Split turn | ✅ Parallel dual summary | ❌ | ❌ | ❌ | ❌ |
| Incremental summary | ✅ previousSummary + UPDATE | ❌ | ❌ | ❌ | ❌ |
| File tracking | ✅ From toolCall | ❌ | ❌ | ✅ Post-Compact Restoration | ❌ |
| Micro-cost | ❌ | ❌ | ❌ | ✅ MicroCompact (zero LLM) | ✅ MicroCompact |
| Hooks | session_before/compact | ❌ | PreCompact/PostCompact | Circuit breaker (3 fails stop) | PreCompressHook |

### Design Insights

| Framework | Highlight | Philosophy |
|-----------|-----------|------------|
| **Pi** | Split turn + incremental + file tracking | Precise semantic compression |
| **Claude Code** | Cache-safe forking + 2-step auto | Pragmatic: zero-cost cleanup first |
| **Codex** | Mid-turn + remote encryption | Server-first, compress anytime |
| **DeepV** | MicroCompact + 3-tier progressive + file restore | Zero-cost buffer + safety net |

### pi-go Future Enhancements (from cross-framework analysis)

| Priority | Enhancement | Source |
|----------|-------------|--------|
| P0 | File operation tracking (extract read/modified files from toolCall) | Pi |
| P0 | Incremental summary (previousSummary + UPDATE prompt) | Pi |
| P0 | Overflow recovery (detect LLM overflow error → trigger compaction) | Codex |
| P1 | Post-Compact File Restoration (re-read recent files after compaction) | DeepV |
| P1 | Split Turn handling (dual summary when cut point lands in assistant message) | Pi |
| P2 | Circuit breaker + PTL degradation (3 fails → stop, truncate 20% per retry) | DeepV |
| P2 | Mid-turn compaction (check context ratio during tool call loop) | Codex |

## Components

| Component | File | Purpose |
|-----------|------|---------|
| `compaction.Settings` | `internal/compaction/compaction.go` | Configuration |
| `ShouldCompact()` | `internal/compaction/compaction.go` | Full compact threshold check |
| `ShouldMicroCompact()` | `internal/compaction/compaction.go` | MicroCompact threshold check |
| `Compact()` | `internal/compaction/compaction.go` | Generate summary via SummarizeFunc |
| `SummarizeFunc` | `internal/compaction/summary.go` | LLM call signature (with customInstructions) |
| `Agent.CompactNow()` | `internal/agent/agent.go` | Manual entry point |
| `maybeCompact()` | `internal/agent/loop.go` | Auto-trigger after each turn |

## Events

- `EventMicroCompacted` — After MicroCompact (Tier 1): `{ClearedResults, TokensBefore}`
- `EventCompacted` — On successful full compaction (Tier 2)
- `EventCompactionFailed` — On failure

## Related

- [[micro-compact]] — Tier 1 compaction details
- [[agent-core]] — Agent holds CompactionSettings and SummarizeFunc
- [[tool-lifecycle-hooks]] — PreCompressHook fires before both tiers
- [[session-persistence]] — Compaction entries stored in session JSONL
- [[competitive-analysis]] — Full DeepV feature gap analysis
- [[goal-driven-loop]] — Goal preservation across compaction (planned)
