package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/earendil-works/pi-go/internal/agent"
)

type WriteTool struct{}

type WriteParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func NewWriteTool() *WriteTool           { return &WriteTool{} }
func (t *WriteTool) Name() string        { return "write" }
func (t *WriteTool) Description() string { return "Write a file to disk." }
func (t *WriteTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}}, "required": []string{"path", "content"}}
}
func (t *WriteTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params WriteParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	return json.Marshal(params)
}
func (t *WriteTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	var params WriteParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	if err := os.MkdirAll(filepath.Dir(params.Path), 0o755); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	if err := os.WriteFile(filepath.Clean(params.Path), []byte(params.Content), 0o644); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	return agent.ToolResult{Content: "written " + params.Path}, nil
}
