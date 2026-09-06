package agent

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/hwj123hwj/pi-go/sdk/ai"
	"github.com/hwj123hwj/pi-go/sdk/ai/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test helpers ─────────────────────────────────────────────────────────────

// progressTool emits partial updates during execution.
type progressTool struct {
	updates []string
}

func (t *progressTool) Name() string        { return "progress" }
func (t *progressTool) Description() string { return "A tool that reports progress" }
func (t *progressTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"steps": map[string]any{"type": "integer"},
		},
		"required": []string{"steps"},
	}
}
func (t *progressTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Steps int `json:"steps"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	return json.Marshal(params)
}
func (t *progressTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(PartialResult)) (ToolResult, error) {
	var params struct {
		Steps int `json:"steps"`
	}
	_ = json.Unmarshal(raw, &params)

	for i := 1; i <= params.Steps; i++ {
		if onUpdate != nil {
			onUpdate(PartialResult{Content: "processing", Done: false})
		}
	}
	return ToolResult{Content: "done"}, nil
}

// ─── Step 2: Partial update tests ────────────────────────────────────────────

func TestPartialUpdate_Emitted(t *testing.T) {
	mp := &mockTestProvider{responses: []mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "progress", Args: `{"steps":3}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{text: "All done.", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	ag := New(Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		Tools:    []Tool{&progressTool{}},
		MaxTurns: 5,
	})

	var mu sync.Mutex
	var updateEvents []EventToolExecutionUpdate
	ag.Subscribe(func(ctx context.Context, event AgentEvent) {
		if e, ok := event.(EventToolExecutionUpdate); ok {
			mu.Lock()
			updateEvents = append(updateEvents, e)
			mu.Unlock()
		}
	})

	ctx := context.Background()
	_, err := ag.Prompt(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 3, len(updateEvents), "should emit 3 partial update events")
	for i, ue := range updateEvents {
		assert.Equal(t, "call_1", ue.ToolCallID)
		assert.Equal(t, "progress", ue.ToolName)
		assert.NotNil(t, ue.PartialResult)
		_ = i
	}
}

func TestPartialUpdate_StreamingEvent(t *testing.T) {
	mp := &mockTestProvider{responses: []mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "progress", Args: `{"steps":2}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{text: "done", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	ag := New(Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		Tools:    []Tool{&progressTool{}},
		MaxTurns: 5,
	})

	ctx := context.Background()
	ch, err := ag.PromptStream(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)

	var updateCount int
	for ev := range ch {
		if ev.Type == StreamEventToolUpdate {
			updateCount++
			assert.Equal(t, "progress", ev.ToolName)
			assert.NotNil(t, ev.PartialResult)
		}
	}
	assert.Equal(t, 2, updateCount, "should receive 2 tool_update stream events")
}

// ─── Step 3: PrepareArguments tests ──────────────────────────────────────────

// prepareTool implements ToolWithPrepareArguments to add a default value.
type prepareTool struct{}

func (t *prepareTool) Name() string        { return "prepare" }
func (t *prepareTool) Description() string { return "A tool with prepare" }
func (t *prepareTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{"type": "string"},
		},
	}
}
func (t *prepareTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	return json.Marshal(params)
}
func (t *prepareTool) PrepareArguments(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Value string `json:"value"`
	}
	_ = json.Unmarshal(params, &p)
	if p.Value == "" {
		p.Value = "default"
	}
	return json.Marshal(p)
}
func (t *prepareTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(PartialResult)) (ToolResult, error) {
	var params struct {
		Value string `json:"value"`
	}
	_ = json.Unmarshal(raw, &params)
	return ToolResult{Content: "value=" + params.Value}, nil
}

func TestPrepareArguments_DefaultValue(t *testing.T) {
	mp := &mockTestProvider{responses: []mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "prepare", Args: `{}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{text: "ok", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	ag := New(Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		Tools:    []Tool{&prepareTool{}},
		MaxTurns: 5,
	})

	var mu sync.Mutex
	var endEvents []EventToolExecutionEnd
	ag.Subscribe(func(ctx context.Context, event AgentEvent) {
		if e, ok := event.(EventToolExecutionEnd); ok {
			mu.Lock()
			endEvents = append(endEvents, e)
			mu.Unlock()
		}
	})

	ctx := context.Background()
	_, err := ag.Prompt(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, endEvents, 1)
	assert.Equal(t, "value=default", endEvents[0].Result)
}

func TestPrepareArguments_ExplicitValue(t *testing.T) {
	mp := &mockTestProvider{responses: []mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "prepare", Args: `{"value":"custom"}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{text: "ok", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	ag := New(Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		Tools:    []Tool{&prepareTool{}},
		MaxTurns: 5,
	})

	var mu sync.Mutex
	var endEvents []EventToolExecutionEnd
	ag.Subscribe(func(ctx context.Context, event AgentEvent) {
		if e, ok := event.(EventToolExecutionEnd); ok {
			mu.Lock()
			endEvents = append(endEvents, e)
			mu.Unlock()
		}
	})

	ctx := context.Background()
	_, err := ag.Prompt(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, endEvents, 1)
	assert.Equal(t, "value=custom", endEvents[0].Result)
}

// ─── Step 4-5: Before/After hook tests ───────────────────────────────────────

func TestBeforeHook_Block(t *testing.T) {
	mp := &mockTestProvider{responses: []mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "echo", Args: `{"message":"test"}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{text: "blocked handled", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	ag := New(Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		Tools:    []Tool{&echoTool{}},
		MaxTurns: 5,
		LifecycleHooks: LifecycleHooks{
			Before: []BeforeToolCallHook{
				func(ctx context.Context, call ToolCallContext) (ToolCallContext, error) {
					return call, assert.AnError // block execution
				},
			},
		},
	})

	var mu sync.Mutex
	var endEvents []EventToolExecutionEnd
	ag.Subscribe(func(ctx context.Context, event AgentEvent) {
		if e, ok := event.(EventToolExecutionEnd); ok {
			mu.Lock()
			endEvents = append(endEvents, e)
			mu.Unlock()
		}
	})

	ctx := context.Background()
	result, err := ag.Prompt(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)

	// The agent should have continued to the next LLM call
	assert.Equal(t, "blocked handled", result.Text)

	// The tool execution should have ended with an error
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, endEvents, 1)
	assert.True(t, endEvents[0].IsError)
}

func TestBeforeHook_RewriteArgs(t *testing.T) {
	mp := &mockTestProvider{responses: []mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "echo", Args: `{"message":"original"}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{text: "ok", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	ag := New(Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		Tools:    []Tool{&echoTool{}},
		MaxTurns: 5,
		LifecycleHooks: LifecycleHooks{
			Before: []BeforeToolCallHook{
				func(ctx context.Context, call ToolCallContext) (ToolCallContext, error) {
					// Rewrite args to different message
					call.Args = json.RawMessage(`{"message":"rewritten"}`)
					return call, nil
				},
			},
		},
	})

	var mu sync.Mutex
	var endEvents []EventToolExecutionEnd
	ag.Subscribe(func(ctx context.Context, event AgentEvent) {
		if e, ok := event.(EventToolExecutionEnd); ok {
			mu.Lock()
			endEvents = append(endEvents, e)
			mu.Unlock()
		}
	})

	ctx := context.Background()
	_, err := ag.Prompt(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, endEvents, 1)
	assert.Equal(t, "echo: rewritten", endEvents[0].Result)
}

func TestBeforeHook_ReadOnly(t *testing.T) {
	mp := &mockTestProvider{responses: []mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "echo", Args: `{"message":"test"}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{text: "ok", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	var auditedName string
	ag := New(Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		Tools:    []Tool{&echoTool{}},
		MaxTurns: 5,
		LifecycleHooks: LifecycleHooks{
			Before: []BeforeToolCallHook{
				func(ctx context.Context, call ToolCallContext) (ToolCallContext, error) {
					auditedName = call.ToolName
					return call, nil // pass through
				},
			},
		},
	})

	ctx := context.Background()
	_, err := ag.Prompt(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)
	assert.Equal(t, "echo", auditedName)
}

func TestAfterHook_RewriteResult(t *testing.T) {
	mp := &mockTestProvider{responses: []mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "echo", Args: `{"message":"test"}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{text: "ok", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	ag := New(Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		Tools:    []Tool{&echoTool{}},
		MaxTurns: 5,
		LifecycleHooks: LifecycleHooks{
			After: []AfterToolCallHook{
				func(ctx context.Context, call ToolCallContext, result ToolResult) (ToolResult, error) {
					result.Content = "intercepted: " + result.Content
					return result, nil
				},
			},
		},
	})

	var mu sync.Mutex
	var endEvents []EventToolExecutionEnd
	ag.Subscribe(func(ctx context.Context, event AgentEvent) {
		if e, ok := event.(EventToolExecutionEnd); ok {
			mu.Lock()
			endEvents = append(endEvents, e)
			mu.Unlock()
		}
	})

	ctx := context.Background()
	_, err := ag.Prompt(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, endEvents, 1)
	assert.Equal(t, "intercepted: echo: test", endEvents[0].Result)
}

func TestAfterHook_Error(t *testing.T) {
	mp := &mockTestProvider{responses: []mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "echo", Args: `{"message":"test"}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{text: "handled", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	ag := New(Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		Tools:    []Tool{&echoTool{}},
		MaxTurns: 5,
		LifecycleHooks: LifecycleHooks{
			After: []AfterToolCallHook{
				func(ctx context.Context, call ToolCallContext, result ToolResult) (ToolResult, error) {
					return result, assert.AnError // treat as failure
				},
			},
		},
	})

	var mu sync.Mutex
	var endEvents []EventToolExecutionEnd
	ag.Subscribe(func(ctx context.Context, event AgentEvent) {
		if e, ok := event.(EventToolExecutionEnd); ok {
			mu.Lock()
			endEvents = append(endEvents, e)
			mu.Unlock()
		}
	})

	ctx := context.Background()
	result, err := ag.Prompt(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)
	assert.Equal(t, "handled", result.Text)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, endEvents, 1)
	assert.True(t, endEvents[0].IsError)
	// Pre-hook result should be preserved in the error message
	assert.Contains(t, endEvents[0].Result, "assert.AnError")
	assert.Contains(t, endEvents[0].Result, "echo: test")
}

func TestMultipleBeforeHooks_Chained(t *testing.T) {
	mp := &mockTestProvider{responses: []mockTestResponse{
		{
			toolCalls: []ai.ToolCall{
				{ID: "call_1", Name: "echo", Args: `{"message":"start"}`},
			},
			stop: ai.StopReasonToolUse,
		},
		{text: "ok", stop: ai.StopReasonStop},
	}}
	registry := providers.NewRegistry()
	registry.Register(mp)

	ag := New(Options{
		Model:    ai.Model{ID: "test", Name: "test", Provider: "mock_test"},
		Registry: registry,
		System:   "test",
		Tools:    []Tool{&echoTool{}},
		MaxTurns: 5,
		LifecycleHooks: LifecycleHooks{
			Before: []BeforeToolCallHook{
				func(ctx context.Context, call ToolCallContext) (ToolCallContext, error) {
					call.Args = json.RawMessage(`{"message":"step1"}`)
					return call, nil
				},
				func(ctx context.Context, call ToolCallContext) (ToolCallContext, error) {
					// Second hook sees the output of first hook
					var p struct {
						Message string `json:"message"`
					}
					_ = json.Unmarshal(call.Args, &p)
					call.Args = json.RawMessage(`{"message":"` + p.Message + "+step2" + `"}`)
					return call, nil
				},
			},
		},
	})

	var mu sync.Mutex
	var endEvents []EventToolExecutionEnd
	ag.Subscribe(func(ctx context.Context, event AgentEvent) {
		if e, ok := event.(EventToolExecutionEnd); ok {
			mu.Lock()
			endEvents = append(endEvents, e)
			mu.Unlock()
		}
	})

	ctx := context.Background()
	_, err := ag.Prompt(ctx, ai.NewTextUserMessage("test"))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, endEvents, 1)
	assert.Equal(t, "echo: step1+step2", endEvents[0].Result)
}

// ─── AfterHookError unit tests ────────────────────────────────────────────────

func TestAfterHookError_PreservesResult(t *testing.T) {
	original := ToolResult{Content: "original output", IsError: false}
	err := NewAfterHookError(assert.AnError, original)

	// Error message should delegate to inner error
	assert.Equal(t, assert.AnError.Error(), err.Error())

	// Original result should be preserved
	assert.Equal(t, "original output", err.Result.Content)
	assert.False(t, err.Result.IsError)

	// Unwrap should return the inner error
	assert.Equal(t, assert.AnError, err.Unwrap())
}
