# Source: Project Root (.) — v15: Music player deep-audit round 2

> Date: 2026-06-25
> Focus: Audio proxy HTTP client hardening, cross-source fallback robustness, React audio lifecycle race conditions.

## Summary

Second round of music playback auditing. Found and fixed 5 additional issues in the Go backend and React frontend that affect playback reliability.

## Issues Found & Fixed

### Backend (Go)

1. **audioProxyClient: unbounded idle connections (FD leak)** (`handler.go`)
   - **Problem**: `audioProxyClient` had `Timeout: 0` (correct for streaming) and `ResponseHeaderTimeout: 10s`, but no limits on idle connections. Each proxied stream opens a new connection to a different CDN host, and the default Go `http.Transport` keeps idle connections alive forever with no cap.
   - **Fix**: Added `MaxIdleConns: 20`, `MaxIdleConnsPerHost: 4`, `IdleConnTimeout: 90s` to the transport. This bounds file descriptor usage while allowing reasonable connection reuse.

2. **403 not treated as stale URL** (`handler.go`)
   - **Problem**: Only 404/410 triggered cache invalidation + retry. But CDNs (both NetEase and Bilibili) also return 403 Forbidden when anti-hotlink tokens expire — functionally identical to URL expiration. A 403'd URL would be served from cache for up to 24h.
   - **Fix**: Added `http.StatusForbidden` to the "stale URL" check in `proxyAudio()`. When 403 is detected, the cache is invalidated and the URL is refreshed + retried. If retry also fails, a 502 is returned.

3. **Retry exhaustion not handled** (`handler.go`)
   - **Problem**: If the retry attempt also returned a stale URL, `proxyAudio` was called a second time but its return value (false) was silently ignored. The client would receive whatever partial/garbage response the second attempt wrote.
   - **Fix**: Added explicit check — if retry also returns `false`, invalidate cache again and write a clean 502 error.

4. **Cross-source fallback only tried 1 netease song** (`play.go`)
   - **Problem**: When all 5 bilibili candidates failed, the fallback to NetEase only tried `neteaseResult.Songs[0]`. If that one song was unplayable (VIP/region-locked), the user got "播放失败" even though other netease results might work.
   - **Fix**: Changed fallback loop to try up to 3 netease candidates, matching the multi-try pattern of the primary source.

### Frontend (React/TypeScript)

5. **Race condition: play() fires before src finishes loading** (`GlobalMusicBar.tsx`)
   - **Problem**: The `music.playing` effect called `audio.play()` immediately when the store changed to `playing: true`. But if the `audioURL` had also changed (new song), the `<audio>` element's `readyState` was still 0 (HAVE_NOTHING) from the old source. `audio.play()` would fail silently or play stale audio.
   - **Fix**: Split into two effects:
     - **audioURL effect**: calls `audio.load()` then waits for the `canplay` event before calling `play()`.
     - **playing effect**: checks `audio.readyState >= HAVE_CURRENT_DATA` before attempting play/pause. If not ready, skips (the audioURL effect will handle it when ready).

6. **Retry re-dispatched same URL but didn't reload element** (`GlobalMusicBar.tsx`)
   - **Problem**: When `handleRetry` called `playMusic(music.current)`, the URL was the same as before. The `audioURL` effect didn't fire (same dependency value), so the audio element was never actually reloaded — the error persisted.
   - **Fix**: `handleRetry` now directly manipulates the audio element: `audio.load()` + `audio.play()`, bypassing the store update cycle entirely.

## Files Changed

| File | Change |
|---|---|
| `internal/music/handler.go` | Added idle conn limits, 403 stale detection, retry exhaustion handling |
| `internal/music/handler_test.go` | Rewrote 403 test to verify cache invalidation + retry with mock source |
| `internal/agents/music/tools/play.go` | Multi-try cross-source fallback (3 netease candidates instead of 1) |
| `desktop/src/components/GlobalMusicBar.tsx` | Split audio effects: audioURL load+canplay vs toggle play/pause; direct element manipulation for retry |

## Cross-references
- [[music-agent]] — Music application architecture
- [[source-project-root-v14]] — Previous music fixes (v14)
- [[desktop-app]] — Desktop frontend architecture
