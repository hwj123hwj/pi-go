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

// EditTool performs exact string replacements in files.
// Supports single replacement (old_string must be unique) and replace_all mode.
// If file doesn't exist and old_string is empty, creates a new file.
type EditTool struct{}

type EditParams struct {
	Path       string `json:"path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

func NewEditTool() *EditTool { return &EditTool{} }

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

	cleanPath := filepath.Clean(params.Path)

	// Read existing file
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) && params.OldString == "" {
			if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
				return agent.ToolResult{IsError: true}, err
			}
			if err := os.WriteFile(cleanPath, []byte(params.NewString), 0o644); err != nil {
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
		if err := os.WriteFile(cleanPath, []byte(newContent), 0o644); err != nil {
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
	if err := os.WriteFile(cleanPath, []byte(newContent), 0o644); err != nil {
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
