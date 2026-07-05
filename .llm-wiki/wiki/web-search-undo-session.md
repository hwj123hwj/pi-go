# Web Search, Session Hardening & Undo System

> Source: `internal/tools/web_search.go`, `internal/tools/backup.go`, `internal/session/jsonl.go`
> Date: 2026-07-05

## Overview

Three major features implemented in this batch:

1. **Web Search Tool** — Agent can now search the web (SearXNG or DuckDuckGo)
2. **Session Crash-Safe Persistence** — fsync + crypto/rand IDs for durability
3. **Undo/Rollback System** — Auto-snapshot before file edits, `/undo` to revert

---

## 1. Web Search Tool (`web_search`)

### Architecture

```
Agent calls web_search("golang context tutorial")
         │
         ▼
  ┌──── engine selection ────┐
  │                           │
  ▼                           ▼
SEARXNG_URL set?         (fallback)
  │                           │
  ▼                           ▼
SearXNG JSON API          DuckDuckGo HTML
GET /search?q=...&format=json   GET /html/?q=...
  │                           │
  └───────┬───────────────────┘
          ▼
   []SearchResult{Title, URL, Snippet}
          │
          ▼
   Formatted text output for agent
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Dual engine (SearXNG + DDG) | SearXNG for self-hosted privacy; DDG as zero-config fallback |
| No API key required | DDG HTML parsing works without any key |
| SSRF protection reused | Shares `isPrivateHost()` with web_fetch |
| Max 20 results | Prevents context flooding |
| HTML parsing without deps | Pure string parsing, no goquery dependency |

### Configuration

```bash
# Option A: Self-hosted SearXNG (recommended)
export SEARXNG_URL=http://localhost:8080
export PI_GO_ENABLE_WEB_SEARCH=true

# Option B: DuckDuckGo fallback (zero config)
export PI_GO_ENABLE_WEB_SEARCH=true
```

---

## 2. Session Crash-Safe Persistence

### Problems Fixed

| Issue | Before | After |
|-------|--------|-------|
| **Data loss on crash** | No `fsync()`, OS buffer only | `f.Sync()` after every append |
| **ID collision** | `time.Now().UnixNano()` | `crypto/rand` + timestamp |
| **Half-written lines** | Possible on power failure | fsync ensures atomic writes |

### New ID Format

```
Before: entry_1757000000000000000
After:  entry_1757000000000_a1b2c3d4e5f6g7h8
                 timestamp     random_hex(16)
```

The random component eliminates collision risk even under rapid successive appends
or concurrent access.

---

## 3. Undo/Rollback System

### Architecture

```
Agent calls edit/write tool
         │
         ▼
BackupManager.Snapshot(path)
  ├── File exists? → Read content, store in stack
  └── File new? → Record "empty" snapshot
         │
         ▼
Tool modifies file
         │
         ▼
User: /undo
         │
         ▼
BackupManager.Restore(path)
  ├── Snapshot was content → Write back original
  └── Snapshot was empty → Delete file
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Per-file backup stack | Multiple undos per file (depth: 50) |
| In-memory only | Fast, no disk I/O for backups. Session-scoped. |
| Auto-snapshot in edit/write | Transparent — no agent prompt changes needed |
| "Empty" snapshot for new files | `/undo` on a created file deletes it |

### Usage

```
/undo             # Revert last file operation
/undo all         # Revert all files in this session
/undo list        # Show available backups
/undo clear       # Clear all backups
```

---

## Test Results

| Package | Tests | Status |
|---------|-------|--------|
| `internal/tools/` | 15+ (web_search + backup) | ✅ |
| `internal/session/` | Existing + crash-safety | ✅ |
| Full suite (30+ packages) | All | ✅ |

## New Slash Commands Summary

| Command | Feature |
|---------|---------|
| `/loop 5m <prompt>` | Watchdog recurring loop |
| `/task <goal>` | TASK.md handoff |
| `/undo` | File rollback |
| `/undo all` | Restore all files |
| `/undo list` | Show backups |
