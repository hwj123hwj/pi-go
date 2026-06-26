package kbtools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hwj123hwj/pi-go/internal/agent"
)

// MaintainTool is the second-brain "stewardship" tool.
//
// Unlike the read-only tools (search/read/list), this tool runs maintenance
// operations to keep the knowledge base healthy:
//
//   - health     : overall report (metadata gaps, duplicates, tag clusters)
//   - duplicates : find entries with near-identical titles
//   - tags       : tag usage overview + normalization suggestions
//   - stats      : quick category/tag distribution
//
// All operations are READ-ONLY — they produce recommendations.
// The AI decides what to fix and uses kb_save to persist changes.
type MaintainTool struct {
	repoPath string
}

func NewMaintainTool(repoPath string) *MaintainTool {
	return &MaintainTool{repoPath: repoPath}
}

func (t *MaintainTool) Name() string { return "kb_maintain" }
func (t *MaintainTool) Description() string {
	return `维护知识库健康度。你是知识库的"管家"，这个工具帮助你发现和诊断问题。

支持的操作（action 参数）：

1. **health** — 全面健康检查
   返回知识库概况 + 发现的所有问题（缺摘要、缺标签、重复条目、标签不一致等）

2. **duplicates** — 重复检测
   查找标题高度相似的条目，可能需要合并

3. **tags** — 标签分析
   显示标签使用频率，找出拼写不一致的标签（如 "Ollama" vs "ollama"）

4. **stats** — 统计概览
   分类和标签的分布情况

建议定期运行 health 检查，保持知识库整洁。`
}
func (t *MaintainTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "维护操作：health（全面检查）、duplicates（查重）、tags（标签分析）、stats（统计概览）",
				"enum":        []string{"health", "duplicates", "tags", "stats"},
			},
		},
		"required": []string{"action"},
	}
}

func (t *MaintainTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	switch p.Action {
	case "health", "duplicates", "tags", "stats":
		return params, nil
	default:
		return nil, fmt.Errorf("invalid action %q: must be one of health, duplicates, tags, stats", p.Action)
	}
}

func (t *MaintainTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal(params, &p)

	idx, err := GetIndex(t.repoPath)
	if err != nil || len(idx.Entries) == 0 {
		return agent.ToolResult{
			Content: fmt.Sprintf("知识库为空或无法读取。请确认路径 %s 下有 Markdown 文件。", t.repoPath),
			IsError: true,
		}, nil
	}

	switch p.Action {
	case "health":
		return t.runHealthCheck(idx), nil
	case "duplicates":
		return t.runDuplicateCheck(idx), nil
	case "tags":
		return t.runTagAnalysis(idx), nil
	case "stats":
		return t.runStats(idx), nil
	default:
		return agent.ToolResult{
			Content: fmt.Sprintf("未知操作: %s", p.Action),
			IsError: true,
		}, nil
	}
}

// ── health ────────────────────────────────────────────────────────────────

func (t *MaintainTool) runHealthCheck(idx *Index) agent.ToolResult {
	report := GenerateHealthReport(idx)
	var b strings.Builder

	b.WriteString(fmt.Sprintf("🔍 **知识库健康报告**\n\n"))
	b.WriteString(fmt.Sprintf("📊 **概况**: %d 条目 | %d 分类 | %d 标签\n\n", report.TotalEntries, report.Categories, report.Tags))

	// ── Metadata gaps ──
	gapCount := 0
	b.WriteString("### 元数据缺失\n\n")
	if len(report.EntriesMissingTitle) > 0 {
		b.WriteString(fmt.Sprintf("- ⚠️ **%d 条目缺标题**:\n", len(report.EntriesMissingTitle)))
		for _, e := range report.EntriesMissingTitle {
			absPath := filepath.Join(t.repoPath, e.RelPath)
			b.WriteString(fmt.Sprintf("  - `%s`\n", absPath))
		}
		gapCount += len(report.EntriesMissingTitle)
	}
	if len(report.EntriesMissingSummary) > 0 {
		b.WriteString(fmt.Sprintf("- ⚠️ **%d 条目缺摘要**:\n", len(report.EntriesMissingSummary)))
		for _, e := range report.EntriesMissingSummary {
			absPath := filepath.Join(t.repoPath, e.RelPath)
			b.WriteString(fmt.Sprintf("  - `%s` — %s\n", absPath, e.Title))
		}
		gapCount += len(report.EntriesMissingSummary)
	}
	if len(report.EntriesMissingTags) > 0 {
		b.WriteString(fmt.Sprintf("- ⚠️ **%d 条目缺标签**:\n", len(report.EntriesMissingTags)))
		// Show first 10 to avoid spamming
		for i, e := range report.EntriesMissingTags {
			if i >= 10 {
				b.WriteString(fmt.Sprintf("  - ...还有 %d 条\n", len(report.EntriesMissingTags)-10))
				break
			}
			absPath := filepath.Join(t.repoPath, e.RelPath)
			b.WriteString(fmt.Sprintf("  - `%s` — %s\n", absPath, e.Title))
		}
		gapCount += len(report.EntriesMissingTags)
	}
	if gapCount == 0 {
		b.WriteString("✅ 所有条目元数据完整\n")
	}
	b.WriteString("\n")

	// ── Duplicates ──
	b.WriteString("### 重复检测\n\n")
	if len(report.DuplicateGroups) > 0 {
		b.WriteString(fmt.Sprintf("⚠️ 发现 %d 组可能重复的条目:\n\n", len(report.DuplicateGroups)))
		for i, g := range report.DuplicateGroups {
			b.WriteString(fmt.Sprintf("%d. **%s** (%d条)\n", i+1, g.CanonicalTitle, len(g.Entries)))
			for _, e := range g.Entries {
				absPath := filepath.Join(t.repoPath, e.RelPath)
				b.WriteString(fmt.Sprintf("   - `%s` (%s)\n", absPath, e.Category))
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString("✅ 未发现重复条目\n")
	}

	// ── Tag clusters ──
	b.WriteString("### 标签一致性\n\n")
	if len(report.TagClusters) > 0 {
		b.WriteString(fmt.Sprintf("⚠️ 发现 %d 组标签不一致:\n\n", len(report.TagClusters)))
		for i, c := range report.TagClusters {
			b.WriteString(fmt.Sprintf("%d. 建议统一为 **%s** ← %s (%d条)\n",
				i+1, c.Canonical, strings.Join(c.Variants, " / "), c.Count))
		}
	} else {
		b.WriteString("✅ 标签命名一致\n")
	}

	b.WriteString("\n---\n")
	b.WriteString(fmt.Sprintf("**总结**: %s\n", report.Summary()))
	if gapCount == 0 && len(report.DuplicateGroups) == 0 && len(report.TagClusters) == 0 {
		b.WriteString("\n🎉 知识库状态良好！")
	}

	return agent.ToolResult{Content: b.String()}
}

// ── duplicates ────────────────────────────────────────────────────────────

func (t *MaintainTool) runDuplicateCheck(idx *Index) agent.ToolResult {
	dups := detectDuplicateTitles(idx.Entries)
	var b strings.Builder

	b.WriteString(fmt.Sprintf("🔍 重复检测（共 %d 条目）\n\n", len(idx.Entries)))
	if len(dups) == 0 {
		b.WriteString("✅ 未发现重复条目")
		return agent.ToolResult{Content: b.String()}
	}

	b.WriteString(fmt.Sprintf("发现 %d 组可能重复的条目:\n\n", len(dups)))
	for i, g := range dups {
		b.WriteString(fmt.Sprintf("### %d. %s (%d条)\n\n", i+1, g.CanonicalTitle, len(g.Entries)))
		for _, e := range g.Entries {
			absPath := filepath.Join(t.repoPath, e.RelPath)
			b.WriteString(fmt.Sprintf("- `%s` — 分类: %s", absPath, e.Category))
			if e.Summary != "" {
				s := e.Summary
				if len(s) > 60 {
					s = s[:60] + "..."
				}
				b.WriteString(fmt.Sprintf(" | %s", s))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("💡 建议：检查这些条目内容是否重复，如果是则合并。")
	return agent.ToolResult{Content: b.String()}
}

// ── tags ──────────────────────────────────────────────────────────────────

func (t *MaintainTool) runTagAnalysis(idx *Index) agent.ToolResult {
	tagStats := TagOverview(idx)
	clusters := detectTagClusters(idx.Entries)
	var b strings.Builder

	b.WriteString(fmt.Sprintf("🏷️ 标签分析（共 %d 个标签）\n\n", len(tagStats)))

	// ── Top tags ──
	b.WriteString("### 使用频率 Top 20\n\n")
	b.WriteString("| 标签 | 使用次数 |\n|------|----------|\n")
	limit := 20
	if len(tagStats) < limit {
		limit = len(tagStats)
	}
	for i := 0; i < limit; i++ {
		b.WriteString(fmt.Sprintf("| %s | %d |\n", tagStats[i].Name, tagStats[i].Count))
	}
	b.WriteString("\n")

	// ── Tag clusters ──
	if len(clusters) > 0 {
		b.WriteString("### ⚠️ 标签不一致\n\n")
		b.WriteString("以下标签可能指同一概念，建议统一:\n\n")
		for i, c := range clusters {
			b.WriteString(fmt.Sprintf("%d. **%s** ← %s (%d条)\n",
				i+1, c.Canonical, strings.Join(c.Variants, " / "), c.Count))
		}
	} else {
		b.WriteString("✅ 标签命名一致\n")
	}

	return agent.ToolResult{Content: b.String()}
}

// ── stats ─────────────────────────────────────────────────────────────────

func (t *MaintainTool) runStats(idx *Index) agent.ToolResult {
	catStats := CategoryOverview(idx)
	tagStats := TagOverview(idx)
	var b strings.Builder

	b.WriteString(fmt.Sprintf("📊 知识库统计（共 %d 条目）\n\n", len(idx.Entries)))

	// ── By category ──
	b.WriteString("### 分类分布\n\n")
	b.WriteString("| 分类 | 数量 |\n|------|------|\n")
	for _, c := range catStats {
		b.WriteString(fmt.Sprintf("| %s | %d |\n", c.Name, c.Count))
	}

	// ── Top tags ──
	b.WriteString("\n### 热门标签 Top 10\n\n")
	b.WriteString("| 标签 | 使用次数 |\n|------|----------|\n")
	limit := 10
	if len(tagStats) < limit {
		limit = len(tagStats)
	}
	for i := 0; i < limit; i++ {
		b.WriteString(fmt.Sprintf("| %s | %d |\n", tagStats[i].Name, tagStats[i].Count))
	}

	return agent.ToolResult{Content: b.String()}
}
