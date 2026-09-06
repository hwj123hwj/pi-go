# TUI Design Document — Bubble Tea Powered Interactive CLI

> Status: DRAFT
> Created: 2026-07-10
> Owner: pi-go team

## 1. Background & Motivation

pi-go's agent backend (tools, AI, session, feishu) is significantly richer than
pi (earendil) and hwjcode. However, the **terminal interaction layer** is minimal:
single-line `bufio.Scanner` input + linear ANSI text output.

Competitor analysis:

| Feature | pi (earendil) | hwjcode | pi-go (current) |
|---------|---------------|---------|-----------------|
| TUI engine | Self-built differential renderer (12K LoC) | Ink (React for CLI) + custom diff | Linear ANSI output (2K LoC) |
| Multi-line input | ✅ Emacs/Vim + undo stack | ✅ (via Ink) | ❌ Single-line Scanner |
| Markdown render | ✅ Syntax highlight + code blocks | ✅ (via Ink) | ❌ Plain text |
| Collapsible tool panels | ✅ Ctrl+O toggle | ✅ | ❌ |
| Differential rendering | ✅ Only redraws changed lines | ✅ backbuffer diff | ❌ Full redraw |
| Fuzzy search | ✅ Built-in | ❌ | ❌ |
| Theme system | ✅ Auto-detect + switcher | ❌ | ❌ |
| Keybindings | ✅ Full system | ❌ | ❌ |

**Goal**: Build a Bubble Tea powered TUI that matches pi/hwjcode UX quality while
keeping pi-go's existing backend strengths.

---

## 2. Tech Stack

| Component | Library | Why |
|-----------|---------|-----|
| TUI Framework | `github.com/charmbracelet/bubbletea` | Go standard, 28k star, Elm architecture |
| Styling | `github.com/charmbracelet/lipgloss` | Already in go.mod (fancy presenter uses it) |
| Markdown | `github.com/charmbracelet/glamour` | Best Go terminal Markdown renderer |
| Fuzzy search | `github.com/sahilm/fuzzy` | Fast fuzzy matching |
| Spinner | (built into bubbletea) | No extra dep |

**Already available**: lipgloss is already a dependency. bubbletea needs to be added.

---

## 3. Architecture

### 3.1 Current Flow (Linear)

```
User types line → Scanner.Scan() → session.Send() → stream events → Presenter.render()
```

### 3.2 Target Flow (Bubble Tea Elm Architecture)

```
┌──────────────────────────────────────────────────────┐
│                    Bubble Tea Program                 │
│                                                       │
│  ┌─────────┐    ┌──────────┐    ┌─────────────────┐  │
│  │  Model   │←──│  Update   │←──│  View           │  │
│  │ (state)  │──→│ (msg →   │    │ (model → string)│  │
│  │          │   │  model)  │    │                 │  │
│  └────┬─────┘   └────┬─────┘    └─────────────────┘  │
│       │              │                                │
│       │         ┌────┴──────────────┐                │
│       │         │  Msg types:        │                │
│       │         │  - KeyMsg          │                │
│       │         │  - AgentEventMsg   │ ← from agent   │
│       │         │  - StreamMsg       │ ← LLM stream   │
│       │         │  - ToolUpdateMsg   │ ← tool output  │
│       │         │  - ErrorMsg        │                │
│       │         │  - TickMsg         │ ← spinner      │
│       │         └───────────────────┘                │
│       │                                              │
│  ┌────┴──────────────────────────────┐              │
│  │        Agent Goroutine             │              │
│  │  session.Stream(ctx, input)        │              │
│  │  → for event := range stream       │              │
│  │      program.Send(AgentEventMsg)   │              │
│  └────────────────────────────────────┘              │
└──────────────────────────────────────────────────────┘
```

### 3.3 Package Structure

```
internal/tui/
├── tui.go              # Main model: orchestrates all components
├── model.go            # TuiModel struct (global state)
├── messages.go         # All tea.Msg types
├── update.go           # Update function (handles all msgs)
├── view.go             # View function (composes layout)
├── theme.go            # Color scheme + style definitions
├── input.go            # Multi-line input editor component
├── viewport.go         # Scrollable message history viewport
├── messages_view.go    # Renders conversation messages
├── tool_panel.go       # Collapsible tool execution panel
├── status_bar.go       # Bottom status bar (model, tokens, mode)
├── markdown.go         # Glamour markdown renderer wrapper
├── spinner.go          # Thinking indicator
├── keybindings.go      # Key binding definitions + handlers
├── completion.go       # Slash command + file path autocomplete
├── completion_popup.go # Autocomplete dropdown popup
└── tui_test.go         # Tests
```

---

## 4. Component Design

### 4.1 Main Model (`tui.go`)

```go
type TuiModel struct {
    // Dimensions
    width, height int

    // Sub-components
    input        *InputModel      // multi-line editor
    viewport     viewport.Model   // scrollable history
    spinner      spinner.Model    // thinking indicator
    toolPanels   []*ToolPanel     // active tool execution panels
    completion   *CompletionModel // autocomplete state

    // State
    messages     []Message        // conversation history
    streaming    bool             // is agent generating?
    agentBusy    bool             // is agent running tools?
    session      *runtime.AgentSession
    slashCmds    *slashcmd.Registry

    // Event channel: agent goroutine → bubble tea main loop
    eventChan    chan tea.Msg
}
```

### 4.2 Multi-line Input (`input.go`)

Inspired by pi-tui's Editor component:

| Feature | Implementation |
|---------|---------------|
| Multi-line | Internal `[]string` line buffer |
| Cursor movement | Arrow keys, Home/End, Ctrl+A/E |
| Word movement | Alt+B/F (Emacs) |
| Delete | Backspace, Delete, Ctrl+K (kill line) |
| Undo/Redo | Ctrl+/ (undo), Alt+/ (redo) |
| History | Up/Down (when on first/last line) |
| Paste | Ctrl+V / bracketed paste |
| Submit | Enter (single line) / Ctrl+Enter or Shift+Enter (multi-line) |

```go
type InputModel struct {
    lines    []string
    cursorX  int
    cursorY  int
    undoStack *UndoStack
    history  []string
    pasteMode bool
}
```

### 4.3 Message View (`messages_view.go`)

```go
type Message struct {
    Role      string    // "user", "assistant", "system"
    Content   string    // raw markdown text
    Timestamp time.Time
    Tools     []ToolCall // tool calls within this message
}

type ToolCall struct {
    Name      string
    Args      string
    Result    string
    IsError   bool
    Collapsed bool       // user can toggle with Ctrl+O
    Streaming bool       // still executing
}
```

Rendering:
- User messages: dim background, right-aligned prefix
- Assistant messages: full width, Markdown rendered via glamour
- Tool calls: collapsible panels with colored borders
- Errors: red border + error icon

### 4.4 Collapsible Tool Panel (`tool_panel.go`)

```
┌─ 🔧 bash ──────────────────────────────── ▸ ─┐  ← collapsed (default)
└────────────────────────────────────────────────┘

┌─ 🔧 bash ──────────────────────────────── ▾ ─┐  ← expanded (Ctrl+O)
│ $ go test ./internal/tools/                   │
│ ok  github.com/hwj123hwj/pi-go/internal/tools │
│ 0.012s                                        │
└────────────────────────────────────────────────┘
```

### 4.5 Status Bar (`status_bar.go`)

```
┌─────────────────────────────────────────────────────────────────┐
│ ● ready │ model: gpt-4o │ tokens: 12.3k/128k │ workspace: pi-go │
└─────────────────────────────────────────────────────────────────┘
```

States: `● ready` / `⠼ thinking...` / `⚡ running: bash` / `⚠ loop detected`

### 4.6 Markdown Rendering (`markdown.go`)

```go
type MarkdownRenderer struct {
    renderer *glamour.TermRenderer
}

func NewMarkdownRenderer(width int) *MarkdownRenderer {
    r, _ := glamour.NewTermRenderer(
        glamour.WithAutoStyle(),
        glamour.WithWordWrap(width),
    )
    return &MarkdownRenderer{renderer: r}
}
```

Features:
- Code blocks with syntax highlighting
- Tables
- Bold/italic/links
- Auto dark/light theme detection

### 4.7 Autocomplete (`completion.go`)

Triggered by:
- `/` → slash commands
- `@` → file paths

```
User types: /mo│
             ┌─────────────────────┐
             │ model    Select model │
             │ models  List models  │
             └─────────────────────┘
```

---

## 5. Key Bindings

| Key | Action | Context |
|-----|--------|---------|
| `Enter` | Submit message | Single-line input |
| `Shift+Enter` / `Ctrl+J` | Newline | Multi-line input |
| `Ctrl+C` | Cancel agent / Clear input | Global |
| `Ctrl+D` | Exit (when input empty) | Global |
| `Ctrl+L` | Clear screen | Global |
| `Ctrl+O` | Toggle tool panel expand/collapse | Tool panel focused |
| `Ctrl+P` | Open model selector | Global |
| `Ctrl+R` | Search command history | Input |
| `Ctrl+Z` | Undo input | Input |
| `↑/↓` | Navigate history / Move cursor | Input |
| `PgUp/PgDn` | Scroll message history | Viewport |
| `Tab` | Accept autocomplete | Completion popup |
| `Esc` | Close popup / Cancel autocomplete | Global |

---

## 6. Event Flow: Agent → TUI

```go
// Agent runs in a goroutine, streams events via channel
func (m *TuiModel) sendMessage(input string) tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()
        stream := m.session.Stream(ctx, input)

        for event := range stream {
            switch e := event.(type) {
            case agent.EventToolExecutionStart:
                m.program.Send(ToolStartMsg{...})
            case agent.EventToolExecutionEnd:
                m.program.Send(ToolEndMsg{...})
            // ...
            }
        }
        return StreamDoneMsg{}
    }
}
```

### Msg Types

```go
type StreamTextMsg struct{ Delta string }
type ToolStartMsg struct{ ID, Name string; Args any }
type ToolUpdateMsg struct{ ID string; Partial any }
type ToolEndMsg struct{ ID, Name string; Result any; IsError bool }
type StreamDoneMsg struct{}
type AgentErrorMsg struct{ Err error }
type ConfirmationMsg struct{ Req agent.ConfirmationRequest }
type CompactionMsg struct{ Summary string }
type LoopDetectedMsg struct{ Tool string; Count int }
```

---

## 7. Implementation Plan

### Phase 1: Core Framework (MVP)
**Goal**: Replace bufio.Scanner with a working Bubble Tea loop.

| Task | File(s) | Est. Lines |
|------|---------|------------|
| Add bubbletea + glamour deps | `go.mod` | — |
| TuiModel + Init/Update/View | `tui.go`, `model.go` | 300 |
| Multi-line input editor | `input.go` | 400 |
| Basic message viewport | `viewport.go` | 200 |
| Agent event bridge | `messages.go` | 150 |
| Wire into interactive mode | `cli/interactive.go` | 50 |
| **Phase 1 total** | | **~1,100** |

**Deliverable**: `pi-agent` launches Bubble Tea, user can type multi-line messages,
agent responses appear in scrollable viewport. No styling yet.

### Phase 2: Rich Rendering
**Goal**: Markdown rendering + tool panels + status bar.

| Task | File(s) | Est. Lines |
|------|---------|------------|
| Glamour markdown renderer | `markdown.go` | 100 |
| Collapsible tool panels | `tool_panel.go` | 300 |
| Status bar (model, tokens) | `status_bar.go` | 150 |
| Theme system | `theme.go` | 200 |
| Diff rendering (edit tool) | `tool_panel.go` | 150 |
| **Phase 2 total** | | **~900** |

**Deliverable**: Full visual parity with pi/hwjcode. Markdown with syntax highlighting,
tool output in collapsible panels, colored status bar.

### Phase 3: UX Polish
**Goal**: Match pi's interaction quality.

| Task | File(s) | Est. Lines |
|------|---------|------------|
| Slash command autocomplete | `completion.go`, `completion_popup.go` | 300 |
| File path autocomplete | `completion.go` | 100 |
| Key binding system | `keybindings.go` | 150 |
| Command history search | `input.go` | 100 |
| Confirmation dialog UI | `confirm.go` (TUI version) | 150 |
| Model selector popup | `model_selector.go` | 200 |
| **Phase 3 total** | | **~1,000** |

**Deliverable**: Full TUI experience — autocomplete, popups, key bindings.

### Total: ~3,000 lines across 3 phases.

---

## 8. Migration Strategy

### Backward Compatibility

The TUI will be the **default** interactive mode. The old linear CLI is kept as
fallback via a `--legacy` flag:

```go
// cmd/pi-agent/main.go
if legacyMode {
    cli.NewInteractiveMode(session, cmds, app).Run(ctx)
} else {
    tui.New(session, cmds, app).Run()
}
```

### Existing Code Reuse

| Existing | Reuse in TUI |
|----------|-------------|
| `internal/ui/presenter.go` DisplayEvent types | Convert to Msg types |
| `internal/ui/diff_renderer.go` | Reuse diff logic in tool panels |
| `internal/ui/markdown_renderer.go` | Replace with glamour |
| `internal/agent/event.go` all Event types | Map to TUI Msg types |
| `internal/slashcmd/` | Reuse for autocomplete |
| `internal/agents/coding/commands/` | Reuse command implementations |

### Phased Rollout

1. **Phase 1**: Hidden behind `--tui` flag (opt-in)
2. **Phase 2**: Default when TTY detected, `--legacy` for fallback
3. **Phase 3**: Remove `--legacy` option, TUI is the only mode

---

## 9. Testing Strategy

| Level | What | How |
|-------|------|-----|
| Unit | InputModel cursor/edit operations | Direct method calls, assert state |
| Unit | Markdown renderer output | Compare rendered string |
| Unit | ToolPanel collapse/expand | State assertion |
| Integration | Agent event → Msg conversion | Mock stream, verify Msg sequence |
| E2E | Full TUI interaction | `tea.TestProgram` with scripted input |

---

## 10. Dependencies to Add

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/glamour@latest
# Already have: lipgloss, bubbles (transitive)
```

---

## 11. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Bubble Tea blocks stdin during agent stream | High | Agent runs in goroutine, events via channel |
| Terminal compatibility (SSH, tmux) | Medium | Feature-detect: fallback to linear mode |
| Memory growth from message history | Medium | Implement scrollback limit (default 1000 msgs) |
| Ctrl+C conflict (cancel vs exit) | Medium | Context-aware: first Ctrl+C cancels agent, second exits |
| Glamour performance on large output | Low | Cache rendered markdown, lazy render |

---

## 12. Success Criteria

- [ ] `pi-agent` launches Bubble Tea TUI by default
- [ ] Multi-line input with cursor movement (arrows, Home/End, Ctrl+A/E)
- [ ] Agent responses rendered as Markdown (code blocks, bold, lists)
- [ ] Tool calls shown in collapsible panels (Ctrl+O toggle)
- [ ] Status bar shows model name + token count
- [ ] Slash command autocomplete (/trigger)
- [ ] Existing slash commands all work (/model, /compact, /tools, etc.)
- [ ] Confirmation dialogs for dangerous tools
- [ ] `--legacy` flag for fallback to old CLI
- [ ] All existing tests still pass
- [ ] SSH/tmux compatibility verified
