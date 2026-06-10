---
type: entity
date: 2026-06-10
tags: [session, persistence, jsonl, storage]
---

# Session Persistence

> Append-only JSONL storage for agent conversations with tree branching.

## Storage Format

Uses JSONL (JSON Lines) format with 3 entry types:

| Entry Type | Purpose |
|------------|---------|
| `message` | A conversation message (user/assistant/tool) |
| `compaction` | A compacted summary entry (see [[context-compaction]]) |
| `checkpoint` | A snapshot point for branching |

## Tree Branching

Sessions support tree-structured navigation:
- `MoveTo(entryID)` — Branch to an entry, creating a new branch
- Forking allows exploration of alternative conversation paths
- `BuildContext()` — Rebuilds the full message history from storage

## Session Manager

The `sessionmgr` package provides:
- Session CRUD (create, list, get, delete)
- Cross-session operations
- Integration with [[runtime-application-interface]]

## Key Files

| File | Purpose |
|------|---------|
| `internal/session/session.go` | Session struct and tree operations |
| `internal/session/jsonl.go` | JSONL read/write with append-only |
| `internal/session/interface.go` | Session interface |
| `internal/sessionmgr/manager.go` | Session management |

## Related

- [[agent-core]] — Agent can attach a Session for persistence
- [[context-compaction]] — Compaction entries are stored in sessions