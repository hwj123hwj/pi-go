package kbtools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/earendil-works/pi-go/internal/agent"
)

// ReadTool reads a file from the agent-lessons repository.
type ReadTool struct {
	repoPath string
}

func NewReadTool(repoPath string) *ReadTool {
	return &ReadTool{repoPath: repoPath}
}

func (t *ReadTool) Name() string { return "kb_read" }
func (t *ReadTool) Description() string {
	return "读取知识库中的文件内容（知识卡片、项目日志、踩坑记录等）。返回文件的完整文本。"
}
func (t *ReadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "文件路径（绝对路径或相对于知识库根目录的相对路径）",
			},
			"offset": map[string]any{
				"type":        "number",
				"description": "从第几行开始读（从0开始，默认0）",
			},
			"limit": map[string]any{
				"type":        "number",
				"description": "读取行数（默认200）",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if p.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	return params, nil
}

func (t *ReadTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		Path   string  `json:"path"`
		Offset float64 `json:"offset"`
		Limit  float64 `json:"limit"`
	}
	_ = json.Unmarshal(params, &p)

	// Resolve path
	path := p.Path
	if !strings.HasPrefix(path, "/") {
		path = strings.Join([]string{t.repoPath, path}, "/")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("无法读取文件: %v", err),
			IsError: true,
		}, nil
	}

	lines := strings.Split(string(data), "\n")

	// Apply offset and limit
	offset := int(p.Offset)
	limit := 200
	if p.Limit > 0 {
		limit = int(p.Limit)
	}

	if offset >= len(lines) {
		return agent.ToolResult{
			Content: fmt.Sprintf("文件只有 %d 行，无法从第 %d 行开始读", len(lines), offset),
			IsError: true,
		}, nil
	}

	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}

	selectedLines := lines[offset:end]
	content := strings.Join(selectedLines, "\n")

	// Truncate if too long (keep under 8K chars for LLM context)
	if len(content) > 8000 {
		content = content[:8000] + "\n\n... (内容过长已截断，使用 offset 参数继续读取)"
	}

	header := fmt.Sprintf("📄 %s (%d行)", path, len(lines))
	if offset > 0 || end < len(lines) {
		header += fmt.Sprintf(" [显示第%d-%d行]", offset+1, end)
	}

	return agent.ToolResult{
		Content:    header + "\n\n" + content,
		UserFacing: content,
	}, nil
}
