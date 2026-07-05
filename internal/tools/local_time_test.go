package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalTimeTool_Basic(t *testing.T) {
	tool := NewLocalTimeTool()
	assert.Equal(t, "local_time", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "iso")
	assert.Contains(t, result.Content, "unix_ms")
	assert.Contains(t, result.Content, "local")
	assert.Contains(t, result.Content, "weekday")
}

func TestLocalTimeTool_WithTimezone(t *testing.T) {
	tool := NewLocalTimeTool()
	ctx := context.Background()

	validated, err := tool.Validate([]byte(`{"timezone":"Asia/Shanghai"}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "Asia/Shanghai")
}

func TestLocalTimeTool_InvalidTimezone(t *testing.T) {
	tool := NewLocalTimeTool()
	_, err := tool.Validate([]byte(`{"timezone":"Invalid/Zone"}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timezone")
}

func TestLocalTimeTool_IsConcurrencySafe(t *testing.T) {
	tool := NewLocalTimeTool()
	assert.True(t, tool.IsConcurrencySafe(nil))
}

func TestLocalTimeTool_JSONOutput(t *testing.T) {
	tool := NewLocalTimeTool()
	ctx := context.Background()

	validated, err := tool.Validate([]byte(`{}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)

	var data map[string]any
	err = json.Unmarshal([]byte(result.Content), &data)
	require.NoError(t, err)

	assert.NotEmpty(t, data["iso"])
	assert.NotEmpty(t, data["local"])
	assert.NotEmpty(t, data["weekday"])
	assert.NotNil(t, data["unix_ms"])
	assert.NotNil(t, data["unix_s"])
}
