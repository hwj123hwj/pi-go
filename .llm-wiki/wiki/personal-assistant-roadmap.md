---
type: concept
date: 2026-06-27
tags: [roadmap, memory, personal-assistant, architecture, implemented]
related: [[four-layer-architecture]], [[runtime-application-interface]], [[music-agent]], [[coding-application]], [[unified-profile]], [session-memory-extraction]]
---

# Personal Assistant Roadmap

> pi-go is evolving from a coding assistant to a **general personal assistant** platform. This page tracks the direction, memory layer design, and phased rollout.

## Direction

The product direction is explicitly: pi-go should become a **通用个人助手** (general personal assistant), not just a coding tool. Music agent was the first non-coding agent, validating the `runtime.Application` plugability.

Future agents planned: accounting (记账), health (健康), diary (日记). Core need: "collect personal habits" (收集个人习惯).

## Architecture Readiness

**Adding new agents**: ✅ Ready. The `Applications map[string]runtime.Application` + `ResolveApplication(name)` pattern in `app.go` is the correct extension point. Music agent validated this.

**Three gaps identified**:

| Gap | Current State | Impact |
|-----|---------------|--------|
| No memory layer | `MusicSessionExt` only has goal/rebuild; `sessionmgr` only manages single session JSONL; `runtime.Application` has no Memory abstraction | Agent is always "one-shot tool", doesn't remember what user did yesterday |
| Frontend agent list hardcoded | `Sidebar.tsx` hardcodes "Coding"/"Music", but backend has `GET /applications` | Adding agent requires frontend code change |
| Panel system hardcoded for coding | `PaneKind` only has chat/diff/plan/tasks/terminal/file; music player/lyrics hacked into `ChatPane` | Accounting needs tables, health needs charts — unsustainable |

## Memory Layer Design

Inspired by [OpenViking](https://github.com/volcengine/OpenViking) — adopt its data model and layered approach, but pi-go only defines thin abstraction interface.

### OpenViking Key Concepts

> **Implementation Note (v17–v25)**: The original interface design (`runtime.Memory` with `Recall/Record`) was **superseded** by a simpler approach — `profile.Store` with category-based facts and a fixed-size `Summary()` string. See [[unified-profile]] for the actual implementation.

**Three context types**:
- **Resource**: User-added external knowledge (documents, manuals)
- **Memory**: Agent's cognition about user/world (auto-extracted) ← "personal habit collection" lives here
- **Skill**: Callable capabilities (already covered by pi-go's tool/skill system)

**Memory has 8 subtypes** (user/agent split):

| Subtype | Purpose | Music Example |
|---------|---------|---------------|
| profile | User basic info | Single file merge |
| **preferences** | Topic-specific preferences | "Likes Jay Chou / electronic / late-night listening" |
| entities | Entity memories (people, projects) | Frequent artists, playlists |
| events | Event records (decisions, milestones) | "First heard X because of Y" |
| trajectories | Reusable operation contracts | — |
| experiences | Reusable execution insights | — |
| tools | Tool usage knowledge | — |
| skills | Skill execution knowledge | — |

**L0/L1/L2 three-tier hierarchy** (load on demand to save tokens):
- L0 (Abstract): One-sentence summary, ~100 tokens
- L1 (Overview): Core info + usage scenarios, ~2k tokens
- L2 (Details): Full text, loaded only when deep-reading

This is **different dimension** from compaction (MicroCompact/AutoCompact). Compaction = "intra-session history cleanup"; L0/L1/L2 = "cross-session memory storage hierarchy".

### Interface Design

```go
// internal/runtime/memory.go
type Memory interface {
    Recall(ctx context.Context, userID, namespace, query string) ([]MemoryEntry, error)
    Record(ctx context.Context, userID, namespace string, session SessionTelemetry) error
}

type MemoryEntry struct {
    Category string // profile/preferences/entities/...
    Content  string
    L0       string // summary (if backend provides)
}
```

Key design points:
- `namespace` separates agents (e.g. `music`/`coding`), but memory can be cross-agent searched under same `userID`
- Interface intentionally thin (only Recall/Record), all storage/vectorization delegated to backend
- Default implementation is `NilMemory` (no-op) — memory is enhancement, not blocker

### Application Integration

Optional interface (duck-typing, like `ToolWithConfirmation`):

```go
type ApplicationWithMemory interface {
    MemoryNamespace() string // e.g. "music"
}
```

`AgentSession` calls Memory at two points:
1. Before prompt build: `Recall` → inject preferences into prompt
2. After turn ends: `Record` → hand off interaction data for async extraction

## Phased Rollout

| Phase | Goal | Key Deliverable | Not Doing |
|-------|------|-----------------|-----------|
| **P0** | ~~Fix music hard bugs~~ | ✅ Done (v14–v15) | — |
| **P1** | ~~Memory layer (core)~~ | ✅ **Done (v17–v20)**: `profile.Store` (local JSON), [[unified-profile]], [[session-memory-extraction]], hotness eviction, KB vector search | — |
| **P1.5** | ~~Upgrade to semantic memory~~ | ✅ Done (v19): SiliconFlow bge-m3 hybrid search implemented directly (no OV SDK needed) | — |
| **P2** | Agent self-description + UI adaptation | Application declares metadata (icon/category/panel declarations); panel system supports agent-declared panels | New agents |
| **P3** | Extend personal agents | Second personal agent (accounting/diary), validate P1/P2 abstractions | — |

## Trigger Conditions

| Signal | Action |
|--------|--------|
| Music seek fails / NetEase API down crashes service | Start P0 |
| User complains "it doesn't remember what I like" (2nd time) | Start P1 memory layer (local JSON) |
| Local JSON sufficient but needs "semantic association" or memory volume too large | Start P1.5, switch OV backend |
| Adding 3rd agent, frontend sidebar needs code change | Start P2 frontend dynamic |
| New agent needs non-chat/diff panel | Start P2 panel system |

## Explicit Non-Goals

- **Don't build own vector DB** — heavy lifting delegated to OV
- **Don't pre-build abstractions for "general"** — P2 panel system waits for real need
- **Don't force all agents to have memory** — `ApplicationWithMemory` is optional
- **Don't stuff memory into session persistence** — session JSONL is intra-session history, memory is cross-session cognition, kept separate
- **Don't do cross-agent memory in P1** — interface leaves `userID`+`namespace` hook, but cross-agent retrieval is P3+

## Source

- [docs/decisions/personal-assistant-roadmap.md](../../docs/decisions/personal-assistant-roadmap.md) — Full decision document
- Related: [[skills-vs-application.md|skills-vs-application]] — Application as abstraction already decided; this extends to memory and self-description
