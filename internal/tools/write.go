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

type WriteTool struct {
	workspace string // 工作目录，用于解析相对路径
}

type WriteParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// WriteToolOption configures a WriteTool during construction.
type WriteToolOption func(*WriteTool)

// WithWriteWorkspace sets the workspace for path resolution.
func WithWriteWorkspace(ws string) WriteToolOption {
	return func(t *WriteTool) { t.workspace = ws }
}

func NewWriteTool(opts ...WriteToolOption) *WriteTool {
	t := &WriteTool{}
	for _, opt := range opts {
		opt(t)
	}
	return t
}
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

	cleanPath := ResolvePath(t.workspace, params.Path)

	// Check path safety if workspace is set
	if t.workspace != "" && !IsPathSafe(t.workspace, cleanPath) {
		return agent.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("path %s is outside workspace %s", params.Path, t.workspace),
		}, fmt.Errorf("path escapes workspace")
	}

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
