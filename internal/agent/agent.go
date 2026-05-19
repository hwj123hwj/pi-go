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

func (a *Agent) marshalMessage(msg ai.Message) (ai.Message, error) {
	return msg, nil
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
