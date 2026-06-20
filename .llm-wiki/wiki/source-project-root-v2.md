---
type: source
source_path: .
date: 2026-06-21
tags: [source, project-root, re-ingest, comprehensive]
---

# Source: Project Root (.) — Re-ingest v2

> Comprehensive re-ingest of the pi-go project root, covering all source files, documentation, and architectural decisions.

## Key Takeaways

- **Pi-Go** is a generic Agent framework written in Go, with a 4-layer architecture: Entrypoints → Application → Platform → Core
- The project has evolved from a learning project to a Beta-stage Agent framework with ~14,000 lines of Go code
- **3 delivery entrypoints**: CLI, Desktop (Electron+React), Server+Feishu bridge
- **Primary application**: coding-agent (code editing assistant) with 7 built-in tools and 14 slash commands
- **Key architectural decisions**: Runtime/Application interface decoupling, Operations abstraction (Local/SSH), Tool lifecycle hooks
- **Goal-driven loop**: LLM-based goal completion evaluator with keyword fallback
- **Loop detection**: SHA256 fingerprinting of consecutive identical tool calls with soft reminders

## Project Statistics

| Metric | Value |
|--------|-------|
| Language | Go 1.24+ |
| Non-test source files | 65 |
| Test files | 30+ |
| Total Go code | ~14,000 lines |
| Test coverage | ~43% |
| Built-in tools | 7 (read/write/edit/bash/grep/find/ls) |
| Slash commands | 14 |
| Interfaces | 17 |
| External dependencies | Minimal (gorilla/websocket, larksuite oapi-sdk, godotenv, testify) |

## Architecture: Four-Layer Design

```
Entrypoints (组装与入口)
  cmd/pi-agent  cmd/pi-feishu-bridge
    ↓
Application (领域应用层，可插拔)
  internal/agents/coding/ — 工具集、提示、命令、Profile
    ↓
Platform (运行时平台层，领域无关)
  internal/runtime/ — AgentSession 生命周期、Application 接口
    ↓
Core (核心层，零领域知识)
  agent/  ai/  session/  compaction/  operations/
  prompt/  skill/  extensions/  slashcmd/  tools/
```

**Dependency Rules**:
- Core → Does not depend on any upper layer
- Platform → Depends only on Core
- Application → Depends on Core + Platform via `runtime.Application` interface
- Entrypoints → Assembles all dependencies, injects Application instance

## Key Entities and Concepts

### Core Layer (`internal/`)

| Entity | Package | Description |
|--------|---------|-------------|
| [[agent-core]] | `agent/` | Agent state machine (4 states) + execution engine |
| [[tool-system]] | `agent/` | Tool interface + 7 built-in tools + optional interfaces |
| [[llm-provider-system]] | `ai/providers/` | Pluggable LLM backends (Anthropic/OpenAI/DeepV/Mock) |
| [[session-persistence]] | `session/` | JSONL append-only storage with tree branching |
| [[context-compaction]] | `compaction/` | LLM-driven conversation summarization |
| [[operations-abstract]] | `operations/` | Local/SSH execution backend switching |
| [[skill-system]] | `skill/` | Markdown skill loading from `.claude/skills/` |
| [[extension-system]] | `extensions/` | Plugin-style tools/commands/hooks |
| [[slash-command-framework]] | `slashcmd/` | 14 built-in slash commands |
| [[tool-lifecycle-hooks]] | `agent/` | Before/After hooks + PrepareArguments + Confirmation |

### Platform Layer

| Entity | Package | Description |
|--------|---------|-------------|
| [[runtime-application-interface]] | `runtime/` | The Platform↔App decoupling boundary |

### Application Layer

| Entity | Package | Description |
|--------|---------|-------------|
| [[coding-application]] | `agents/coding/` | Coding agent (primary application layer) |

### Entrypoints

| Entity | Package | Description |
|--------|---------|-------------|
| [[desktop-app]] | `desktop/` | Electron + React GUI client |
| [[feishu-integration]] | `feishu/` | Lark/Feishu bot bridge |
| [[server-websocket]] | `server/` | HTTP REST + SSE + WebSocket server |
| [[config-system]] | `config/` | Environment-driven configuration |

### Concepts

| Concept | Description |
|---------|-------------|
| [[four-layer-architecture]] | Entrypoints → Application → Platform → Core |
| [[agent-loop]] | Outer follow-up + inner tool-call dual loop |
| [[goal-driven-loop]] | Autonomous goal-directed agent execution |
| [[ai-transform-retry]] | Message transform, retry, cost, model registry |
| [[tui-presenter]] | Terminal UI rendering system |
| [[external-tools]] | HTTP callback tool registration |
| [[web-embed]] | Embedded SPA static file serving |

## Notable Design Patterns

### 1. Goal-Driven Loop with LLM Evaluator

The goal system uses a dual evaluation approach:

1. **Primary**: LLM-based evaluator (`goal_evaluator.go`) that judges completion via a focused prompt
2. **Fallback**: Keyword-based detection (`goal.go`) for when the LLM evaluator fails

```go
// goal_evaluator.go
func evaluateGoalCompletion(...) (bool, string) {
    // 1. Build evaluator prompt
    // 2. Call LLM with focused completion judgment
    // 3. Parse JSON response {"ok": true/false, "reason": "..."}
    // 4. Fallback to keyword matching on any error
}
```

### 2. Loop Detection with SHA256 Fingerprinting

Consecutive identical tool calls are detected via SHA256 hashing:

```go
// loop_detect.go
func fingerprint(name, args string) string {
    h := sha256.Sum256([]byte(name + ":" + args))
    return hex.EncodeToString(h[:])
}
```

- Threshold: 5 consecutive identical calls (configurable)
- Response: Soft reminder injected into follow-up queue (non-blocking)
- Reset: Per-prompt lifecycle (not cross-conversation)

### 3. Tool Lifecycle with Confirmation Gate

```
raw args → Validate → PrepareArguments → Before hooks → [Confirmation Gate] → Execute → After hooks
```

The confirmation gate (`ToolWithConfirmation` interface) enables user approval for dangerous operations:
- Tool declares `RequiresConfirmation(params) (description, ok)`
- Agent emits `EventConfirmationRequest` with description
- `ConfirmFunc` (injected by entrypoint) asks user for approval
- If denied: "user declined" returned as tool result (IsError=false)

### 4. Session-Level Observer Hooks

Three non-blocking observer hooks for session lifecycle:

| Hook | Event | Purpose |
|------|-------|---------|
| `SessionStartHook` | `SessionStartEvent{Goal}` | When Prompt/PromptStream begins |
| `SessionEndHook` | `SessionEndEvent{Err}` | When Prompt/PromptStream ends |
| `PreCompressHook` | `PreCompressEvent{ContextTokens, ContextWindow, MessageCount}` | Before context compaction runs |

## Key Files Referenced

### Core Agent
- `internal/agent/agent.go` — Agent struct, state machine, Prompt/PromptStream methods
- `internal/agent/loop.go` — Dual-loop implementation (605 lines)
- `internal/agent/tool.go` — Tool interface + optional interfaces
- `internal/agent/tool_lifecycle.go` — Before/After hooks, Confirmation gate
- `internal/agent/event.go` — 11 event types (including MicroCompacted, LoopDetected)
- `internal/agent/goal.go` — Keyword-based goal completion detection
- `internal/agent/goal_evaluator.go` — LLM-based goal completion evaluator
- `internal/agent/loop_detect.go` — SHA256 fingerprinting loop detection
- `internal/agent/message.go` — Thread-safe MessageQueue
- `internal/agent/errors.go` — ErrAgentBusy error type
- `internal/agent/partition_test.go` — Tool call partitioning tests

### AI Layer
- `internal/ai/stream.go` — EventStream with buffered channel
- `internal/ai/transform.go` — Message transformation (image downgrade, ID normalization)
- `internal/ai/retry.go` — Retry logic with exponential backoff
- `internal/ai/cost.go` — Token cost calculation
- `internal/ai/models/registry.go` — Model context window registry
- `internal/ai/providers/interface.go` — Provider interface + Registry
- `internal/ai/providers/anthropic.go` — Anthropic Messages API (411 lines)
- `internal/ai/providers/openai.go` — OpenAI Chat Completions API (395 lines)
- `internal/ai/providers/deepv.go` — DeepVcode Server API
- `internal/ai/providers/mock.go` — Mock provider for testing

### Tools
- `internal/tools/read.go` — File reading
- `internal/tools/write.go` — File writing
- `internal/tools/edit.go` — String-based file editing (179 lines)
- `internal/tools/bash.go` — Shell command execution
- `internal/tools/grep.go` — Content search (332 lines, pure Go regex)
- `internal/tools/find.go` — File search
- `internal/tools/ls.go` — Directory listing
- `internal/tools/truncate.go` — Output truncation (80/20 split)
- `internal/tools/path.go` — Path resolution and safety checks

### Session
- `internal/session/session.go` — Session struct and tree operations
- `internal/session/jsonl.go` — JSONL read/write with append-only
- `internal/session/interface.go` — SessionStorage interface
- `internal/sessionmgr/manager.go` — Session management (232 lines)

### Runtime
- `internal/runtime/application.go` — Application interface (BuildTools + BuildPrompt + NewSessionExt)
- `internal/runtime/agent_session.go` — AgentSession lifecycle (412 lines)
- `internal/runtime/session_registry.go` — Thread-safe session registry

### Application
- `internal/agents/coding/application.go` — CodingApplication implementation
- `internal/agents/coding/session_ext.go` — Per-session state (profile, goal)
- `internal/agents/coding/cli/interactive.go` — Interactive CLI mode
- `internal/agents/coding/commands/builtins.go` — 14 slash commands
- `internal/agents/coding/profile/profile.go` — Profile types & system prompts
- `internal/agents/coding/prompt/builder.go` — System prompt builder
- `internal/agents/coding/tools/tools.go` — Tool assembly
- `internal/agents/coding/tools/file_mutation.go` — Per-file FIFO serialization

### Server
- `internal/server/server.go` — Route definitions and handlers
- `internal/server/websocket.go` — WebSocket handler

### Desktop
- `desktop/electron/main.ts` — Electron main process
- `desktop/electron/pi-go-manager.ts` — Go backend process lifecycle
- `desktop/src/store.ts` — Zustand state store
- `desktop/src/App.tsx` — Root React component

### Documentation
- `README.md` — Project overview and quick start
- `AGENTS.md` / `.claude/CLAUDE.md` — AI agent guidance
- `docs/CONTRIBUTING.md` — Development workflow and coding standards
- `docs/PROJECT_CONTEXT.md` — High-level project snapshot
- `docs/PRODUCT_ROADMAP.md` — 5-phase product roadmap
- `docs/decisions/goal-compact-cross-framework.md` — Cross-framework analysis of /compact and /goal
- `docs/decisions/skills-vs-application.md` — Decision framework for Skills vs Application
- `docs/references/pi-go-analysis.md` — Deep analysis report (65 source files, 30 tests)

## Contradictions with Existing Wiki

None detected. This re-ingest confirms and expands upon existing wiki content.

## Changes Since Last Ingest (2026-06-14)

1. **New documents**: `docs/decisions/goal-compact-cross-framework.md`, `docs/decisions/skills-vs-application.md`
2. **Event types expanded**: Added `EventToolBatchStart`, `EventGoalCompleted`, `EventMicroCompacted`
3. **Loop detection**: Full implementation with SHA256 fingerprinting and configurable threshold
4. **Goal evaluator**: LLM-based evaluator with keyword fallback
5. **Confirmation gate**: `ToolWithConfirmation` interface for dangerous operations
6. **Session observer hooks**: `SessionStartHook`, `SessionEndHook`, `PreCompressHook`
