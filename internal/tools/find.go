package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/earendil-works/pi-go/internal/agent"
)

// FindTool 在目录中搜索匹配名称模式的文件。
// 支持递归搜索和 glob 模式匹配。
type FindTool struct{}

type FindParams struct {
	Path       string `json:"path"`
	Pattern    string `json:"pattern,omitempty"`    // 文件名 glob 模式（如 "*.go"）
	MaxDepth   int    `json:"max_depth,omitempty"`  // 最大搜索深度
	MaxResults int    `json:"max_results,omitempty"` // 最大结果数
	Type       string `json:"type,omitempty"`       // "file" | "dir" | "" (both)
}

func NewFindTool() *FindTool { return &FindTool{} }

func (t *FindTool) Name() string { return "find" }

func (t *FindTool) Description() string {
	return "Find files and directories by name pattern. Supports glob patterns and recursive search."
}

func (t *FindTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":        map[string]any{"type": "string", "description": "Directory to search in."},
			"pattern":     map[string]any{"type": "string", "description": "Glob pattern for file names (e.g. *.go, test_*.ts)."},
			"max_depth":   map[string]any{"type": "integer", "description": "Maximum search depth (default unlimited)."},
			"max_results": map[string]any{"type": "integer", "description": "Maximum number of results (default 50)."},
			"type":        map[string]any{"type": "string", "description": "Filter by type: 'file', 'dir', or empty for both."},
		},
		"required": []string{"path"},
	}
}

func (t *FindTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params FindParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if params.MaxResults <= 0 {
		params.MaxResults = 50
	}
	if params.Type != "" && params.Type != "file" && params.Type != "dir" {
		return nil, fmt.Errorf("type must be 'file', 'dir', or empty")
	}
	return json.Marshal(params)
}

func (t *FindTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	var params FindParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	searchPath := filepath.Clean(params.Path)

	info, err := os.Stat(searchPath)
	if err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	if !info.IsDir() {
		return agent.ToolResult{IsError: true}, fmt.Errorf("%s is not a directory", searchPath)
	}

	var results []string
	rootDepth := strings.Count(searchPath, string(filepath.Separator))

	filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 深度控制
		if params.MaxDepth > 0 {
			currentDepth := strings.Count(path, string(filepath.Separator)) - rootDepth
			if currentDepth > params.MaxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// 跳过隐藏目录
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		// 类型过滤
		if params.Type == "file" && d.IsDir() {
			return nil
		}
		if params.Type == "dir" && !d.IsDir() {
			return nil
		}

		// 名称模式匹配
		if params.Pattern != "" {
			matched, _ := filepath.Match(params.Pattern, d.Name())
			if !matched {
				return nil
			}
		}

		results = append(results, path)
		if len(results) >= params.MaxResults {
			return fmt.Errorf("max results reached")
		}
		return nil
	})

	if len(results) == 0 {
		return agent.ToolResult{Content: "no files found"}, nil
	}

	var b strings.Builder
	for _, r := range results {
		b.WriteString(r)
		b.WriteByte('\n')
	}
	b.WriteString(fmt.Sprintf("\n%d results found.", len(results)))

	return agent.ToolResult{Content: b.String()}, nil
}
