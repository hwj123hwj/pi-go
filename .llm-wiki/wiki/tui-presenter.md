---
type: entity
date: 2026-06-14
tags: [cli, tui, presenter, renderer, terminal]
---

# Terminal UI & Presenter System

> The terminal user interface system for interactive CLI mode, with two presenter implementations.

## Source: `internal/ui/`

```
internal/ui/
├── presenter.go         ← DisplayEvent types, event-to-DisplayEvent conversion
├── renderer.go          ← TUIRenderer interface
├── markdown_renderer.go ← Terminal markdown rendering (519 lines)
├── diff_renderer.go     ← Color-coded unified diff rendering
├── enhanced_presenter.go ← Default lightweight TUI presenter (~638 lines, ANSI)
├── fancy_presenter.go   ← Bubble Tea / Lipgloss fancy TUI presenter
├── factory_fancy.go     ← Factory for fancy presenter (build with -tags fancy)
└── factory_default.go   ← Factory for enhanced presenter (default build)
```

## Display Events

The `DisplayEvent` system converts internal agent events to formatted terminal output:

| Event Type | Display |
|------------|---------|
| `TextDelta` | Streaming text chunks |
| `TurnStart` | "Thinking..." indicator |
| `TurnEnd` | Turn completion marker |
| `ToolStart` | Tool name + arguments |
| `ToolUpdate` | Progress updates |
| `ToolEnd` | Tool result |
| `ToolError` | Red error message |
| `SystemMsg` | Dimmed system message |

## TUIRenderer Interface

```go
type TUIRenderer interface {
    Render(event DisplayEvent) error
    Close() error
}
```

## Presenter Implementations

### Enhanced Presenter (Default)
- Lightweight ANSI-based formatting
- No external dependencies
- Supports: markdown rendering, diff display, progress indicators, typing dots
- File: `enhanced_presenter.go`

### Fancy Presenter (Tagged Build)
- Built with `-tags fancy` compilation flag
- Uses `charmbracelet/bubbletea` and `charmbracelet/lipgloss`
- Richer visual rendering: borders, colored frames, aligned components
- File: `fancy_presenter.go`

## Markdown Renderer

`markdown_renderer.go` (519 lines) handles:
- Headings (H1-H4) with ANSI bold
- Inline formatting: bold, italic, strikethrough
- Code blocks with syntax hints
- Inline code
- Links (shows URL in dimmed text)
- Lists (ordered and unordered)
- Blockquotes with vertical bar
- Tables (column alignment)
- Thematic breaks

## Diff Renderer

`diff_renderer.go` provides:
- Color-coded unified diff display
- Green for additions, red for deletions
- Line number annotations
- Hunk headers in bold

## Factory Pattern

Two factory files provide conditional compilation:
- `factory_fancy.go` — Returns `FancyPresenter` when built with `-tags fancy`
- `factory_default.go` — Returns `EnhancedPresenter` as fallback

## Related

- [[coding-application]] — CLI interactive mode uses the presenter
- [[server-websocket]] — Alternative event display via desktop
