package extensions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/earendil-works/pi-go/internal/agent"
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

// testExt is a minimal Extension for testing.
type testExt struct {
	name  string
	tools []agent.Tool
	cmds  []CommandDef
	hooks []Hook
}

func (e *testExt) Name() string                       { return e.name }
func (e *testExt) Init(ctx InitContext) error          { return nil }
func (e *testExt) Tools() []agent.Tool                 { return e.tools }
func (e *testExt) Commands() []CommandDef              { return e.cmds }
func (e *testExt) Hooks() []Hook                       { return e.hooks }
