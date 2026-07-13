---
type: entity
date: 2026-07-13
tags: [tui, bubble-tea, elm-architecture, lipgloss, glamour, terminal, interactive]
---

# TUI: Bubble Tea Interactive Terminal

> Full-screen interactive TUI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm architecture), replacing the legacy linear EnhancedPresenter for `--mode chat`. 17 Go files, 3,697 lines in `internal/tui/`.

**Source**: `internal/tui/`
**First appeared**: v0.10.1 (Phase 1)
**Design doc**: `docs/design/tui-bubbletea.md`
**Related**: [[tui-presenter]] (legacy, still used for `--mode run`), [[agent-loop]], [[tui-design]]

## Architecture

```
┌─────────────────────────────────────────────┐
│              TuiModel (root)                 │
│  ┌─────────────────────────────────────┐    │
│  │     MessageViewport (scrollable)      │    │
│  │  - Markdown rendering (glamour)       │    │
│  │  - Tool panels (collapsible borders)  │    │
│  │  - Streaming text (live)              │    │
│  │  - Smart auto-scroll                  │    │
│  └─────────────────────────────────────┘    │
│  ┌─────────────────────────────────────┐    │
│  │     InputModel (multi-line)           │    │
│  │  - Cursor, selection, copy            │    │
│  │  - History (Ctrl+R / ↑↓)             │    │
│  │  - Completion trigger (/ and @)      │    │
│  └─────────────────────────────────────┘    │
│  ┌─────────────────────────────────────┐    │
│  │     StatusBar (bottom)                │    │
│  │  - Provider/model, workspace, tokens  │    │
│  └─────────────────────────────────────┘    │
└─────────────────────────────────────────────┘
```

### Elm Architecture Flow

```
User Input → tea.KeyMsg → Update() → handleKeyPress()
                                    ↓
Agent Stream → goroutine → program.Send(tea.Msg) → Update()
                                    ↓
                              View() → string (rendered by Bubble Tea)
```

**Critical**: Agent stream runs in a **goroutine** and must NOT mutate model fields directly. All communication is via `program.Send(msg)` which safely delivers to `Update()` on the main goroutine.

### Program Initialization (`run.go`)

```go
p := tea.NewProgram(m,
    tea.WithAltScreen(),        // Full-screen alternate buffer
    tea.WithMouseCellMotion(),  // Capture mouse wheel for scrolling
)
```

- **AltScreen**: Prevents TUI output from leaking into terminal scrollback
- **MouseCellMotion**: Captures mouse wheel events so the TUI handles scrolling (not the terminal emulator)

## Key Components

### TuiModel (`tui.go`, 425 lines)

Root model holding all state: dimensions, messages, viewport, input, status bar, streaming buffer, session reference, confirmation callback, model selector.

Event routing in `Update()`:
- `tea.WindowSizeMsg` → resize viewport
- `tea.MouseMsg` → scroll viewport (wheel up/down)
- `tea.KeyMsg` → `handleKeyPress()` (priority: confirmation > completion > model select > normal input)
- `StreamTextMsg` → append to streaming buffer
- `ToolStartMsg/ToolEndMsg` → manage tool call lifecycle
- `StreamDoneMsg` → finalize assistant message

### MessageViewport (`viewport.go`, 275 lines)

Scrollable viewport with:
- **Smart auto-scroll**: Jumps to bottom unless user has scrolled up (`userScrolled` flag)
- **Scroll indicators**: Shows "↑ N new" or "↑ X%" when scrolled up
- **Incremental rendering**: `cachedLines` for messages, only re-renders streaming text
- **Slice safety**: View() **copies** the visible slice before modifying (v0.10.12 fix for Go slice aliasing bug)

### ToolPanel (`tool_panel.go`, 260 lines)

Collapsible bordered panel for each tool execution:
- **Collapsed**: `┌─ ⚡ bash ─── ✓ 1.2s ▸ ─┐` (name + args + status)
- **Expanded**: Header + body (result text, diff highlight for edit tools)
- Toggle via `Ctrl+O`
- Width calculation uses `innerWidth()` (width - 4 for border + padding)

### InputModel (`input.go`, 424 lines)

Multi-line text input with:
- Cursor movement (arrows, Home, End, Ctrl+A/E)
- Word-level deletion (Ctrl+W)
- History navigation (↑↓, Ctrl+R)
- Completion trigger (`/` for slash commands, `@` for files)

### Keybindings (`keybindings.go`, 192 lines)

| Key | Action |
|-----|--------|
| Enter | Submit message |
| Ctrl+J | Newline |
| Ctrl+C | Cancel (if busy) / Exit (if idle) |
| Ctrl+D | Exit |
| Ctrl+L | Clear screen |
| Ctrl+O | Toggle tool panel collapse |
| Ctrl+P | Model selector popup |
| Ctrl+R | Search history |
| ↑↓ | History navigation |
| PageUp/PageDown | Scroll viewport |

### Confirmation Dialog (`confirm_tui.go`, 189 lines)

Yes/No dialog for dangerous tools (bash, write, edit). Rendered as floating popup with `[Y] Yes / [N] No / [Esc] Cancel`. Blocks all other input until resolved.

### Completion System (`completion.go` + `completion_popup.go`, 487 lines)

- **Slash command completion**: `/he` → `/help`, `/new`, etc.
- **File completion**: `@main` → `@main.go`, etc.
- Popup rendered as bordered dropdown with highlighted selection

### Theme (`theme.go`, 181 lines)

Lipgloss-based color theme:
- UserLabel (blue bold), AssistantLabel (magenta bold)
- ToolHeader (cyan), ToolBody (dim)
- StatusBar styles, SuccessText (green), ErrorText (red)
- Border styles: ToolDoneBorder (dim), ToolActiveBorder (cyan), ToolErrorBorder (red)

## Agent Event Bridge

The agent stream is consumed in a goroutine (`startAgentStream` in `update.go`). Each `agent.AgentStreamEvent` is mapped to a `tea.Msg` and sent via `program.Send()`:

| Agent Event | tea.Msg | TUI Action |
|-------------|---------|------------|
| TextDelta | StreamTextMsg | Append to streamBuf, update viewport |
| ToolStart | ToolStartMsg | Create pending tool entry |
| ToolUpdate | ToolUpdateMsg | Update partial result |
| ToolEnd | ToolEndMsg | Finalize tool result |
| Compacted | CompactionMsg | Show compaction notice |
| LoopDetected | LoopDetectedMsg | Show loop warning |
| Done | StreamDoneMsg | Finalize message, update tokens |
| Error | AgentErrorMsg | Show error |
| ConfirmationReq | ConfirmationMsg | Show confirmation dialog |

## Bug History

See [[source-project-root-v41]] for the full 12-round bug fix table (v0.10.1–v0.10.12). Key lessons:

1. **Go slice aliasing** (v0.10.12): `slice[start:end]` shares the underlying array. Always `copy()` before modifying.
2. **Terminal scrollback leak** (v0.10.10–11): AltScreen alone doesn't prevent mouse wheel from scrolling terminal buffer. Need `WithMouseCellMotion()`.
3. **ANSI injection** (v0.10.1): Terminal OSC sequences can leak into program input. Always sanitize.
4. **Dual presenter conflict** (v0.10.2): Having both EnhancedPresenter and TUI active causes duplicate output.

## Related

- [[tui-presenter]] — Legacy presenter, still used for `--mode run`
- [[tui-design]] — Design document
- [[agent-loop]] — Agent stream events bridge to TUI
- [[tool-lifecycle-hooks]] — Tool start/end events feed tool panels
- [[config-system]] — PI_GO_ENABLE_BASH for bash tool access
- [[deployment-infrastructure]] — Release pipeline
