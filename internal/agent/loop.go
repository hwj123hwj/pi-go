package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/compaction"
)

// consumeStreamFunc 定义如何消费 LLM 事件流。
// 不同的调用者（RunLoop vs PromptStream）提供不同的实现。
type consumeStreamFunc func(stream *ai.EventStream) (ai.StreamAssistantMessage, error)

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

	summary, err := compaction.Compact(ctx, historyPart, recentPart, a.summarizeFunc)
	if err != nil {
		// 压缩失败，继续使用完整历史
		a.emit(ctx, EventCompactionFailed{Error: err.Error()})
		return history
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

	// 判断是否有需要顺序执行的工具
	hasSequential := false
	for _, call := range calls {
		if tool, ok := a.tools[call.Name]; ok {
			if tm, ok := tool.(ToolWithMode); ok && tm.ExecutionMode() == ExecutionModeSequential {
				hasSequential = true
				break
			}
		}
	}

	if hasSequential {
		return executeToolCallsSequential(ctx, a, calls)
	}
	return executeToolCallsParallel(ctx, a, calls)
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
	tool, ok := a.tools[call.Name]
	if !ok {
		return ai.ToolResultMessage{ToolCallID: call.ID, Content: fmt.Sprintf("tool %q not found", call.Name), IsError: true}
	}

	a.emit(ctx, EventToolExecutionStart{ToolCallID: call.ID, ToolName: call.Name})

	validated, err := a.decodeToolArgs(call.Args, tool)
	if err != nil {
		a.emit(ctx, EventToolExecutionEnd{ToolCallID: call.ID, ToolName: call.Name, Result: err.Error(), IsError: true})
		return ai.ToolResultMessage{ToolCallID: call.ID, Content: err.Error(), IsError: true}
	}

	rawResult, err := tool.Execute(ctx, validated, nil)
	if err != nil {
		a.emit(ctx, EventToolExecutionEnd{ToolCallID: call.ID, ToolName: call.Name, Result: err.Error(), IsError: true})
		return ai.ToolResultMessage{ToolCallID: call.ID, Content: err.Error(), IsError: true}
	}

	a.emit(ctx, EventToolExecutionEnd{ToolCallID: call.ID, ToolName: call.Name, Result: rawResult.Content, IsError: rawResult.IsError})
	return a.appendToolResult(call, rawResult)
}

func marshalArgs(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
