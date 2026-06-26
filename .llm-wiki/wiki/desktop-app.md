---
type: entity
date: 2026-06-27
tags: [desktop, electron, react, frontend, gui, workspace, global-music-player, profile-panel]
related: [[server-websocket]], [[config-system]], [[agent-guidance-system]], [[unified-profile]]
---

# Desktop Application

> Electron + React GUI client for pi-go, providing a visual interface for the agent.
> Manages the Go backend as an embedded subprocess.

## Technology Stack

| Layer | Technology | Version |
|-------|------------|---------|
| Desktop Shell | Electron | 33.x |
| Frontend Framework | React | 19.x |
| Bundler | Vite | 6.x |
| Language | TypeScript | 5.x |
| State Management | Zustand | |
| i18n | Custom | EN + ZH |

## Architecture

```
electron/main.ts            ← Electron main process (window, IPC, menu)
electron/preload.ts         ← Context bridge (exposes PiAPI to renderer)
electron/pi-go-manager.ts   ← Manages Go backend process lifecycle
electron/update-checker.ts  ← GitHub Releases polling for updates
src/main.tsx                ← React entrypoint
src/App.tsx                 ← Root component
src/store.ts                ← Zustand state store (REST + WebSocket)
src/types.ts                ← TypeScript type definitions
src/theme.ts                ← Theme configuration (dark/light)
src/sessionTitle.ts         ← Session title derivation from messages
src/styles/                 ← CSS Modules + CSS Variables
src/i18n/                   ← i18n framework (useT hook, EN/ZH locales)
src/components/             ← UI components
```

## Go Backend Process Management (PiGoManager)

`pi-go-manager.ts` manages the pi-agent subprocess lifecycle:

### Startup Flow
1. Find free port (random, via `net.createServer`)
2. Locate `pi-agent` binary (packaged: `Contents/Resources/pi-agent`; dev: project root)
3. Spawn `pi-agent -mode serve -listen 127.0.0.1:<port>`
4. Health poll `GET /health` until 200 (max 30 attempts, 500ms interval)
5. Return `{ url, port }` to renderer

### Two Modes

| Mode | Binary Location | Data Dir | Config |
|------|----------------|----------|--------|
| **Packaged** | `process.resourcesPath/pi-agent` | `userData/data` | `userData/.env` (auto-created with defaults) |
| **Development** | Project root `pi-agent` | Project root | Project root `.env` |

### Packaged Mode Defaults
```env
PI_GO_PROVIDER=openai
OPENAI_API_KEY=sk-local-gateway-hwj123hwj
OPENAI_BASE_URL=http://localhost:4001
OPENAI_MODEL=mimo-opus
PI_GO_ENABLE_BASH=true
```

macOS: Automatically removes quarantine attributes (`xattr -cr`) from binary.

## Update Checker

`update-checker.ts` polls GitHub Releases API (`hwj123hwj/pi-go`):
- Compares semver (`0.3.0` vs `0.2.0`)
- Prefers arm64 DMG asset
- Falls back to release HTML URL if no DMG found
- Returns `UpdateInfo { version, downloadUrl, releaseNotes }` or null

## IPC Bridge (Preload)

`preload.ts` exposes `PiAPI` to renderer via `contextBridge`:

| Method | Purpose |
|--------|---------|
| `getServerUrl()` | Get active backend URL |
| `startServer()` | Trigger backend start |
| `checkForUpdate()` | Poll GitHub for new version |
| `openDownloadPage(url)` | Open browser for download |
| `pickFolder()` | Native folder picker dialog |

## State Management (Zustand Store)

`store.ts` is the single source of truth, talking directly to backend via REST + WebSocket (no IPC for data).

### State Shape

| State | Description |
|-------|-------------|
| `sessions` | Map of `SessionView` objects (transcript, metadata, plan, diffs) |
| `activeSessionId` | Currently active session |
| `models` | Available LLM models (fetched from `/models`) |
| `currentModel` | Currently selected model |
| `lang` / `theme` | UI preferences (persisted locally) |
| `update` | Update state (idle/available/downloading) |
| `connected` | WebSocket connection status |

### WebSocket Event Handling

The `WSService` class handles real-time events:
- `event:text_delta` — Stream text chunks to assistant message
- `event:tool_start` / `event:tool_end` — Tool call lifecycle visualization
- `event:error` — Error display
- `type:status` — Streaming completion detection
- Auto-reconnect on disconnect (2s delay)

### Tool Kind Inference

`inferToolKind()` maps tool names to visual categories:
- `read` / `cat` / `view` → `'read'`
- `edit` / `write` / `replace` → `'edit'`
- `bash` / `exec` / `shell` / `run` → `'execute'`
- `grep` / `glob` / `find` → `'search'`
- `fetch` / `http` / `web` → `'fetch'`

## Workspace Layout (v5)

The Views dropdown has been **removed entirely**. The main area always shows Chat. Feature panels live in the right sidebar.

### Right Sidebar Rail

6-icon vertical rail (VSCode/Codex-style):

| Icon | View | Description |
|------|------|-------------|
| Review | `review` | Git diff viewer |
| Files | `files` | File browser (tree, tabs, search, highlighting) |
| Plan | `plan` | Agent execution plan / TODO progress |
| Tasks | `tasks` | In-flight + recent tool calls |
| KB | `kb` | Knowledge base browser |
| Profile | `profile` | User profile viewer (v25) |

`RightView` type: `'review' | 'files' | 'plan' | 'tasks' | 'kb' | 'profile'`

### Sidebar Toggle (v26)

Clicking the already-active rail icon now **toggles the sidebar closed** instead of being a no-op. The `toggleWorkspaceView` store action checks if the clicked view is already active — if so, it closes the sidebar; otherwise it switches to the new view.

### Knowledge Base Panel (NEW)

The KB panel provides visual access to the second brain (`~/agent-lessons`).

**Three views:**
- **Browse**: Category chips + entry list + entry detail (Markdown preview)
- **Tags**: Tag cloud with usage counts, click to filter entries
- **Health**: Visual dashboard showing metadata gaps, duplicates, tag clusters

**Backend endpoints:** See [[kb-agent#Desktop KB Panel]] for API details.

Density toggle (summary/normal/verbose) moved from Views dropdown to toolbar inline buttons.

### Profile Panel (v25)

The profile panel provides visual access to the [[unified-profile]] — the "condensed second brain" that agents use.

**Features:**
- **Agent Summary**: Renders the exact Markdown string injected into agent prompts
- **Category sections**: Expandable/collapsible by category (coding/music/general)
- **Per-fact metadata**: Source agent, last-updated (relative time), access count (hotness indicator)
- **Delete**: Hover-reveal delete button for individual facts
- **Empty state**: Helpful hint explaining auto-learning behavior
- **Error state (v26)**: On load failure, shows the actual error message + retry button instead of silently failing

**Backend endpoint:** `GET /profile` → categories + facts + summary; `DELETE /profile` → remove fact

## Pane System (pre-v5, deprecated)

| Pane | Description |
|------|-------------|
| Chat | Conversation transcript with density modes |
| Diff | Git diff viewer |
| Plan | Plan display panel |
| Tasks | Task list panel |
| Terminal | Terminal output panel |
| File | File content viewer + editor |

## Bottom Terminal Panel (v26)

The bottom panel toggle (accessible from the toolbar toggles) now renders a `BottomTerminal` component showing aggregated command/tool output from the active session.

**Architecture:**
- `BottomTerminal.tsx` — renders in a horizontally resizable panel at the bottom of SessionView
- Wired via `bottomOpen` + `bottomHeight` state in the Zustand store
- Uses the [[workspace/Resizer]] component for drag-to-resize (y-axis)
- SessionView conditionally renders `{bottomOpen && <Resizer> + <BottomTerminal>}`

**Store state additions:**
- `workspace.bottomOpen: boolean` — panel visibility
- `workspace.bottomHeight: number` — pixel height (resizable)
- `toggleBottomPanel()` — toggle open/close

## Density Modes

| Mode | Description |
|------|-------------|
| `summary` | Compact single-line; hides thoughts/system |
| `normal` | Default with expandable tool calls |
| `verbose` | Fully expanded everything |

## Design System

- Dark theme default with warm neutral palette
- Terracotta accent (`#d97757`)
- CSS custom properties for all colors, shadows, radii
- Light theme via `data-theme='light'` or `prefers-color-scheme`

## Build & Distribution

```bash
npm run electron:dev        # Development (hot reload)
npm run electron:build      # Production build
npm run electron:build:arm64  # ARM64 macOS
```

Or via script: `./scripts/build-desktop.sh [--x64]`

Output: macOS `.dmg`, Windows `.exe`, Linux `.AppImage`.

## Path Clicking

File paths in the agent's output are rendered as clickable links that open in the File Pane.

### Two Detection Layers

**Layer 1: Markdown renderer** (`Markdown.tsx`)
- **Backtick-wrapped paths**: `renderInline()` splits on backticks, checks `isFilePath()` on code content
- **Plain text paths**: `renderEmphasis()` calls `renderTextWithFilePaths()` which scans with `PATH_RE` regex
- Both render as `<code className="file-path-link">` with `onClick → dispatch('open-file')`

**Layer 2: Tool result locations** (`store.ts`)
- `extractLocationsFromText()` runs on tool_end results with 4 regex patterns:
  1. Labeled paths: `文件: /path` or `File: /path`
  2. Known extension paths: `.go`, `.ts`, `.md`, etc.
  3. Directory paths: `/a/b/c/` (≥2 segments)
  4. No-extension paths: `/a/b/c` (≥3 segments)
- Results stored in `ChatItem.locations` for sidebar display

### isFilePath Validation

A string is considered a file path if:
- Starts with `/` or `~/`
- Not a URL (`http://`)
- Has known extension, OR ends with `/` (directory), OR has ≥2 path segments with no extension in last segment

## Session History Restoration

When `setActive(id)` is called and the session has no messages loaded:
1. Fetches `GET /sessions/{id}/messages`
2. Reconstructs `ChatItem[]` from user/assistant/tool messages
3. Tool calls are matched to results by `tool_call_id`
4. Title derived from first user message via `deriveTitleFromMessage()`

## File Pane

- `openFile(id, path)` → `GET /sessions/{id}/file?path=` → displays content
- `saveFile(id, path, content)` → `PUT /sessions/{id}/file?path=` → updates view
- File pane is one of 6 pane kinds: chat, diff, plan, tasks, terminal, file

## Global Music Player (v5)

Music playback was refactored from per-session local audio to a **global, persistent player** that survives session switches.

### Architecture

```
GlobalMusicBar.tsx (mounted in App.tsx, never unmounts)
  └── <audio> element (single, persistent)
      ↓ Zustand store: music state
  MusicPlayer.tsx (inline in chat)
  └── dispatches playMusic() to store
      ↓ no own <audio> element
```

### Store State

```typescript
interface MusicState {
  current: MusicTrack | null;
  playing: boolean;
  currentTime: number;
  duration: number;
  error: boolean;
}
```

Actions: `playMusic(song)`, `toggleMusic()`, `clearMusic()`, `setMusicPlaying/Time/Duration/Error`

### Floating Capsule Design

- Bottom-center, `min-width: 420px`, `border-radius: 14px`
- `backdrop-filter: blur(20px)` frosted glass
- Slide-up entrance animation (`cubic-bezier(0.22, 1, 0.36, 1)`)
- Close (✕) button to clear current track
- `.app.music-active .main { padding-bottom: 56px }` — prevents overlap with prompt bar

### Inline MusicPlayer

Chat transcripts show `MusicPlayer` cards. Clicking ▶ dispatches `playMusic()` to global store. If the song is already the active global track, it toggles play/pause instead.

## Markdown Link Resolution (v5)

`Markdown.tsx` accepts optional `basePath` to resolve relative links:

- **External links** (`http://`): `window.piAPI.openExternal()` in system browser
- **Anchor links** (`#xxx`): default browser behavior
- **Relative file links** (`file.md`, `../dir/file.md`): `resolveHref()` resolves against `basePath`, dispatches `open-file` event → `openFileTab()` in Files panel

Passed from:
- `ChatPane`: `basePath={cwd}` (session working directory)
- `FilesPanel`: `basePath={path}` (file being viewed)
- `SidePanes`: `basePath={open.path}` (open file)

## Backend Workspace APIs (v5)

New endpoints in `server.go`:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/workspace/list-dir` | List directory entries (lazy tree) |
| `GET` | `/workspace/search-files` | Fuzzy file search |
| `GET` | `/workspace/read-file` | Read file content (text or base64) |

Plus Electron IPC handlers for system integration:
- `openInFinder(path)` — Reveal file in macOS Finder
- `openInTerminal(path)` — Open Terminal at directory
- `openExternal(url)` — Open URL in system browser

## Related

- [[server-websocket]] — The Go backend that serves the desktop API
- [[config-system]] — Environment variables for desktop mode
- [[deployment-infrastructure]] — Server-side deployment (separate from desktop)
- [[source-project-root-v5]] — Documents workspace layout + global music player changes

## Mobile / Capacitor Platform (v27)

The frontend is packaged as an Android APK via **Capacitor**. The same React
codebase serves both Electron desktop and mobile, with `isElectron` checks
for platform-specific behavior.

### Server Connect Flow

On mobile, the backend is remote. `ServerConnect.tsx` shows a full-screen
URL input where the user enters the server address (e.g.
`http://8.141.97.21:8080`). A `/health` check validates connectivity before
proceeding. The URL is persisted in `localStorage` (`pi-go-server-url`).

### Platform Detection

- `isElectron` — `typeof window.piAPI !== 'undefined'`
- `isRemotePlatform` — `!isElectron` (covers Capacitor + browser/PWA)
- `body.mobile` class added on remote platforms for CSS targeting

### Mobile-Specific Optimizations (v27)

| Area | Desktop | Mobile |
|------|---------|--------|
| Sidebar | Resizable panel | Slide-in drawer (85% width, max 320px) |
| PromptBar model selector | Visible chip | Hidden (saves vertical space) |
| PromptBar ⌘V hint | Visible | Hidden |
| PromptBar send button | Small square | 36px circular (larger touch target) |
| PromptBar input | 13.5px font | 16px font (prevents iOS zoom) |
| Music bar | Floating capsule (centered, 420-560px) | Full-width bottom bar (edge-to-edge) |
| Music bar padding | `12px` bottom offset | `env(safe-area-inset-bottom)` |
| Session items | `min-height: 48px` | `min-height: 64px` (larger touch targets) |
| Toolbar density toggle | Visible | Hidden |
| Right sidebar | Resizable panel | Full-screen overlay |
| Resizers | Visible | Hidden (`display: none`) |
| New project button | Visible | Hidden (no folder picker on mobile) |
| Workspace toggles | Icon buttons | Bottom bar items (36px min touch) |
| Chat transcript padding | `28px` | `16px 14px` (edge-efficient) |
| Message spacing | `22px` | `16px` (denser reading) |
| User message | `12px radius` | `16px radius` (rounder, chat-like) |
| Assistant text | 13.5px | 14px + `1.65` line-height (readability) |
| Tool card head | `9px padding` | `10px padding`, 44px min-height (HIG) |
| Tool status badge | Text + icon | Icon only (space saving) |
| Inline music card progress | Visible | Hidden (global bar handles playback) |
| Markdown tables | Static | Horizontal scroll (`overflow-x: auto`) |
| Markdown code blocks | `border-radius: var(--radius)` | `10px radius`, max-height 280px |
| Empty state input | 13px | 16px (iOS zoom prevention) |
| Sidebar search input | 13px | 15px, 36px min-height |
| Sidebar head | `env(safe-area-inset-top)` | `calc(12px + safe-area-inset-top)` |
| Right sidebar rail buttons | Icon size | 44px min-width/height (HIG) |
| App height | `100vh` | `100dvh` (dynamic viewport — keyboard adaptive) |

### Mobile-Specific Optimizations (v29, Round 3)

| Area | Desktop | Mobile |
|------|---------|--------|
| Diff preview gutters | Line numbers visible | Hidden (saves horizontal space) |
| Diff preview font | 12px | 11px, compact line-height |
| Tool body text | Full height | max-height 200px, momentum scroll |
| Console output | Full height | max-height 200px, momentum scroll |
| Modal/dialog | 580px centered | 94vw, 88vh max, 14px radius |
| Modal buttons | Default height | 40px min-height, 14px font |
| Ask option cards | Default height | 44px min-height (HIG touch target) |
| Code block copy button | Hover-visible | Always visible (touch devices) |
| Long code blocks | Full text shown | Collapsed to 12 lines + "Show N more" expand |
| Markdown headings | 19/16/14px | 18/15/14px |
| Markdown lists/links | Default | Compact spacing, word-break links |
| Session list items | 48px min | 56px min-height (touch friendly) |
| Transcript scroll | Default | Momentum scroll + overscroll-contain |
| Sidebar scroll | Default | Momentum scroll + overscroll-contain |
| Role tags | 11px | 10px, 18px badge |
| Typing dots | 7px | 6px |
| Server connect input | 15px font | 16px (iOS zoom prevention) |
| Server connect button | 13px padding | 48px min-height, 16px font |
| Toolbar status | Text + dot | Dot only (text hidden, saves space) |

**Code Block Component** (`Markdown.tsx`): The `<pre>` element was replaced by
a `CodeBlock` component wrapping `pre` in a positioned container with:
- A copy button (clipboard API) in the top-right corner
- A collapse/expand mechanism for blocks >12 lines on mobile
- Gradient mask fade on collapsed code for visual hint

### Audio Streaming on Mobile (v27)

The `GlobalMusicBar` component manages a single `<audio>` element mounted at
the app root, so music persists across session switches. Audio proxy URLs are
relative (`/music/audio/netease_12345`), resolved to the server base URL via
`getBaseUrl()` in `MusicPlayer.tsx::rewriteAudioURL()`.

The backend audio proxy (`internal/music/handler.go`) detects and rejects
non-audio Content-Types (`text/html`, `text/plain`, `application/json`)
even when upstream returns HTTP 200, preventing HTML error pages from being
sent to the audio player.

### Capacitor Config

`capacitor.config.json` enables:
- `androidScheme: http` + `cleartext: true` — allows HTTP audio streaming
- `allowMixedContent: true` — mixed HTTP/HTTPS content
- `webContentsDebuggingEnabled: true` — remote debugging via `chrome://inspect`

### Mobile Right Sidebar Fix (v30)

**Bug**: The mobile CSS rules for the right sidebar (Files, Review, Plan, KB,
Profile panels) targeted `.right-sidebar`, `.right-sidebar-rail`, and
`.right-sidebar-content`, but the actual component classes are `.rsidebar`,
`.rsidebar-rail`, and `.rsidebar-content`. The mismatch meant the full-screen
overlay styling never applied on mobile — the file browser was inaccessible.

**Fix**: Updated all CSS selectors from `.right-sidebar*` → `.rsidebar*`. On
mobile, the right sidebar now:
- Opens as a full-screen fixed overlay (z-index 200)
- Uses `flex-direction: column-reverse` so the rail sits at the bottom
- Rail buttons display as a horizontal bottom bar with icon + 9px label
- Each rail item: 44px minimum touch target, column layout

### Mobile File Panel (v30)

When the right sidebar Files panel is open on mobile:
- The file tree sidebar is hidden (`display: none`) — saves space, users
  navigate via file tabs and fuzzy search
- File tabs: 13px font, close buttons always visible (opacity:1)
- Code content: 12px font with momentum scrolling

### Global Music Bar Close Button (v30)

Added a `✕` close button to `GlobalMusicBar` that calls `clearMusic()` — this
stops playback (pauses the audio element) and sets `music.current = null`,
which causes the bar to unmount (the `if (!music.current) return null` guard).
On mobile the close button is 32px for comfortable touch interaction.

### Mobile Right Sidebar Close Button (v31)

When a right sidebar panel (Files, Review, Plan, KB, Profile) is open on
mobile, a floating **← back arrow** button appears in the top-left corner
of the panel. It calls `toggleWorkspaceRight()` to close the sidebar and
return to the chat. The `rsidebar-content` receives `padding-top:
calc(env(safe-area-inset-top) + 56px)` on mobile so the content doesn't
overlap with the floating button.

On desktop, `.rsidebar-mobile-close` is `display: none`.

### Mobile Density Toggle (v32)

The density toggle (summary/normal/verbose) was previously hidden entirely on
mobile. Now it uses a separate `toolbar-density-mobile` class with compact
28px buttons and 10px font — small enough to fit the toolbar but still
functional for users who want to switch between detailed and compact views.

### Mobile Empty State (v32)

On mobile (non-Electron), the empty state screen hides:
- **Folder picker** — no native folder dialog on mobile
- **Model selector** — server uses its configured default

Mobile users see a clean prompt textarea + 44px send button only. The model
and working directory are determined server-side.

### Mobile Keyboard Auto-Dismiss (v32)

When a mobile user touches or scrolls the chat transcript while the keyboard
is open, the active input element is automatically blurred. This dismisses
the soft keyboard, reclaiming screen space for reading. Implemented via
`onTouchStart` handler on `.pane-body` that checks `document.activeElement`.

### Code Review Bug Fixes (v33)

After 6 rounds of mobile optimizations (v27–v32), a code review found 4 bugs:

1. **Duplicate `.toolbar-density-mobile` CSS** — Two separate blocks existed
   in the mobile media query (lines ~5151 and ~5858). The second block was a
   leftover from v32. Consolidated into a single definition with `display:
   flex` and `margin-left: auto` so the toggle pushes to the right.

2. **Duplicate `.rsidebar-content` CSS** — Three definitions existed (line 494
   desktop + line 5221 mobile + line 5255 mobile). The third was a leftover
   `padding-top` rule. Merged into the single mobile definition.

3. **GlobalMusicBar close didn't release audio resource** — `clearMusic()`
   set `music.current = null` and called `audio.pause()`, but the `src`
   attribute remained, keeping the media resource loaded in memory. Fixed by
   calling `audio.removeAttribute('src')` + `audio.load()` to fully release
   the resource.

4. **Unused `playMusic` store subscription** — After the v30 refactor added
   `clearMusic`, the `playMusic` subscription was no longer needed but still
   present, causing unnecessary re-renders when `playMusic` was called
   elsewhere. Removed.

### Code Review Bug Fixes Round 2 (v34)

1. **File tree not hidden on mobile** — The CSS rule `.files-panel
   .files-sidebar { display: none }` targeted a class that doesn't exist.
   The FilesPanel component renders `<Resizer>` and `<FileTree>` directly
   inside `.files-body`, not wrapped in `.files-sidebar`. Fixed to target
   `.files-panel .file-tree` and `.files-body > .resizer`.

2. **ChatPane onTouchStart steals focus from interactive elements** — The
   keyboard-dismiss handler blurred `document.activeElement` on every touch,
   including taps on code copy buttons, tool card expand/collapse, etc.
   Added a `target.closest('button, a, input, textarea, .tool-head,
   .md-code-copy, .gm-btn')` guard so only empty-area touches trigger blur.

### Code Review Bug Fixes Round 3 (v35)

1. **Sidebar backdrop double-dimming on mobile** — The app uses both a
   `.app::before` pseudo-element overlay (z-index 150, 50% black) AND a
   separate `.sidebar-mobile-backdrop` div (z-index 150, 50% black, has
   click handler to close sidebar). With `pointer-events: none` on `::before`,
   clicks on the backdrop area passed through to the underlying app instead
   of closing the sidebar. However, changing `::before` to `pointer-events:
   auto` would block clicks on the backdrop div. Fixed by keeping
   `::before` as `pointer-events: none` but making it transparent when the
   mobile backdrop is present (using `:has(.sidebar-mobile-backdrop)`).

2. **`.btn-stop` mobile CSS missing `!important`** — In the mobile media
   query, several `.btn-stop` properties (`width`, `height`, `font-size`,
   `gap`) lacked `!important` while the base `.btn-stop` definition (at
   line 3179) sets conflicting values. Depending on CSS specificity, the
   base values could override the mobile overrides. Fixed: all critical
   properties now have `!important`.

### Code Review Bug Fixes Round 4 (v36)

1. **`.rsidebar-content` overflow conflict** — Desktop definition (line 494)
   sets `overflow: hidden` to prevent content spilling. Mobile definition
   (line 5227) sets `overflow-y: auto` to enable scrolling. Without
   `!important`, CSS cascade order could let the desktop rule win, making
   the right sidebar panel unscrollable on mobile — users couldn't scroll
   through long file content or KB articles. Fixed: `overflow-y: auto
   !important` on mobile.

2. **CodeBlock clipboard unhandled rejection** —
   `navigator.clipboard?.writeText(code).then(...)` in `Markdown.tsx`
   had no error handler. On mobile browsers serving over HTTP (not HTTPS),
   the Clipboard API is blocked (`NotAllowedError`), causing an unhandled
   promise rejection warning in the console. Fixed: added `.catch()` to
   silently ignore clipboard failures.

### Mobile PromptBar Stop Button (v31)

The stop button (shown when agent is thinking) was originally a pill with
text + icon. On mobile it is now a **36px circular icon-only button** to
match the send button's size and visual rhythm. The text label is removed
(`font-size: 0`).

