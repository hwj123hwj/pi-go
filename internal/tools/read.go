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
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":   map[string]any{"type": "string", "description": "Absolute or relative path to the file."},
			"offset": map[string]any{"type": "integer", "description": "Starting line number (0-indexed, default 0)."},
			"limit":  map[string]any{"type": "integer", "description": "Maximum number of lines to read (default 2000)."},
		},
		"required": []string{"path"},
	}
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
		params.Limit = 2000
	}
	return json.Marshal(params)
}
func (t *ReadTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	var params ReadParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	// Resolve path
	cleanPath := filepath.Clean(params.Path)

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	start := params.Offset
	if start < 0 {
		start = 0
	}
	if start > totalLines {
		start = totalLines
	}
	end := start + params.Limit
	if end > totalLines {
		end = totalLines
	}

	// Output with line numbers
	var b strings.Builder
	selectedLines := lines[start:end]
	for i, line := range selectedLines {
		lineNum := start + i + 1 // 1-indexed
		b.WriteString(fmt.Sprintf("%6d\t%s\n", lineNum, line))
	}

	// Hint about remaining lines for large files
	remaining := totalLines - end
	if remaining > 0 {
		b.WriteString(fmt.Sprintf("\n... %d more lines (use offset=%d to read further)\n", remaining, end))
	}

	content := b.String()
	content = TruncateOutput(content, DefaultMaxOutputLen)

	return agent.ToolResult{Content: content}, nil
}

// Format is used by prompt_info to give snippet
func lineCount(s string) string {
	return strconv.Itoa(len(strings.Split(s, "\n")))
}
