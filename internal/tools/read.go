package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/earendil-works/pi-go/internal/agent"
)

type ReadTool struct{}

type ReadParams struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

func NewReadTool() *ReadTool            { return &ReadTool{} }
func (t *ReadTool) Name() string        { return "read" }
func (t *ReadTool) Description() string { return "Read a file from disk." }
func (t *ReadTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "offset": map[string]any{"type": "integer"}, "limit": map[string]any{"type": "integer"}}, "required": []string{"path"}}
}
func (t *ReadTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params ReadParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if params.Limit <= 0 {
		params.Limit = 200
	}
	return json.Marshal(params)
}
func (t *ReadTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	var params ReadParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	data, err := os.ReadFile(filepath.Clean(params.Path))
	if err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	lines := strings.Split(string(data), "\n")
	start := params.Offset
	if start < 0 {
		start = 0
	}
	if start > len(lines) {
		start = len(lines)
	}
	end := start + params.Limit
	if end > len(lines) {
		end = len(lines)
	}
	return agent.ToolResult{Content: strconv.Itoa(start) + ":" + strconv.Itoa(end) + "\n" + strings.Join(lines[start:end], "\n")}, nil
}
