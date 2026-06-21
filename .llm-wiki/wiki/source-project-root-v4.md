---
type: source
source_path: .
date: 2026-06-22
tags: [project-root, tool-lifecycle, session-management, websocket, desktop, path-clicking]
---

# Source: Project Root (.) — v4

> Fourth ingest focusing on tool execution internals, session management architecture, WebSocket protocol, and desktop frontend improvements.

## Key Takeaways

1. **Tool execution has a precise 9-step lifecycle** — `executeOneTool()` in `loop.go` follows: Find → EmitStart → Validate → PrepareArguments → BeforeHooks → ConfirmationGate → BuildOnUpdate → Execute → AfterHooks → EmitEnd. Each step has well-defined error handling and event emission.

2. **Two-layer session management** — `sessionmgr.Manager` handles persistence (file-based CRUD, forking, listing with `meta.json`) while `runtime.SessionRegistry` handles in-memory runtime session routing. They serve different purposes and are both injected into `App`.

3. **WebSocket protocol is message-type based** — Client sends `{type, session_id, prompt}` messages; server responds with typed messages (`event`, `session_id`, `status`, `model_info`, `error`, `pong`). Supports prompt, cancel, switch_model, and ping.

4. **Desktop path clicking works at two levels** — `store.ts` extracts paths from tool results via `extractLocationsFromText` (4 regex patterns); `Markdown.tsx` renders paths in both backtick-wrapped and plain text via `renderTextWithFilePaths` (regex scanning + `isFilePath` validation).

5. **Server exposes file read/write endpoints** — `GET /sessions/{id}/file?path=` reads any file; `PUT /sessions/{id}/file?path=` writes files. CORS middleware includes PUT method. Used by desktop file pane.

6. **Dynamic model discovery from gateway** — `GET /models` tries to fetch from OpenAI-compatible `/v1/models` endpoint first, falls back to hardcoded list. Desktop store caches models.

7. **Session history restoration on setActive** — When desktop selects a session, it loads transcript via `GET /sessions/{id}/messages`, reconstructing user/assistant/tool/thought items. Tool call results are matched by `tool_call_id`.

8. **App assembly uses Application map** — `app.go` maintains `applications map[string]runtime.Application` with `ResolveApplication(name)` for per-session application selection. Server's `createSession` accepts `application` field.

9. **ConfirmationResult event carries user decision** — `EventConfirmationResult{ToolCallID, Approved, Reason}` is emitted after user approves/rejects a dangerous tool. Rejection reason is sent back to LLM as tool result (IsError=false).

10. **inferToolKind maps tool names to UI categories** — Desktop `store.ts` has a heuristic function mapping tool names to kinds: read/edit/delete/move/search/execute/think/fetch/other. Used for icon and display differentiation.

## Notable Architecture Details

- **partitionToolCalls** batches tool calls: consecutive concurrency-safe calls → parallel batch; each unsafe call → serial batch. Safety determined by `ConcurrencySafeChecker` and `ToolWithMode` interfaces.
- **maybeCompact** runs two-tier compaction: MicroCompact (clear old tool results at threshold) → full LLM compaction (higher threshold). Both emit events.
- **Goal evaluator** uses a focused LLM call (256 max tokens) with structured JSON output `{"ok": bool, "reason": string}`. Falls back to keyword matching on any error.
- **loopDetector** uses SHA256(name + ":" + args) fingerprinting. Per-prompt lifecycle (reset on new prompt). Threshold default: 5 consecutive identical calls.

## Files Read

| File | Lines | Key Content |
|------|-------|-------------|
| `internal/agent/loop.go` | 605 | Agent dual loop, executeOneTool 9-step flow, partitionToolCalls, maybeCompact |
| `internal/agent/tool_lifecycle.go` | ~170 | ToolCallContext, hooks, ConfirmationRequest/Decision, observer hooks |
| `internal/agent/tool.go` | ~100 | Tool interface, optional interfaces (Mode, PromptInfo, ConcurrencySafe, Confirmation) |
| `internal/agent/event.go` | ~130 | 16 event types including ConfirmationRequest/Result, LoopDetected, MicroCompacted |
| `internal/agent/loop_detect.go` | ~80 | SHA256 fingerprinting, threshold-based detection, reminder injection |
| `internal/agent/goal.go` | ~50 | Keyword-based goal completion fallback |
| `internal/agent/goal_evaluator.go` | ~140 | LLM-based goal evaluator with JSON parsing and keyword fallback |
| `internal/agent/external_tool.go` | ~100 | HTTP callback tool with URL validation |
| `internal/sessionmgr/manager.go` | ~200 | Session CRUD, forking, listing with meta.json |
| `internal/runtime/agent_session.go` | 459 | AgentSession lifecycle, model switching, compaction |
| `internal/runtime/session_registry.go` | ~120 | Thread-safe in-memory session routing |
| `internal/runtime/application.go` | ~60 | Application interface, SessionExt, build options |
| `internal/app/app.go` | 362 | App assembly, Application map, session deps |
| `internal/server/server.go` | 1001 | REST handlers, middleware, file endpoints, git diff |
| `internal/server/websocket.go` | ~250 | WebSocket protocol, prompt/cancel/switch_model |
| `internal/mode/serve.go` | ~40 | Thin serve mode wrapper |
| `internal/mode/interactive.go` | ~20 | Thin interactive mode wrapper |
| `internal/web/handler.go` | ~50 | SPA static file serving with fallback |
| `desktop/src/store.ts` | 780 | Zustand store, WebSocket events, extractLocationsFromText, inferToolKind |
| `desktop/src/components/Markdown.tsx` | 320 | Markdown renderer with path detection (backtick + plain text) |
| `desktop/electron/pi-go-manager.ts` | ~200 | Go backend subprocess management |

## Cross-References

- [[tool-lifecycle-hooks]] — Updated with full 9-step execution flow
- [[server-websocket]] — Updated with WebSocket protocol details, file endpoints, CORS fix
- [[desktop-app]] — Updated with path clicking implementation, store internals
- [[session-manager]] — NEW: sessionmgr package architecture
- [[session-persistence]] — Updated with sessionmgr reference
