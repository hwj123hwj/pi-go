package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "Absolute path to the file to write."},
			"content": map[string]any{"type": "string", "description": "The content to write."},
		},
		"required": []string{"path", "content"},
	}
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

	cleanPath := filepath.Clean(params.Path)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	content := []byte(params.Content)
	if err := os.WriteFile(cleanPath, content, 0o644); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	// Count bytes and lines
	byteCount := len(content)
	lineCount := strings.Count(params.Content, "\n")
	if !strings.HasSuffix(params.Content, "\n") && len(params.Content) > 0 {
		lineCount++
	}

	return agent.ToolResult{
		Content: fmt.Sprintf("written %s (%s, %d lines)", cleanPath, FormatByteCount(byteCount), lineCount),
	}, nil
}
