# hwjcode Feature Absorption Batch

> 2026-06-29 — Parallel worktree absorption of 4 hwjcode modules (~10,800 lines)

## What was absorbed

### 1. Enhanced Tools (`internal/tools/`) ✅ Wired
- `multiedit.go` — Transactional multi-edit on single file
- `patch.go` — Unified diff patch application
- `batch.go` — Batch tool execution (needs ToolRegistry)
- `todo.go` — Todo list management (`.pi-go/todo.json`)
- `memory.go` — Long-term memory (`~/.pi-go/memory.json`)
- `local_time.go` — Local time with timezone support
- `delete_file.go` — Safe file deletion with backup
- `read_many_files.go` — Multi-file batch read (supports external paths)
- `ask_user.go` — User interaction ⚠️ **STUB** (see Issues)

**Status**: ✅ Registered in `tools.go` BuildList. All use `IsPathSafe()` for workspace path validation.

### 2. Hooks System (`internal/hooks/`) ⚠️ NOT WIRED
- `types.go` — Hook event types (PreToolCall, PostToolCall, PreMessage, etc.)
- `registry.go` — Hook registration with priority sorting
- `runner.go` — Sequential execution with timeout
- `system.go` — Main entry point aggregating registry + runner
- `aggregator.go` — Hook result aggregation

**Status**: ⚠️ **NOT INTEGRATED**. Agent has `HookSystemInterface` field but runtime never sets it. Dead code — compiles and passes tests but never runs.

### 3. Policy Engine (`internal/policy/`) ⚠️ NOT WIRED
- `engine.go` — Allow/Deny/AskUser decision engine
- `rules.go` — Pattern matching (tool name + file path glob)
- `updater.go` — Policy persistence to `.pi-go/policy.json`

**Status**: ⚠️ **NOT INTEGRATED**. No import outside its own package.

### 4. MCP Client (`internal/mcp/`) ⚠️ NOT WIRED
- `client.go` — JSON-RPC 2.0 over stdio/SSE
- `manager.go` — Multi-server management with auto-reconnect
- `tool.go` — MCP→agent.Tool adapter
- `config.go` — Config loading from `.pi-go/mcp.json`

**Status**: ⚠️ **NOT INTEGRATED**. No import outside its own package. Needs wiring into tool registration.

### 5. LSP Tools (`internal/lsp/` + `internal/tools/lsp.go`) ✅ Wired
- `manager.go` — gopls process management (start/stop/timeout)
- `server.go` — Per-language server instance
- `jsonrpc.go` — JSON-RPC 2.0 communication
- `binary.go` — LSP binary discovery
- 6 navigation tools: go-to-def, find-refs, hover, doc-symbols, workspace-symbols, implementation

**Status**: ✅ Registered in `tools.go` via `NewLSPTools(opts.Workspace)`.

## Issues Found (Post-Merge Review)

### 🔴 FIXED: LSP Cache Dir Namespace Leak
LSP binary cache was `~/.easycode-user/lsp/` (copied from hwjcode).
Fixed → `~/.pi-go/lsp/`.

### 🔴 FIXED: batch Tool ToolRegistry Never Injected
`BuildList` creates batch tool but never receives `ToolRegistry`, so batch tool always errors with "no tool registry".
Fixed → runtime `agent_session.go` now builds a `toolListRegistry` from the tool list after BuildTools() and injects it via `SetRegistry()` interface assertion. No dead code left.

### 🔴 FIXED: LSP Process Leak on Exit
gopls process was started lazily via global singleton but never cleaned up on agent exit.
Fixed → Added `defer tools.ResetLSPManager()` in `cmd/pi-agent/main.go`.

### 🟡 MAJOR-1: Hooks/Policy/MCP are "Island Code"
Three packages compile and pass tests but are never imported by the runtime.
Need integration into `agent_session.go` or `tools.go` BuildList.

**Fix**: Wire hooks into agent.NewAgent, policy into tool execution, MCP into tool registration.

### 🟡 MAJOR-2: ask_user Tool is a Stub
Execute() just formats text and returns. Does not actually collect user input.
Comment in code: "In a real implementation, this would pause and wait for user answers."

**Fix**: Needs confirmation callback mechanism integration (like existing confirmation flow).

### 🟢 OK: Security
- Path traversal: all file-mutating tools use `IsPathSafe()`
- read_many_files: supports external paths with explicit opt-in flag
- No injection vectors found

### 🟢 OK: Concurrency
- LSP: `sync.Mutex` on manager + `sync.Mutex` on doc state
- MCP: `sync.RWMutex` on manager + `sync.Mutex` on client
- Race detector: PASS (all 5 packages)

### 🟢 OK: Process Lifecycle
- LSP: `Close()` kills process via `cmd.Process.Kill()`
- MCP: `Close()` stops servers, has auto-reconnect goroutine
- No zombie process leaks detected

## Test Results
- `go build ./...`: ✅ PASS
- `go vet ./...`: ✅ PASS
- `go test ./...`: ✅ ALL PASS (0 failures)
- `go test -race`: ✅ PASS (hooks, policy, mcp, lsp, tools)
