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

// EditTool 对文件执行精确的字符串替换。
// 支持单次替换（old_string → new_string），要求 old_string 在文件中唯一。
// 如果文件不存在且 old_string 为空，则创建新文件。
type EditTool struct{}

type EditParams struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func NewEditTool() *EditTool { return &EditTool{} }

func (t *EditTool) Name() string { return "edit" }

func (t *EditTool) Description() string {
	return "Perform exact string replacements in files. old_string must be unique in the file."
}

func (t *EditTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":       map[string]any{"type": "string", "description": "Absolute path to the file to edit."},
			"old_string": map[string]any{"type": "string", "description": "The text to replace. Must match exactly, including whitespace and indentation."},
			"new_string": map[string]any{"type": "string", "description": "The text to replace it with."},
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

	// 读取现有文件内容
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) && params.OldString == "" {
			// 创建新文件
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

	// 检查 old_string 是否存在
	if !strings.Contains(content, params.OldString) {
		return agent.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("old_string not found in %s", cleanPath),
		}, fmt.Errorf("old_string not found in %s", cleanPath)
	}

	// 检查 old_string 是否唯一
	count := strings.Count(content, params.OldString)
	if count > 1 {
		return agent.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("old_string appears %d times in %s; it must be unique. Add more surrounding context to make it unique.", count, cleanPath),
		}, fmt.Errorf("old_string is not unique (found %d occurrences) in %s", count, cleanPath)
	}

	// 执行替换
	newContent := strings.Replace(content, params.OldString, params.NewString, 1)

	if err := os.WriteFile(cleanPath, []byte(newContent), 0o644); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	// 计算受影响的行数范围
	oldLines := strings.Count(params.OldString, "\n") + 1
	newLines := strings.Count(params.NewString, "\n") + 1
	before := content[:strings.Index(content, params.OldString)]
	startLine := strings.Count(before, "\n") + 1
	endLine := startLine + oldLines - 1

	return agent.ToolResult{
		Content: fmt.Sprintf("edited %s (lines %d-%d, %d lines replaced with %d lines)", cleanPath, startLine, endLine, oldLines, newLines),
	}, nil
}
