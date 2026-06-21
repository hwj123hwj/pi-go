---
type: entity
date: 2026-06-22
tags: [session, management, persistence, forking, metadata]
related: [[session-persistence]], [[runtime-application-interface]], [[server-websocket]]
---

# Session Manager

> Storage/indexing layer for agent sessions. Handles file-based CRUD, forking, listing, and metadata persistence.
> Distinct from `runtime.SessionRegistry` which manages in-memory runtime session routing.

## Package

`internal/sessionmgr/manager.go`

## Storage Layout

```
{dataDir}/
  sessions/
    sess_1234567890/
      session.jsonl    ← JSONL conversation data (managed by session.Session)
      meta.json        ← Session metadata (workspace path)
    sess_1234567891/
      session.jsonl
      meta.json
```

Each session is a directory containing:
- `session.jsonl` — Append-only JSONL conversation storage (see [[session-persistence]])
- `meta.json` — Metadata like `{"workspace": "/path/to/project"}`

## Manager API

```go
type Manager struct { dataDir string }

func NewManager(dataDir string) *Manager
func (m *Manager) Create(ctx) (id, path, error)        // New session directory + JSONL init
func (m *Manager) Open(ctx, id) (*Session, path, error) // Load existing session
func (m *Manager) Fork(ctx, sourceID, entryID) (id, path, error) // Copy + optional branch
func (m *Manager) List(ctx) ([]SessionInfo, error)      // All sessions sorted by LastActive
func (m *Manager) Delete(id) error                      // Remove session directory
func (m *Manager) Exists(id) bool                       // Check existence
func (m *Manager) SaveMeta(id, workspace) error         // Write meta.json
func (m *Manager) SessionsDir() string                  // Base sessions directory
func (m *Manager) SessionPath(id) string                // JSONL file path
```

## SessionInfo

Metadata returned by `List()`:

```go
type SessionInfo struct {
    ID           string `json:"id"`
    CreatedAt    int64  `json:"created_at"`
    MessageCount int    `json:"message_count"`
    LastActive   int64  `json:"last_active"`
    Workspace    string `json:"workspace,omitempty"`
}
```

- `CreatedAt` / `LastActive` derived from directory mod time and JSONL timestamps
- `MessageCount` counted by scanning JSONL for `type == "message"` entries
- `Workspace` read from `meta.json`

## Forking

`Fork(sourceID, entryID)` creates a new session by:
1. Creating a new session directory
2. Copying the source JSONL file
3. If `entryID` is specified, calling `MoveTo(entryID)` to branch at that point

This enables conversation branching — exploring alternative paths from a specific point.

## Relationship to SessionRegistry

| Aspect | `sessionmgr.Manager` | `runtime.SessionRegistry` |
|--------|---------------------|--------------------------|
| **Purpose** | Persistence & indexing | In-memory runtime routing |
| **Storage** | File system (JSONL + meta.json) | `map[string]*AgentSession` |
| **Operations** | Create, Open, Fork, List, Delete | Get, Create, Load, Delete, List |
| **Used by** | `App.SessionManager()` | `App.SessionStore()` |
| **Thread safety** | File-level (OS handles) | `sync.Mutex` protected |

Both are injected into `App` and used together: the server calls `SessionRegistry.Load()` which internally uses `sessionmgr.Manager.Open()` to load the JSONL data.

## Key Design Decisions

1. **Directory-per-session** — Each session gets its own directory, making cleanup (Delete) a simple `os.RemoveAll` and enabling metadata files alongside the JSONL.
2. **ID format** — `sess_{unix_nano}` — time-ordered, unique, human-readable.
3. **meta.json separation** — Workspace metadata stored separately from conversation JSONL, allowing quick listing without parsing the full conversation.
4. **No database** — Pure filesystem storage, consistent with the project's lightweight philosophy.

## Related

- [[session-persistence]] — JSONL format and tree branching details
- [[runtime-application-interface]] — AgentSession uses Manager for persistence
- [[server-websocket]] — Server endpoints use Manager for session CRUD
- [[desktop-app]] — Frontend lists sessions via `GET /sessions` which calls `Manager.List()`
