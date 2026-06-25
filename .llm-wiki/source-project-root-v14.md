# Source: Project Root (.) — v14: Music player robustness fixes

> Date: 2026-06-25
> Focus: Music playback reliability across backend proxy, caching, and frontend audio element lifecycle.

## Summary

Six music-related bugs identified and fixed across the Go backend (`internal/music/`) and the Electron/React frontend (`desktop/src/components/`).

## Issues Found & Fixed

### Backend (Go)

1. **Stale audio URL cache invalidation** (`handler.go`)
   - **Problem**: NetEase CDN audio URLs expire after a period. When the proxy served a cached URL that had expired (upstream returns 404/410), the proxy forwarded the 404 to the client instead of refreshing the URL.
   - **Fix**: Extracted `proxyAudio()` helper. When upstream returns 404/410, the handler now invalidates the cache entry via `Cache.Delete()`, fetches a fresh URL, and retries once.

2. **Lyrics JSON injection vulnerability** (`handler.go`)
   - **Problem**: Lyrics were serialized via `fmt.Fprintf` with manual newline escaping. Characters like `"`, `\`, and control characters (tab, etc.) in LRC lyrics were not escaped, producing invalid JSON or potential injection.
   - **Fix**: Replaced manual string escaping with `json.Marshal()` for proper escaping of all JSON special characters.

3. **Cache memory unbounded growth** (`cache.go`)
   - **Problem**: Expired entries were never purged — only read-time checks skipped them. Over a long-running server session, the map could accumulate thousands of stale entries.
   - **Fix**: Added opportunistic cleanup in `Set()` — every 128 inserts triggers a sweep of expired entries.

4. **Dead code removal** (`cache.go`)
   - **Problem**: `itoa()` function was defined in cache.go but unused (only used in handler_test.go). It was moved to handler.go (where the handler logic lives) to be accessible to tests.

### Frontend (React/TypeScript)

5. **Audio element not reloading on src change** (`GlobalMusicBar.tsx`)
   - **Problem**: When switching songs, React updates the `<audio>` `src` attribute, but the HTMLMediaElement doesn't automatically abandon the old stream. The `useEffect` that handled play/pause ran before the new src was loaded, causing stale playback or race conditions.
   - **Fix**: Added a dedicated `useEffect` keyed on `audioURL` that calls `audio.load()` to force the media element to reset and start loading the new source.

6. **Error state no recovery** (`GlobalMusicBar.tsx` + `MusicPlayer.tsx`)
   - **Problem**: When audio failed to load (e.g., stale URL, network error), the play button became permanently disabled (`disabled={music.error}`). The only way to recover was to play a different song.
   - **Fix**: Error state now shows a "retry" button (↻ icon) that re-dispatches the current song to the global player, clearing the error and reloading. Progress bar is no longer disabled on error.

### i18n

7. **New translation keys** (`zh.ts`, `en.ts`)
   - Added `music.retry`: "重试" (zh) / "Retry" (en)

## Files Changed

| File | Change |
|---|---|
| `internal/music/handler.go` | Stale URL retry logic, `proxyAudio()` helper, `json.Marshal` for lyrics, `itoa()` moved here |
| `internal/music/cache.go` | Opportunistic cleanup in `Set()`, added `Delete()` method, removed dead `itoa()` |
| `desktop/src/components/GlobalMusicBar.tsx` | `audio.load()` on src change, retry button on error, progress bar not disabled on error |
| `desktop/src/components/MusicPlayer.tsx` | Inline player retry on error, button never disabled |
| `desktop/src/i18n/locales/zh.ts` | Added `music.retry` |
| `desktop/src/i18n/locales/en.ts` | Added `music.retry` |

## Cross-references
- [[music-agent]] — Music application architecture and tools
- [[desktop-app]] — Desktop frontend architecture
