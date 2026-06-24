---
type: source
source_path: "."
date: 2026-06-23
tags: [music, desktop, bilibili-primary, global-player, workspace-layout, markdown-links, netease-fixes, bilibili-cookies]
related: [[music-agent]], [[desktop-app]], [[server-websocket]], [[source-project-root-v4]]
---

# Source: Project Root v5

> Re-ingest after desktop UI overhaul + music source strategy pivot.
> Covers commits `c1fd082..dc2411f` (5 commits, 2026-06-23).

## Key Takeaways

1. **Bilibili is now the primary music playback source** — `ParseSource` defaults to `bilibili` (was `netease`). Cross-source fallback reversed: B站失败 → 降级网易云 (was the opposite). System prompt rewritten to reflect this.
2. **NetEase demoted to "recommendation engine"** — still powers `music_recommend` (newsong/ranking/top), `music_lyrics`, `music_playlist`; but not the default for `music_play`.
3. **Global Music Player architecture** — single `<audio>` element in `GlobalMusicBar.tsx` mounted in `App.tsx`, never unmounts during session switches. `MusicPlayer.tsx` (inline chat) dispatches to global Zustand store instead of creating local audio.
4. **Workspace layout overhaul** — Views dropdown removed entirely; Plan/Tasks moved into right sidebar rail (now 4 icons: Review/Files/Plan/Tasks). Main area always Chat.
5. **Markdown relative link resolution** — `Markdown.tsx` accepts `basePath`, resolves `[text](file.md)` relative to the file's directory, dispatches `open-file` event to open in Files panel.
6. **NetEase `checkURL` fix** — HEAD → GET+Range (NetEase blocks HEAD). Multi-bitrate fallback (320k→192k→128k). Outer URL returned as last resort even if check fails.
7. **Bilibili cookie initialization** — `ensureCookies()` fetches buvid3/buvid4 from SPI endpoint, visits search page for additional cookies. Required since B站 search API started rejecting requests without fingerprint cookies.
8. **New workspace backend APIs** — `list-dir`, `search-files`, `read-file` (text/base64) endpoints in `server.go`.

## Important Entities & Concepts

### Music Source Strategy Change
- `ParseSource("")` → `SourceBilibili` (was `SourceNetease`) — `internal/agents/music/tools/tools.go`
- `playByQuery`: searches bilibili first, falls back to netease — `internal/agents/music/tools/play.go`
- System prompt: "B站是播放主力源" / "网易云提供推荐能力" — `internal/agents/music/prompt/prompt.go`
- `SourceRouter.default_` still configurable per router instance (tests use netease for backward compat)

### Global Music Player
- `MusicState` in store: `{ current: MusicTrack|null, playing, currentTime, duration, error }`
- Actions: `playMusic(song)`, `toggleMusic()`, `clearMusic()`, `setMusicPlaying/Time/Duration/Error`
- `GlobalMusicBar.tsx`: floating capsule design, bottom-center, backdrop-blur
- `MusicPlayer.tsx`: inline chat card, dispatches to global store, shows play state if active track
- `.app.music-active .main { padding-bottom: 56px }` — prevents overlap

### Markdown basePath Resolution
- `resolveHref(href, basePath)`: handles absolute, external, `./`, `../` paths
- External links: `window.piAPI.openExternal()` (system browser)
- Relative file links: `dispatchEvent('open-file', { path: resolved })`
- Passed from: ChatPane (`basePath={cwd}`), FilesPanel (`basePath={path}`), SidePanes (`basePath={open.path}`)

### NetEase Audio URL Strategy
1. GET + `Range: bytes=0-1023` on outer URL (was HEAD, blocked by NetEase)
2. enhance/player API at 320k → 192k → 128k
3. Return outer URL unchecked as last resort (proxy tries it)

### Bilibili Cookie Flow
1. Visit `bilibili.com` (basic cookies)
2. GET `/x/frontend/finger/spi` → buvid3, buvid4
3. Visit `search.bilibili.com/all?keyword=1` (additional cookies: b_lsid, _uuid)
4. `sync.Once` — runs once per client instance

## Notable Code References

| File | Change |
|------|--------|
| `internal/agents/music/tools/tools.go` | `ParseSource` default → bilibili |
| `internal/agents/music/tools/play.go` | Fallback reversed, descriptions updated |
| `internal/agents/music/prompt/prompt.go` | System prompt: bili primary, netease recommend |
| `internal/music/netease/song.go` | checkURL GET+Range, multi-bitrate, outer URL fallback |
| `internal/music/bilibili/client.go` | `ensureCookies()` with SPI + search page |
| `desktop/src/components/GlobalMusicBar.tsx` | NEW: persistent global audio element |
| `desktop/src/components/MusicPlayer.tsx` | Refactored: dispatch to global store |
| `desktop/src/store.ts` | MusicState + actions, RightView adds plan/tasks |
| `desktop/src/components/workspace/RightSidebar.tsx` | 4-icon rail: Review/Files/Plan/Tasks |
| `desktop/src/components/workspace/PlanPanel.tsx` | NEW: Plan display in sidebar |
| `desktop/src/components/workspace/TasksPanel.tsx` | NEW: Task list in sidebar |
| `desktop/src/components/Markdown.tsx` | basePath + resolveHref + open-file dispatch |
| `desktop/src/App.tsx` | GlobalMusicBar mount, music-active class |

## Contradictions with Existing Wiki

- **[[music-agent]]**: Previously documented "NetEase primary, Bilibili fallback" and `SourceRouter.default_ = netease`. Now **reversed** — Bilibili is primary. The SourceRouter struct field is unchanged (still set per-instance), but the effective default via `ParseSource("")` is now bilibili.
- **[[desktop-app]]**: Previously documented 6-pane system with Views dropdown. Now **removed** — Views dropdown gone, Plan/Tasks moved to right sidebar rail. Music player was per-session local audio; now global persistent player.
