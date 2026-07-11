package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ToolPanel renders a single tool execution as a collapsible bordered panel.
//
// Collapsed (default):
// ┌─ 🔧 bash ───────────────────────── ▸ ─┐
// └────────────────────────────────────────┘
//
// Expanded (Ctrl+O toggle):
// ┌─ 🔧 bash ───────────────────────── ▾ ─┐
// │ go test ./internal/tools/              │
// │ ok  github.com/hwj123hwj/pi-go/...     │
// │ 0.012s                                 │
// └────────────────────────────────────────┘
type ToolPanel struct {
	info     ToolCallInfo
	width    int
	theme    *Theme
	md       *MarkdownRenderer
}

// NewToolPanel creates a panel for a tool call.
func NewToolPanel(info ToolCallInfo, width int) *ToolPanel {
	return &ToolPanel{
		info:  info,
		width: width,
		theme: DefaultTheme(),
		md:    SharedMarkdown(),
	}
}

// Render returns the rendered panel string (one or more lines).
func (tp *ToolPanel) Render() []string {
	if tp.width < 30 {
		tp.width = 60 // safe minimum
	}

	// Build header
	icon := toolIcon(tp.info.Name)
	name := tp.info.Name
	args := truncateArg(tp.info.Args, tp.width-len(name)-10)

	var statusIcon string
	var statusColor lipgloss.Style

	if tp.info.Streaming {
		statusIcon = "⠋"
		statusColor = tp.theme.StatusBusy
	} else if tp.info.IsError {
		statusIcon = "✗"
		statusColor = tp.theme.StatusError
	} else {
		statusIcon = "✓"
		statusColor = tp.theme.SuccessText
	}

	// Collapse indicator
	collapseIcon := "▸"
	if !tp.info.Collapsed {
		collapseIcon = "▾"
	}

	// Build header line: "🔧 tool_name(args)  ✓ ▸"
	headerParts := []string{
		icon,
		tp.theme.ToolHeader.Render(name),
	}
	if args != "" {
		headerParts = append(headerParts, tp.theme.StatusDim.Render(args))
	}

	// Right side: status + collapse
	rightPart := statusColor.Render(statusIcon) + " " + tp.theme.StatusDim.Render(collapseIcon)

	leftWidth := visibleLength(headerParts) + len(headerParts) - 1 // parts + spaces between
	rightWidth := lipgloss.Width(rightPart)
	fillWidth := tp.width - leftWidth - rightWidth - 4 // account for padding
	if fillWidth < 1 {
		fillWidth = 1
	}

	headerLine := fmt.Sprintf("%s %s  %s",
		strings.Join(headerParts, " "),
		strings.Repeat(" ", fillWidth),
		rightPart,
	)

	if tp.info.Collapsed || tp.info.Result == "" {
		// Collapsed view — just the header inside a border
		return tp.wrapInBorder([]string{headerLine}, tp.info.Streaming, tp.info.IsError)
	}

	// Expanded view — header + body
	bodyLines := tp.renderBody()
	allLines := append([]string{headerLine}, bodyLines...)

	return tp.wrapInBorder(allLines, tp.info.Streaming, tp.info.IsError)
}

// renderBody renders the tool result content.
func (tp *ToolPanel) renderBody() []string {
	result := tp.info.Result
	if result == "" {
		return nil
	}

	// For edit/replace tools, try diff highlighting
	if isEditTool(tp.info.Name) {
		return RenderDiff(result, tp.theme)
	}

	// For other tools, render as-is (truncate long output)
	lines := strings.Split(result, "\n")

	// Truncate very long outputs to maxLines
	maxLines := 30
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], tp.theme.StatusDim.Render(fmt.Sprintf("… (%d more lines)", len(lines)-maxLines)))
	}

	// Render each line with subtle indentation
	var rendered []string
	for _, line := range lines {
		rendered = append(rendered, line)
	}
	return rendered
}

// wrapInBorder wraps lines in a rounded border with appropriate color.
func (tp *ToolPanel) wrapInBorder(lines []string, active, isError bool) []string {
	var borderStyle lipgloss.Style
	switch {
	case isError:
		borderStyle = tp.theme.ToolErrorBorder
	case active:
		borderStyle = tp.theme.ToolActiveBorder
	default:
		borderStyle = tp.theme.ToolDoneBorder
	}

	content := strings.Join(lines, "\n")
	bordered := borderStyle.Width(tp.width - 2).Render(content)

	return strings.Split(bordered, "\n")
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func toolIcon(name string) string {
	switch {
	case strings.Contains(name, "bash") || strings.Contains(name, "shell"):
		return "⚡"
	case strings.Contains(name, "edit") || strings.Contains(name, "replace") || strings.Contains(name, "write"):
		return "✏️"
	case strings.Contains(name, "read") || strings.Contains(name, "file"):
		return "📄"
	case strings.Contains(name, "search") || strings.Contains(name, "grep") || strings.Contains(name, "glob"):
		return "🔍"
	case strings.Contains(name, "web") || strings.Contains(name, "fetch") || strings.Contains(name, "curl"):
		return "🌐"
	case strings.Contains(name, "git"):
		return "🌿"
	case strings.Contains(name, "music") || strings.Contains(name, "play"):
		return "🎵"
	default:
		return "🔧"
	}
}

func isEditTool(name string) bool {
	return strings.Contains(name, "edit") || strings.Contains(name, "replace") ||
		strings.Contains(name, "write") || strings.Contains(name, "patch")
}

func truncateArg(args string, maxLen int) string {
	args = strings.ReplaceAll(args, "\n", " ")
	runes := []rune(args)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "…"
	}
	return args
}

// visibleLength counts runes, ignoring ANSI escape sequences.
func visibleLength(parts []string) int {
	total := 0
	for _, p := range parts {
		total += lipgloss.Width(p)
	}
	return total
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ToggleCollapsed toggles the collapsed state of a tool panel.
func (tp *ToolPanel) ToggleCollapsed() {
	tp.info.Collapsed = !tp.info.Collapsed
}
