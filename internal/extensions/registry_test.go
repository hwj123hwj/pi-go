package extensions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hwj123hwj/pi-go/internal/agent"
)

func TestRegistry_New(t *testing.T) {
	reg := NewRegistry()
	assert.NotNil(t, reg)
	assert.Empty(t, reg.Names())
}

func TestRegistry_Names(t *testing.T) {
	reg := NewRegistry()

	ext := &testExt{name: "test-ext"}
	err := reg.Register(ext)
	require.NoError(t, err)

	assert.Contains(t, reg.Names(), "test-ext")
}

func TestRegistry_EmitHook(t *testing.T) {
	reg := NewRegistry()
	called := false

	ext := &testExt{
		name: "hook-ext",
		hooks: []Hook{
			{
				Event: "test.event",
				Handler: func(ctx context.Context, data any) error {
					called = true
					return nil
				},
			},
		},
	}

	err := reg.Register(ext)
	require.NoError(t, err)

	err = reg.EmitHook(context.Background(), "test.event", nil)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestRegistry_EmitHook_NoHandlers(t *testing.T) {
	reg := NewRegistry()
	err := reg.EmitHook(context.Background(), "nonexistent.event", nil)
	assert.NoError(t, err)
}

func TestRegistry_Tools(t *testing.T) {
	reg := NewRegistry()
	tools := reg.Tools()
	assert.Empty(t, tools)
}

func TestRegistry_Commands(t *testing.T) {
	reg := NewRegistry()
	cmds := reg.Commands()
	assert.Empty(t, cmds)
}

// ─── Lifecycle hook tests ────────────────────────────────────────────────────

func TestRegistry_LifecycleHooks_Empty(t *testing.T) {
	reg := NewRegistry()
	hooks := reg.LifecycleHooks()
	assert.Empty(t, hooks.Before)
	assert.Empty(t, hooks.After)
}

func TestRegistry_RegisterBeforeToolCallHook(t *testing.T) {
	reg := NewRegistry()
	called := false

	reg.RegisterBeforeToolCallHook(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		called = true
		return call, nil
	})

	hooks := reg.LifecycleHooks()
	assert.Len(t, hooks.Before, 1)

	// Call the hook to verify it works
	_, err := hooks.Before[0](context.Background(), agent.ToolCallContext{ToolName: "test"})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestRegistry_RegisterAfterToolCallHook(t *testing.T) {
	reg := NewRegistry()
	called := false

	reg.RegisterAfterToolCallHook(func(ctx context.Context, call agent.ToolCallContext, result agent.ToolResult) (agent.ToolResult, error) {
		called = true
		return result, nil
	})

	hooks := reg.LifecycleHooks()
	assert.Len(t, hooks.After, 1)

	// Call the hook to verify it works
	_, err := hooks.After[0](context.Background(), agent.ToolCallContext{ToolName: "test"}, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestRegistry_LifecycleHooks_Multiple(t *testing.T) {
	reg := NewRegistry()

	var order []string
	reg.RegisterBeforeToolCallHook(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		order = append(order, "before1")
		return call, nil
	})
	reg.RegisterBeforeToolCallHook(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		order = append(order, "before2")
		return call, nil
	})
	reg.RegisterAfterToolCallHook(func(ctx context.Context, call agent.ToolCallContext, result agent.ToolResult) (agent.ToolResult, error) {
		order = append(order, "after1")
		return result, nil
	})

	hooks := reg.LifecycleHooks()
	assert.Len(t, hooks.Before, 2)
	assert.Len(t, hooks.After, 1)

	// Call in order
	_, _ = hooks.Before[0](context.Background(), agent.ToolCallContext{})
	_, _ = hooks.Before[1](context.Background(), agent.ToolCallContext{})
	_, _ = hooks.After[0](context.Background(), agent.ToolCallContext{}, agent.ToolResult{})

	assert.Equal(t, []string{"before1", "before2", "after1"}, order)
}

func TestRegistry_LifecycleHooks_ImmutableSnapshot(t *testing.T) {
	reg := NewRegistry()

	reg.RegisterBeforeToolCallHook(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		return call, nil
	})

	hooks1 := reg.LifecycleHooks()
	assert.Len(t, hooks1.Before, 1)

	// Register another hook after taking snapshot
	reg.RegisterBeforeToolCallHook(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		return call, nil
	})

	// Original snapshot should be unchanged
	assert.Len(t, hooks1.Before, 1)

	// New snapshot should have 2
	hooks2 := reg.LifecycleHooks()
	assert.Len(t, hooks2.Before, 2)
}

// testExt is a minimal Extension for testing.
type testExt struct {
	name  string
	tools []agent.Tool
	cmds  []CommandDef
	hooks []Hook
}

func (e *testExt) Name() string               { return e.name }
func (e *testExt) Init(ctx InitContext) error { return nil }
func (e *testExt) Tools() []agent.Tool        { return e.tools }
func (e *testExt) Commands() []CommandDef     { return e.cmds }
func (e *testExt) Hooks() []Hook              { return e.hooks }

// ─── ExtensionWithLifecycle tests ─────────────────────────────────────────────

// testLifecycleExt implements ExtensionWithLifecycle.
type testLifecycleExt struct {
	testExt
	beforeHooks []agent.BeforeToolCallHook
	afterHooks  []agent.AfterToolCallHook
}

func (e *testLifecycleExt) BeforeToolCallHooks() []agent.BeforeToolCallHook { return e.beforeHooks }
func (e *testLifecycleExt) AfterToolCallHooks() []agent.AfterToolCallHook   { return e.afterHooks }

func TestRegister_ExtensionWithLifecycle(t *testing.T) {
	reg := NewRegistry()

	beforeCalled := false
	afterCalled := false

	ext := &testLifecycleExt{
		testExt: testExt{name: "lifecycle-ext"},
		beforeHooks: []agent.BeforeToolCallHook{
			func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
				beforeCalled = true
				return call, nil
			},
		},
		afterHooks: []agent.AfterToolCallHook{
			func(ctx context.Context, call agent.ToolCallContext, result agent.ToolResult) (agent.ToolResult, error) {
				afterCalled = true
				return result, nil
			},
		},
	}

	err := reg.Register(ext)
	require.NoError(t, err)

	// Hooks should be collected automatically
	hooks := reg.LifecycleHooks()
	assert.Len(t, hooks.Before, 1)
	assert.Len(t, hooks.After, 1)

	// Call them to verify
	_, err = hooks.Before[0](context.Background(), agent.ToolCallContext{ToolName: "test"})
	require.NoError(t, err)
	assert.True(t, beforeCalled)

	_, err = hooks.After[0](context.Background(), agent.ToolCallContext{ToolName: "test"}, agent.ToolResult{Content: "ok"})
	require.NoError(t, err)
	assert.True(t, afterCalled)
}

func TestRegister_ExtensionWithoutLifecycle(t *testing.T) {
	reg := NewRegistry()

	// Plain extension (no ExtensionWithLifecycle)
	ext := &testExt{name: "plain-ext"}
	err := reg.Register(ext)
	require.NoError(t, err)

	hooks := reg.LifecycleHooks()
	assert.Empty(t, hooks.Before)
	assert.Empty(t, hooks.After)
}

func TestRegister_MixedExtensions(t *testing.T) {
	reg := NewRegistry()

	// Register a plain extension
	reg.Register(&testExt{name: "plain"})

	// Register a lifecycle extension
	reg.Register(&testLifecycleExt{
		testExt:     testExt{name: "lifecycle"},
		beforeHooks: []agent.BeforeToolCallHook{func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) { return call, nil }},
	})

	// Register another lifecycle extension
	reg.Register(&testLifecycleExt{
		testExt: testExt{name: "lifecycle2"},
		afterHooks: []agent.AfterToolCallHook{func(ctx context.Context, call agent.ToolCallContext, result agent.ToolResult) (agent.ToolResult, error) {
			return result, nil
		}},
	})

	// Also add a manually registered hook
	reg.RegisterBeforeToolCallHook(func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) {
		return call, nil
	})

	hooks := reg.LifecycleHooks()
	assert.Len(t, hooks.Before, 2) // 1 from lifecycle ext + 1 manual
	assert.Len(t, hooks.After, 1)  // 1 from lifecycle2 ext
}

// ─── Session / Compress observer hook tests ─────────────────────────────────

// testSessionExt implements ExtensionWithSessionHooks.
type testSessionExt struct {
	testExt
	startHooks []agent.SessionStartHook
	endHooks   []agent.SessionEndHook
}

func (e *testSessionExt) SessionStartHooks() []agent.SessionStartHook { return e.startHooks }
func (e *testSessionExt) SessionEndHooks() []agent.SessionEndHook     { return e.endHooks }

// testCompressExt implements ExtensionWithCompressHook.
type testCompressExt struct {
	testExt
	preCompressHooks []agent.PreCompressHook
}

func (e *testCompressExt) PreCompressHooks() []agent.PreCompressHook { return e.preCompressHooks }

func TestRegister_SessionAndCompressHooks(t *testing.T) {
	reg := NewRegistry()

	require.NoError(t, reg.Register(&testSessionExt{
		testExt:    testExt{name: "session-ext"},
		startHooks: []agent.SessionStartHook{func(ctx context.Context, e agent.SessionStartEvent) error { return nil }},
		endHooks:   []agent.SessionEndHook{func(ctx context.Context, e agent.SessionEndEvent) error { return nil }},
	}))
	require.NoError(t, reg.Register(&testCompressExt{
		testExt:          testExt{name: "compress-ext"},
		preCompressHooks: []agent.PreCompressHook{func(ctx context.Context, e agent.PreCompressEvent) error { return nil }},
	}))

	hooks := reg.LifecycleHooks()
	assert.Len(t, hooks.SessionStart, 1)
	assert.Len(t, hooks.SessionEnd, 1)
	assert.Len(t, hooks.PreCompress, 1)
	// 不影响原有 before/after
	assert.Empty(t, hooks.Before)
	assert.Empty(t, hooks.After)
}

// 纯 lifecycle extension 不贡献 session/compress hook（独立接口，互不干扰）。
func TestRegister_LifecycleExtNoSessionHooks(t *testing.T) {
	reg := NewRegistry()
	require.NoError(t, reg.Register(&testLifecycleExt{
		testExt:     testExt{name: "lifecycle-only"},
		beforeHooks: []agent.BeforeToolCallHook{func(ctx context.Context, call agent.ToolCallContext) (agent.ToolCallContext, error) { return call, nil }},
	}))

	hooks := reg.LifecycleHooks()
	assert.Len(t, hooks.Before, 1)
	assert.Empty(t, hooks.SessionStart, "lifecycle ext 不应贡献 session hook")
	assert.Empty(t, hooks.PreCompress)
}
