---
type: entity
date: 2026-06-10
tags: [tools, lifecycle, hooks, middleware, confirmation]
---

# Tool Lifecycle Hooks

> Before/After hooks, argument preparation, confirmation gates, and session-level observer hooks.

## Execution-Level Hooks

| Hook | Timing | Purpose |
|------|--------|---------|
| `PrepareArguments` | Before Validate → Execute | Argument transformation and enrichment |
| `Before` | Before Execute | Pre-processing, validation, logging, can block execution |
| `After` | After Execute | Post-processing, auditing, can modify results |

## Execution Flow

```
raw args → Validate → PrepareArguments → Before hooks → [Confirmation Gate] → Execute → After hooks
```

## ToolWithPrepareArguments

Optional interface for normalizing/enriching validated arguments:

```go
type ToolWithPrepareArguments interface {
    Tool
    PrepareArguments(ctx context.Context, params json.RawMessage) (json.RawMessage, error)
}
```

Full execution order: raw args → `Validate()` → `PrepareArguments()` → before hooks → execute → after hooks.

## Confirmation Gate

Tools can implement `ToolWithConfirmation` to require user approval before execution:

```go
type ToolWithConfirmation interface {
    Tool
    RequiresConfirmation(params json.RawMessage) (description string, ok bool)
}
```

When `RequiresConfirmation` returns `ok=true`:
1. Agent emits `EventConfirmationRequest` with the description
2. `ConfirmFunc` (injected by entrypoint) asks the user for approval
3. If denied: tool is not executed, "user declined" is returned as tool result (IsError=false)
4. If `ConfirmFunc` is nil (serve/feishu): default approve

## Session-Level Observer Hooks

These hooks are **observation-only**: errors are logged via `slog.Warn` but never block execution.

| Hook | Event Type | Timing |
|------|-----------|--------|
| `SessionStartHook` | `SessionStartEvent{Goal}` | When `Prompt`/`PromptStream` begins |
| `SessionEndHook` | `SessionEndEvent{Err}` | When `Prompt`/`PromptStream` ends |
| `PreCompressHook` | `PreCompressEvent{ContextTokens, ContextWindow, MessageCount}` | Before context compaction runs |

## LifecycleHooks Aggregate

```go
type LifecycleHooks struct {
    Before       []BeforeToolCallHook   // Tool execution level (blocking)
    After        []AfterToolCallHook    // Tool execution level (blocking)
    SessionStart []SessionStartHook     // Session level (observer)
    SessionEnd   []SessionEndHook       // Session level (observer)
    PreCompress  []PreCompressHook      // Session level (observer)
}
```

## ToolCallContext

Carries the full context for a single tool invocation through lifecycle stages:

```go
type ToolCallContext struct {
    ToolCallID string
    ToolName   string
    RawArgs    json.RawMessage // original args from LLM
    Args       json.RawMessage // after Validate (and optionally PrepareArguments)
}
```

## Confirmation Gate

Tools can implement `ToolWithConfirmation` to require user approval before execution:

```go
type ToolWithConfirmation interface {
    Tool
    RequiresConfirmation(params json.RawMessage) (description string, ok bool)
}
```

**Flow**:
1. Tool's `RequiresConfirmation(args)` returns `(description, true)`
2. Agent emits `EventConfirmationRequest` with the description
3. `ConfirmFunc` (injected by entrypoint) asks the user for approval
4. If denied: tool is not executed, "user declined" is returned as tool result (IsError=false)
5. If `ConfirmFunc` is nil (serve/feishu): default approve

**ConfirmationRequest**:
```go
type ConfirmationRequest struct {
    ToolCallID  string
    ToolName    string
    Args        json.RawMessage // validated + prepared args
    Description string          // tool's operation description for user
}
```

**ConfirmDecision**:
```go
type ConfirmDecision struct {
    Approved bool   // true=allow execution; false=block
    Reason   string // rejection reason (returned to LLM when approved=false)
}
```

## AfterHookError

Wraps an error from an after-tool-call hook while preserving the original ToolResult:

```go
type AfterHookError struct {
    Err    error
    Result ToolResult
}
```

This allows debugging the pre-hook result even when the after hook fails.

## Related

- [[tool-system]] — Hooks modify tool execution; `ToolWithConfirmation` is an optional interface
- [[agent-core]] — LifecycleHooks and ConfirmFunc are part of Agent Options; 13 event types include confirmation events
- [[context-compaction]] — PreCompressHook fires before compaction
- [[goal-driven-loop]] — SessionStartHook receives goal in SessionStartEvent