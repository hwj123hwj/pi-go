package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// safeMockTool 是一个标记为并发安全的 mock 工具。
type safeMockTool struct {
	echoTool // embed echoTool to satisfy Tool
}

func (t *safeMockTool) IsConcurrencySafe(params json.RawMessage) bool { return true }

// sequentialMockTool 是一个标记为顺序执行的 mock 工具。
type sequentialMockTool struct {
	echoTool
}

func (t *sequentialMockTool) ExecutionMode() ExecutionMode { return ExecutionModeSequential }

func TestPartitionToolCalls_MixedSafeAndUnsafe(t *testing.T) {
	ag := &Agent{
		tools: map[string]Tool{
			"safe_read":   &safeMockTool{},
			"unsafe_edit": &echoTool{},
			"safe_grep":   &safeMockTool{},
		},
	}
	calls := []ai.ToolCall{
		{ID: "c1", Name: "safe_read", Args: `{}`},
		{ID: "c2", Name: "safe_grep", Args: `{}`},
		{ID: "c3", Name: "unsafe_edit", Args: `{}`},
	}
	batches := partitionToolCalls(nil, ag, calls)
	require.Len(t, batches, 2)
	assert.True(t, batches[0].safe)
	assert.Len(t, batches[0].calls, 2)
	assert.False(t, batches[1].safe)
	assert.Len(t, batches[1].calls, 1)
}

func TestPartitionToolCalls_AllSafe(t *testing.T) {
	ag := &Agent{
		tools: map[string]Tool{
			"read": &safeMockTool{},
			"grep": &safeMockTool{},
		},
	}
	calls := []ai.ToolCall{
		{ID: "c1", Name: "read", Args: `{}`},
		{ID: "c2", Name: "grep", Args: `{}`},
	}
	batches := partitionToolCalls(nil, ag, calls)
	require.Len(t, batches, 1)
	assert.True(t, batches[0].safe)
	assert.Len(t, batches[0].calls, 2)
}

func TestPartitionToolCalls_AllUnsafe(t *testing.T) {
	ag := &Agent{
		tools: map[string]Tool{
			"edit":  &echoTool{},
			"write": &echoTool{},
		},
	}
	calls := []ai.ToolCall{
		{ID: "c1", Name: "edit", Args: `{}`},
		{ID: "c2", Name: "write", Args: `{}`},
	}
	batches := partitionToolCalls(nil, ag, calls)
	require.Len(t, batches, 2)
	for _, b := range batches {
		assert.False(t, b.safe)
		assert.Len(t, b.calls, 1)
	}
}

// bothTool 实现了 ConcurrencySafeChecker（返回 true）和 ToolWithMode（Sequential）。
// 用于测试 ToolWithMode.Sequential 优先级高于 ConcurrencySafeChecker。
type bothTool struct{}

func (bothTool) Name() string        { return "both" }
func (bothTool) Description() string { return "" }
func (bothTool) Parameters() map[string]any {
	return nil
}
func (bothTool) Validate(raw json.RawMessage) (json.RawMessage, error) { return raw, nil }
func (bothTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(PartialResult)) (ToolResult, error) {
	return ToolResult{}, nil
}
func (bothTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (bothTool) ExecutionMode() ExecutionMode              { return ExecutionModeSequential }

func TestPartitionToolCalls_ToolWithModeSequentialOverridesSafe(t *testing.T) {
	ag := &Agent{tools: map[string]Tool{"both": bothTool{}}}
	calls := []ai.ToolCall{{ID: "c1", Name: "both", Args: `{}`}}
	batches := partitionToolCalls(nil, ag, calls)
	require.Len(t, batches, 1)
	assert.False(t, batches[0].safe, "ToolWithMode.Sequential should override ConcurrencySafeChecker")
}
