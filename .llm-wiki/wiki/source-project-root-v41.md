---
type: source
source_path: "."
date: 2026-07-13
tags: [tui, bubble-tea, installer, ci-cd, release, bugfix, mouse, scroll, slice-bug, bash-tool]
---

# Source: Project Root (.) — v41: TUI Bubble Tea Implementation + Installer + Release Pipeline

> Comprehensive re-ingest covering the full Bubble Tea TUI rewrite (Phase 1–3), one-line installer with config wizard, GitHub Actions CI/CD, and 12 rounds of TUI bug fixes (v0.10.1–v0.10.12).

## Key Takeaways

1. **TUI is now the default interactive mode** — The old linear `EnhancedPresenter` is replaced by a full-screen Bubble Tea TUI with Elm architecture (Model/Update/View). 3,697 lines across 17 Go files in `internal/tui/`.
2. **Three-phase TUI rollout**: Phase 1 (core framework: multi-line input, viewport, agent event bridge), Phase 2 (rich rendering: theme, markdown, tool panels, status bar), Phase 3 (UX polish: autocomplete, keybindings, confirmation dialog, model selector).
3. **One-line installer** (`scripts/install.sh`) downloads pre-compiled binaries from GitHub Releases, configures PATH, creates `.env` with interactive API key wizard, enables bash tool by default.
4. **GitHub Actions CI/CD** (`.github/workflows/deploy.yml`) cross-compiles darwin/linux × amd64/arm64 binaries, attaches to release tags, deploys to Ubuntu server via systemd.
5. **12 rounds of TUI bug fixes** (v0.10.1–v0.10.12) addressing: ANSI escape injection, garbled output, tool panel rendering, confirmation dialog, spinner, token tracking, /new /switch /clear, mouse scrollback leak, banner scrollback leak, **Go slice aliasing corruption**.

## Important Entities & Concepts

- [[tui-bubbletea]] — Full-screen Bubble Tea TUI (Elm architecture, 17 files, 3,697 lines)
- [[tui-presenter]] — Legacy enhanced presenter (still used for `--mode run` single-shot mode)
- [[deployment-infrastructure]] — Updated: GitHub Actions cross-compile release workflow
- [[config-system]] — Updated: `PI_GO_ENABLE_BASH` now default in installer

## TUI File Map

```
internal/tui/
├── tui.go              (425 lines) — Root TuiModel, Init/Update/View, event routing
├── update.go           (452 lines) — handleKeyPress, agent stream goroutine, slash commands
├── input.go            (424 lines) — Multi-line input model with cursor, history, copy
├── viewport.go         (275 lines) — Scrollable message viewport with smart auto-scroll
├── tool_panel.go       (260 lines) — Collapsible bordered tool execution panels
├── completion.go       (283 lines) — Autocomplete state machine (slash commands, files)
├── completion_popup.go (204 lines) — Popup rendering for autocomplete dropdown
├── confirm_tui.go      (189 lines) — Yes/No confirmation dialog for dangerous tools
├── keybindings.go      (192 lines) — Action enum + key resolver (input/completion modes)
├── theme.go            (181 lines) — Lipgloss theme: colors, styles, borders
├── status_bar.go       (114 lines) — Bottom status bar (model, workspace, token counts)
├── markdown.go         (130 lines) — Glamour markdown renderer wrapper
├── diff_renderer.go    (96 lines)  — Color-coded unified diff rendering
├── messages.go         (97 lines)  — tea.Msg types + ChatMessage/ToolCallInfo structs
├── run.go              (67 lines)  — Entry point: tea.NewProgram with AltScreen + MouseCellMotion
└── tui_test.go         (308 lines) — Unit tests
```

## Notable Bug Fixes (v0.10.1–v0.10.12)

| Version | Bug | Root Cause | Fix |
|---------|-----|-----------|-----|
| v0.10.1 | ANSI escape codes garbling input | OSC 11 background query leaking into input | Filter ANSI from input buffer + sanitize config |
| v0.10.2 | Duplicate display + garbled output | EnhancedPresenter + TUI both active | TUI handles all rendering, presenter disabled |
| v0.10.3 | Tool panel rendering bugs | `v.width` instead of `innerWidth()` | Use `tp.innerWidth()` for content width |
| v0.10.4 | Tool args showing `<nil>` | `fmt.Sprintf("%v", nil)` in args display | `formatToolArgs()` handles nil/JSON/raw |
| v0.10.5 | Tool panel status icons cut off | Border width not subtracted from content | Rewrite panel layout with `innerWidth()` |
| v0.10.6 | Confirmation dialog, cancel flow | Missing event types in TUI update loop | Wire all agent events to tea.Msg |
| v0.10.7 | Spinner, token tracking, history | Missing spinner tick, no token display | Add `spinnerTick()` cmd, status bar tokens |
| v0.10.8 | /new, /switch, /clear broken | Session switch not propagating to TUI | Handle `result.SessionSwitchTo` in TUI |
| v0.10.9 | Tool panel status icons still wrapping | Width calculation still off | Comprehensive rewrite of panel layout |
| v0.10.10 | Banner leaks into terminal scrollback | `fmt.Print(BannerText())` before `p.Run()` | Remove banner print, AltScreen handles it |
| v0.10.11 | Mouse scroll leaks to terminal scrollback | No mouse capture, terminal handles wheel | Enable `WithMouseCellMotion()`, handle `MouseMsg` |
| v0.10.12 | **Scroll indicator corrupts message content** | **Go slice aliasing**: `v.lines[start:end]` shares array, writing indicator to `visible[last]` corrupts `v.lines` | **Copy slice before modifying**: `copy(visible, tmp)` |

### Critical Bug: Go Slice Aliasing (v0.10.12)

The most subtle bug. In `viewport.go` `View()`:

```go
// BUG: v.lines[start:end] shares the underlying array
visible := v.lines[start:end]
// Writing to visible[last] corrupts v.lines permanently!
visible[len(visible)-1] = indicator

// FIX: Copy the slice first
tmp := v.lines[start:end]
visible := make([]string, len(tmp))
copy(visible, tmp)
```

This caused the scroll indicator "↑ N new (scroll down to view)" to be **permanently written into message content**, appearing on every redraw — users saw it repeated every few lines throughout the conversation.

## Installer Features

- Platform detection (linux/darwin × amd64/arm64)
- Pre-compiled binary download from GitHub Releases (fallback: source build with Go)
- Interactive API key wizard (OpenAI / Anthropic / skip)
- `PI_GO_ENABLE_BASH=true` now included by default
- PATH auto-configuration for `.zshrc` / `.bashrc` / `.config/fish/config.fish`
- `pi-go` wrapper script (subcommand shortcuts: chat, serve, run)

## Contradictions with Existing Wiki

1. **[[tui-presenter]] describes EnhancedPresenter as default** — Now TUI is default for `--mode chat`. EnhancedPresenter only used for `--mode run` single-shot. The fancy_presenter.go / factory pattern is effectively dead code.
2. **[[tui-design]] concept page referenced in index** but file doesn't exist at `wiki/tui-design.md` — needs creation.

## Cross-References

- [[tui-bubbletea]] — NEW entity page for the Bubble Tea TUI
- [[tui-design]] — Design document (docs/design/tui-bubbletea.md)
- [[deployment-infrastructure]] — Updated with release workflow
- [[config-system]] — Updated with PI_GO_ENABLE_BASH
- [[agent-loop]] — TUI bridges agent stream events to tea.Msg via goroutine
