---
type: entity
date: 2026-06-22
tags: [tools, lifecycle, hooks, middleware, confirmation, execution-flow]
related: [[tool-system]], [[agent-core]], [[context-compaction]], [[goal-driven-loop]]
---

# Tool Lifecycle Hooks

> Before/After hooks, argument preparation, confirmation gates, session-level observer hooks, and the full 9-step execution flow.

## Full Execution Flow (executeOneTool)

The `executeOneTool()` function in `loop.go` implements a precise 9-step lifecycle for each tool call:

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. Find tool         — Lookup in agent.tools map               │
│ 2. Emit start        — EventToolExecutionStart                 │
│ 3. Validate args     — tool.Validate(rawArgs) → validated      │
│ 4. PrepareArguments  — (optional) tool.PrepareArguments(validated)│
│ 5. Before hooks      — LifecycleHooks.Before[] (can block)     │
│ 5.5 Confirmation     — ToolWithConfirmation gate (user approval)│
│ 6. Build onUpdate    — Create PartialResult callback            │
│ 7. Execute           — tool.Execute(ctx, args, onUpdate)        │
│ 8. After hooks       — LifecycleHooks.After[] (can modify)     │
│ 9. Emit end          — EventToolExecutionEnd + appendToolResult │
└─────────────────────────────────────────────────────────────────┘
```

### Error Handling at Each Step

| Step | Error Behavior |
|------|---------------|
| 1. Find tool | Tool not found → `ToolResultMessage{IsError: true}` |
| 3. Validate | Validation error → emit end with error, return error result |
| 4. PrepareArguments | Error → emit end with error, return error result |
| 5. Before hooks | Hook returns error → emit end with error, block execution |
| 5.5 Confirmation | User rejects → emit ConfirmationResult(false), return "user declined" (IsError=false) |
| 7. Execute | Error → prefer rawResult.Content if available, emit end with error |
| 8. After hooks | Hook error → wrap with AfterHookError preserving pre-hook result |

### Key Design: IsError=false on Rejection

When a user rejects a tool execution, the result has `IsError=false`. This prevents the Agent from interpreting the rejection as a system error and automatically retrying. The content clearly states "user declined this action: {reason}".

## Execution-Level Hooks

| Hook | Timing | Purpose |
|------|--------|---------|
| `PrepareArguments` | Between Validate and Before hooks | Argument transformation and enrichment |
| `Before` | Before Execute | Pre-processing, validation, logging, can block execution |
| `After` | After Execute | Post-processing, auditing, can modify results |

## ToolWithPrepareArguments

Optional interface for normalizing/enriching validated arguments:

```go
type ToolWithPrepareArguments interface {
    Tool
    PrepareArguments(ctx context.Context, params json.RawMessage) (json.RawMessage, error)
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
2. Agent emits `EventConfirmationRequest{ToolCallID, ToolName, Description}`
3. `ConfirmFunc` (injected by entrypoint) asks the user for approval
4. Agent emits `EventConfirmationResult{ToolCallID, Approved, Reason}`
5. If denied: tool not executed, "user declined" returned as tool result (IsError=false)
6. If `ConfirmFunc` is nil (serve/feishu): default approve

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

## AfterHookError

Wraps an error from an after-tool-call hook while preserving the original ToolResult:

```go
type AfterHookError struct {
    Err    error
    Result ToolResult
}
```

When an after hook fails, the error message includes both the hook error and the original result content: `"{hook_error} (original result: {pre_hook_content})"`.

## Tool Batching (partitionToolCalls)

Before execution, tool calls are partitioned into batches:

| Condition | Batch Type |
|-----------|-----------|
| `ConcurrencySafeChecker.IsConcurrencySafe() == true` | Parallel batch (concurrent execution) |
| `ToolWithMode.ExecutionMode() == Sequential` | Serial batch (one at a time) |
| Neither interface implemented | Serial batch (conservative default) |

Consecutive safe calls are merged into a single parallel batch. Each unsafe call gets its own serial batch.

## Related

- [[tool-system]] — Tool interface and optional interfaces
- [[agent-core]] — LifecycleHooks and ConfirmFunc are Agent options; 16 event types
- [[context-compaction]] — PreCompressHook fires before compaction
- [[goal-driven-loop]] — SessionStartHook receives goal in SessionStartEvent
