package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/compaction"
)

// consumeStreamFunc 定义如何消费 LLM 事件流。
// 不同的调用者（RunLoop vs PromptStream）提供不同的实现。
type consumeStreamFunc func(stream *ai.EventStream) (ai.StreamAssistantMessage, error)

// toolBatch 表示一组可一起执行的工具调用。
type toolBatch struct {
	safe  bool          // true = 批次内可并行执行
	calls []ai.ToolCall // 批次内的工具调用
}

// turnAction 表示一轮处理后的动作。
type turnAction int

const (
	actionContinue turnAction = iota // 继续内层循环（有 tool call）
	actionFollowUp                   // 有 follow-up 消息，继续外层循环
	actionDone                       // 对话完成
)

// turnResult 表示一轮处理的结果。
type turnResult struct {
	assistant  ai.AssistantMessage
	action     turnAction
	followUp   []ai.Message // action == actionFollowUp 时有值
}

// RunLoop 实现双层 Agent 循环。
// 外层循环处理 followUp 消息，内层循环处理 tool call + steering 消息。
func RunLoop(ctx context.Context, a *Agent) (ai.AssistantMessage, error) {
	provider, err := a.provider()
	if err != nil {
		return ai.AssistantMessage{}, err
	}

	// RunLoop 的 stream consumer：只关注 EventDone 和 EventError
	consume := func(stream *ai.EventStream) (ai.StreamAssistantMessage, error) {
		var streamMsg ai.StreamAssistantMessage
		for event := range stream.Events() {
			select {
			case <-ctx.Done():
				return streamMsg, ctx.Err()
			default:
			}
			switch e := event.(type) {
			case ai.EventDone:
				streamMsg = e.Message
			case ai.EventError:
				return streamMsg, fmt.Errorf("llm error: %s", e.Error)
			}
		}
		return streamMsg, nil
	}

	return runAgentLoop(ctx, a, provider, consume)
}

// runAgentLoop 是共享的 Agent 循环核心逻辑。
// RunLoop 和 PromptStream 都调用此函数，通过不同的 consume 回调区分行为。
func runAgentLoop(ctx context.Context, a *Agent, provider interface {
	Stream(ctx context.Context, req ai.StreamRequest) (*ai.EventStream, error)
}, consume consumeStreamFunc) (ai.AssistantMessage, error) {
	pending := a.steeringQueue.Drain()
	turns := 0
	var lastAssistant ai.AssistantMessage

	history := make([]ai.Message, 0, 32)

	// 如果有 session storage，从 session 恢复历史
	if a.session != nil {
		sessionHistory, err := a.session.BuildContext(ctx)
		if err == nil && len(sessionHistory) > 0 {
			history = sessionHistory
		}
	}

	for {
		if a.maxTurns > 0 && turns >= a.maxTurns {
			return lastAssistant, nil
		}
		turns++
		a.emit(ctx, EventTurnStart{})

		result, updatedHistory, err := processTurn(ctx, a, provider, consume, history, pending)
		if err != nil {
			return lastAssistant, err
		}

		history = updatedHistory
		lastAssistant = result.assistant
		pending = nil

		switch result.action {
		case actionContinue:
			// 内层循环继续（tool call 后继续）
		case actionFollowUp:
			pending = result.followUp
			// 外层循环继续
		case actionDone:
			return lastAssistant, nil
		}
	}
}

// processTurn 处理一个 Agent 轮次。
// 返回轮次结果、更新后的历史和可能的错误。
func processTurn(ctx context.Context, a *Agent, provider interface {
	Stream(ctx context.Context, req ai.StreamRequest) (*ai.EventStream, error)
}, consume consumeStreamFunc, history []ai.Message, pending []ai.Message) (turnResult, []ai.Message, error) {
	if len(pending) == 0 && len(history) == 0 {
		pending = []ai.Message{ai.NewTextUserMessage("hello")}
	}

	// 将 pending 消息追加到历史
	history = append(history, pending...)

	// 只保存本轮新增的 pending 消息到 session（避免 O(n²)）
	if a.session != nil {
		for _, msg := range pending {
			_ = a.session.AppendMessage(ctx, msg)
		}
	}

	// 检查是否需要 compaction
	history = a.maybeCompact(ctx, history)

	// 用完整历史调用 LLM
	stream, err := provider.Stream(ctx, a.llmRequest(history))
	if err != nil {
		return turnResult{}, history, err
	}

	// 消费事件流
	streamMsg, err := consume(stream)
	if err != nil {
		return turnResult{}, history, err
	}

	message, err := a.handleAssistantMessage(streamMsg)
	if err != nil {
		return turnResult{}, history, err
	}

	// 将 assistant message 追加到历史
	history = append(history, message)

	// 保存到 session
	if a.session != nil {
		_ = a.session.AppendMessage(ctx, message)
	}

	// 如果 LLM 返回 error/aborted，终止循环
	if message.StopReason == ai.StopReasonError || message.StopReason == ai.StopReasonAborted {
		return turnResult{assistant: message, action: actionDone}, history, nil
	}

	// 执行 tool calls（如果有的话）
	toolResults, err := executeToolCalls(ctx, a, message.ToolCalls)
	if err != nil {
		return turnResult{assistant: message}, history, err
	}
	a.emit(ctx, EventTurnEnd{Message: message, ToolResults: toolResults})

	// 如果有 tool calls，将 tool results 追加到历史并继续内层循环
	if message.StopReason == ai.StopReasonToolUse && len(toolResults) > 0 {
		history = append(history, toolResults...)

		// 保存 tool results 到 session
		if a.session != nil {
			for _, tr := range toolResults {
				_ = a.session.AppendMessage(ctx, tr)
			}
		}

		return turnResult{assistant: message, action: actionContinue}, history, nil
	}

	// 内层循环结束（无 tool call），检查 follow-up
	if next := a.followUpQueue.Drain(); len(next) > 0 {
		return turnResult{assistant: message, action: actionFollowUp, followUp: next}, history, nil
	}

	return turnResult{assistant: message, action: actionDone}, history, nil
}

// maybeCompact 检查是否需要压缩上下文，如果需要则执行压缩并返回新的历史。
func (a *Agent) maybeCompact(ctx context.Context, history []ai.Message) []ai.Message {
	if !a.compactionSettings.Enabled || a.summarizeFunc == nil {
		return history
	}

	// 估算当前 token 数
	contextTokens := compaction.EstimateTokens(history)
	contextWindow := a.model.ContextWindow
	if contextWindow <= 0 {
		contextWindow = 128000
	}

	if !compaction.ShouldCompact(contextTokens, contextWindow, a.compactionSettings) {
		return history
	}

	// 执行压缩
	historyPart, recentPart := compaction.SplitMessages(history, a.compactionSettings.KeepRecentTokens)
	if len(historyPart) == 0 {
		return history
	}

	summary, err := compaction.Compact(ctx, historyPart, recentPart, "", a.summarizeFunc)
	if err != nil {
		// 压缩失败，继续使用完整历史
		a.emit(ctx, EventCompactionFailed{Error: err.Error()})
		return history
	}

	// Persist compaction entry to session storage so it survives across prompts.
	if a.session != nil {
		if pErr := a.session.AppendCompaction(ctx, summary); pErr != nil {
			slog.Warn("failed to persist compaction entry", "error", pErr)
		}
	}

	a.emit(ctx, EventCompacted{
		Summary:     summary,
		TrimmedFrom: len(historyPart),
		TrimmedTo:   len(recentPart) + 1,
	})

	// 替换历史：summary + recent
	newHistory := make([]ai.Message, 0, len(recentPart)+1)
	newHistory = append(newHistory, ai.NewTextUserMessage("Context summary from previous conversation:\n\n"+summary))
	newHistory = append(newHistory, recentPart...)

	return newHistory
}

func executeToolCalls(ctx context.Context, a *Agent, calls []ai.ToolCall) ([]ai.Message, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	batches := partitionToolCalls(ctx, a, calls)
	var allResults []ai.Message

	for i, batch := range batches {
		// 批次开始事件
		names := make([]string, 0, len(batch.calls))
		for _, c := range batch.calls {
			names = append(names, c.Name)
		}
		a.emit(ctx, EventToolBatchStart{
			BatchIndex: i,
			Safe:       batch.safe,
			ToolNames:  names,
		})

		var results []ai.Message
		var err error

		if batch.safe {
			results, err = executeToolCallsParallel(ctx, a, batch.calls)
		} else {
			results, err = executeToolCallsSequential(ctx, a, batch.calls)
		}

		if err != nil {
			// 返回已执行的结果（支持未来 continueOnError 扩展）
			return allResults, err
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// partitionToolCalls 将工具调用按并发安全性分区为有序的批次。
//
// 分区规则：
//  1. 查找每个 call 对应的 tool
//  2. 若 tool 实现了 ConcurrencySafeChecker 且 IsConcurrencySafe(params) == true → safe
//     若 tool 实现了 ToolWithMode 且 ExecutionMode() == Sequential → unsafe（保守策略）
//     两者都不满足 → unsafe（保守策略）
//  3. 连续的 safe call 合入同一并行批次
//  4. 每个 unsafe call 独占一个串行批次
//
// 注：此处直接将 call.Args（string）转为 json.RawMessage 传给 IsConcurrencySafe，
// 无需先 validate。对只读工具而言 IsConcurrencySafe 始终返回 true，不依赖参数内容。
// 参数预留用于未来可能的动态判断。
func partitionToolCalls(ctx context.Context, a *Agent, calls []ai.ToolCall) []toolBatch {
	var batches []toolBatch
	var currentBatch *toolBatch

	for _, call := range calls {
		safe := isToolCallSafe(a, call)

		if safe {
			if currentBatch == nil || !currentBatch.safe {
				// 开始新的并行批次
				batches = append(batches, toolBatch{safe: true})
				currentBatch = &batches[len(batches)-1]
			}
			currentBatch.calls = append(currentBatch.calls, call)
		} else {
			// 不安全：独占串行批次
			batches = append(batches, toolBatch{safe: false, calls: []ai.ToolCall{call}})
			currentBatch = nil
		}
	}

	return batches
}

// isToolCallSafe 判断单个 tool call 是否可安全并发执行。
func isToolCallSafe(a *Agent, call ai.ToolCall) bool {
	tool, ok := a.tools[call.Name]
	if !ok {
		return false // 未知工具，保守策略
	}

	// 优先级 1: ToolWithMode.Sequential → 保守不安全
	if tm, ok := tool.(ToolWithMode); ok && tm.ExecutionMode() == ExecutionModeSequential {
		return false
	}

	// 优先级 2: ConcurrencySafeChecker
	if csc, ok := tool.(ConcurrencySafeChecker); ok {
		return csc.IsConcurrencySafe(json.RawMessage(call.Args))
	}

	// 默认不安全
	return false
}

func executeToolCallsParallel(ctx context.Context, a *Agent, calls []ai.ToolCall) ([]ai.Message, error) {
	results := make([]ai.Message, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(idx int, call ai.ToolCall) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					results[idx] = ai.ToolResultMessage{
						ToolCallID: call.ID,
						Content:    fmt.Sprintf("tool panic: %v", r),
						IsError:    true,
					}
				}
			}()
			results[idx] = executeOneTool(ctx, a, call)
		}(i, call)
	}
	wg.Wait()
	return results, nil
}

func executeToolCallsSequential(ctx context.Context, a *Agent, calls []ai.ToolCall) ([]ai.Message, error) {
	results := make([]ai.Message, len(calls))
	for i, call := range calls {
		func(idx int, call ai.ToolCall) {
			defer func() {
				if r := recover(); r != nil {
					results[idx] = ai.ToolResultMessage{
						ToolCallID: call.ID,
						Content:    fmt.Sprintf("tool panic: %v", r),
						IsError:    true,
					}
				}
			}()
			results[idx] = executeOneTool(ctx, a, call)
		}(i, call)
	}
	return results, nil
}

func executeOneTool(ctx context.Context, a *Agent, call ai.ToolCall) ai.Message {
	// 1. Find tool
	tool, ok := a.tools[call.Name]
	if !ok {
		return ai.ToolResultMessage{ToolCallID: call.ID, Content: fmt.Sprintf("tool %q not found", call.Name), IsError: true}
	}

	// 2. Emit start
	a.emit(ctx, EventToolExecutionStart{ToolCallID: call.ID, ToolName: call.Name})

	// 3. Validate args
	validated, err := a.decodeToolArgs(call.Args, tool)
	if err != nil {
		a.emit(ctx, EventToolExecutionEnd{ToolCallID: call.ID, ToolName: call.Name, Result: err.Error(), IsError: true})
		return ai.ToolResultMessage{ToolCallID: call.ID, Content: err.Error(), IsError: true}
	}

	// 4. Optional PrepareArguments
	args := validated
	if preparer, ok := tool.(ToolWithPrepareArguments); ok {
		prepared, err := preparer.PrepareArguments(ctx, validated)
		if err != nil {
			a.emit(ctx, EventToolExecutionEnd{ToolCallID: call.ID, ToolName: call.Name, Result: err.Error(), IsError: true})
			return ai.ToolResultMessage{ToolCallID: call.ID, Content: err.Error(), IsError: true}
		}
		args = prepared
	}

	// Build call context for hooks
	callCtx := ToolCallContext{
		ToolCallID: call.ID,
		ToolName:   call.Name,
		RawArgs:    json.RawMessage(call.Args),
		Args:       args,
	}

	// 5. Run before hooks
	for _, hook := range a.lifecycleHooks.Before {
		callCtx, err = hook(ctx, callCtx)
		if err != nil {
			// Before hook blocked execution — emit end with error
			a.emit(ctx, EventToolExecutionEnd{ToolCallID: call.ID, ToolName: call.Name, Result: err.Error(), IsError: true})
			return ai.ToolResultMessage{ToolCallID: call.ID, Content: err.Error(), IsError: true}
		}
	}
	args = callCtx.Args

	// 6. Build onUpdate callback
	onUpdate := func(pr PartialResult) {
		a.emit(ctx, EventToolExecutionUpdate{
			ToolCallID:    call.ID,
			ToolName:      call.Name,
			Args:          string(args),
			PartialResult: pr,
		})
	}

	// 7. Execute tool
	rawResult, err := tool.Execute(ctx, args, onUpdate)
	if err != nil {
		a.emit(ctx, EventToolExecutionEnd{ToolCallID: call.ID, ToolName: call.Name, Result: err.Error(), IsError: true})
		return ai.ToolResultMessage{ToolCallID: call.ID, Content: err.Error(), IsError: true}
	}

	// 8. Run after hooks
	preHookResult := rawResult
	for _, hook := range a.lifecycleHooks.After {
		rawResult, err = hook(ctx, callCtx, rawResult)
		if err != nil {
			// After hook failed — treat as execution failure.
			// Wrap with AfterHookError to preserve pre-hook result for debugging.
			hookErr := NewAfterHookError(err, preHookResult)
			errMsg := fmt.Sprintf("%s (original result: %s)", hookErr.Error(), preHookResult.Content)
			a.emit(ctx, EventToolExecutionEnd{ToolCallID: call.ID, ToolName: call.Name, Result: errMsg, IsError: true})
			return ai.ToolResultMessage{ToolCallID: call.ID, Content: errMsg, IsError: true}
		}
	}

	// 9. Emit end
	a.emit(ctx, EventToolExecutionEnd{ToolCallID: call.ID, ToolName: call.Name, Result: rawResult.Content, IsError: rawResult.IsError})
	return a.appendToolResult(call, rawResult)
}

func marshalArgs(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
