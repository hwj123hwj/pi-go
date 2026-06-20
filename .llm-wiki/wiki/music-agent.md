---
type: entity
date: 2026-06-20
tags: [music, agent, application, netease, streaming]
---

# Music Agent

> The second application layer in pi-go, built on NetEase Cloud Music. Parallel to [[coding-application]] in the [[four-layer-architecture]].

## Overview

The music agent is a full [[runtime-application-interface|Application]] implementation that lets users search, play, and manage music through natural conversation. It integrates with NetEase Cloud Music APIs and serves audio through the pi-go HTTP server.

## Architecture

```
┌─────────────────────────────────────┐
│  MusicApplication                   │
│  (implements runtime.Application)   │
├─────────────────────────────────────┤
│  6 Music Tools                      │
│  search / play / lyrics / detail    │
│  playlist / recommend               │
├─────────────────────────────────────┤
│  Music Service Layer                │
│  netease.Client + Cache             │
│  HTTP Handler (audio/lyrics proxy)  │
└─────────────────────────────────────┘
```

## Music Tools (6)

| Tool | File | Description |
|------|------|-------------|
| `music_search` | `search.go` | Search songs by keyword |
| `music_play` | `play.go` | Play a song (returns audio URL) |
| `music_lyrics` | `lyrics.go` | Get lyrics for a song |
| `music_detail` | `detail.go` | Get song metadata |
| `music_playlist` | `playlist.go` | Get playlist contents |
| `music_recommend` | `recommend.go` | Get personalized recommendations |

Tools are defined in `internal/agents/music/tools/` and support the same AllowedTools/BlockedTools filtering as [[coding-application]].

## MusicApplication

```go
type MusicApplication struct {
    Cfg     config.Config
    Client  *netease.Client
    Cache   *music.Cache
}
```

Implements `runtime.Application`:
- `BuildTools()` — Assembles the 6 music tools
- `BuildPrompt()` — Constructs music-agent system prompt
- `NewSessionExt()` — Creates per-session extension ([[runtime-application-interface|SessionExt]])
- `ToolNames()` — Returns canonical tool names

## Per-Session Extension

`MusicSessionExt` implements `runtime.SessionExt` with:
- Goal support (set/clear triggers agent rebuild)
- Single "default" profile (no profile switching for now)
- Rebuild callback for goal changes

## Music Service Layer

Located in `internal/music/`:

| Component | File | Purpose |
|-----------|------|---------|
| `netease.Client` | `music/netease/` | NetEase Cloud Music API client |
| `Cache` | `music/cache.go` | In-memory cache with TTL (audio URLs, lyrics) |
| `Handler` | `music/handler.go` | HTTP endpoints for audio/lyrics proxy |

### HTTP Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/music/audio/{song_id}` | Proxies audio stream from NetEase |
| `GET` | `/music/lyrics/{song_id}` | Returns LRC lyrics as JSON |

Audio proxy strategy: outer URL first, enhance API as fallback. Includes proper Referer/User-Agent headers for NetEase compatibility.

## Integration

The music agent is integrated into `pi-agent` via per-session application mode — the same process can serve both coding and music sessions. Audio is served on the same HTTP port as the main server.

## Related

- [[runtime-application-interface]] — MusicApplication implements this interface
- [[coding-application]] — Parallel application layer
- [[four-layer-architecture]] — Music agent lives in the Application layer
- [[agent-core]] — Music tools are registered in the Agent
- [[config-system]] — Music config via environment variables
