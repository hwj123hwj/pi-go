package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderDiff renders tool output as a colored diff.
// It detects common diff formats (unified diff, git diff) and applies
// syntax highlighting:
//   + Added lines → green
//   - Removed lines → red
//   @@ Hunk headers → blue/bold
//   Context lines → dim
func RenderDiff(content string, theme *Theme) []string {
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")

	// Detect diff format
	isDiff := false
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") ||
			strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") {
			isDiff = true
			break
		}
	}

	if !isDiff {
		return lines
	}

	var result []string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			result = append(result, theme.DiffAdded.Render(line))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			result = append(result, theme.DiffRemoved.Render(line))
		case strings.HasPrefix(line, "@@"):
			result = append(result, theme.DiffHeader.Render(line))
		case strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index "):
			result = append(result, theme.DiffHeader.Render(line))
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			result = append(result, theme.DiffHeader.Render(line))
		default:
			// Context line — dim if it looks like diff context
			if isDiff {
				result = append(result, theme.DiffContext.Render(line))
			} else {
				result = append(result, line)
			}
		}
	}

	// Truncate long diffs
	maxLines := 50
	if len(result) > maxLines {
		ellipsis := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E")).Render(
			"... (" + itoa(len(result)-maxLines) + " more lines)",
		)
		result = append(result[:maxLines], ellipsis)
	}

	return result
}

// itoa is a lightweight strconv.Itoa without the import.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
