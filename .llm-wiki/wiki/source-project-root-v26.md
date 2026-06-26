---
type: source
source_path: "."
date: 2026-06-27
tags: [desktop, ui-fix, bottom-terminal, sidebar-toggle, kb-paths, entrypoint-cleanup, readme-update]
related: [[desktop-app]], [[kb-agent]], [[four-layer-architecture]], [[music-agent]], [[overview]]
---

# Source: Project Root v26 — Desktop UI Fixes, KB Path Fix, Entrypoint Cleanup

> Re-ingest of `.` covering 4 commits since v25 (`d3c95ef..b0b7fee`).
> Focus: desktop robustness polish, KB clickable paths, dead entrypoint removal, README sync.

## Commits Covered

| Commit | Message | Scope |
|--------|---------|-------|
| `037b606` | docs: update README with v17-v25 features and accurate stats | README.md |
| `313e6ad` | refactor: remove redundant cmd/pi-music entrypoint | cmd/pi-music/ deleted |
| `d559148` | fix(kb): output absolute paths so desktop renders clickable file links | 6 KB tool files |
| `b0b7fee` | fix(desktop): fix 4 UI issues — file menu, bottom terminal, sidebar toggle, profile error | 8 desktop files |

## Key Takeaways

### 1. cmd/pi-music Entry Point Removed (⚠️ Contradicts Existing Wiki)

`cmd/pi-music/main.go` was deleted — it was fully redundant with `cmd/pi-agent`, which already bundles all three applications (coding, music, kb) via unified configuration.

**Wiki contradiction**: Three wiki pages still reference `cmd/pi-music`:
- [[four-layer-architecture]] — layer diagram lists `cmd/pi-music` as an entrypoint
- [[music-agent]] — Source section lists `cmd/pi-music/` as music-specific entrypoint
- [[source-project-root-v17]] — mentions `cmd/pi-music/main.go` in profile wiring

All three need updating.

### 2. KB Tools Now Emit Absolute Paths

Previously KB tools returned relative paths (e.g. `issues/xxx.md`). The desktop Markdown renderer's `isFilePath()` requires a leading `/` or `~/` to render clickable links, so these relative paths were never clickable.

**Fix**: All 6 KB tool files now emit `filepath.Join(repoPath, relPath)` (absolute):
- `kb_search` — absolute path in results
- `kb_list` — absolute path in backticks
- `kb_maintain` — absolute paths in health/duplicate/tag reports
- `kb_save` — absolute targetPath
- `kb_read` — header shows resolved absolute path
- `prompt.go` — instructions updated to tell agent to preserve absolute paths

### 3. Desktop: Four UI Fixes

| # | Issue | Fix |
|---|-------|-----|
| 1 | File explorer "Open in" dropdown clipped by `.file-toolbar { overflow: hidden }` | Changed to `overflow: visible` |
| 2 | Bottom panel toggle had no rendered component | Created `BottomTerminal.tsx`, wired into SessionView with Resizer |
| 3 | Right sidebar rail: clicking active icon was a no-op | Added `toggleWorkspaceView` action — clicking active icon now toggles closed |
| 4 | Profile panel silently failed on error | Error state now shows actual error message + retry button |

### 4. README Synced to v25 State

- Removed DeepV/Mock provider references (removed v6/v7)
- Added KB agent as 3rd application in architecture diagram
- Added new feature descriptions (unified profile, KB vector search, memory extraction, tool synopsis, desktop client)
- Added `/profile`, `/kb/*`, `/workspace/*` to HTTP API table
- Added `PI_GO_KB_*` embedding config vars
- Updated stats: **144 source files, 67 test files, ~26k LOC, 16 commands**
- Slash command count corrected to 16 (was 15)

## Notable Entities

- [[desktop-app]] — BottomTerminal component, sidebar toggle, profile error state
- [[kb-agent]] — absolute path output for clickable desktop links
- [[four-layer-architecture]] — entrypoint list reduced (pi-music removed)
- [[music-agent]] — source list updated
- [[overview]] — stats updated

## Contradictions Flagged

1. **cmd/pi-music**: Listed as entrypoint in [[four-layer-architecture]], [[music-agent]], and [[source-project-root-v17]] but now **deleted**. Fixed in this ingest for architecture + music-agent.
2. **Slash command count**: [[overview]] said 15, README now says 16. Updated overview.
3. **Source file count**: Overview estimated "~14,000 行 Go + 54 test files"; README now says "144 source files, 67 test files, ~26k LOC". Updated overview stats.
