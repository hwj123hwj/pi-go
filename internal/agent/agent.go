package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/ai/providers"
	"github.com/earendil-works/pi-go/internal/compaction"
	"github.com/earendil-works/pi-go/internal/session"
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
	Model              ai.Model
	Registry           *providers.Registry
	System             string
	Tools              []Tool
	MaxTurns           int
	Session            *session.Session        // 可选：会话持久化
	CompactionSettings compaction.Settings      // 上下文压缩设置
	SummarizeFunc      compaction.SummarizeFunc // 可选：摘要生成函数
	LifecycleHooks     LifecycleHooks           // 可选：工具执行生命周期钩子
}

type Agent struct {
	mu                 sync.RWMutex
	state              State
	registry           *providers.Registry
	model              ai.Model
	system             string
	tools              map[string]Tool
	listeners          []EventHandler
	steeringQueue      *MessageQueue
	followUpQueue      *MessageQueue
	maxTurns           int
	session            *session.Session
	compactionSettings compaction.Settings
	summarizeFunc      compaction.SummarizeFunc
	lifecycleHooks     LifecycleHooks
}

func New(opts Options) *Agent {
	tools := make(map[string]Tool, len(opts.Tools))
	for _, tool := range opts.Tools {
		tools[tool.Name()] = tool
	}
	return &Agent{
		state:              StateIdle,
		registry:           opts.Registry,
		model:              opts.Model,
		system:             opts.System,
		tools:              tools,
		listeners:          make([]EventHandler, 0),
		steeringQueue:      NewMessageQueue(),
		followUpQueue:      NewMessageQueue(),
		maxTurns:           opts.MaxTurns,
		session:            opts.Session,
		compactionSettings: opts.CompactionSettings,
		summarizeFunc:      opts.SummarizeFunc,
		lifecycleHooks:     opts.LifecycleHooks,
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
		a.mu.Unlock()
		return ai.AssistantMessage{}, ErrAgentBusy
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

	// 订阅事件，转发到 channel（带背压控制）
	unsubscribe := a.Subscribe(func(ctx context.Context, event AgentEvent) {
		var ev AgentStreamEvent
		switch e := event.(type) {
		case EventTurnEnd:
			ev = AgentStreamEvent{Type: StreamEventTurnEnd, Message: e.Message}
		case EventToolExecutionStart:
			ev = AgentStreamEvent{Type: StreamEventToolStart, ToolName: e.ToolName, ToolCallID: e.ToolCallID}
		case EventToolExecutionUpdate:
				ev = AgentStreamEvent{Type: StreamEventToolUpdate, ToolName: e.ToolName, ToolCallID: e.ToolCallID, PartialResult: e.PartialResult}
			case EventToolExecutionEnd:
			ev = AgentStreamEvent{Type: StreamEventToolEnd, ToolName: e.ToolName, ToolCallID: e.ToolCallID, ToolResult: e.Result, IsError: e.IsError}
		case EventCompacted:
			ev = AgentStreamEvent{Type: StreamEventCompacted, Summary: e.Summary}
		case EventCompactionFailed:
			ev = AgentStreamEvent{Type: StreamEventError, Error: "compaction failed: " + e.Error}
		default:
			return
		}
		select {
		case ch <- ev:
		case <-ctx.Done():
		}
	})

	a.mu.Lock()
	if a.state == StateRunning {
		a.mu.Unlock()
		unsubscribe()
		return nil, ErrAgentBusy
	}
	a.state = StateRunning
	a.mu.Unlock()

	go func() {
		defer close(ch)
		defer unsubscribe()

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

		// PromptStream 的 stream consumer：转发 text delta 事件到 channel
		consume := func(stream *ai.EventStream) (ai.StreamAssistantMessage, error) {
			var streamMsg ai.StreamAssistantMessage
			for event := range stream.Events() {
				select {
				case <-ctx.Done():
					return streamMsg, ctx.Err()
				default:
				}
				switch e := event.(type) {
				case ai.EventTextDelta:
					select {
					case ch <- AgentStreamEvent{Type: StreamEventTextDelta, TextDelta: e.Delta}:
					case <-ctx.Done():
						return streamMsg, ctx.Err()
					}
				case ai.EventDone:
					streamMsg = e.Message
				case ai.EventError:
					select {
					case ch <- AgentStreamEvent{Type: StreamEventError, Error: e.Error}:
					case <-ctx.Done():
						return streamMsg, ctx.Err()
					}
				}
			}
			return streamMsg, nil
		}

		lastAssistant, err := runAgentLoop(ctx, a, provider, consume)
		if err != nil {
			ch <- AgentStreamEvent{Type: StreamEventError, Error: err.Error()}
		}
		ch <- AgentStreamEvent{Type: StreamEventDone, FinalMessage: lastAssistant}

		a.mu.Lock()
		a.state = StateIdle
		a.mu.Unlock()
		a.emit(ctx, EventAgentEnd{})
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
	StreamEventTextDelta  StreamEventType = "text_delta"
	StreamEventTurnEnd    StreamEventType = "turn_end"
	StreamEventToolStart  StreamEventType = "tool_start"
	StreamEventToolUpdate StreamEventType = "tool_update"
	StreamEventToolEnd    StreamEventType = "tool_end"
	StreamEventDone       StreamEventType = "done"
	StreamEventError      StreamEventType = "error"
	StreamEventCompacted  StreamEventType = "compacted"
)

type AgentStreamEvent struct {
	Type         StreamEventType     `json:"type"`
	TextDelta    string              `json:"text_delta,omitempty"`
	Message      ai.Message          `json:"message,omitempty"`
	ToolName     string              `json:"tool_name,omitempty"`
	ToolCallID   string              `json:"tool_call_id,omitempty"`
	ToolResult   any                 `json:"tool_result,omitempty"`
	PartialResult any                `json:"partial_result,omitempty"`
	IsError      bool                `json:"is_error,omitempty"`
	FinalMessage ai.AssistantMessage `json:"final_message,omitempty"`
	Error        string              `json:"error,omitempty"`
	Summary      string              `json:"summary,omitempty"`
}
