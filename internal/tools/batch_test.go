package tools

import (
	"context"
	"testing"

	"github.com/hwj123hwj/pi-go/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRegistry is a simple test registry for batch tool tests.
type mockRegistry struct {
	tools map[string]agent.Tool
}

func newMockRegistry(tools ...agent.Tool) *mockRegistry {
	r := &mockRegistry{tools: make(map[string]agent.Tool)}
	for _, t := range tools {
		r.tools[t.Name()] = t
	}
	return r
}

func (r *mockRegistry) GetTool(name string) (agent.Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func TestBatchTool_Basic(t *testing.T) {
	// Use LocalTimeTool as a simple mock tool
	registry := newMockRegistry(NewLocalTimeTool())
	tool := NewBatchTool(WithBatchRegistry(registry))
	assert.Equal(t, "batch", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"tool_calls":[{"tool":"local_time","parameters":{}}]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "1/1 succeeded")
}

func TestBatchTool_MultipleCalls(t *testing.T) {
	registry := newMockRegistry(NewLocalTimeTool())
	tool := NewBatchTool(WithBatchRegistry(registry))
	ctx := context.Background()

	validated, err := tool.Validate([]byte(`{"tool_calls":[` +
		`{"tool":"local_time","parameters":{}},` +
		`{"tool":"local_time","parameters":{"timezone":"UTC"}}` +
		`]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "2/2 succeeded")
}

func TestBatchTool_ToolNotFound(t *testing.T) {
	registry := newMockRegistry()
	tool := NewBatchTool(WithBatchRegistry(registry))
	ctx := context.Background()

	validated, err := tool.Validate([]byte(`{"tool_calls":[{"tool":"nonexistent","parameters":{}}]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "0/1 succeeded")
	assert.Contains(t, result.Content, "not found")
}

func TestBatchTool_NoNestedBatch(t *testing.T) {
	registry := newMockRegistry()
	tool := NewBatchTool(WithBatchRegistry(registry))
	ctx := context.Background()

	validated, err := tool.Validate([]byte(`{"tool_calls":[{"tool":"batch","parameters":{"tool_calls":[]}}]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "Cannot nest batch")
}

func TestBatchTool_EmptyCalls(t *testing.T) {
	tool := NewBatchTool()
	_, err := tool.Validate([]byte(`{"tool_calls":[]}`))
	assert.Error(t, err)
}

func TestBatchTool_NoRegistry(t *testing.T) {
	tool := NewBatchTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"tool_calls":[{"tool":"local_time","parameters":{}}]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
	assert.True(t, result.IsError)
}

func TestBatchTool_MixedResults(t *testing.T) {
	registry := newMockRegistry(NewLocalTimeTool())
	tool := NewBatchTool(WithBatchRegistry(registry))
	ctx := context.Background()

	validated, err := tool.Validate([]byte(`{"tool_calls":[` +
		`{"tool":"local_time","parameters":{}},` +
		`{"tool":"nonexistent","parameters":{}}` +
		`]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "1/2 succeeded")
}

func TestBatchTool_MaxCalls(t *testing.T) {
	tool := NewBatchTool()
	// Build a JSON with 21 tool calls (exceeds max of 20)
	calls := `[`
	for i := 0; i < 21; i++ {
		if i > 0 {
			calls += ","
		}
		calls += `{"tool":"local_time","parameters":{}}`
	}
	calls += `]`

	_, err := tool.Validate([]byte(`{"tool_calls":` + calls + `}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum 20")
}
