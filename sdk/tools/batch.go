package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/hwj123hwj/pi-go/sdk/agent"
)

// ToolRegistry provides lookup and execution of tools by name.
// This interface is used by BatchTool to find and call other tools.
// The agent.Agent naturally satisfies this interface.
type ToolRegistry interface {
	GetTool(name string) (agent.Tool, bool)
}

// BatchTool executes multiple independent tool calls sequentially.
// It is designed for 5+ truly independent operations that have no dependencies.
type BatchTool struct {
	registry ToolRegistry
}

type BatchParams struct {
	ToolCalls []BatchToolCall `json:"tool_calls"`
}

type BatchToolCall struct {
	Tool       string          `json:"tool"`
	Parameters json.RawMessage `json:"parameters"`
}

type BatchOption func(*BatchTool)

func WithBatchRegistry(registry ToolRegistry) BatchOption {
	return func(t *BatchTool) { t.registry = registry }
}

func NewBatchTool(opts ...BatchOption) *BatchTool {
	t := &BatchTool{}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// SetRegistry allows external callers to inject a ToolRegistry after construction.
func (t *BatchTool) SetRegistry(r interface {
	GetTool(string) (agent.Tool, bool)
}) {
	// Type-assert to our own ToolRegistry interface.
	if reg, ok := r.(ToolRegistry); ok {
		t.registry = reg
	}
}

func (t *BatchTool) Name() string { return "batch" }

func (t *BatchTool) Description() string {
	return `Execute multiple independent tools sequentially.

Use this tool ONLY when you need to perform 5+ truly independent operations that have no sequential dependencies. For most cases, prefer individual tool calls in sequence.

AVOID using batch for:
- Operations with dependencies (one result feeds into another)
- File edits followed by testing/validation
- Less than 5 independent operations
- Different tool types that may need result inspection between calls

Example (when appropriate - 5+ independent file reads):
[
  {"tool": "read", "parameters": {"path": "/path/to/file1.ts"}},
  {"tool": "read", "parameters": {"path": "/path/to/file2.ts"}},
  {"tool": "read", "parameters": {"path": "/path/to/file3.ts"}},
  {"tool": "read", "parameters": {"path": "/path/to/file4.ts"}},
  {"tool": "read", "parameters": {"path": "/path/to/file5.ts"}}
]`
}

func (t *BatchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tool_calls": map[string]any{
				"type":        "array",
				"description": "Array of tool calls to execute.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tool":       map[string]any{"type": "string", "description": "The name of the tool to execute."},
						"parameters": map[string]any{"type": "object", "description": "Parameters for the tool."},
					},
					"required": []string{"tool", "parameters"},
				},
			},
		},
		"required": []string{"tool_calls"},
	}
}

func (t *BatchTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params BatchParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if len(params.ToolCalls) == 0 {
		return nil, fmt.Errorf("at least one tool call is required")
	}
	if len(params.ToolCalls) > 20 {
		return nil, fmt.Errorf("maximum 20 tool calls allowed in batch")
	}
	return json.Marshal(params)
}

func (t *BatchTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	var params BatchParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	if t.registry == nil {
		return agent.ToolResult{
			IsError: true,
			Content: "BatchTool: no tool registry available — cannot execute sub-tools",
		}, fmt.Errorf("no tool registry")
	}

	type resultEntry struct {
		Tool    string
		Success bool
		Result  string
		Error   string
	}

	results := make([]resultEntry, 0, len(params.ToolCalls))

	for i, call := range params.ToolCalls {
		if call.Tool == "" {
			results = append(results, resultEntry{
				Tool:    "unknown",
				Success: false,
				Error:   "Missing tool name in batch call.",
			})
			continue
		}

		// Prevent recursive batch calls
		if call.Tool == "batch" {
			results = append(results, resultEntry{
				Tool:    call.Tool,
				Success: false,
				Error:   "Cannot nest batch calls.",
			})
			continue
		}

		tool, ok := t.registry.GetTool(call.Tool)
		if !ok {
			results = append(results, resultEntry{
				Tool:    call.Tool,
				Success: false,
				Error:   fmt.Sprintf("Tool %q not found.", call.Tool),
			})
			continue
		}

		// Validate parameters
		validatedParams, err := tool.Validate(call.Parameters)
		if err != nil {
			results = append(results, resultEntry{
				Tool:    call.Tool,
				Success: false,
				Error:   fmt.Sprintf("Validation failed: %v", err),
			})
			continue
		}

		// Execute
		result, err := tool.Execute(ctx, validatedParams, onUpdate)
		if err != nil {
			slog.Warn("batch: sub-tool failed", "tool", call.Tool, "index", i, "error", err)
			results = append(results, resultEntry{
				Tool:    call.Tool,
				Success: false,
				Error:   err.Error(),
			})
			continue
		}

		results = append(results, resultEntry{
			Tool:    call.Tool,
			Success: !result.IsError,
			Result:  result.Content,
		})
	}

	// Build output
	successCount := 0
	var b strings.Builder
	for _, r := range results {
		if r.Success {
			successCount++
			b.WriteString(fmt.Sprintf("[%s]: Success\n%s\n", r.Tool, r.Result))
		} else {
			b.WriteString(fmt.Sprintf("[%s]: Failed\n%s\n", r.Tool, r.Error))
		}
		b.WriteString("\n---\n\n")
	}

	return agent.ToolResult{
		Content: fmt.Sprintf("Batch execution: %d/%d succeeded.\n%s", successCount, len(results), b.String()),
	}, nil
}
