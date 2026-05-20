package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/ai/providers"
)

type EventHandler func(ctx context.Context, event AgentEvent)

type State int

const (
	StateIdle State = iota
	StateRunning
	StateWaiting
	StateError
)

type Options struct {
	Model    ai.Model
	Registry *providers.Registry
	System   string
	Tools    []Tool
	MaxTurns int
}

type Agent struct {
	mu            sync.RWMutex
	state         State
	registry      *providers.Registry
	model         ai.Model
	system        string
	tools         map[string]Tool
	listeners     []EventHandler
	steeringQueue *MessageQueue
	followUpQueue *MessageQueue
	maxTurns      int
}

func New(opts Options) *Agent {
	tools := make(map[string]Tool, len(opts.Tools))
	for _, tool := range opts.Tools {
		tools[tool.Name()] = tool
	}
	return &Agent{
		state:         StateIdle,
		registry:      opts.Registry,
		model:         opts.Model,
		system:        opts.System,
		tools:         tools,
		listeners:     make([]EventHandler, 0),
		steeringQueue: NewMessageQueue(),
		followUpQueue: NewMessageQueue(),
		maxTurns:      opts.MaxTurns,
	}
}

func (a *Agent) Subscribe(handler EventHandler) func() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.listeners = append(a.listeners, handler)
	idx := len(a.listeners) - 1
	return func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if idx >= 0 && idx < len(a.listeners) {
			a.listeners = append(a.listeners[:idx], a.listeners[idx+1:]...)
		}
	}
}

func (a *Agent) emit(ctx context.Context, event AgentEvent) {
	a.mu.RLock()
	listeners := append([]EventHandler(nil), a.listeners...)
	a.mu.RUnlock()
	for _, listener := range listeners {
		listener(ctx, event)
	}
}

func (a *Agent) State() State {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// Prompt 发起一次 Agent 对话，等待完成后返回最终 assistant message。
func (a *Agent) Prompt(ctx context.Context, msg ai.Message) (ai.AssistantMessage, error) {
	a.mu.Lock()
	if a.state == StateRunning {
		a.followUpQueue.Enqueue(msg)
		a.mu.Unlock()
		return ai.AssistantMessage{}, nil
	}
	a.state = StateRunning
	a.mu.Unlock()

	a.emit(ctx, EventAgentStart{})
	a.steeringQueue.Enqueue(msg)
	assistant, err := RunLoop(ctx, a)
	a.mu.Lock()
	if err != nil {
		a.state = StateError
	} else {
		a.state = StateIdle
	}
	a.mu.Unlock()
	a.emit(ctx, EventAgentEnd{Messages: []ai.Message{msg}})
	return assistant, err
}

// PromptStream 发起一次 Agent 对话，返回一个 event channel 供流式消费。
// 消费者可以从 channel 读取 AgentStreamEvent，channel 关闭表示对话完成。
// 最终结果通过最后一个 AgentStreamResult 事件传递。
func (a *Agent) PromptStream(ctx context.Context, msg ai.Message) (<-chan AgentStreamEvent, error) {
	ch := make(chan AgentStreamEvent, 64)

	// 订阅事件，转发到 channel
	unsubscribe := a.Subscribe(func(ctx context.Context, event AgentEvent) {
		switch e := event.(type) {
		case EventTurnEnd:
			ch <- AgentStreamEvent{Type: StreamEventTurnEnd, Message: e.Message}
		case EventToolExecutionStart:
			ch <- AgentStreamEvent{Type: StreamEventToolStart, ToolName: e.ToolName, ToolCallID: e.ToolCallID}
		case EventToolExecutionEnd:
			ch <- AgentStreamEvent{Type: StreamEventToolEnd, ToolName: e.ToolName, ToolCallID: e.ToolCallID, ToolResult: e.Result, IsError: e.IsError}
		}
	})

	a.mu.Lock()
	if a.state == StateRunning {
		a.followUpQueue.Enqueue(msg)
		a.mu.Unlock()
		unsubscribe()
		close(ch)
		return ch, nil
	}
	a.state = StateRunning
	a.mu.Unlock()

	go func() {
		defer close(ch)
		defer unsubscribe()

		// 同时订阅 LLM stream 事件来获取 token delta
		provider, pErr := a.provider()
		if pErr != nil {
			ch <- AgentStreamEvent{Type: StreamEventError, Error: pErr.Error()}
			a.mu.Lock()
			a.state = StateError
			a.mu.Unlock()
			return
		}

		a.emit(ctx, EventAgentStart{})
		a.steeringQueue.Enqueue(msg)

		pending := a.steeringQueue.Drain()
		turns := 0
		history := make([]ai.Message, 0, 32)
		var lastAssistant ai.AssistantMessage

		for {
			if a.maxTurns > 0 && turns >= a.maxTurns {
				break
			}
			turns++
			a.emit(ctx, EventTurnStart{})

			if len(pending) == 0 && len(history) == 0 {
				pending = []ai.Message{ai.NewTextUserMessage("hello")}
			}
			if len(pending) > 0 {
				history = append(history, pending...)
			}
			pending = nil

			// 调用 LLM 并流式转发 token
			stream, err := provider.Stream(ctx, a.llmRequest(history))
			if err != nil {
				ch <- AgentStreamEvent{Type: StreamEventError, Error: err.Error()}
				break
			}

			var streamMsg ai.StreamAssistantMessage
			for event := range stream.Events() {
				switch e := event.(type) {
				case ai.EventTextDelta:
					ch <- AgentStreamEvent{Type: StreamEventTextDelta, TextDelta: e.Delta}
				case ai.EventDone:
					streamMsg = e.Message
				case ai.EventError:
					ch <- AgentStreamEvent{Type: StreamEventError, Error: e.Error}
				}
			}

			message, err := a.handleAssistantMessage(streamMsg)
			if err != nil {
				ch <- AgentStreamEvent{Type: StreamEventError, Error: err.Error()}
				break
			}
			lastAssistant = message
			history = append(history, message)

			if message.StopReason == ai.StopReasonError || message.StopReason == ai.StopReasonAborted {
				break
			}

			// 执行工具
			toolResults, err := executeToolCalls(ctx, a, message.ToolCalls)
			if err != nil {
				ch <- AgentStreamEvent{Type: StreamEventError, Error: err.Error()}
				break
			}
			a.emit(ctx, EventTurnEnd{Message: message, ToolResults: toolResults})

			if message.StopReason == ai.StopReasonToolUse && len(toolResults) > 0 {
				history = append(history, toolResults...)
				continue
			}

			if next := a.followUpQueue.Drain(); len(next) > 0 {
				pending = next
				continue
			}
			break
		}

		ch <- AgentStreamEvent{Type: StreamEventDone, FinalMessage: lastAssistant}

		a.mu.Lock()
		a.state = StateIdle
		a.mu.Unlock()
		a.emit(ctx, EventAgentEnd{Messages: history})
	}()

	return ch, nil
}

func (a *Agent) Steer(ctx context.Context, msg ai.Message) {
	a.steeringQueue.Enqueue(msg)
}

func (a *Agent) FollowUp(ctx context.Context, msg ai.Message) {
	a.followUpQueue.Enqueue(msg)
}

func (a *Agent) provider() (providers.Provider, error) {
	provider, ok := a.registry.Get(a.model.Provider)
	if !ok {
		return nil, fmt.Errorf("provider %q not found", a.model.Provider)
	}
	return provider, nil
}

func (a *Agent) toolDefinitions() []ai.ToolDefinition {
	defs := make([]ai.ToolDefinition, 0, len(a.tools))
	for _, tool := range a.tools {
		defs = append(defs, ai.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		})
	}
	return defs
}

func (a *Agent) appendToolResult(call ai.ToolCall, result ToolResult) ai.ToolResultMessage {
	content := result.Content
	if content == "" {
		content = "ok"
	}
	return ai.ToolResultMessage{ToolCallID: call.ID, Content: content, IsError: result.IsError}
}

func (a *Agent) llmRequest(messages []ai.Message) ai.StreamRequest {
	return ai.StreamRequest{
		Model:      a.model,
		Messages:   messages,
		System:     a.system,
		Tools:      a.toolDefinitions(),
		MaxTokens:  nil,
		ToolChoice: nil,
	}
}

func (a *Agent) handleAssistantMessage(message ai.StreamAssistantMessage) (ai.AssistantMessage, error) {
	assistant := ai.AssistantMessage{
		Text:       message.Text,
		Thinking:   message.Thinking,
		ToolCalls:  message.ToolCalls,
		StopReason: message.StopReason,
		ErrorMsg:   message.ErrorMsg,
	}
	return assistant, nil
}

func (a *Agent) decodeToolArgs(raw string, tool Tool) (json.RawMessage, error) {
	if raw == "" {
		raw = "{}"
	}
	validated, err := tool.Validate(json.RawMessage(raw))
	if err != nil {
		return nil, err
	}
	return validated, nil
}

// ─── Stream Event 类型 ───────────────────────────────────────────────────────

type StreamEventType string

const (
	StreamEventTextDelta StreamEventType = "text_delta"
	StreamEventTurnEnd   StreamEventType = "turn_end"
	StreamEventToolStart StreamEventType = "tool_start"
	StreamEventToolEnd   StreamEventType = "tool_end"
	StreamEventDone      StreamEventType = "done"
	StreamEventError     StreamEventType = "error"
)

type AgentStreamEvent struct {
	Type         StreamEventType    `json:"type"`
	TextDelta    string             `json:"text_delta,omitempty"`
	Message      ai.Message         `json:"message,omitempty"`
	ToolName     string             `json:"tool_name,omitempty"`
	ToolCallID   string             `json:"tool_call_id,omitempty"`
	ToolResult   any                `json:"tool_result,omitempty"`
	IsError      bool               `json:"is_error,omitempty"`
	FinalMessage ai.AssistantMessage `json:"final_message,omitempty"`
	Error        string             `json:"error,omitempty"`
}
