package kbtools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/earendil-works/pi-go/internal/agent"
)

// QueryTool performs cross-module search across the agent-lessons repository.
type QueryTool struct {
	repoPath string
}

func NewQueryTool(repoPath string) *QueryTool {
	return &QueryTool{repoPath: repoPath}
}

func (t *QueryTool) Name() string { return "kb_query" }
func (t *QueryTool) Description() string {
	return "跨模块全文检索知识库。搜索 5 大模块：跨项目知识库、踩坑记录、项目日志、知识卡片、原始对话。返回匹配的文件和相关行。"
}
func (t *QueryTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "搜索关键词（支持多个词，空格分隔）",
			},
			"module": map[string]any{
				"type":        "string",
				"description": "限定搜索模块（可选）：knowledge_base/issues/journals/cards/exports",
				"enum":        []string{"knowledge_base", "issues", "journals", "cards", "exports"},
			},
			"max_files": map[string]any{
				"type":        "number",
				"description": "每个模块最多返回几个文件，默认 3",
			},
		},
		"required": []string{"query"},
	}
}

func (t *QueryTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if p.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	return params, nil
}

type searchResult struct {
	module   string
	file     string
	hitCount int
	lines    []string
}

func (t *QueryTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		Query     string  `json:"query"`
		Module    string  `json:"module"`
		MaxFiles  float64 `json:"max_files"`
	}
	_ = json.Unmarshal(params, &p)

	maxFiles := 3
	if p.MaxFiles > 0 {
		maxFiles = int(p.MaxFiles)
	}

	keywords := strings.Fields(p.Query)
	if len(keywords) == 0 {
		return agent.ToolResult{Content: "请提供搜索关键词"}, nil
	}

	// Define modules to search
	type module struct {
		name string
		dir  string
		desc string
	}
	modules := []module{
		{"跨项目知识库", filepath.Join(t.repoPath, "project-journals", "KNOWLEDGE_BASE.md"), ""},
		{"踩坑记录", filepath.Join(t.repoPath, "issues"), ""},
		{"项目日志", filepath.Join(t.repoPath, "project-journals"), ""},
		{"知识卡片", filepath.Join(t.repoPath, "doubao-knowledge"), ""},
		{"原始对话", filepath.Join(t.repoPath, "doubao-export"), ""},
	}

	// Filter by module if specified
	if p.Module != "" {
		filtered := []module{}
		switch p.Module {
		case "knowledge_base":
			filtered = append(filtered, modules[0])
		case "issues":
			filtered = append(filtered, modules[1])
		case "journals":
			filtered = append(filtered, modules[2])
		case "cards":
			filtered = append(filtered, modules[3])
		case "exports":
			filtered = append(filtered, modules[4])
		}
		modules = filtered
	}

	var allResults []searchResult

	for _, mod := range modules {
		results := t.searchModule(mod.name, mod.dir, keywords, maxFiles)
		allResults = append(allResults, results...)
	}

	if len(allResults) == 0 {
		return agent.ToolResult{Content: "没有找到匹配的内容"}, nil
	}

	// Format output
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🔍 搜索「%s」— 命中 %d 个来源：\n\n", p.Query, len(allResults)))

	for _, r := range allResults {
		b.WriteString(fmt.Sprintf("📄 %s — %s (%d处命中)\n", r.module, r.file, r.hitCount))
		for _, line := range r.lines {
			b.WriteString(fmt.Sprintf("   %s\n", line))
		}
		b.WriteString("\n")
	}

	b.WriteString("使用 kb_read 工具读取完整文件内容。")

	return agent.ToolResult{Content: b.String()}, nil
}

func (t *QueryTool) searchModule(name, dir string, keywords []string, maxFiles int) []searchResult {
	var results []searchResult

	// Check if dir is a single file
	info, err := os.Stat(dir)
	if err != nil {
		return nil
	}

	if !info.IsDir() {
		// Single file search
		hits := t.searchFile(dir, keywords)
		if hits.hitCount > 0 {
			results = append(results, searchResult{
				module:   name,
				file:     dir,
				hitCount: hits.hitCount,
				lines:    hits.lines,
			})
		}
		return results
	}

	// Directory search
	err = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		// Skip INDEX.md files
		if fi.Name() == "INDEX.md" {
			return nil
		}

		hits := t.searchFile(path, keywords)
		if hits.hitCount > 0 {
			relPath, _ := filepath.Rel(t.repoPath, path)
			results = append(results, searchResult{
				module:   name,
				file:     relPath,
				hitCount: hits.hitCount,
				lines:    hits.lines,
			})
		}
		return nil
	})
	if err != nil {
		return nil
	}

	// Sort by hit count, take top N
	sort.Slice(results, func(i, j int) bool {
		return results[i].hitCount > results[j].hitCount
	})
	if len(results) > maxFiles {
		results = results[:maxFiles]
	}

	return results
}

type fileHits struct {
	hitCount int
	lines    []string
}

func (t *QueryTool) searchFile(path string, keywords []string) fileHits {
	f, err := os.Open(path)
	if err != nil {
		return fileHits{}
	}
	defer f.Close()

	var hitCount int
	var matchedLines []string
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lowerLine := strings.ToLower(line)

		matched := true
		for _, kw := range keywords {
			if !strings.Contains(lowerLine, strings.ToLower(kw)) {
				matched = false
				break
			}
		}

		if matched && strings.TrimSpace(line) != "" {
			hitCount++
			if len(matchedLines) < 3 { // keep top 3 matching lines
				// Truncate long lines
				displayLine := line
				if len(displayLine) > 120 {
					displayLine = displayLine[:120] + "..."
				}
				matchedLines = append(matchedLines, fmt.Sprintf("L%d: %s", lineNum, displayLine))
			}
		}
	}

	return fileHits{hitCount: hitCount, lines: matchedLines}
}
