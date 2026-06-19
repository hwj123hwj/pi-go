---
type: entity
date: 2026-06-14
tags: [desktop, electron, react, frontend, gui]
---

# Desktop Application

> Electron + React GUI client for pi-go, providing a visual interface for the agent.

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
electron/main.ts        ← Electron main process (window, IPC, menu)
electron/preload.ts     ← Context bridge (exposes APIs to renderer)
electron/pi-go-manager.ts ← Manages Go backend process lifecycle
electron/update-checker.ts ← Auto-update logic
src/main.tsx            ← React entrypoint
src/App.tsx             ← Root component
src/store.ts            ← Zustand state store (REST + WebSocket)
src/types.ts            ← TypeScript type definitions
src/theme.ts            ← Theme configuration (dark/light)
src/styles/app.css      ← Global design system
src/i18n/              ← i18n framework (useT hook, EN/ZH locales)
src/components/         ← UI components
```

## Backend Communication

The desktop app communicates with the Go backend via:

| Method | Protocol | Endpoint | Purpose |
|--------|----------|----------|---------|
| HTTP REST | HTTP/1.1 | `http://localhost:${port}/api/*` | CRUD operations (sessions, models, tools) |
| Server-Sent Events | HTTP/SSE | `POST /chat/stream` | Streaming chat responses |
| WebSocket | WS | `/ws` | Real-time bidirectional communication |

The `pi-go-manager.ts` starts and manages the Go backend as a child process, setting environment variables and forwarding stdout/stderr.

## State Management (Zustand)

The `store.ts` manages the following state:

| State Slice | Description |
|-------------|-------------|
| `sessions` | Map of `SessionView` objects (transcript, metadata) |
| `activeSessionId` | Currently active session |
| `models` | Available LLM models (fetched from `/models`) |
| `panes` | Visible [[desktop-app#Pane System\|panes]] per session |
| `sidebarCollapsed` | Sidebar visibility toggle |
| `language` | UI language (`en`/`zh`) |
| `theme` | UI theme (`dark`/`light`/`system`) |

Actions include: `createSession`, `sendPrompt`, `pickFolder`, `togglePane`, `setDensity`, `refreshDiff`, `openFile`.

## Pane System

The workspace supports multiple side panes:

| Pane | Component | Description |
|------|-----------|-------------|
| Chat | `ChatPane.tsx` | Conversation transcript with [[desktop-app#Density Modes\|density modes]] |
| Diff | `DiffPane.tsx` | Git diff viewer with inline comments |
| Plan | `SidePanes.tsx` | Plan display panel |
| Tasks | `SidePanes.tsx` | Task list panel |
| Terminal | `SidePanes.tsx` | Terminal output panel |
| File | `SidePanes.tsx` | File content viewer |

Panes are toggled via a "Views" dropdown in the toolbar.

## Density Modes

Chat messages support three display densities:

| Mode | Description |
|------|-------------|
| `summary` | Compact single-line summaries for tool calls; hides thoughts/system messages |
| `normal` | Default display with expandable tool calls |
| `verbose` | Fully expanded tool calls and all message types |

## Design System

The CSS (`app.css`) implements a complete design system with:
- **Dark theme** as default (`:root`)
- **Light theme** via `data-theme='light'` or system `prefers-color-scheme: light`
- Warm neutral palette with terracotta accent (`#d97757`)
- CSS custom properties for all colors, shadows, radii, and fonts
- Custom scrollbar styling

## Electron Main Process

`electron/main.ts` handles:
- Browser window creation (1280x800 default)
- Native folder picker dialog (`ipcMain.handle('pick-folder')`)
- Auto-updater integration
- Developer tools toggle (Cmd+Shift+I)
- Preload script injection for secure IPC

## Build & Distribution

```bash
# Development (hot reload)
npm run electron:dev

# Production build
npm run electron:build

# Architecture-specific builds
npm run electron:build:arm64
npm run electron:build:x64
```

Built via `vite.config.ts` + `electron-builder.yml`. Output: macOS `.dmg`, Windows `.exe`, Linux `.AppImage`.

## Related

- [[server-websocket]] — The Go backend that serves the desktop API
- [[config-system]] — Environment variables for desktop mode
- [[source-project-root]] — Scripts/build-desktop.sh
