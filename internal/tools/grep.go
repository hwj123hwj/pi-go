package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/earendil-works/pi-go/internal/agent"
)

// GrepTool 在文件中搜索正则表达式模式。
// 支持递归目录搜索，返回匹配的文件路径、行号和行内容。
type GrepTool struct{}

type GrepParams struct {
	Pattern     string `json:"pattern"`
	Path        string `json:"path"`
	Include     string `json:"include,omitempty"`     // glob 模式过滤文件名（如 "*.go"）
	IgnoreCase  bool   `json:"ignore_case,omitempty"` // 忽略大小写
	MaxResults  int    `json:"max_results,omitempty"` // 最大结果数
	ShowContext int    `json:"show_context,omitempty"` // 显示匹配行上下文行数
}

type grepMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

func NewGrepTool() *GrepTool { return &GrepTool{} }

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Description() string {
	return "Search file contents using regex patterns. Supports recursive directory search and file glob filtering."
}

func (t *GrepTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern":      map[string]any{"type": "string", "description": "The regex pattern to search for."},
			"path":         map[string]any{"type": "string", "description": "File or directory path to search in."},
			"include":      map[string]any{"type": "string", "description": "Glob pattern to filter files (e.g. *.go, *.ts)."},
			"ignore_case":  map[string]any{"type": "boolean", "description": "Case-insensitive search."},
			"max_results":  map[string]any{"type": "integer", "description": "Maximum number of results (default 100)."},
			"show_context": map[string]any{"type": "integer", "description": "Number of context lines around matches."},
		},
		"required": []string{"pattern", "path"},
	}
}

func (t *GrepTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params GrepParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if params.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	// 验证 pattern 是合法正则
	flags := ""
	if params.IgnoreCase {
		flags = "(?i)"
	}
	if _, err := regexp.Compile(flags + params.Pattern); err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}
	if params.MaxResults <= 0 {
		params.MaxResults = 100
	}
	return json.Marshal(params)
}

func (t *GrepTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	var params GrepParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	flags := ""
	if params.IgnoreCase {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + params.Pattern)
	if err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	searchPath := filepath.Clean(params.Path)

	// 判断是文件还是目录
	info, err := os.Stat(searchPath)
	if err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	var matches []grepMatch

	if info.IsDir() {
		matches = t.searchDir(ctx, re, searchPath, params, params.MaxResults)
	} else {
		matches = t.searchFile(ctx, re, searchPath, params, params.MaxResults)
	}

	if len(matches) == 0 {
		return agent.ToolResult{Content: "no matches found"}, nil
	}

	// 格式化输出
	var b strings.Builder
	for _, m := range matches {
		if params.ShowContext > 0 {
			b.WriteString(fmt.Sprintf("%s:%d: %s\n", m.File, m.Line, m.Content))
		} else {
			b.WriteString(fmt.Sprintf("%s:%d: %s\n", m.File, m.Line, m.Content))
		}
	}
	b.WriteString(fmt.Sprintf("\n%d matches found.", len(matches)))

	return agent.ToolResult{Content: b.String()}, nil
}

func (t *GrepTool) searchDir(ctx context.Context, re *regexp.Regexp, root string, params GrepParams, maxResults int) []grepMatch {
	var matches []grepMatch

	// 构建文件名过滤 pattern
	var includeRe *regexp.Regexp
	if params.Include != "" {
		// 将 glob 转为简单的正则
		globPattern := globToRegex(params.Include)
		includeRe = regexp.MustCompile(globPattern)
	}

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 跳过隐藏目录和常见的非搜索目录
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// 文件名过滤
		if includeRe != nil && !includeRe.MatchString(d.Name()) {
			return nil
		}

		fileMatches := t.searchFile(ctx, re, path, params, maxResults-len(matches))
		matches = append(matches, fileMatches...)

		if len(matches) >= maxResults {
			return fmt.Errorf("max results reached")
		}
		return nil
	})

	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}
	return matches
}

func (t *GrepTool) searchFile(ctx context.Context, re *regexp.Regexp, path string, params GrepParams, maxResults int) []grepMatch {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var matches []grepMatch
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if re.MatchString(line) {
			// 截断过长的行
			content := line
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			matches = append(matches, grepMatch{
				File:    path,
				Line:    lineNum,
				Content: strings.TrimSpace(content),
			})
			if len(matches) >= maxResults {
				break
			}
		}

		select {
		case <-ctx.Done():
			return matches
		default:
		}
	}

	return matches
}

// globToRegex 将简单的 glob 模式转换为正则表达式。
// 支持 * (匹配非路径分隔符) 和 ? (单字符)。
func globToRegex(glob string) string {
	var b strings.Builder
	b.WriteString("^")
	for _, ch := range glob {
		switch ch {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '.', '(', ')', '+', '|', '^', '$', '@', '%', '[', ']', '{', '}':
			b.WriteRune('\\')
			b.WriteRune(ch)
		default:
			b.WriteRune(ch)
		}
	}
	b.WriteString("$")
	return b.String()
}
