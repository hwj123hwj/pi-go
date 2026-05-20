package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/earendil-works/pi-go/internal/ai"
)

// RunLoop 实现双层 Agent 循环。
// 外层循环处理 followUp 消息，内层循环处理 tool call + steering 消息。
func RunLoop(ctx context.Context, a *Agent) (ai.AssistantMessage, error) {
	provider, err := a.provider()
	if err != nil {
		return ai.AssistantMessage{}, err
	}

	// 从 steering queue 获取初始用户消息
	pending := a.steeringQueue.Drain()
	turns := 0
	var lastAssistant ai.AssistantMessage

	// 完整的消息历史（累积所有轮次）
	// 每次 LLM 调用都发送完整历史
	history := make([]ai.Message, 0, 32)

	for {
		if a.maxTurns > 0 && turns >= a.maxTurns {
			return lastAssistant, nil
		}
		turns++
		a.emit(ctx, EventTurnStart{})

		if len(pending) == 0 && len(history) == 0 {
			pending = []ai.Message{ai.NewTextUserMessage("hello")}
		}

		// 将 pending 消息追加到历史
		history = append(history, pending...)
		pending = nil

		// 用完整历史调用 LLM
		stream, err := provider.Stream(ctx, a.llmRequest(history))
		if err != nil {
			return lastAssistant, err
		}

		// 消费事件流，累积 assistant message
		var streamMsg ai.StreamAssistantMessage
		for event := range stream.Events() {
			switch e := event.(type) {
			case ai.EventDone:
				streamMsg = e.Message
			case ai.EventError:
				return lastAssistant, fmt.Errorf("llm error: %s", e.Error)
			}
		}

		message, err := a.handleAssistantMessage(streamMsg)
		if err != nil {
			return lastAssistant, err
		}
		lastAssistant = message

		// 将 assistant message 追加到历史
		history = append(history, message)

		// 如果 LLM 返回 error/aborted，终止循环
		if message.StopReason == ai.StopReasonError || message.StopReason == ai.StopReasonAborted {
			return lastAssistant, nil
		}

		// 执行 tool calls（如果有的话）
		toolResults, err := executeToolCalls(ctx, a, message.ToolCalls)
		if err != nil {
			return lastAssistant, err
		}
		a.emit(ctx, EventTurnEnd{Message: message, ToolResults: toolResults})

		// 如果有 tool calls，将 tool results 追加到历史并继续内层循环
		if message.StopReason == ai.StopReasonToolUse && len(toolResults) > 0 {
			history = append(history, toolResults...)
			// 继续内层循环
			continue
		}

		// 内层循环结束（无 tool call），检查 follow-up
		if next := a.followUpQueue.Drain(); len(next) > 0 {
			pending = next
			continue // 外层循环继续
		}

		break
	}

	return lastAssistant, nil
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
			results[idx] = executeOneTool(ctx, a, call)
		}(i, call)
	}
	wg.Wait()
	return results, nil
}

func executeToolCallsSequential(ctx context.Context, a *Agent, calls []ai.ToolCall) ([]ai.Message, error) {
	results := make([]ai.Message, len(calls))
	for i, call := range calls {
		results[i] = executeOneTool(ctx, a, call)
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
