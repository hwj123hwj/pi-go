package kbtools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hwj123hwj/pi-go/internal/agent"
)

// SearchTool searches across all knowledge entries in the repository.
//
// It delegates the actual retrieval to a pluggable SearchStrategy
// (default: KeywordSearcher). The tool itself is a thin wrapper that
// handles parameter parsing, result formatting, and error handling.
type SearchTool struct {
	repoPath string
	strategy SearchStrategy
}

// NewSearchTool creates a SearchTool with the default keyword strategy.
func NewSearchTool(repoPath string) *SearchTool {
	return &SearchTool{
		repoPath: repoPath,
		strategy: KeywordSearcher{},
	}
}

// NewSearchToolWithStrategy creates a SearchTool with a custom strategy.
// This is the extension point for future vector/hybrid search.
func NewSearchToolWithStrategy(repoPath string, strategy SearchStrategy) *SearchTool {
	return &SearchTool{
		repoPath: repoPath,
		strategy: strategy,
	}
}

func (t *SearchTool) Name() string { return "kb_search" }
func (t *SearchTool) Description() string {
	return `搜索知识库中的所有条目。自动扫描仓库中的 Markdown 文件，支持关键词搜索、按标签筛选、按分类筛选。
搜索范围包括标题、摘要、标签和文件路径。返回匹配的条目标题、摘要、标签和文件路径。

工作流建议：
1. 先用 kb_search 搜索 → 找到相关条目
2. 用 kb_read 读取完整内容
3. 需要浏览特定分类时用 category 参数`
}
func (t *SearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "搜索关键词（可选，支持中英文，多个词空格分隔）",
			},
			"tag": map[string]any{
				"type":        "string",
				"description": "按标签筛选（不区分大小写的精确匹配）",
			},
			"category": map[string]any{
				"type":        "string",
				"description": "按分类筛选（即仓库顶层目录名，如 issues, tech 等）",
			},
			"limit": map[string]any{
				"type":        "number",
				"description": "返回数量，默认 10",
			},
		},
	}
}

func (t *SearchTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	return params, nil
}

func (t *SearchTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		Query    string  `json:"query"`
		Tag      string  `json:"tag"`
		Category string  `json:"category"`
		Limit    float64 `json:"limit"`
	}
	_ = json.Unmarshal(params, &p)

	limit := 10
	if p.Limit > 0 {
		limit = int(p.Limit)
	}

	// Build (or get cached) index
	idx, err := GetIndex(t.repoPath)
	if err != nil || len(idx.Entries) == 0 {
		return agent.ToolResult{
			Content: fmt.Sprintf("知识库为空或无法读取。请确认路径 %s 下有 Markdown 文件。", t.repoPath),
			IsError: true,
		}, nil
	}

	// Delegate to search strategy
	results := t.strategy.Search(idx.Entries, SearchQuery{
		Query:    p.Query,
		Tag:      p.Tag,
		Category: p.Category,
		Limit:    limit,
	})

	if len(results) == 0 {
		msg := "没有找到匹配的条目"
		if strings.TrimSpace(p.Query) != "" {
			msg = fmt.Sprintf("没有找到包含「%s」的条目", p.Query)
		}
		return agent.ToolResult{Content: msg}, nil
	}

	// Format output
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📚 找到 %d 条记录（知识库共 %d 条）：\n\n", len(results), len(idx.Entries)))
	for i, r := range results {
		e := r.Entry
		absPath := filepath.Join(t.repoPath, e.RelPath)
		b.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, e.Title))
		// Metadata line
		var meta []string
		if e.Category != "" {
			meta = append(meta, fmt.Sprintf("分类: %s", e.Category))
		}
		if len(e.Tags) > 0 {
			meta = append(meta, fmt.Sprintf("标签: %s", strings.Join(e.Tags, ", ")))
		}
		if len(meta) > 0 {
			b.WriteString(fmt.Sprintf("   %s\n", strings.Join(meta, " | ")))
		}
		if e.Summary != "" {
			b.WriteString(fmt.Sprintf("   > %s\n", e.Summary))
		}
		b.WriteString(fmt.Sprintf("   📄 `%s`\n\n", absPath))
	}
	b.WriteString("使用 kb_read 工具读取完整内容，参数 path 填上面的文件路径（绝对路径或相对路径均可）。")

	return agent.ToolResult{Content: b.String()}, nil
}

// resolvePath converts a relative path to absolute, used by other tools.
func resolvePath(repoPath, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(repoPath, p)
}

// formatRelPath returns the relative path for display, with a fallback to the
// absolute path if the file is outside the repository.
func formatRelPath(repoPath, absPath string) string {
	rel, err := filepath.Rel(repoPath, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return absPath
	}
	return rel
}
