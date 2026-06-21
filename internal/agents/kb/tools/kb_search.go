package kbtools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/earendil-works/pi-go/internal/agent"
)

// SearchTool searches knowledge cards in the agent-lessons repository.
type SearchTool struct {
	repoPath string // path to agent-lessons repo, e.g. ~/agent-lessons
}

func NewSearchTool(repoPath string) *SearchTool {
	return &SearchTool{repoPath: repoPath}
}

func (t *SearchTool) Name() string { return "kb_search" }
func (t *SearchTool) Description() string {
	return "搜索知识库中的知识卡片。支持关键词搜索、按标签筛选、按分类筛选。返回匹配的卡片标题、摘要、标签和文件路径。"
}
func (t *SearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "搜索关键词（可选，支持中英文）",
			},
			"tag": map[string]any{
				"type":        "string",
				"description": "按标签筛选（精确匹配）",
			},
			"category": map[string]any{
				"type":        "string",
				"description": "按分类筛选：tech/work/english/writing/life/other",
				"enum":        []string{"tech", "work", "english", "writing", "life", "other"},
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

	// Load tags-index.json
	indexPath := filepath.Join(t.repoPath, "doubao-knowledge", "tags-index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("无法读取知识库索引: %v\n请确认路径 %s 下存在 tags-index.json", err, t.repoPath),
			IsError: true,
		}, nil
	}

	var index struct {
		Total int `json:"total"`
		Cards []struct {
			File     string   `json:"file"`
			Title    string   `json:"title"`
			Tags     []string `json:"tags"`
			Summary  string   `json:"summary"`
			Category string   `json:"category"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("索引文件解析失败: %v", err),
			IsError: true,
		}, nil
	}

	// Filter and score
	type scoredCard struct {
		file    string
		title   string
		tags    []string
		summary string
		score   float64
	}

	var results []scoredCard
	q := strings.ToLower(p.Query)

	for _, card := range index.Cards {
		// Category filter
		if p.Category != "" && card.Category != p.Category {
			continue
		}
		// Tag filter
		if p.Tag != "" {
			tagMatch := false
			for _, t := range card.Tags {
				if strings.EqualFold(t, p.Tag) {
					tagMatch = true
					break
				}
			}
			if !tagMatch {
				continue
			}
		}

		// Score (weighted search)
		var score float64
		if q != "" {
			titleLower := strings.ToLower(card.Title)
			summaryLower := strings.ToLower(card.Summary)

			if strings.Contains(titleLower, q) {
				score += 3.0
			}
			for _, tag := range card.Tags {
				if strings.EqualFold(tag, p.Query) {
					score += 2.0
				} else if strings.Contains(strings.ToLower(tag), q) {
					score += 1.5
				}
			}
			if strings.Contains(summaryLower, q) {
				score += 1.0
			}
			if score == 0 {
				continue
			}
		} else {
			score = 1.0 // no query, return all (filtered by tag/category)
		}

		results = append(results, scoredCard{
			file:    card.File,
			title:   card.Title,
			tags:    card.Tags,
			summary: card.Summary,
			score:   score,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	if len(results) == 0 {
		return agent.ToolResult{Content: "没有找到匹配的知识卡片"}, nil
	}

	// Format output
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📚 找到 %d 张知识卡片（共 %d 张）：\n\n", len(results), index.Total))
	for i, r := range results {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.title))
		b.WriteString(fmt.Sprintf("   分类: %s | 标签: %s\n", filepath.Dir(r.file), strings.Join(r.tags, ", ")))
		if r.summary != "" {
			summary := r.summary
			if len(summary) > 100 {
				summary = summary[:100] + "..."
			}
			b.WriteString(fmt.Sprintf("   %s\n", summary))
		}
		b.WriteString(fmt.Sprintf("   文件: %s\n\n", filepath.Join(t.repoPath, "doubao-knowledge", r.file)))
	}
	b.WriteString("使用 kb_read 工具读取完整卡片内容，参数 path 填文件路径。")

	return agent.ToolResult{Content: b.String()}, nil
}
