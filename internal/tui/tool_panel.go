package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ToolPanel renders a single tool execution as a collapsible bordered panel.
//
// Collapsed (default):
// ┌─ 🔧 bash ────────────────────── ✓ 1.2s ▸ ─┐
// └────────────────────────────────────────────┘
//
// Expanded (Ctrl+O toggle):
// ┌─ 🔧 bash ────────────────────── ✓ 1.2s ▾ ─┐
// │ go test ./internal/tools/                   │
// │ ok  github.com/hwj123hwj/pi-go/...          │
// └─────────────────────────────────────────────┘
type ToolPanel struct {
	info  ToolCallInfo
	width int
	theme *Theme
	md    *MarkdownRenderer
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

// innerWidth returns the actual text content area inside the border + padding.
func (tp *ToolPanel) innerWidth() int {
	// Border: 2 chars (left+right), Padding: 2 chars (1 each side)
	return tp.width - 4
}

// Render returns the rendered panel string (one or more lines).
func (tp *ToolPanel) Render() []string {
	if tp.width < 30 {
		tp.width = 60 // safe minimum
	}

	contentWidth := tp.innerWidth()

	// ── Build left side of header ──
	icon := toolIcon(tp.info.Name)
	name := tp.info.Name

	argsDisplay := formatToolArgs(tp.info.Args)
	args := truncateArg(argsDisplay, contentWidth-len(name)-12) // leave room for status

	leftParts := []string{icon, tp.theme.ToolHeader.Render(name)}
	if args != "" {
		leftParts = append(leftParts, tp.theme.StatusDim.Render(args))
	}
	leftStr := strings.Join(leftParts, " ")
	leftWidth := lipgloss.Width(leftStr)

	// ── Build right side of header ──
	var statusStr string

	if tp.info.Streaming {
		statusStr = tp.theme.StatusBusy.Render("● running")
	} else if tp.info.IsError {
		statusStr = tp.theme.StatusError.Render("✗")
	} else {
		// Show elapsed time if available
		if !tp.info.StartTime.IsZero() {
			elapsed := time.Since(tp.info.StartTime)
			if elapsed >= time.Second {
				statusStr = tp.theme.SuccessText.Render(fmt.Sprintf("✓ %s", formatDuration(elapsed)))
			} else {
				statusStr = tp.theme.SuccessText.Render("✓")
			}
		} else {
			statusStr = tp.theme.SuccessText.Render("✓")
		}
	}

	// Collapse indicator
	collapseIcon := "▸"
	if !tp.info.Collapsed {
		collapseIcon = "▾"
	}
	rightStr := statusStr + " " + tp.theme.StatusDim.Render(collapseIcon)
	rightWidth := lipgloss.Width(rightStr)

	// ── Assemble header with proper fill ──
	fillWidth := contentWidth - leftWidth - rightWidth
	if fillWidth < 1 {
		fillWidth = 1
	}

	headerLine := leftStr + strings.Repeat(" ", fillWidth) + rightStr

	if tp.info.Collapsed || tp.info.Result == "" {
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

	return lines
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
	if maxLen < 10 {
		maxLen = 10
	}
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "…"
	}
	return args
}

// formatToolArgs parses the raw args string (which might be JSON, Go fmt "%v", or <nil>)
// and returns a compact human-readable display string.
func formatToolArgs(raw string) string {
	if raw == "" || raw == "<nil>" {
		return ""
	}

	// Try to parse as JSON object
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &m); err == nil {
			return formatJSONArgs(m)
		}
	}

	// Fallback: just show the raw string
	return raw
}

// formatJSONArgs converts a JSON object to a compact "key: value" display.
func formatJSONArgs(m map[string]interface{}) string {
	var parts []string
	for k, v := range m {
		switch val := v.(type) {
		case string:
			s := val
			if len(s) > 40 {
				s = s[:40] + "…"
			}
			parts = append(parts, k+": "+strconv.Quote(s))
		default:
			parts = append(parts, k+": "+fmt.Sprintf("%v", v))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

// formatDuration renders a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
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
