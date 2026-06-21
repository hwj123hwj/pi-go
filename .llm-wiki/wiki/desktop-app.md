---
type: entity
date: 2026-06-22
tags: [desktop, electron, react, frontend, gui]
related: [[server-websocket]], [[config-system]], [[agent-guidance-system]]
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

## Pane System

| Pane | Description |
|------|-------------|
| Chat | Conversation transcript with density modes |
| Diff | Git diff viewer |
| Plan | Plan display panel |
| Tasks | Task list panel |
| Terminal | Terminal output panel |
| File | File content viewer + editor |

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

## Related

- [[server-websocket]] — The Go backend that serves the desktop API
- [[config-system]] — Environment variables for desktop mode
- [[deployment-infrastructure]] — Server-side deployment (separate from desktop)
