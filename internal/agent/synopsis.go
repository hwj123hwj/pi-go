package agent

// ────────────────────────────────────────────────────────────────────────────
//  Tool Output Synopsis
//
//  Adapted from OpenViking's tool_result_synopsis.py.
//  When a tool returns a large output (over threshold chars), the After hook
//  automatically generates a deterministic synopsis (structure + excerpt) and
//  replaces the LLM-facing Content with it. The full output is preserved in
//  UserFacing (for the UI to display).
//
//  This prevents large tool outputs from consuming context window tokens.
//  The synopsis gives the LLM enough information to decide if it needs to
//  re-read the full output (e.g. via kb_read without overview mode).
//
//  Key differences from OpenViking:
//  1. No external storage — synopsis replaces Content in-place, full text
//     stays in UserFacing (OpenViking stores to VikingFS with a ref)
//  2. Pure rules-based — no LLM call needed for synopsis generation
//  3. Content-type aware: code gets imports/symbols, text gets headers/excerpt
// ────────────────────────────────────────────────────────────────────────────

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const (
	synopsisThreshold = 4000 // chars — outputs larger than this get a synopsis
	maxExcerptChars   = 600
	maxHeadersShown   = 10
	maxCodeSymbols    = 15
)

// SynopsisAfterHook is an AfterToolCallHook that auto-synopsizes large outputs.
// Usage:
//
//	hooks.After = append(hooks.After, SynopsisAfterHook)
func SynopsisAfterHook(_ context.Context, _ ToolCallContext, result ToolResult) (ToolResult, error) {
	if len(result.Content) <= synopsisThreshold {
		return result, nil // small enough, no synopsis needed
	}

	// Preserve full output for the UI (only if not already set by the tool)
	if result.UserFacing == "" {
		result.UserFacing = result.Content
	}

	// Generate synopsis and replace LLM-facing content
	synopsis := GenerateSynopsis(result.Content)
	result.Content = synopsis

	return result, nil
}

// GenerateSynopsis produces a deterministic synopsis of a large text output.
// It detects content type (code, markdown, JSON, plain text) and extracts
// the most relevant structural information.
func GenerateSynopsis(content string) string {
	var b strings.Builder

	originalLen := len(content)
	b.WriteString(fmt.Sprintf("📋 [输出概览] 原始内容 %d 字符\n\n", originalLen))

	// Detect content type and extract structure
	lines := strings.Split(content, "\n")

	switch detectContentType(content, lines) {
	case "code":
		b.WriteString(summarizeCode(lines))
	case "markdown":
		b.WriteString(summarizeMarkdown(lines))
	case "json":
		b.WriteString(summarizeJSON(lines))
	default:
		b.WriteString(summarizeText(content, lines))
	}

	// Always include head + tail excerpt
	b.WriteString("\n📎 开头摘录:\n")
	b.WriteString(truncateToFirstN(content, maxExcerptChars/2))
	b.WriteString("\n\n📎 结尾摘录:\n")
	b.WriteString(truncateToLastN(content, maxExcerptChars/2))

	b.WriteString(fmt.Sprintf("\n\n💡 完整内容已对用户可见（%d 字符）。如需查看完整内容请告知。", originalLen))

	return b.String()
}

// detectContentType guesses the content type from text patterns.
func detectContentType(content string, lines []string) string {
	// Check for code patterns
	codeRegexes := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*(func |func\(|package |import |var |const |type )`),
		regexp.MustCompile(`(?m)^\s*(class |def |import |from .* import|public |private )`),
		regexp.MustCompile(`(?m)^\s*(fn |pub fn|use |mod )`),
	}
	for _, r := range codeRegexes {
		if r.MatchString(content) {
			return "code"
		}
	}

	// Check for JSON
	trimmed := strings.TrimSpace(content)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		return "json"
	}

	// Check for markdown
	headerCount := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			headerCount++
		}
	}
	if headerCount >= 3 {
		return "markdown"
	}

	return "text"
}

// summarizeCode extracts imports and function/class definitions.
func summarizeCode(lines []string) string {
	var b strings.Builder
	b.WriteString("📦 代码结构:\n")

	symbolRegex := regexp.MustCompile(`^\s*(func\s+|func\(|class\s+|def\s+|async\s+def\s+|fn\s+|pub\s+fn|public\s+class|public\s+static|type\s+)\s*(\w+)`)
	importRegex := regexp.MustCompile(`^\s*(import\s+|from\s+|use\s+|#include|require\()`)

	symbols := 0
	imports := 0
	for _, line := range lines {
		if importRegex.MatchString(line) && imports < 8 {
			b.WriteString(fmt.Sprintf("  📥 %s\n", strings.TrimSpace(line)))
			imports++
			continue // don't double-count an import line as a symbol
		}
		if m := symbolRegex.FindStringSubmatch(line); m != nil {
			b.WriteString(fmt.Sprintf("  • %s\n", strings.TrimSpace(m[0])))
			symbols++
			if symbols >= maxCodeSymbols {
				b.WriteString("  ...(更多符号省略)\n")
				break
			}
		}
	}

	b.WriteString(fmt.Sprintf("\n  共 %d 行\n", len(lines)))
	return b.String()
}

// summarizeMarkdown extracts headers and section structure.
func summarizeMarkdown(lines []string) string {
	var b strings.Builder
	b.WriteString("📑 文档结构:\n")

	headers := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			b.WriteString(fmt.Sprintf("  %s\n", trimmed))
			headers++
			if headers >= maxHeadersShown {
				b.WriteString("  ...(更多标题省略)\n")
				break
			}
		}
	}

	b.WriteString(fmt.Sprintf("\n  共 %d 行\n", len(lines)))
	return b.String()
}

// summarizeJSON extracts top-level keys and structure.
func summarizeJSON(lines []string) string {
	var b strings.Builder
	b.WriteString("🔧 JSON 结构:\n")

	// Simple approach: find top-level keys (lines with low indentation)
	keyRegex := regexp.MustCompile(`^[\{\[]?\s*"([^"]+)"\s*:`)

	keys := make(map[string]bool)
	for _, line := range lines {
		if m := keyRegex.FindStringSubmatch(line); m != nil {
			if !keys[m[1]] {
				b.WriteString(fmt.Sprintf("  • %s\n", m[1]))
				keys[m[1]] = true
				if len(keys) >= 15 {
					break
				}
			}
		}
	}

	b.WriteString(fmt.Sprintf("\n  共 %d 行\n", len(lines)))
	return b.String()
}

// summarizeText extracts statistics for plain text.
func summarizeText(content string, lines []string) string {
	var b strings.Builder
	b.WriteString("📝 文本概要:\n")
	b.WriteString(fmt.Sprintf("  • %d 行\n", len(lines)))
	b.WriteString(fmt.Sprintf("  • %d 字符\n", len(content)))
	b.WriteString(fmt.Sprintf("  • %d 词\n", len(strings.Fields(content))))
	return b.String()
}

// truncateToFirstN returns the first n runes, with "..." if truncated.
// Rune-safe: does not split multi-byte UTF-8 characters.
func truncateToFirstN(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

// truncateToLastN returns the last n runes, with "..." if truncated.
// Rune-safe: does not split multi-byte UTF-8 characters.
func truncateToLastN(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return "..." + string(runes[len(runes)-n:])
}
