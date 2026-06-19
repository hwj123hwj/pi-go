package agent

import (
	"context"
	"encoding/json"
)

// ToolCallContext carries the full context for a single tool invocation through
// the lifecycle stages (prepare → before → execute → after → finish).
type ToolCallContext struct {
	ToolCallID string
	ToolName   string
	RawArgs    json.RawMessage // original args from LLM
	Args       json.RawMessage // after Validate (and optionally PrepareArguments)
}

// ToolExecutionResult holds the outcome of a tool execution.
type ToolExecutionResult struct {
	Result ToolResult
	Err    error
}

// ─── Hook function types ─────────────────────────────────────────────────────

// BeforeToolCallHook is called before a tool executes.
//
//   - Return the (possibly modified) ToolCallContext and nil to continue.
//   - Return a non-nil error to block execution (the tool will not run).
//     The error message becomes the tool result content.
type BeforeToolCallHook func(ctx context.Context, call ToolCallContext) (ToolCallContext, error)

// AfterToolCallHook is called after a tool finishes execution.
//
//   - Return the (possibly modified) ToolResult and nil to continue.
//   - Return a non-nil error to treat the overall execution as failed.
//     The pre-hook ToolResult is preserved in AfterHookError for debugging.
type AfterToolCallHook func(ctx context.Context, call ToolCallContext, result ToolResult) (ToolResult, error)

// ─── Confirmation (pre-execution approval) ──────────────────────────────────

// ConfirmationRequest 描述一次需要用户确认的工具调用。
// 当工具实现了 ToolWithConfirmation 且 RequiresConfirmation 返回 ok=true 时构造。
type ConfirmationRequest struct {
	ToolCallID  string
	ToolName    string
	Args        json.RawMessage // validated + prepared args
	Description string          // 工具给出的操作描述，展示给用户
}

// ConfirmDecision 是用户对一次确认请求的裁决。
type ConfirmDecision struct {
	Approved bool   // true=放行执行；false=阻断
	Reason   string // 拒绝理由（approved=false 时回告 LLM，让 Agent 知道动作未做成）
}

// ConfirmFunc 由各入口（chat/serve/feishu）注入，负责向用户发起确认并同步等待裁决。
//
//   - 注入了 ConfirmFunc：执行链在确认点暂停，调用此函数等待用户回复。
//   - 未注入（nil）：执行链默认放行（适用于 serve/feishu 等单向流入口）。
//
// 返回的 ConfirmDecision.Approved=false 时，工具不执行，"用户拒绝"作为
// tool result 回告 LLM（IsError=false，避免 Agent 误判为系统错误而重试）。
type ConfirmFunc func(ctx context.Context, req ConfirmationRequest) ConfirmDecision

// AfterHookError wraps an error from an after-tool-call hook while preserving
// the original (pre-hook) ToolResult for debugging.
type AfterHookError struct {
	Err    error
	Result ToolResult
}

func (e *AfterHookError) Error() string { return e.Err.Error() }
func (e *AfterHookError) Unwrap() error { return e.Err }

// NewAfterHookError creates an AfterHookError that preserves the pre-hook result.
func NewAfterHookError(err error, result ToolResult) *AfterHookError {
	return &AfterHookError{Err: err, Result: result}
}

// LifecycleHooks aggregates the before/after hook slices consumed by the agent.
type LifecycleHooks struct {
	Before []BeforeToolCallHook
	After  []AfterToolCallHook
}

// ─── Optional tool interfaces ────────────────────────────────────────────────

// ToolWithPrepareArguments is an optional interface that tools can implement to
// normalize or enrich their validated arguments before execution.
//
// The execution order is:
//
//  1. raw args → tool.Validate(...)
//  2. validated args → tool.PrepareArguments(...)  (if implemented)
//  3. prepared args → before hooks → execute → after hooks
type ToolWithPrepareArguments interface {
	Tool
	PrepareArguments(ctx context.Context, params json.RawMessage) (json.RawMessage, error)
}
