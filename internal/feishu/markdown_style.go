package feishu

import (
	"fmt"
	"regexp"
	"strings"
)

// OptimizeMarkdownStyle adapts markdown for Feishu CardKit display.
// Processing steps:
//  1. Extract code blocks (placeholder protection)
//  2. Heading downgrade: H1→H4, H2~H6→H5 (only when original has H1~H3)
//  3. Add <br> spacing between consecutive headings
//  4. Add <br> around tables
//  5. Restore code blocks with <br> before/after
//  6. Compress 3+ consecutive newlines to 2
//  7. Strip invalid image keys
func OptimizeMarkdownStyle(text string, cardVersion int) string {
	if text == "" {
		return text
	}

	r := optimizeMarkdownStyleInner(text, cardVersion)
	r = StripInvalidImageKeys(r)
	return r
}

func optimizeMarkdownStyleInner(text string, cardVersion int) string {
	// ── 1. Extract code blocks, protect with placeholders ──
	const mark = "___CB_"
	var codeBlocks []string

	// Go's regexp doesn't support backreferences, so we use line scanning.
	lines := strings.Split(text, "\n")
	var result []string
	var blockLines []string
	inBlock := false
	fenceChar := byte('`')
	fenceLen := 0

	for _, line := range lines {
		if !inBlock {
			trimmed := strings.TrimLeft(line, " ")
			if len(trimmed) >= 3 && trimmed[0] == '`' {
				// Check if it's a fence opening
				n := 0
				for n < len(trimmed) && trimmed[n] == '`' {
					n++
				}
				if n >= 3 {
					inBlock = true
					fenceChar = '`'
					fenceLen = n
					blockLines = []string{line}
					continue
				}
			}
			result = append(result, line)
		} else {
			blockLines = append(blockLines, line)
			// Check for closing fence: same char, same or greater length, only whitespace after
			trimmed := strings.TrimLeft(line, " ")
			if len(trimmed) >= fenceLen && trimmed[0] == fenceChar {
				n := 0
				for n < len(trimmed) && trimmed[n] == fenceChar {
					n++
				}
				rest := strings.TrimSpace(trimmed[n:])
				if n >= fenceLen && rest == "" {
					// Close the block
					inBlock = false
					idx := len(codeBlocks)
					codeBlocks = append(codeBlocks, strings.Join(blockLines, "\n"))
					result = append(result, fmt.Sprintf("%s%d___", mark, idx))
					blockLines = nil
				}
			}
		}
	}

	// If unclosed block, treat as regular text
	if inBlock {
		result = append(result, blockLines...)
	}

	r := strings.Join(result, "\n")

	// ── 2. Heading downgrade ──
	// Only when original text has H1~H3 headings
	hasH1toH3 := regexp.MustCompile(`(?m)^#{1,3} `).MatchString(text)
	if hasH1toH3 {
		// Order matters: H2~H6 → H5 first, then H1 → H4
		// If we did H1→H4 first, the resulting #### would be re-matched by #{2,6}
		r = regexp.MustCompile(`(?m)^#{2,6} (.+)$`).ReplaceAllString(r, "##### $1")
		r = regexp.MustCompile(`(?m)^# (.+)$`).ReplaceAllString(r, "#### $1")
	}

	if cardVersion >= 2 {
		// ── 3. Add <br> between consecutive headings ──
		r = regexp.MustCompile(`(?m)(^#{4,5} .+)\n{1,2}(#{4,5} )`).ReplaceAllString(r, "$1\n<br>\n$2")

		// ── 4. Add <br> around tables ──
		r = addTableBrSpacing(r)

		// ── 5. Restore code blocks with <br> ──
		for i, block := range codeBlocks {
			placeholder := fmt.Sprintf("%s%d___", mark, i)
			r = strings.Replace(r, placeholder, "\n<br>\n"+block+"\n<br>\n", 1)
		}
	} else {
		// ── 5. Restore code blocks (no <br>) ──
		for i, block := range codeBlocks {
			placeholder := fmt.Sprintf("%s%d___", mark, i)
			r = strings.Replace(r, placeholder, block, 1)
		}
	}

	// ── 6. Compress 3+ consecutive newlines → 2 ──
	r = regexp.MustCompile(`\n{3,}`).ReplaceAllString(r, "\n\n")

	return r
}

var imageRe = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)

// StripInvalidImageKeys removes ![alt](value) where value is not a valid
// Feishu image key (img_xxx). Prevents CardKit error 200570.
func StripInvalidImageKeys(text string) string {
	if !strings.Contains(text, "![") {
		return text
	}
	return imageRe.ReplaceAllStringFunc(text, func(match string) string {
		submatch := imageRe.FindStringSubmatch(match)
		if len(submatch) < 3 {
			return match
		}
		value := submatch[2]
		if strings.HasPrefix(value, "img_") {
			return match
		}
		return ""
	})
}

// isTableRow returns true if the line looks like a markdown table row.
func isTableRow(line string) bool {
	s := strings.TrimSpace(line)
	return len(s) > 2 && s[0] == '|' && s[len(s)-1] == '|'
}

// addTableBrSpacing adds <br> tags before and after table blocks.
func addTableBrSpacing(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	inTable := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		isRow := isTableRow(line)

		if isRow && !inTable {
			// Entering a table — add <br> before if there's preceding content
			inTable = true

			// Ensure blank line before table
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			// Add <br> before the blank line
			out = append(out, "<br>")
			// Ensure blank line between <br> and table
			if len(out) > 0 && out[len(out)-1] != "<br>" {
				// already added
			}
			out = append(out, "")
		}

		if !isRow && inTable {
			// Exiting a table — add <br> after
			inTable = false
			out = append(out, "<br>")
			// Add blank line after <br> if next line is not blank
			if line != "" {
				out = append(out, "")
			}
		}

		out = append(out, line)
	}

	// Handle table at end of text
	if inTable {
		out = append(out, "<br>")
	}

	// Clean up: remove duplicate blank lines and <br> adjacent to blank lines
	result := strings.Join(out, "\n")
	// Remove blank line + <br> + blank line → just <br> + blank line
	result = regexp.MustCompile(`\n\n<br>\n\n`).ReplaceAllString(result, "\n<br>\n\n")
	// Remove <br> + blank line + <br> → just <br>
	result = regexp.MustCompile(`<br>\n\n<br>`).ReplaceAllString(result, "<br>")
	return result
}
