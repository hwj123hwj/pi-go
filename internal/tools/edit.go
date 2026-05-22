package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/operations"
)

// EditTool performs exact string replacements in files.
// Supports single replacement (old_string must be unique) and replace_all mode.
// If file doesn't exist and old_string is empty, creates a new file.
type EditTool struct {
	workspace string // 工作目录，用于解析相对路径
	ops       operations.FileOperations
}

type EditParams struct {
	Path       string `json:"path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

// EditToolOption configures an EditTool during construction.
type EditToolOption func(*EditTool)

// WithEditWorkspace sets the workspace for path resolution.
func WithEditWorkspace(ws string) EditToolOption {
	return func(t *EditTool) { t.workspace = ws }
}

// WithEditOperations sets the FileOperations backend.
func WithEditOperations(ops operations.FileOperations) EditToolOption {
	return func(t *EditTool) { t.ops = ops }
}

func NewEditTool(opts ...EditToolOption) *EditTool {
	t := &EditTool{}
	for _, opt := range opts {
		opt(t)
	}
	if t.ops == nil {
		t.ops = operations.LocalFileOperations{}
	}
	return t
}

func (t *EditTool) Name() string { return "edit" }

func (t *EditTool) Description() string {
	return "Perform exact string replacements in files. old_string must be unique unless replace_all is true."
}

func (t *EditTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":        map[string]any{"type": "string", "description": "Absolute path to the file to edit."},
			"old_string":  map[string]any{"type": "string", "description": "The text to replace. Must match exactly, including whitespace and indentation."},
			"new_string":  map[string]any{"type": "string", "description": "The text to replace it with."},
			"replace_all": map[string]any{"type": "boolean", "description": "Replace all occurrences (default false, requires unique old_string)."},
		},
		"required": []string{"path", "old_string", "new_string"},
	}
}

func (t *EditTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params EditParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	return json.Marshal(params)
}

func (t *EditTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	var params EditParams
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

	// Read existing file
	data, err := t.ops.ReadFile(ctx, cleanPath)
	if err != nil {
		// File doesn't exist: create new file if old_string is empty
		if isNotExist(err) && params.OldString == "" {
			if err := t.ops.MkdirAll(ctx, parentDir(cleanPath), 0o755); err != nil {
				return agent.ToolResult{IsError: true}, err
			}
			if err := t.ops.WriteFile(ctx, cleanPath, []byte(params.NewString), 0o644); err != nil {
				return agent.ToolResult{IsError: true}, err
			}
			return agent.ToolResult{Content: fmt.Sprintf("created %s", cleanPath)}, nil
		}
		return agent.ToolResult{IsError: true}, err
	}

	content := string(data)

	// Check old_string exists
	if !strings.Contains(content, params.OldString) {
		return agent.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("old_string not found in %s", cleanPath),
		}, fmt.Errorf("old_string not found in %s", cleanPath)
	}

	count := strings.Count(content, params.OldString)

	if params.ReplaceAll {
		// Replace all occurrences
		newContent := strings.ReplaceAll(content, params.OldString, params.NewString)
		if err := t.ops.WriteFile(ctx, cleanPath, []byte(newContent), 0o644); err != nil {
			return agent.ToolResult{IsError: true}, err
		}

		return agent.ToolResult{
			Content: fmt.Sprintf("edited %s (%d replacements)", cleanPath, count),
		}, nil
	}

	// Single replacement: require uniqueness
	if count > 1 {
		return agent.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("old_string appears %d times in %s; it must be unique. Add more surrounding context to make it unique, or use replace_all.", count, cleanPath),
		}, fmt.Errorf("old_string is not unique (found %d occurrences) in %s", count, cleanPath)
	}

	newContent := strings.Replace(content, params.OldString, params.NewString, 1)
	if err := t.ops.WriteFile(ctx, cleanPath, []byte(newContent), 0o644); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	// Show diff context
	oldLines := strings.Count(params.OldString, "\n") + 1
	newLines := strings.Count(params.NewString, "\n") + 1
	before := content[:strings.Index(content, params.OldString)]
	startLine := strings.Count(before, "\n") + 1
	endLine := startLine + oldLines - 1

	// Collect context lines around the change
	allLines := strings.Split(newContent, "\n")
	ctxStart := startLine - 3
	if ctxStart < 1 {
		ctxStart = 1
	}
	ctxEnd := endLine + 3
	if ctxEnd > len(allLines) {
		ctxEnd = len(allLines)
	}

	var diffCtx strings.Builder
	diffCtx.WriteString(fmt.Sprintf("edited %s (lines %d-%d, %d→%d lines)\n\n", cleanPath, startLine, endLine, oldLines, newLines))
	for i := ctxStart; i <= ctxEnd; i++ {
		marker := "  "
		if i >= startLine && i <= startLine+newLines-1 {
			marker = "> "
		}
		if i <= len(allLines) {
			line := allLines[i-1]
			if len(line) > 120 {
				line = line[:120] + "..."
			}
			diffCtx.WriteString(fmt.Sprintf("%s%4d | %s\n", marker, i, line))
		}
	}

	return agent.ToolResult{
		Content: diffCtx.String(),
	}, nil
}

// isNotExist checks if an error indicates a file does not exist.
// Works for both local errors (os.IsNotExist) and operations-level errors.
func isNotExist(err error) bool {
	if os.IsNotExist(err) {
		return true
	}
	// For SSH operations where the error is a string from remote
	msg := err.Error()
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "not exist")
}
