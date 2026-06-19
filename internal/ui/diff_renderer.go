package ui

import (
	"fmt"
	"strings"
)

// DiffRenderer renders unified diffs with color-coded output for the terminal.
// It produces a simple line-by-line diff suitable for showing edits before/after.
type DiffRenderer struct{}

// NewDiffRenderer creates a new DiffRenderer.
func NewDiffRenderer() *DiffRenderer {
	return &DiffRenderer{}
}

// RenderEdit renders a single string replacement as a diff.
// Returns a colored diff showing oldStr → newStr context.
func (d *DiffRenderer) RenderEdit(filePath, oldStr, newStr string) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("%s%s┌─ %s ─%s\n", ColorCyan, ColorBold, filePath, ColorReset))

	// Show removed lines (oldStr)
	if oldStr != "" {
		oldLines := strings.Split(oldStr, "\n")
		for _, line := range oldLines {
			sb.WriteString(fmt.Sprintf("%s%s│ %s- %s%s\n",
				ColorGray, ColorDim, ColorRed, line, ColorReset))
		}
	}

	// Show added lines (newStr)
	if newStr != "" {
		newLines := strings.Split(newStr, "\n")
		for _, line := range newLines {
			sb.WriteString(fmt.Sprintf("%s%s│ %s+ %s%s\n",
				ColorGray, ColorDim, ColorGreen, line, ColorReset))
		}
	}

	// Footer
	sb.WriteString(fmt.Sprintf("%s%s└─%s", ColorGray, ColorDim, ColorReset))
	return sb.String()
}

// RenderWrite renders the creation of a new file or overwrite as a diff.
func (d *DiffRenderer) RenderWrite(filePath, content string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s%s┌─ %s (create/overwrite) ─%s\n",
		ColorCyan, ColorBold, filePath, ColorReset))

	lines := strings.Split(content, "\n")
	// Limit preview to first 20 lines
	maxPreview := 20
	displayed := lines
	truncated := false
	if len(lines) > maxPreview {
		displayed = lines[:maxPreview]
		truncated = true
	}

	for _, line := range displayed {
		sb.WriteString(fmt.Sprintf("%s%s│ %s+ %s%s\n",
			ColorGray, ColorDim, ColorGreen, line, ColorReset))
	}

	if truncated {
		sb.WriteString(fmt.Sprintf("%s%s│ %s... (%d more lines)%s\n",
			ColorGray, ColorDim, ColorYellow, len(lines)-maxPreview, ColorReset))
	}

	sb.WriteString(fmt.Sprintf("%s%s└─%s", ColorGray, ColorDim, ColorReset))
	return sb.String()
}

// RenderUnifiedDiff renders a unified diff string with color coding.
// Lines starting with "+" are green, lines starting with "-" are red,
// lines starting with "@@" are cyan (hunk headers).
func (d *DiffRenderer) RenderUnifiedDiff(diff string) string {
	var sb strings.Builder
	lines := strings.Split(diff, "\n")

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			sb.WriteString(ColorBold)
			sb.WriteString(line)
			sb.WriteString(ColorReset)
		case strings.HasPrefix(line, "@@"):
			sb.WriteString(ColorCyan)
			sb.WriteString(line)
			sb.WriteString(ColorReset)
		case strings.HasPrefix(line, "+"):
			sb.WriteString(ColorGreen)
			sb.WriteString(line)
			sb.WriteString(ColorReset)
		case strings.HasPrefix(line, "-"):
			sb.WriteString(ColorRed)
			sb.WriteString(line)
			sb.WriteString(ColorReset)
		default:
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// EditPreview holds parsed edit tool arguments for preview rendering.
type EditPreview struct {
	Path       string
	OldString  string
	NewString  string
	ReplaceAll bool
	IsMulti    bool
	EditCount  int
}

// WritePreview holds parsed write tool arguments for preview rendering.
type WritePreview struct {
	Path    string
	Content string
}
