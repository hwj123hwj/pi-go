package kbtools

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hwj123hwj/pi-go/internal/agent"
)

// SaveTool writes a new knowledge entry to the repository.
// This is the "second brain" core capability — the AI can now persist
// what it learns, not just retrieve.
type SaveTool struct {
	repoPath string
}

func NewSaveTool(repoPath string) *SaveTool {
	return &SaveTool{repoPath: repoPath}
}

func (t *SaveTool) Name() string { return "kb_save" }
func (t *SaveTool) Description() string {
	return `将一条新知识保存到知识库。这是你的"第二大脑"写入能力。

当用户说"记住这个"、"存到知识库"、"记录一下"、"这个值得记录"时使用。
也可以在解决了有意思的问题后主动使用（先征求用户同意）。

条目保存为 Markdown 文件，支持 YAML frontmatter（title, tags, category, date）。
保存路径规则：{repo}/issues/ 或 {repo}/{category}/{slug}.md

保存成功后返回文件路径，用户可以用 kb_read 验证或 kb_search 重新搜索。`
}
func (t *SaveTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "条目标题（简洁概括，不超过50字）",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "条目正文（Markdown 格式，自由发挥，详细描述知识/问题/方案）",
			},
			"tags": map[string]any{
				"type":        "array",
				"description": "标签列表（如 [\"Go\", \"并发\", \"踩坑\"]）",
				"items":       map[string]any{"type": "string"},
			},
			"category": map[string]any{
				"type":        "string",
				"description": "分类（决定存储目录）。常用：issues（踩坑记录）、tech、work、english、writing、life。默认 issues",
			},
		},
		"required": []string{"title", "content"},
	}
}

func (t *SaveTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if p.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if p.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	return params, nil
}

func (t *SaveTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Tags     []string `json:"tags"`
		Category string   `json:"category"`
	}
	_ = json.Unmarshal(params, &p)

	// Default category
	category := p.Category
	if category == "" {
		category = "issues"
	}

	// Determine target directory
	targetDir := filepath.Join(t.repoPath, category)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("无法创建目录 %s: %v", targetDir, err),
			IsError: true,
		}, nil
	}

	// Generate filename: YYYY-MM-DD-slug.md
	dateStr := time.Now().Format("2006-01-02")
	slug := slugify(p.Title)
	if slug == "" {
		slug = hashSlug(p.Title)
	}
	// Limit slug length
	if len(slug) > 60 {
		slug = slug[:60]
	}
	filename := fmt.Sprintf("%s-%s.md", dateStr, slug)
	targetPath := filepath.Join(targetDir, filename)

	// Avoid overwriting if file already exists (append -2, -3, ...)
	if _, err := os.Stat(targetPath); err == nil {
		for i := 2; ; i++ {
			filename = fmt.Sprintf("%s-%s-%d.md", dateStr, slug, i)
			targetPath = filepath.Join(targetDir, filename)
			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				break
			}
		}
	}

	// Build markdown content with YAML frontmatter
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: %q\n", p.Title))
	b.WriteString(fmt.Sprintf("date: %s\n", dateStr))
	b.WriteString(fmt.Sprintf("category: %s\n", category))
	if len(p.Tags) > 0 {
		tagStrs := make([]string, len(p.Tags))
		for i, tag := range p.Tags {
			tagStrs[i] = fmt.Sprintf("%q", tag)
		}
		b.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(tagStrs, ", ")))
	}
	b.WriteString("---\n\n")
	b.WriteString(fmt.Sprintf("# %s\n\n", p.Title))
	b.WriteString(p.Content)
	b.WriteString("\n")

	if err := os.WriteFile(targetPath, []byte(b.String()), 0644); err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("写入失败: %v", err),
			IsError: true,
		}, nil
	}

	// Invalidate cache so next search includes the new entry
	ClearCache()

	var b2 strings.Builder
	b2.WriteString(fmt.Sprintf("✅ 已保存到知识库！\n\n"))
	b2.WriteString(fmt.Sprintf("📄 `%s`\n\n", targetPath))
	b2.WriteString(fmt.Sprintf("**标题**: %s\n", p.Title))
	b2.WriteString(fmt.Sprintf("**分类**: %s\n", category))
	if len(p.Tags) > 0 {
		b2.WriteString(fmt.Sprintf("**标签**: %s\n", strings.Join(p.Tags, ", ")))
	}
	b2.WriteString(fmt.Sprintf("\n可以用 `kb_read path=\"%s\"` 验证，或用 `kb_search query=\"%s\"` 重新搜索。", targetPath, p.Title))

	return agent.ToolResult{
		Content:    b2.String(),
		UserFacing: fmt.Sprintf("已保存知识条目「%s」到 %s", p.Title, targetPath),
	}, nil
}

// slugify converts a title to a URL/filename-safe slug.
// For ASCII titles (English), keeps alphanumerics and hyphens.
// For non-ASCII titles (Chinese etc.), returns empty so caller can use hashSlug.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	var b strings.Builder
	hasASCII := false
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			b.WriteRune(c)
			hasASCII = true
		}
	}
	result := b.String()
	// Collapse multiple hyphens
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	result = strings.Trim(result, "-")

	// If the original title had no meaningful ASCII content, return empty
	if !hasASCII || result == "" {
		return ""
	}
	return result
}

// hashSlug creates a short hash-based slug for non-ASCII titles.
func hashSlug(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])[:8]
}
