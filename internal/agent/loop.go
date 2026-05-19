package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/earendil-works/pi-go/internal/ai"
)

func RunLoop(ctx context.Context, a *Agent) (ai.AssistantMessage, error) {
	provider, err := a.provider()
	if err != nil {
		return ai.AssistantMessage{}, err
	}

	pending := a.steeringQueue.Drain()
	turns := 0
	var lastAssistant ai.AssistantMessage

	for {
		if a.maxTurns > 0 && turns >= a.maxTurns {
			return lastAssistant, nil
		}
		turns++
		a.emit(ctx, EventTurnStart{})

		if len(pending) == 0 {
			pending = []ai.Message{ai.NewTextUserMessage("hello")}
		}

		stream, err := provider.Stream(ctx, a.llmRequest(pending))
		if err != nil {
			return lastAssistant, err
		}

		var assistant ai.StreamAssistantMessage
		for event := range stream.Events() {
			switch e := event.(type) {
			case ai.EventStart:
				assistant = e.Partial
			case ai.EventTextStart:
				assistant = e.Partial
			case ai.EventTextDelta:
				assistant = e.Partial
				assistant.Text += e.Delta
			case ai.EventTextEnd:
				assistant = e.Partial
				assistant.Text = e.Text
			case ai.EventToolCallStart:
				assistant = e.Partial
			case ai.EventToolCallDelta:
				assistant = e.Partial
			case ai.EventToolCallEnd:
				assistant = e.Partial
				assistant.ToolCalls = append(assistant.ToolCalls, e.ToolCall)
			case ai.EventDone:
				assistant = e.Message
			case ai.EventError:
				return lastAssistant, fmt.Errorf("llm error: %s", e.Error)
			}
		}

		message, err := a.handleAssistantMessage(assistant)
		if err != nil {
			return lastAssistant, err
		}
		lastAssistant = message
		toolResults, err := executeToolCalls(ctx, a, message.ToolCalls)
		if err != nil {
			return lastAssistant, err
		}
		a.emit(ctx, EventTurnEnd{Message: message, ToolResults: toolResults})

		if message.StopReason == ai.StopReasonToolUse && len(toolResults) > 0 {
			pending = toolResults
			continue
		}

		if next := a.followUpQueue.Drain(); len(next) > 0 {
			pending = next
			continue
		}

		break
	}

	return lastAssistant, nil
}

func executeToolCalls(ctx context.Context, a *Agent, calls []ai.ToolCall) ([]ai.Message, error) {
	if len(calls) == 0 {
		return nil, nil
	}
	var (
		mu      sync.Mutex
		results = make([]ai.Message, 0, len(calls))
	)
	for _, call := range calls {
		tool, ok := a.tools[call.Name]
		if !ok {
			results = append(results, ai.ToolResultMessage{ToolCallID: call.ID, Content: "tool not found", IsError: true})
			continue
		}
		validated, err := a.decodeToolArgs(call.Args, tool)
		if err != nil {
			results = append(results, ai.ToolResultMessage{ToolCallID: call.ID, Content: err.Error(), IsError: true})
			continue
		}
		rawResult, err := tool.Execute(ctx, validated, nil)
		if err != nil {
			results = append(results, ai.ToolResultMessage{ToolCallID: call.ID, Content: err.Error(), IsError: true})
			continue
		}
		mu.Lock()
		results = append(results, a.appendToolResult(call, rawResult))
		mu.Unlock()
	}
	return results, nil
}

func marshalArgs(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
