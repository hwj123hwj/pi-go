---
type: entity
date: 2026-06-23
tags: [music, agent, application, netease, bilibili, multi-source, quality-filtering, bilibili-primary]
related: [[runtime-application-interface]], [[coding-application]], [[four-layer-architecture]], [[agent-core]], [[config-system]], [[personal-assistant-roadmap]]
---

# Music Agent

> The second application layer in pi-go, supporting **multi-source music** (NetEase Cloud Music + Bilibili). Parallel to [[coding-application]] in the [[four-layer-architecture]].
> First non-coding agent, validating the [[personal-assistant-roadmap|personal assistant direction]].

## Overview

The music agent is a full [[runtime-application-interface|Application]] implementation that lets users search, play, and manage music through natural conversation. It integrates with **two music sources** (NetEase Cloud Music and Bilibili) via a unified `MusicSource` interface and `SourceRouter` multiplexer.

## Architecture

```
┌───────────────────────────────────────────────────┐
│  MusicApplication                                  │
│  (implements runtime.Application)                  │
├───────────────────────────────────────────────────┤
│  6 Music Tools                                     │
│  music_search / music_play / music_lyrics          │
│  music_detail / music_playlist / music_recommend   │
├───────────────────────────────────────────────────┤
│  SourceRouter (multi-source multiplexer)            │
│  ┌──────────────┐  ┌─────────────────┐            │
│  │ NetEaseAdapter│  │ BilibiliAdapter  │            │
│  │  (primary)    │  │  (fallback)      │            │
│  └──────┬───────┘  └────────┬────────┘            │
│         │                    │                      │
│  netease.Client        bilibili.Client              │
│         │                    │                      │
│    Cloud Music API      Bilibili wbi API            │
├───────────────────────────────────────────────────┤
│  Cache (TTL: audio 24h, lyrics 24h)                │
│  Handler (audio proxy + lyrics endpoints)           │
└───────────────────────────────────────────────────┘
```

## Multi-Source Design

### MusicSource Interface (`internal/music/source.go`)

Unified contract for all music backends:

```go
type MusicSource interface {
    Source() Source
    Search(ctx, query, limit) (*SearchResult, error)
    GetSongByID(ctx, rawID) (*Song, error)
    GetAudioURL(ctx, rawID) (string, error)
    GetLyrics(ctx, rawID) (*Lyrics, error)
    GetPlaylistDetail(ctx, rawID) (*PlaylistDetail, error)
    GetRankings(ctx) ([]RankingEntry, error)
    GetTopList(ctx, rawID) (*PlaylistDetail, error)
    GetNewSongs(ctx, limit) ([]Song, error)
    GetDailyRecommend(ctx) (*PlaylistDetail, error)
}
```

### SourceRouter (`internal/music/router.go`)

Multiplexes multiple `MusicSource` backends. Routes by **composite ID** format: `"netease:12345"` or `"bilibili:BV1qD4y1U7fs"`.

```go
type SourceRouter struct {
    sources  map[Source]MusicSource
    default_ Source  // netease
}
```

Key methods:
- `Resolve(src)` — Get source by name
- `ByCompositeID(id)` — Parse `"netease:12345"` → (source, rawID)
- `Search(ctx, query, limit, src)` — Search specific or default source
- All other methods delegate to the resolved source

### Composite IDs (`internal/music/types.go`)

Format: `"<source>:<rawID>"` — e.g., `"netease:576466"`, `"bilibili:BV1qD4y1U7fs"`

```go
func SourceID(source Source, rawID string) string { return string(source) + ":" + rawID }
func ParseSourceID(id string) (Source, string)    // defaults to netease if no ":"
```

### Sources

| Source | Constant | Backend | Role (v5) |
|--------|----------|---------|-----------|
| **Bilibili** | `SourceBilibili` | `bilibili.Client` (wbi-signed API) | **Primary playback** (default for `music_play`) |
| **NetEase** | `SourceNetease` | `netease.Client` (Cloud Music API) | **Recommendation engine** (newsong/ranking/top), lyrics, playlist, playback fallback |

> **v5 change** (2026-06-23): Bilibili is now the **primary playback source** with better coverage. NetEase remains for recommendations/rankings/lyrics (better discovery). `ParseSource("")` returns `SourceBilibili` (was `SourceNetease`). See [[source-project-root-v5]].

## Bilibili Source Implementation

### Client (`internal/music/bilibili/client.go`)

Handles Bilibili's **wbi-signed API** requests:
- **wbi signature**: MD5-hashed params + mixin key (refreshed daily from `/x/web-interface/nav`)
- **Mixin key shuffle**: Standard `wbiKeyMixinEncTab` permutation table
- **DASH audio extraction**: Selects highest-bandwidth audio stream from `/x/player/playurl`
- **Cookie initialization** (v5): `ensureCookies()` via `sync.Once`:
  1. Visit `bilibili.com` (basic cookies)
  2. GET `/x/frontend/finger/spi` → buvid3, buvid4 (fingerprint cookies)
  3. Visit `search.bilibili.com/all?keyword=1` (additional cookies: b_lsid, _uuid)
  4. Required since B站 search API rejects requests without valid fingerprint cookies

### Quality Filtering (`internal/music/bilibili/filter.go`)

Two-gate quality filter applied inside `Client.Search()` — all callers (adapter, playByQuery, cross-source fallback) benefit automatically.

**Gate 1 — Blacklist** (hard rule, never relaxed):
Removes titles containing:
- Teaching/sheet music: 教学, 教程, 鼓谱, 钢琴谱, 吉他谱, 简谱, 跟练...
- Reaction/commentary: reaction, react, 首次反应, 听歌反应...
- Mashup/compilation: 合集, 串烧, medley, mashup...

Does NOT blacklist: cover/翻唱/学唱 (legitimate playable), 伴奏 (KTV versions are valid).

**Gate 2 — Same-name check** (soft, OR relation):
Extracts song name candidates from query:
- English: keeps whole phrases (e.g., "love story" stays as one token, NOT split into "love"/"story")
- Chinese: segments by punctuation, requires ≥2 characters per candidate

**Two-pass fallback**: If Pass 1 (blacklist ∩ same-name) is empty, relaxes to Pass 2 (blacklist only). Blacklist is NEVER relaxed — returning empty is preferred over playing teaching/reaction videos.

## Cross-Source Fallback

`music_play` tool implements automatic cross-source fallback:

```
1. Search in preferred source (default: bilibili since v5)
2. Try up to 5 results — if audio URL fetch fails...
3. Fall back to NetEase search
4. Try first NetEase result
5. Mark as fallback in response
```

> **v5 change** (2026-06-23): Fallback direction **reversed**. Was: netease primary → bilibili fallback. Now: bilibili primary → netease fallback. See [[source-project-root-v5]].

The `PlayDetails` struct includes `IsFallback: true` and `Source: "bilibili"` so the frontend can display the source.

## Music Tools (6)

| Tool | File | Description |
|------|------|-------------|
| `music_search` | `search.go` | Search songs by keyword (supports `source` param) |
| `music_play` | `play.go` | Play a song by ID or query; cross-source fallback; returns structured `PlayDetails` |
| `music_lyrics` | `lyrics.go` | Get LRC lyrics |
| `music_detail` | `detail.go` | Get song metadata |
| `music_playlist` | `playlist.go` | Get playlist contents |
| `music_recommend` | `recommend.go` | Get recommendations (netease: daily recommend; bilibili: music ranking) |

Tools support `AllowedTools`/`BlockedTools` filtering (same as [[coding-application]]).

## MusicApplication

```go
type MusicApplication struct {
    Cfg    config.Config
    Router *music.SourceRouter   // ← multi-source router (was *netease.Client)
    Cache  *music.Cache
}
```

Implements `runtime.Application`:
- `BuildTools()` — Assembles 6 music tools via `musictools.BuildList()`
- `BuildPrompt()` — Music-agent system prompt
- `NewSessionExt()` — Per-session extension
- `ToolNames()` — Canonical tool names

## Per-Session Extension

`MusicSessionExt` implements `runtime.SessionExt`:
- Goal support (set/clear triggers agent rebuild)
- Single "default" profile (no profile switching for now)
- Rebuild callback for goal changes

## HTTP Handler

`Handler` in `internal/music/handler.go` provides audio proxy and lyrics:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/music/audio/{song_id}` | Proxies audio stream (multi-source, Range support) |
| `GET` | `/music/lyrics/{song_id}` | Returns LRC lyrics as JSON |

Audio proxy features:
- **Multi-source routing**: Parses composite ID (supports `_` separator for URL paths)
- **Per-source Referer**: `music.163.com` for NetEase, `bilibili.com` for Bilibili
- **Range header passthrough**: Supports seek/scrub in players
- **Cache**: Audio URLs cached 24h, lyrics cached 24h

### NetEase Audio URL Strategy (v5 fix)

NetEase blocks HEAD requests. Updated `GetAudioURL` strategy:

1. **GET + Range** on outer URL (`Range: bytes=0-1023`) — was HEAD, now uses GET with small range
2. **Multi-bitrate enhance/player API** — tries 320k → 192k → 128k sequentially (was single 320k attempt)
3. **Outer URL unchecked** — returns outer URL as last resort even if `checkURL` fails; proxy handler attempts it directly

`checkURL` validates 200 (must be audio/mpeg content-type), 206 (partial content), 302/301 (CDN redirect). Non-audio 200 responses (HTML error pages) are rejected.

Simple TTL cache with source-prefixed keys:
- `AudioKey("netease", "12345")` → `"audio:netease:12345"`
- `LyricsKey("bilibili", "BV1xx")` → `"lyrics:bilibili:BV1xx"`
- TTL: 24h for both audio URLs and lyrics

## Integration

The music agent is integrated into `pi-agent` via per-session application mode — the same process serves both coding and music sessions. Audio is served on the same HTTP port as the main server.

## Source

- `internal/music/` — Music infrastructure (source.go, router.go, types.go, cache.go, handler.go)
- `internal/music/bilibili/` — Bilibili client (client.go, search.go, filter.go, types.go)
- `internal/music/netease/` — NetEase client
- `internal/music/netease_adapter.go` — NetEase adapter implementing MusicSource
- `internal/music/bilibili_adapter.go` — Bilibili adapter implementing MusicSource
- `internal/agents/music/` — MusicApplication + tools + prompt
- `cmd/pi-music/` — Music-specific entrypoint

## System Prompt (v5)

The music-agent system prompt (`internal/agents/music/prompt/prompt.go`) has been updated to reflect the source strategy change:

- **B站** = `source="bilibili"` (默认) — "UP主视频音频，播放主力源，覆盖率广"
- **网易云** = `source="netease"` — "正版音乐，提供推荐/排行榜/新歌/歌词能力，作为播放降级源"
- **Recommend/Discover** → uses NetEase (better discovery quality)
- **Play** → defaults to bilibili with auto-fallback to netease

## Related

- [[runtime-application-interface]] — MusicApplication implements this interface
- [[coding-application]] — Parallel application layer
- [[four-layer-architecture]] — Music agent lives in Application layer; validates the split decision
- [[personal-assistant-roadmap]] — Music is the first non-coding agent, PoC for memory layer
- [[source-project-root-v5]] — Documents the bilibili-primary source strategy change
