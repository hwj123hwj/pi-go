package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hwj123hwj/pi-go/internal/ai"
	"github.com/hwj123hwj/pi-go/internal/ai/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTestProvider 用于测试的 mock LLM provider
type mockTestProvider struct {
	responses []mockTestResponse
	callIndex int
}

type mockTestResponse struct {
	text      string
	toolCalls []ai.ToolCall
	stop      ai.StopReason
}

func (m *mockTestProvider) Name() string { return "mock_test" }

func (m *mockTestProvider) StreamSimple(ctx context.Context, req ai.SimpleStreamRequest) (*ai.EventStream, error) {
	return m.Stream(ctx, ai.StreamRequest{
		Model:    req.Model,
		Messages: req.Messages,
		System:   req.System,
		Tools:    req.Tools,
	})
}

func (m *mockTestProvider) Stream(ctx context.Context, req ai.StreamRequest) (*ai.EventStream, error) {
	stream := ai.NewEventStream(8)
	resp := m.responses[m.callIndex]
	m.callIndex++

	go func() {
		defer stream.Close()
		partial := ai.StreamAssistantMessage{Text: resp.text, ToolCalls: resp.toolCalls, StopReason: resp.stop}

		_ = stream.Push(ctx, ai.EventStart{Partial: partial})

		if resp.text != "" {
			_ = stream.Push(ctx, ai.EventTextStart{ContentIndex: 0, Partial: partial})
			_ = stream.Push(ctx, ai.EventTextDelta{ContentIndex: 0, Delta: resp.text, Partial: partial})
			_ = stream.Push(ctx, ai.EventTextEnd{ContentIndex: 0, Text: resp.text, Partial: partial})
		}
		for i, tc := range resp.toolCalls {
			_ = stream.Push(ctx, ai.EventToolCallStart{ContentIndex: i, Partial: partial})
			_ = stream.Push(ctx, ai.EventToolCallDelta{ContentIndex: i, Delta: tc.Args, Partial: partial})
			_ = stream.Push(ctx, ai.EventToolCallEnd{ContentIndex: i, ToolCall: tc, Partial: partial})
		}

		_ = stream.Push(ctx, ai.EventDone{Reason: resp.stop, Message: partial})
		stream.SetResult(partial, nil)
	}()
	return stream, nil
}

// echoTool 用于测试的简单工具
type echoTool struct{}

func (t *echoTool) Name() string        { return "echo" }
func (t *echoTool) Description() string { return "Echo the input" }
func (t *echoTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{"type": "string"},
		},
		"required": []string{"message"},
	}
}
func (t *echoTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	return json.Marshal(params)
}
func (t *echoTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(PartialResult)) (ToolResult, error) {
	var params struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &params)
	return ToolResult{Content: "echo: " + params.Message}, nil
}

func newTestAgentWithProvider(responses []mockTestResponse) *Agent {
	mp := &mockTestProvider{responses: responses}
	registry := providers.NewRegistry()
	registry.Register(mp)

	return New(Options{
		Model: ai.Model{
			ID:       "test",
			Name:     "test",
			Provider: "mock_test",
		},
		Registry: registry,
		System:   "test system",
		Tools:    []Tool{&echoTool{}},
		MaxTurns: 5,
	})
}

func TestRunLoop_SimpleTextResponse(t *testing.T) {
	ag := newTestAgentWithProvider([]mockTestResponse{
		{text: "Hello!", stop: ai.StopReasonStop},
	})

	ctx := context.Background()
	result, err := ag.Prompt(ctx, ai.NewTextUserMessage("hi"))
	require.NoError(t, err)
	assert.Equal(t, "Hello!", result.Text)
	assert.Equal(t, ai.StopReasonStop, result.StopReason)
}

func TestRunLoop_ToolCallAndResponse(t *testing.T) {
	ag := newTestAgentWithProvider([]mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "echo", Args: `{"message":"test"}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{
			text: "Echo result received.",
			stop: ai.StopReasonStop,
		},
	})

	ctx := context.Background()
	result, err := ag.Prompt(ctx, ai.NewTextUserMessage("echo test"))
	require.NoError(t, err)
	assert.Equal(t, "Echo result received.", result.Text)
}

func TestRunLoop_MaxTurns(t *testing.T) {
	ag := newTestAgentWithProvider([]mockTestResponse{
		{toolCalls: []ai.ToolCall{{ID: "c1", Name: "echo", Args: `{"message":"a"}`}}, stop: ai.StopReasonToolUse},
		{toolCalls: []ai.ToolCall{{ID: "c2", Name: "echo", Args: `{"message":"b"}`}}, stop: ai.StopReasonToolUse},
		{toolCalls: []ai.ToolCall{{ID: "c3", Name: "echo", Args: `{"message":"c"}`}}, stop: ai.StopReasonToolUse},
		{text: "done", stop: ai.StopReasonStop},
	})
	ag.maxTurns = 2

	ctx := context.Background()
	result, err := ag.Prompt(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)
	// 应该在 maxTurns 后停止，不会返回 "done"（第 4 个响应）
	assert.NotEqual(t, "done", result.Text)
}

func TestRunLoop_ToolNotFound(t *testing.T) {
	ag := newTestAgentWithProvider([]mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "nonexistent_tool", Args: `{}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{text: "Tool not found, handled gracefully.", stop: ai.StopReasonStop},
	})

	ctx := context.Background()
	result, err := ag.Prompt(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)
	assert.Equal(t, "Tool not found, handled gracefully.", result.Text)
}
