package ui

import (
	"fmt"
	"strings"
)

// RenderMarkdown renders markdown text for terminal display
func RenderMarkdown(text string) string {
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	var result strings.Builder
	inCodeBlock := false
	codeLang := ""
	inTable := false
	tableRows := []string{}

	for i, line := range lines {
		// Code block start/end
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End code block
				result.WriteString(ColorGray)
				result.WriteString("└─")
				result.WriteString(ColorReset)
				result.WriteString("\n")
				inCodeBlock = false
				continue
			}
			// Start code block
			inCodeBlock = true
			codeLang = strings.TrimPrefix(line, "```")
			codeLang = strings.TrimSpace(codeLang)
			result.WriteString(ColorCyan)
			if codeLang != "" {
				result.WriteString(fmt.Sprintf("┌─ %s ─", codeLang))
			} else {
				result.WriteString("┌─")
			}
			result.WriteString(ColorReset)
			result.WriteString("\n")
			continue
		}

		if inCodeBlock {
			// Inside code block - render with line prefix
			result.WriteString(ColorGray)
			result.WriteString("│ ")
			result.WriteString(ColorReset)
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// Table handling
		if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
			if !inTable {
				inTable = true
				tableRows = []string{}
			}
			tableRows = append(tableRows, line)
			continue
		} else if inTable {
			// End of table - render it
			result.WriteString(renderTerminalTable(tableRows))
			inTable = false
			tableRows = []string{}
		}

		// Headers
		if strings.HasPrefix(line, "# ") {
			result.WriteString(ColorBold)
			result.WriteString(ColorCyan)
			result.WriteString(strings.TrimPrefix(line, "# "))
			result.WriteString(ColorReset)
			result.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			result.WriteString(ColorBold)
			result.WriteString(strings.TrimPrefix(line, "## "))
			result.WriteString(ColorReset)
			result.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "### ") {
			result.WriteString(ColorBold)
			result.WriteString(strings.TrimPrefix(line, "### "))
			result.WriteString(ColorReset)
			result.WriteString("\n")
			continue
		}

		// Blockquote
		if strings.HasPrefix(line, "> ") {
			result.WriteString(ColorGray)
			result.WriteString("│ ")
			result.WriteString(ColorReset)
			result.WriteString(renderInlineMarkdown(strings.TrimPrefix(line, "> ")))
			result.WriteString("\n")
			continue
		}

		// Unordered list
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			result.WriteString(ColorCyan)
			result.WriteString("  • ")
			result.WriteString(ColorReset)
			result.WriteString(renderInlineMarkdown(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")))
			result.WriteString("\n")
			continue
		}

		// Ordered list (simple check)
		if len(line) > 2 && line[0] >= '0' && line[0] <= '9' && strings.Contains(line, ". ") {
			parts := strings.SplitN(line, ". ", 2)
			if len(parts) == 2 {
				result.WriteString(ColorCyan)
				result.WriteString(fmt.Sprintf("  %s. ", parts[0]))
				result.WriteString(ColorReset)
				result.WriteString(renderInlineMarkdown(parts[1]))
				result.WriteString("\n")
				continue
			}
		}

		// Horizontal rule
		if strings.TrimSpace(line) == "---" || strings.TrimSpace(line) == "***" || strings.TrimSpace(line) == "___" {
			result.WriteString(ColorGray)
			result.WriteString(strings.Repeat("─", 60))
			result.WriteString(ColorReset)
			result.WriteString("\n")
			continue
		}

		// Empty line
		if strings.TrimSpace(line) == "" {
			// Add empty line only if next line is not empty
			if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" {
				result.WriteString("\n")
			}
			continue
		}

		// Regular text
		result.WriteString(renderInlineMarkdown(line))
		result.WriteString("\n")
	}

	// Close any open table
	if inTable && len(tableRows) > 0 {
		result.WriteString(renderTerminalTable(tableRows))
	}

	return result.String()
}

// renderInlineMarkdown renders inline markdown elements
func renderInlineMarkdown(text string) string {
	// Bold + italic (***text***)
	text = replacePattern(text, `\*\*\*(.+?)\*\*\*`, func(match string) string {
		inner := match[3 : len(match)-3]
		return ColorBold + ColorCyan + inner + ColorReset
	})

	// Bold (**text**)
	text = replacePattern(text, `\*\*(.+?)\*\*`, func(match string) string {
		inner := match[2 : len(match)-2]
		return ColorBold + inner + ColorReset
	})

	// Italic (*text*)
	text = replacePattern(text, `\*(.+?)\*`, func(match string) string {
		inner := match[1 : len(match)-1]
		return ColorCyan + inner + ColorReset
	})

	// Inline code (`code`)
	text = replacePattern(text, "`([^`]+)`", func(match string) string {
		inner := match[1 : len(match)-1]
		return ColorYellow + inner + ColorReset
	})

	// Links [text](url) - show as text
	text = replacePattern(text, `\[([^\]]+)\]\([^)]+\)`, func(match string) string {
		// Extract text between [ and ]
		start := strings.Index(match, "[")
		end := strings.Index(match, "]")
		if start >= 0 && end > start {
			return ColorBlue + match[start+1:end] + ColorReset
		}
		return match
	})

	// Strikethrough (~~text~~)
	text = replacePattern(text, `~~(.+?)~~`, func(match string) string {
		inner := match[2 : len(match)-2]
		return ColorGray + inner + ColorReset
	})

	return text
}

// replacePattern replaces patterns with a transformation function
func replacePattern(text, pattern string, transform func(string) string) string {
	// Simple implementation without regex for performance
	// This handles the most common cases
	result := text

	// Bold + italic
	for {
		start := strings.Index(result, "***")
		if start == -1 {
			break
		}
		end := strings.Index(result[start+3:], "***")
		if end == -1 {
			break
		}
		end += start + 3
		inner := result[start+3 : end]
		replacement := ColorBold + ColorCyan + inner + ColorReset
		result = result[:start] + replacement + result[end+3:]
	}

	// Bold
	for {
		start := strings.Index(result, "**")
		if start == -1 {
			break
		}
		end := strings.Index(result[start+2:], "**")
		if end == -1 {
			break
		}
		end += start + 2
		inner := result[start+2 : end]
		replacement := ColorBold + inner + ColorReset
		result = result[:start] + replacement + result[end+2:]
	}

	// Italic (single *)
	for {
		start := strings.Index(result, "*")
		if start == -1 {
			break
		}
		// Skip if it's part of ** or ***
		if start+1 < len(result) && (result[start+1] == '*') {
			start++
			continue
		}
		end := strings.Index(result[start+1:], "*")
		if end == -1 {
			break
		}
		end += start + 1
		inner := result[start+1 : end]
		replacement := ColorCyan + inner + ColorReset
		result = result[:start] + replacement + result[end+1:]
	}

	// Inline code
	for {
		start := strings.Index(result, "`")
		if start == -1 {
			break
		}
		end := strings.Index(result[start+1:], "`")
		if end == -1 {
			break
		}
		end += start + 1
		inner := result[start+1 : end]
		replacement := ColorYellow + inner + ColorReset
		result = result[:start] + replacement + result[end+1:]
	}

	return result
}

// renderTerminalTable renders a table with box-drawing characters
func renderTerminalTable(rows []string) string {
	if len(rows) == 0 {
		return ""
	}

	// Parse table rows
	var header []string
	var dataRows [][]string
	var alignments []string

	for i, row := range rows {
		// Split by | and trim
		cells := strings.Split(row, "|")
		// Remove empty first and last elements
		if len(cells) > 0 && cells[0] == "" {
			cells = cells[1:]
		}
		if len(cells) > 0 && cells[len(cells)-1] == "" {
			cells = cells[:len(cells)-1]
		}

		// Trim whitespace from each cell
		for j := range cells {
			cells[j] = strings.TrimSpace(cells[j])
		}

		if i == 0 {
			header = cells
		} else if i == 1 {
			// Alignment row (e.g., |---|---|)
			alignments = make([]string, len(cells))
			for j, cell := range cells {
				if strings.HasPrefix(cell, ":") && strings.HasSuffix(cell, ":") {
					alignments[j] = "center"
				} else if strings.HasSuffix(cell, ":") {
					alignments[j] = "right"
				} else {
					alignments[j] = "left"
				}
			}
		} else {
			dataRows = append(dataRows, cells)
		}
	}

	// Calculate column widths (considering Chinese characters as double-width)
	colWidths := make([]int, len(header))
	for i, h := range header {
		colWidths[i] = getStringWidth(h)
	}
	for _, row := range dataRows {
		for i, cell := range row {
			if i < len(colWidths) {
				w := getStringWidth(cell)
				if w > colWidths[i] {
					colWidths[i] = w
				}
			}
		}
	}

	// Ensure minimum width
	for i := range colWidths {
		if colWidths[i] < 4 {
			colWidths[i] = 4
		}
	}

	var result strings.Builder

	// Top border
	result.WriteString(ColorGray)
	result.WriteString("┌")
	for i, w := range colWidths {
		result.WriteString(strings.Repeat("─", w+2))
		if i < len(colWidths)-1 {
			result.WriteString("┬")
		}
	}
	result.WriteString("┐")
	result.WriteString(ColorReset)
	result.WriteString("\n")

	// Header
	result.WriteString(ColorGray)
	result.WriteString("│")
	result.WriteString(ColorReset)
	for i, h := range header {
		result.WriteString(ColorBold)
		result.WriteString(" ")
		result.WriteString(h)
		result.WriteString(strings.Repeat(" ", colWidths[i]-getStringWidth(h)))
		result.WriteString(" ")
		result.WriteString(ColorReset)
		result.WriteString(ColorGray)
		result.WriteString("│")
		result.WriteString(ColorReset)
	}
	result.WriteString("\n")

	// Header separator
	result.WriteString(ColorGray)
	result.WriteString("├")
	for i, w := range colWidths {
		result.WriteString(strings.Repeat("─", w+2))
		if i < len(colWidths)-1 {
			result.WriteString("┼")
		}
	}
	result.WriteString("┤")
	result.WriteString(ColorReset)
	result.WriteString("\n")

	// Data rows
	for _, row := range dataRows {
		result.WriteString(ColorGray)
		result.WriteString("│")
		result.WriteString(ColorReset)
		for i, cell := range row {
			if i < len(colWidths) {
				// Apply alignment
				align := "left"
				if i < len(alignments) {
					align = alignments[i]
				}

				cellWidth := getStringWidth(cell)
				padding := colWidths[i] - cellWidth

				result.WriteString(" ")
				switch align {
				case "right":
					result.WriteString(strings.Repeat(" ", padding))
					result.WriteString(cell)
				case "center":
					leftPad := padding / 2
					rightPad := padding - leftPad
					result.WriteString(strings.Repeat(" ", leftPad))
					result.WriteString(cell)
					result.WriteString(strings.Repeat(" ", rightPad))
				default:
					result.WriteString(cell)
					result.WriteString(strings.Repeat(" ", padding))
				}
				result.WriteString(" ")
			}
			result.WriteString(ColorGray)
			result.WriteString("│")
			result.WriteString(ColorReset)
		}
		result.WriteString("\n")
	}

	// Bottom border
	result.WriteString(ColorGray)
	result.WriteString("└")
	for i, w := range colWidths {
		result.WriteString(strings.Repeat("─", w+2))
		if i < len(colWidths)-1 {
			result.WriteString("┴")
		}
	}
	result.WriteString("┘")
	result.WriteString(ColorReset)

	return result.String()
}

// getStringWidth returns the display width of a string, counting CJK characters and emoji as 2
func getStringWidth(s string) int {
	width := 0
	for _, r := range s {
		if isWideChar(r) {
			width += 2
		} else {
			width++
		}
	}
	return width
}

// isWideChar checks if a rune occupies 2 columns in the terminal (CJK + emoji + symbols)
func isWideChar(r rune) bool {
	// Common emoji and symbols (East Asian Wide / Fullwidth / Emoji)
	switch {
	case r >= 0x1F300 && r <= 0x1F6FF: // Emoji & pictographs
		return true
	case r >= 0x1F900 && r <= 0x1F9FF: // Supplemental Symbols and Pictographs
		return true
	case r >= 0x2600 && r <= 0x27BF: // Miscellaneous Symbols and Dingbats (✅🔄 etc)
		return true
	case r >= 0x2300 && r <= 0x23FF: // Miscellaneous Technical (⚡)
		return true
	case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified Ideographs
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK Extension A
		return true
	case r >= 0x20000 && r <= 0x2A6DF: // CJK Extension B
		return true
	case r >= 0x2A700 && r <= 0x2B73F: // CJK Extension C
		return true
	case r >= 0x2B740 && r <= 0x2B81F: // CJK Extension D
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK Compatibility Ideographs
		return true
	case r >= 0x2F800 && r <= 0x2FA1F: // CJK Compatibility Ideographs Supplement
		return true
	case r >= 0x3000 && r <= 0x303F: // CJK Symbols and Punctuation
		return true
	case r >= 0xFF00 && r <= 0xFFEF: // Fullwidth Forms
		return true
	case r >= 0x1100 && r <= 0x115F: // Hangul Jamo
		return true
	case r >= 0xAC00 && r <= 0xD7A3: // Hangul Syllables
		return true
	case r >= 0x3040 && r <= 0x309F: // Hiragana
		return true
	case r >= 0x30A0 && r <= 0x30FF: // Katakana
		return true
	}

	// Variation selectors and zero-width joiners shouldn't count
	if r == 0xFE0F || r == 0x200D || (r >= 0xFE00 && r <= 0xFE0F) {
		return false
	}

	return false
}

// isCJK kept for backwards compatibility
func isCJK(r rune) bool {
	return isWideChar(r)
}
