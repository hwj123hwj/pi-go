---
type: source
source_path: /
date: 2026-06-20
tags: [source, project-root, full-ingest]
---

# Source: Project Root (.)

> Full ingest of the project directory structure and key source files at `/`.

## Key Takeaways

- Pi-Go is a **generic Agent framework** written in Go, structured as 4 layers: Entrypoints → Application → Platform → Core
- The project has ~12,400 lines of Go code across 22 internal packages + 39 test files
- Module path: `github.com/earendil-works/pi-go` (Go 1.24+)
- External dependencies are minimal: gorilla/websocket, larksuite oapi-sdk, testify, godotenv, html-to-markdown
- 3 delivery entrypoints: CLI, Desktop (Electron+React), Server+Feishu bridge
- 2 application layers: coding-agent (primary) and music-agent (new)
- 8 built-in tools (added `web_fetch`) and 14 slash commands

## Notable Facts

| Fact | Source |
|------|--------|
| Agent has 4 states: Idle, Running, Waiting, Error | `internal/agent/agent.go` |
| Tool interface has 5 methods: Name, Description, Parameters, Validate, Execute | `internal/agent/tool.go` |
| 5 optional Tool interfaces: ToolWithMode, ToolWithPromptInfo, ConcurrencySafeChecker, ToolWithPrepareArguments, ToolWithConfirmation | `internal/agent/tool.go` |
| `runtime.Application` is the key decoupling interface between Platform and Application layers | `internal/runtime/application.go` |
| Provider system uses plugin-style `Get(name)` registry with lazy loading | `internal/ai/providers/` |
| Session storage uses JSONL append-only format with tree branching | `internal/session/jsonl.go` |
| Goal-driven loop disables maxTurns when goal is non-empty | `internal/agent/agent.go` |
| Loop detection uses SHA256(name+":"+args) fingerprinting, per-prompt reset | `internal/agent/loop_detect.go` |
| Two-tier compaction: MicroCompact (60% threshold, no LLM) → Full Compact (90%, LLM summary) | `internal/compaction/compaction.go` |
| web_fetch has SSRF protection: isPrivateHost check at entry + per-redirect CheckRedirect | `internal/tools/web_fetch_security.go` |
| Music agent has 6 tools: search, play, lyrics, detail, playlist, recommend | `internal/agents/music/tools/` |
| Session-level hooks (SessionStart/SessionEnd/PreCompress) are observation-only, errors logged but never block | `internal/agent/tool_lifecycle.go` |
| Tool confirmation: `ToolWithConfirmation` + `ConfirmFunc` injection; nil ConfirmFunc = default approve | `internal/agent/tool_lifecycle.go` |
| 11 stream event types including `confirmation_request`, `loop_detected`, `micro_compacted` | `internal/agent/agent.go` |

## Key Files Referenced

- `go.mod` — Module definition (Go 1.24+)
- `.env.example` — Configuration template
- `internal/agent/agent.go` — Agent struct and core methods
- `internal/agent/tool.go` — Tool interface definition
- `internal/agent/loop_detect.go` — Loop detection system
- `internal/agent/tool_lifecycle.go` — Lifecycle hooks + confirmation + session hooks
- `internal/runtime/application.go` — Application interface
- `internal/agent/loop.go` — Dual-loop implementation + MicroCompact integration
- `internal/session/jsonl.go` — Session persistence
- `internal/compaction/compaction.go` — Two-tier compaction (MicroCompact + Full)
- `internal/tools/web_fetch.go` — web_fetch built-in tool
- `internal/tools/web_fetch_security.go` — SSRF protection for web_fetch
- `internal/tools/` — 8 built-in tools
- `internal/agents/music/application.go` — MusicApplication implementation
- `internal/agents/music/tools/` — 6 music-agent tools
- `internal/music/` — Music service (NetEase client, cache, HTTP handler)
- `internal/ai/providers/` — LLM provider plugins
- `internal/extensions/` — Extension system
- `internal/slashcmd/` — Slash command framework
