package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/hwj123hwj/pi-go/sdk/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Registry Tests ───────────────────────────────────────────────────────────

func TestRegistry_AddAndGetBeforeToolCallHooks(t *testing.T) {
	r := NewRegistry()

	// Add hooks with different priorities
	var order []int
	r.AddBeforeToolCall(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		order = append(order, 2)
		return call, nil
	}, WithPriority(200))

	r.AddBeforeToolCall(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		order = append(order, 1)
		return call, nil
	}, WithPriority(50))

	r.AddBeforeToolCall(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		order = append(order, 3)
		return call, nil
	}, WithPriority(300))

	hooks := r.GetBeforeToolCallHooks()
	require.Len(t, hooks, 3, "should have 3 hooks")

	// Execute in order — priority 50, 200, 300
	ctx := context.Background()
	callCtx := agent.ToolCallContext{ToolCallID: "test-1", ToolName: "bash"}
	for _, h := range hooks {
		var err error
		callCtx, err = h(ctx, callCtx)
		require.NoError(t, err)
	}

	assert.Equal(t, []int{1, 2, 3}, order, "hooks should execute in priority order")
}

func TestRegistry_AddAndGetAfterToolCallHooks(t *testing.T) {
	r := NewRegistry()

	r.AddAfterToolCall(func(ctx context.Context, call agent.ToolCallContext, result agent.ToolResult) (agent.ToolResult, error) {
		return agent.ToolResult{Content: "modified"}, nil
	}, WithPriority(100))

	hooks := r.GetAfterToolCallHooks()
	require.Len(t, hooks, 1)

	ctx := context.Background()
	result, err := hooks[0](ctx, agent.ToolCallContext{}, agent.ToolResult{Content: "original"})
	require.NoError(t, err)
	assert.Equal(t, "modified", result.Content)
}

func TestRegistry_GetByEvent(t *testing.T) {
	r := NewRegistry()

	r.AddBeforeToolCall(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		return call, nil
	}, WithPriority(100))

	r.AddAfterToolCall(func(ctx context.Context, call agent.ToolCallContext, result agent.ToolResult) (agent.ToolResult, error) {
		return result, nil
	}, WithPriority(200))

	entries := r.GetByEvent(EventBeforeTool)
	assert.Len(t, entries, 1)
	assert.Equal(t, EventBeforeTool, entries[0].Event)

	entries = r.GetByEvent(EventAfterTool)
	assert.Len(t, entries, 1)
	assert.Equal(t, EventAfterTool, entries[0].Event)

	entries = r.GetByEvent(EventSessionStart)
	assert.Len(t, entries, 0)
}

func TestRegistry_LenAndClear(t *testing.T) {
	r := NewRegistry()

	assert.Equal(t, 0, r.Len())

	r.AddBeforeToolCall(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		return call, nil
	})
	r.AddAfterToolCall(func(ctx context.Context, call agent.ToolCallContext, result agent.ToolResult) (agent.ToolResult, error) {
		return result, nil
	})

	assert.Equal(t, 2, r.Len())

	r.Clear()
	assert.Equal(t, 0, r.Len())
}

func TestRegistry_SessionStartHooks(t *testing.T) {
	r := NewRegistry()

	var called atomic.Bool
	r.AddSessionStart(func(ctx context.Context, e agent.SessionStartEvent) error {
		called.Store(true)
		return nil
	}, WithPriority(50))

	hooks := r.GetSessionStartHooks()
	require.Len(t, hooks, 1)

	err := hooks[0](context.Background(), agent.SessionStartEvent{Goal: "test"})
	assert.NoError(t, err)
	assert.True(t, called.Load(), "hook should have been called")
}

func TestRegistry_SessionEndHooks(t *testing.T) {
	r := NewRegistry()

	var called atomic.Bool
	r.AddSessionEnd(func(ctx context.Context, e agent.SessionEndEvent) error {
		called.Store(true)
		return nil
	})

	hooks := r.GetSessionEndHooks()
	require.Len(t, hooks, 1)

	err := hooks[0](context.Background(), agent.SessionEndEvent{})
	assert.NoError(t, err)
	assert.True(t, called.Load())
}

func TestRegistry_PreCompressHooks(t *testing.T) {
	r := NewRegistry()

	var called atomic.Bool
	r.AddPreCompress(func(ctx context.Context, e agent.PreCompressEvent) error {
		called.Store(true)
		return nil
	})

	hooks := r.GetPreCompressHooks()
	require.Len(t, hooks, 1)

	err := hooks[0](context.Background(), agent.PreCompressEvent{ContextTokens: 1000})
	assert.NoError(t, err)
	assert.True(t, called.Load())
}

// ─── System Tests ─────────────────────────────────────────────────────────────

func TestSystem_RunBefore_NoHooks(t *testing.T) {
	s := NewSystem()
	ctx := context.Background()
	callCtx := agent.ToolCallContext{ToolCallID: "test", ToolName: "bash"}

	result, err := s.RunBefore(ctx, nil, callCtx)
	assert.NoError(t, err)
	assert.Equal(t, "bash", result.ToolName)
}

func TestSystem_RunBefore_ExistingHooks(t *testing.T) {
	s := NewSystem()
	ctx := context.Background()
	callCtx := agent.ToolCallContext{ToolCallID: "test", ToolName: "bash"}

	existing := []agent.BeforeToolCallHook{
		func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
			call.Args = json.RawMessage(`{"modified":true}`)
			return call, nil
		},
	}

	result, err := s.RunBefore(ctx, existing, callCtx)
	assert.NoError(t, err)
	assert.Equal(t, json.RawMessage(`{"modified":true}`), result.Args)
}

func TestSystem_RunBefore_HookBlocksExecution(t *testing.T) {
	s := NewSystem()
	ctx := context.Background()
	callCtx := agent.ToolCallContext{ToolCallID: "test", ToolName: "bash"}

	existing := []agent.BeforeToolCallHook{
		func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
			return call, errors.New("access denied")
		},
	}

	_, err := s.RunBefore(ctx, existing, callCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestSystem_RunBefore_RegisteredAndExistingCombined(t *testing.T) {
	s := NewSystem()
	ctx := context.Background()
	callCtx := agent.ToolCallContext{ToolCallID: "test", ToolName: "bash"}

	var order []string

	// Registered hook (higher priority = runs first)
	s.AddBeforeToolCall(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		order = append(order, "registered")
		return call, nil
	}, WithPriority(50))

	// Existing hooks (run after registered)
	existing := []agent.BeforeToolCallHook{
		func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
			order = append(order, "existing")
			return call, nil
		},
	}

	_, err := s.RunBefore(ctx, existing, callCtx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"registered", "existing"}, order)
}

func TestSystem_RunAfter_NoHooks(t *testing.T) {
	s := NewSystem()
	ctx := context.Background()
	callCtx := agent.ToolCallContext{ToolCallID: "test", ToolName: "bash"}
	origResult := agent.ToolResult{Content: "original"}

	result, err := s.RunAfter(ctx, nil, callCtx, origResult)
	assert.NoError(t, err)
	assert.Equal(t, "original", result.Content)
}

func TestSystem_RunAfter_HookModifiesResult(t *testing.T) {
	s := NewSystem()
	ctx := context.Background()
	callCtx := agent.ToolCallContext{ToolCallID: "test", ToolName: "bash"}

	existing := []agent.AfterToolCallHook{
		func(ctx context.Context, call agent.ToolCallContext, result agent.ToolResult) (agent.ToolResult, error) {
			result.Content = result.Content + " (enriched)"
			return result, nil
		},
	}

	result, err := s.RunAfter(ctx, existing, callCtx, agent.ToolResult{Content: "data"})
	assert.NoError(t, err)
	assert.Equal(t, "data (enriched)", result.Content)
}

func TestSystem_FromLifecycleHooks(t *testing.T) {
	var beforeCalled, afterCalled atomic.Bool

	lifecycle := agent.LifecycleHooks{
		Before: []agent.BeforeToolCallHook{
			func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
				beforeCalled.Store(true)
				return call, nil
			},
		},
		After: []agent.AfterToolCallHook{
			func(ctx context.Context, call agent.ToolCallContext, result agent.ToolResult) (agent.ToolResult, error) {
				afterCalled.Store(true)
				return result, nil
			},
		},
	}

	s := FromLifecycleHooks(lifecycle)
	assert.Equal(t, 2, s.Registry().Len())

	ctx := context.Background()
	callCtx := agent.ToolCallContext{ToolCallID: "test", ToolName: "bash"}

	_, err := s.RunBefore(ctx, nil, callCtx)
	assert.NoError(t, err)
	assert.True(t, beforeCalled.Load())

	_, err = s.RunAfter(ctx, nil, callCtx, agent.ToolResult{})
	assert.NoError(t, err)
	assert.True(t, afterCalled.Load())
}

func TestSystem_ToLifecycleHooks(t *testing.T) {
	s := NewSystem()

	s.AddBeforeToolCall(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		return call, nil
	})
	s.AddAfterToolCall(func(ctx context.Context, call agent.ToolCallContext, result agent.ToolResult) (agent.ToolResult, error) {
		return result, nil
	})
	s.AddSessionStart(func(ctx context.Context, e agent.SessionStartEvent) error {
		return nil
	})

	lifecycle := s.ToLifecycleHooks()
	assert.Len(t, lifecycle.Before, 1)
	assert.Len(t, lifecycle.After, 1)
	assert.Len(t, lifecycle.SessionStart, 1)
	assert.Len(t, lifecycle.SessionEnd, 0)
}

func TestSystem_Status(t *testing.T) {
	s := NewSystem()
	assert.Equal(t, 0, s.Status().TotalHooks)

	s.AddBeforeToolCall(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		return call, nil
	})
	assert.Equal(t, 1, s.Status().TotalHooks)
}

func TestSystem_RunBefore_RegisteredHookBlocks(t *testing.T) {
	s := NewSystem()
	ctx := context.Background()
	callCtx := agent.ToolCallContext{ToolCallID: "test", ToolName: "bash"}

	// Registered hook blocks
	s.AddBeforeToolCall(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		return call, errors.New("policy denied")
	}, WithPriority(PriorityPolicy))

	// Existing hook should NOT be called
	var existingCalled atomic.Bool
	existing := []agent.BeforeToolCallHook{
		func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
			existingCalled.Store(true)
			return call, nil
		},
	}

	_, err := s.RunBefore(ctx, existing, callCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy denied")
	assert.False(t, existingCalled.Load(), "existing hook should not run after registered hook blocks")
}

func TestSystem_RunSessionStart_NonBlocking(t *testing.T) {
	s := NewSystem()
	ctx := context.Background()

	// Hook that fails — should not panic or propagate error
	s.AddSessionStart(func(ctx context.Context, e agent.SessionStartEvent) error {
		return errors.New("hook error")
	})

	// Should not panic
	s.RunSessionStart(ctx, nil, agent.SessionStartEvent{Goal: "test"})
}

// ─── Runner Tests ─────────────────────────────────────────────────────────────

func TestRunner_Run_Success(t *testing.T) {
	r := NewRunner()
	ctx := context.Background()

	result := r.Run(ctx, DefaultHookTimeout, func(ctx context.Context) error {
		return nil
	})

	assert.NoError(t, result.Err)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
}

func TestRunner_Run_Error(t *testing.T) {
	r := NewRunner()
	ctx := context.Background()

	result := r.Run(ctx, DefaultHookTimeout, func(ctx context.Context) error {
		return errors.New("hook failed")
	})

	assert.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "hook failed")
}

func TestRunner_Run_Timeout(t *testing.T) {
	r := NewRunner()
	ctx := context.Background()

	// Use a very short timeout
	result := r.Run(ctx, 1, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	assert.Error(t, result.Err)
}

func TestRunner_Run_Panic(t *testing.T) {
	r := NewRunner()
	ctx := context.Background()

	result := r.Run(ctx, DefaultHookTimeout, func(ctx context.Context) error {
		panic("test panic")
	})

	assert.Error(t, result.Err)
	var pErr *PanicError
	assert.True(t, errors.As(result.Err, &pErr), "error should be a PanicError")
}
