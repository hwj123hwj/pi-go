---
type: source
source_path: /
date: 2026-06-10
tags: [source, project-root, full-ingest]
---

# Source: Project Root (.)

> Full ingest of the project directory structure and key source files at `/`.

## Key Takeaways

- Pi-Go is a **generic Agent framework** written in Go, structured as 4 layers: Entrypoints → Application → Platform → Core
- The project has ~21,000 lines of Go code across 20 internal packages
- Module path: `github.com/earendil-works/pi-go` (Go 1.22+)
- External dependencies are minimal: gorilla/websocket, larksuite oapi-sdk, testify, godotenv
- 3 delivery entrypoints: CLI, Desktop (Electron+React), Server+Feishu bridge
- Coding-agent is the primary application layer, with 7 built-in tools and 14 slash commands

## Notable Facts

| Fact | Source |
|------|--------|
| Agent has 4 states: Idle, Running, Waiting, Error | `internal/agent/agent.go` |
| Tool interface has 5 methods: Name, Description, Parameters, Validate, Execute | `internal/agent/tool.go` |
| 3 optional Tool interfaces: ToolWithMode, ToolWithPromptInfo, ConcurrencySafeChecker | `internal/agent/tool.go` |
| `runtime.Application` is the key decoupling interface between Platform and Application layers | `internal/runtime/application.go` |
| Provider system uses plugin-style `Get(name)` registry with lazy loading | `internal/ai/providers/` |
| Session storage uses JSONL append-only format with tree branching | `internal/session/jsonl.go` |
| Goal-driven loop disables maxTurns when goal is non-empty | `internal/agent/agent.go` |

## Key Files Referenced

- `go.mod` — Module definition
- `.env.example` — Configuration template
- `.gitignore` — Ignore patterns
- `internal/agent/agent.go` — Agent struct and core methods
- `internal/agent/tool.go` — Tool interface definition
- `internal/runtime/application.go` — Application interface
- `internal/agent/loop.go` — Dual-loop implementation
- `internal/session/jsonl.go` — Session persistence
- `internal/compaction/compaction.go` — Context compaction
- `internal/tools/` — 7 built-in tools
- `internal/ai/providers/` — LLM provider plugins
- `internal/extensions/` — Extension system
- `internal/slashcmd/` — Slash command framework