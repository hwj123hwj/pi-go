package feishu

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// mdToPostContent converts Markdown text to Feishu post message content JSON.
// Supports: headings, bold, inline code, code blocks, links, lists, tables.
func mdToPostContent(md string) string {
	var blocks [][]map[string]any
	lines := strings.Split(md, "\n")

	var inCodeBlock bool
	var codeLines []string

	for _, line := range lines {
		// Code block toggle
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End code block
				inCodeBlock = false
				codeText := strings.Join(codeLines, "\n")
				codeLines = nil
				blocks = append(blocks, []map[string]any{
					{"tag": "text", "text": codeText + "\n"},
				})
				continue
			}
			inCodeBlock = true
			codeLines = nil
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Headings: # ... ######
		if strings.HasPrefix(line, "#") {
			level := 0
			for _, c := range line {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			text := strings.TrimSpace(line[level:])
			blocks = append(blocks, []map[string]any{
				{"tag": "text", "text": strings.Repeat("#", level) + " "},
				{"tag": "text", "text": text, "style": []string{"bold"}},
			})
			continue
		}

		// Table rows
		if strings.Contains(line, "|") && strings.Contains(line, "|") {
			cells := parseTableRow(line)
			if cells != nil {
				var row []map[string]any
				for _, cell := range cells {
					row = append(row, map[string]any{"tag": "text", "text": cell + "  "})
				}
				blocks = append(blocks, row)
				continue
			}
		}

		// List items: - or * or numbered
		if strings.HasPrefix(strings.TrimSpace(line), "- ") || strings.HasPrefix(strings.TrimSpace(line), "* ") {
			text := strings.TrimSpace(line)[2:]
			blocks = append(blocks, []map[string]any{
				{"tag": "text", "text": "• "},
			})
			blocks = append(blocks, parseInlineMarkdown(text)...)
			continue
		}
		if isNumberedList(line) {
			prefix := parseNumberedPrefix(line)
			text := strings.TrimSpace(line[len(prefix):])
			blocks = append(blocks, []map[string]any{
				{"tag": "text", "text": prefix + " "},
			})
			blocks = append(blocks, parseInlineMarkdown(text)...)
			continue
		}

		// Regular line: parse inline markdown
		blocks = append(blocks, parseInlineMarkdown(line)...)
	}

	// Build post content JSON
	post := map[string]any{
		"zh_cn": map[string]any{
			"title":   "",
			"content": blocks,
		},
	}

	data, _ := json.Marshal(post)
	return string(data)
}

// parseInlineMarkdown converts a line with inline markdown to post content blocks.
func parseInlineMarkdown(line string) [][]map[string]any {
	var result [][]map[string]any
	remaining := line

	for len(remaining) > 0 {
		// Bold: **text**
		if idx := strings.Index(remaining, "**"); idx >= 0 {
			if idx > 0 {
				result = append(result, plainBlock(remaining[:idx]))
			}
			remaining = remaining[idx+2:]
			end := strings.Index(remaining, "**")
			if end >= 0 {
				result = append(result, []map[string]any{
					{"tag": "text", "text": remaining[:end], "style": []string{"bold"}},
				})
				remaining = remaining[end+2:]
			} else {
				result = append(result, plainBlock("**"+remaining))
				remaining = ""
			}
			continue
		}

		// Inline code: `text`
		if idx := strings.Index(remaining, "`"); idx >= 0 {
			if idx > 0 {
				result = append(result, plainBlock(remaining[:idx]))
			}
			remaining = remaining[idx+1:]
			end := strings.Index(remaining, "`")
			if end >= 0 {
				result = append(result, []map[string]any{
					{"tag": "text", "text": remaining[:end], "style": []string{"background"}},
				})
				remaining = remaining[end+1:]
			} else {
				result = append(result, plainBlock("`"+remaining))
				remaining = ""
			}
			continue
		}

		// Link: [text](url)
		linkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
		if match := linkRe.FindStringIndex(remaining); match != nil {
			if match[0] > 0 {
				result = append(result, plainBlock(remaining[:match[0]]))
			}
			submatch := linkRe.FindStringSubmatch(remaining[match[0]:match[1]])
			result = append(result, []map[string]any{
				{"tag": "a", "text": submatch[1], "href": submatch[2]},
			})
			remaining = remaining[match[1]:]
			continue
		}

		// No more inline formatting: emit rest as plain text
		result = append(result, plainBlock(remaining+"\n"))
		remaining = ""
	}

	if len(result) == 0 {
		result = append(result, plainBlock("\n"))
	}

	return result
}

func plainBlock(text string) []map[string]any {
	return []map[string]any{{"tag": "text", "text": text}}
}

func parseTableRow(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil
	}
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")

	// Skip separator rows like |---|---|
	if isSeparatorRow(line) {
		return nil
	}

	parts := strings.Split(line, "|")
	var cells []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cells = append(cells, p)
	}
	return cells
}

func isSeparatorRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	for _, part := range strings.Split(trimmed, "|") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, c := range part {
			if c != '-' && c != ':' {
				return false
			}
		}
	}
	return true
}

func isNumberedList(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 {
		return false
	}
	for i, c := range trimmed {
		if c == '.' || c == ')' {
			return i > 0 && isDigit(trimmed[:i])
		}
		if c == ' ' {
			return false
		}
	}
	return false
}

func parseNumberedPrefix(line string) string {
	trimmed := strings.TrimSpace(line)
	for i, c := range trimmed {
		if c == '.' || c == ')' {
			return trimmed[:i+1]
		}
	}
	return ""
}

func isDigit(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// Ensure fmt is used
var _ = fmt.Sprintf
