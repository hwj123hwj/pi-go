package kbtools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hwj123hwj/pi-go/internal/agent"
)

// SearchTool searches across all knowledge entries in the repository.
// Unlike the old version, it does NOT depend on a pre-built tags-index.json.
// Instead, it builds an in-memory index on-the-fly (cached) and does weighted
// full-text search across title, tags, summary, and file path.
type SearchTool struct {
	repoPath string
}

func NewSearchTool(repoPath string) *SearchTool {
	return &SearchTool{repoPath: repoPath}
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

type scoredEntry struct {
	entry Entry
	score float64
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

	keywords := strings.Fields(p.Query)

	var results []scoredEntry
	for _, entry := range idx.Entries {
		// Category filter
		if p.Category != "" && !strings.EqualFold(entry.Category, p.Category) {
			continue
		}
		// Tag filter
		if p.Tag != "" {
			found := false
			for _, tag := range entry.Tags {
				if strings.EqualFold(tag, p.Tag) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Score
		var score float64
		if len(keywords) > 0 {
			score = scoreEntry(entry, keywords)
			if score == 0 {
				continue
			}
		} else {
			score = 1.0 // no query → return all (filtered by tag/category)
		}

		results = append(results, scoredEntry{entry: entry, score: score})
	}

	if len(results) == 0 {
		msg := "没有找到匹配的条目"
		if len(keywords) > 0 {
			msg = fmt.Sprintf("没有找到包含「%s」的条目", p.Query)
		}
		return agent.ToolResult{Content: msg}, nil
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	// Format output
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📚 找到 %d 条记录（知识库共 %d 条）：\n\n", len(results), len(idx.Entries)))
	for i, r := range results {
		e := r.entry
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
		b.WriteString(fmt.Sprintf("   📄 %s\n\n", e.RelPath))
	}
	b.WriteString("使用 kb_read 工具读取完整内容，参数 path 填上面的文件路径。")

	return agent.ToolResult{Content: b.String()}, nil
}

// scoreEntry computes a weighted relevance score for an entry against keywords.
// Matching areas (by weight): title > tags > summary > path > category
func scoreEntry(entry Entry, keywords []string) float64 {
	var total float64
	for _, kw := range keywords {
		kwLower := strings.ToLower(kw)
		if kwLower == "" {
			continue
		}
		// Title match (weight 5)
		if strings.Contains(strings.ToLower(entry.Title), kwLower) {
			total += 5.0
		}
		// Tag match (weight 3)
		for _, tag := range entry.Tags {
			if strings.EqualFold(tag, kw) {
				total += 3.0
			} else if strings.Contains(strings.ToLower(tag), kwLower) {
				total += 2.0
			}
		}
		// Summary match (weight 2)
		if strings.Contains(strings.ToLower(entry.Summary), kwLower) {
			total += 2.0
		}
		// Path match (weight 1)
		if strings.Contains(strings.ToLower(entry.RelPath), kwLower) {
			total += 1.0
		}
	}
	return total
}

// resolvePath converts a relative path to absolute, used by other tools.
func resolvePath(repoPath, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(repoPath, p)
}
