---
type: concept
date: 2026-07-13
tags: [tui, design, bubble-tea, elm-architecture, three-phase, planning]
---

# TUI Design Document

> Design document for the Bubble Tea TUI rewrite. Located at `docs/design/tui-bubbletea.md`.

**Related**: [[tui-bubbletea]], [[tui-presenter]]

## Context

The original CLI used a linear `EnhancedPresenter` that printed output line-by-line to stdout. This was functional but lacked interactivity: no scrollback within a session, no multi-line input, no autocomplete, no collapsible tool panels.

The decision was made to build a full-screen TUI using [Bubble Tea](https://github.com/charmbracelet/bubbletea) — the Go standard for terminal UIs.

## Framework Selection

| Framework | Verdict |
|-----------|---------|
| **Bubble Tea** (charmbracelet) | ✅ Chosen — Go-native, Elm architecture, differential rendering, mature ecosystem |
| tview | ❌ Rejected — Widget-based, less flexible, harder to customize |
| Raw ANSI | ❌ Rejected — Too much manual work, no differential rendering |

Supporting libraries:
- **lipgloss** — Styling (colors, borders, layout). Already a dependency.
- **glamour** — Terminal markdown rendering.

## Three-Phase Plan

### Phase 1: Core Framework (~1,329 lines)
- `TuiModel` struct with Init/Update/View
- Multi-line `InputModel` with cursor, history
- `MessageViewport` with scrollable rendering
- Agent stream event bridge (goroutine → tea.Msg)
- Basic slash command handling

**Result**: Functional interactive chat with streaming.

### Phase 2: Rich Rendering (~1,000 lines)
- Lipgloss `Theme` system (colors, borders, text styles)
- Glamour `MarkdownRenderer` for assistant messages
- Collapsible `ToolPanel` with status indicators
- `StatusBar` with model/workspace/token info
- `DiffRenderer` for edit/replace tools

**Result**: Visually polished output matching modern CLI tools.

### Phase 3: UX Polish (~1,000 lines)
- `CompletionState` — Autocomplete for `/commands` and `@files`
- `KeyBindings` — Configurable key resolver with mode-aware routing
- `ConfirmationPopup` — Yes/No dialog for dangerous tools
- Model selector popup (Ctrl+P)
- `CompletionPopup` rendering

**Result**: Full-featured TUI with modern UX patterns.

## Key Design Decisions

1. **TUI is not parallelizable** — Model/Update/View is tightly coupled. Can't split across agents/worktrees.
2. **AltScreen + MouseCellMotion** — Required for clean full-screen rendering without scrollback leaks.
3. **Goroutine event bridge** — Agent stream consumed in goroutine, events sent via `program.Send()`. Never mutate model from goroutine.
4. **Cached rendering** — `cachedLines` stores rendered message lines, only re-renders streaming text on each tick for performance.
5. **`--legacy` flag planned** — Original presenter preserved as fallback (not yet implemented).

## Migration

- `pi-go chat` / `pi-agent --mode chat` → TUI (default)
- `pi-go run -p "..."` → EnhancedPresenter (single-shot, no TUI)
- `pi-go serve` → HTTP server (desktop/mobile clients)

## Related

- [[tui-bubbletea]] — Implementation details
- [[tui-presenter]] — Legacy presenter system
- [[source-project-root-v41]] — Full implementation summary
