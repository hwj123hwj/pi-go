package kbtools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"path/filepath"

	"github.com/hwj123hwj/pi-go/sdk/agent"
)

// ListTool browses and lists all entries in the knowledge base.
// It replaces the old kb_query's cross-module grep with a structured overview.
type ListTool struct {
	repoPath string
}

func NewListTool(repoPath string) *ListTool {
	return &ListTool{repoPath: repoPath}
}

func (t *ListTool) Name() string { return "kb_list" }
func (t *ListTool) Description() string {
	return `列出知识库中的所有条目，支持按分类或标签过滤。

用途：
- "知识库里有什么" → kb_list（浏览全貌）
- "看看 tech 分类" → kb_list(category="tech")
- "有哪些带 Go 标签的" → kb_list(tag="Go")
- "最近添加了什么" → kb_list(sort="recent")`
}
func (t *ListTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"category": map[string]any{
				"type":        "string",
				"description": "按分类筛选（即仓库顶层目录名）",
			},
			"tag": map[string]any{
				"type":        "string",
				"description": "按标签筛选（不区分大小写）",
			},
			"sort": map[string]any{
				"type":        "string",
				"description": "排序方式：recent（按修改时间降序，默认）或 title（按标题排序）",
				"enum":        []string{"recent", "title"},
			},
			"limit": map[string]any{
				"type":        "number",
				"description": "返回数量，默认 20",
			},
		},
	}
}

func (t *ListTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	return params, nil
}

func (t *ListTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		Category string  `json:"category"`
		Tag      string  `json:"tag"`
		Sort     string  `json:"sort"`
		Limit    float64 `json:"limit"`
	}
	_ = json.Unmarshal(params, &p)

	limit := 20
	if p.Limit > 0 {
		limit = int(p.Limit)
	}
	if p.Sort == "" {
		p.Sort = "recent"
	}

	idx, err := GetIndex(t.repoPath)
	if err != nil || len(idx.Entries) == 0 {
		return agent.ToolResult{
			Content: fmt.Sprintf("知识库为空或无法读取。请确认路径 %s 下有 Markdown 文件。", t.repoPath),
			IsError: true,
		}, nil
	}

	// Filter
	var entries []Entry
	for _, e := range idx.Entries {
		if p.Category != "" && !strings.EqualFold(e.Category, p.Category) {
			continue
		}
		if p.Tag != "" {
			found := false
			for _, tag := range e.Tags {
				if strings.EqualFold(tag, p.Tag) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		entries = append(entries, e)
	}

	// Sort
	switch p.Sort {
	case "title":
		sort.Slice(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].Title) < strings.ToLower(entries[j].Title)
		})
	default: // recent
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Modified.After(entries[j].Modified)
		})
	}

	totalInCategory := len(entries)

	if len(entries) > limit {
		entries = entries[:limit]
	}

	if len(entries) == 0 {
		msg := "知识库为空"
		if p.Category != "" || p.Tag != "" {
			msg = "没有匹配的条目"
		}
		return agent.ToolResult{Content: msg}, nil
	}

	// Format output — grouped by category for readability
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📚 知识库（共 %d 条", len(idx.Entries)))
	if p.Category != "" {
		b.WriteString(fmt.Sprintf("，筛选分类: %s", p.Category))
	}
	if p.Tag != "" {
		b.WriteString(fmt.Sprintf("，筛选标签: %s", p.Tag))
	}
	b.WriteString(fmt.Sprintf("）— 显示 %d/%d：\n\n", len(entries), totalInCategory))

	// Group by category for display
	currentCategory := ""
	for _, e := range entries {
		if e.Category != currentCategory {
			currentCategory = e.Category
			displayCat := currentCategory
			if displayCat == "" {
				displayCat = "other"
			}
			b.WriteString(fmt.Sprintf("## %s\n\n", displayCat))
		}
		absPath := filepath.Join(t.repoPath, e.RelPath)
		b.WriteString(fmt.Sprintf("- **%s**", e.Title))
		var meta []string
		if len(e.Tags) > 0 {
			meta = append(meta, fmt.Sprintf("#%s", strings.Join(e.Tags, " #")))
		}
		if e.Summary != "" {
			summary := e.Summary
			if len(summary) > 80 {
				summary = summary[:80] + "..."
			}
			meta = append(meta, summary)
		}
		if len(meta) > 0 {
			b.WriteString(fmt.Sprintf(" — %s", strings.Join(meta, " ")))
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  📄 `%s`\n", absPath))
	}

	b.WriteString("\n使用 kb_read 读取完整内容（参数 path 支持绝对路径或相对路径），或 kb_search 搜索特定关键词。")

	return agent.ToolResult{Content: b.String()}, nil
}
