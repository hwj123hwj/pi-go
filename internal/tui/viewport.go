package tui

import (
	"fmt"
	"strings"
)

// MessageViewport is a scrollable viewport that renders conversation messages.
// Phase 1: Simple line-based rendering. Phase 2 will add Markdown + tool panels.
type MessageViewport struct {
	width   int
	height  int
	messages []ChatMessage
	streaming string // text being streamed (not yet finalized)
	lines   []string // rendered lines (cached)
	scrollOffset int
}

// NewMessageViewport creates a new viewport.
func NewMessageViewport(width, height int) MessageViewport {
	return MessageViewport{
		width:  width,
		height: height,
	}
}

// Resize updates the viewport dimensions.
func (v *MessageViewport) Resize(width, height int) {
	v.width = width
	v.height = height
	v.rebuildLines()
}

// SetMessages updates the message list and rebuilds the view.
func (v *MessageViewport) SetMessages(msgs []ChatMessage) {
	v.messages = msgs
	v.rebuildLines()
}

// SetStreaming sets the current streaming text (shown live as agent types).
func (v *MessageViewport) SetStreaming(text string) {
	v.streaming = text
	v.rebuildLines()
}

// Clear resets the viewport.
func (v *MessageViewport) Clear() {
	v.messages = nil
	v.streaming = ""
	v.lines = nil
	v.scrollOffset = 0
}

// GotoBottom scrolls to the bottom of the messages.
func (v *MessageViewport) GotoBottom() {
	maxScroll := len(v.lines) - v.height
	if maxScroll > 0 {
		v.scrollOffset = maxScroll
	} else {
		v.scrollOffset = 0
	}
}

// View renders the visible portion of the viewport.
func (v *MessageViewport) View() string {
	if len(v.lines) == 0 {
		return strings.Repeat("\n", v.height-1)
	}

	start := v.scrollOffset
	end := start + v.height
	if end > len(v.lines) {
		end = len(v.lines)
	}
	if start > end {
		start = end
	}

	visible := v.lines[start:end]

	// Pad to fill height
	for len(visible) < v.height {
		visible = append(visible, "")
	}

	return strings.Join(visible, "\n")
}

// ── Rendering ─────────────────────────────────────────────────────────────

func (v *MessageViewport) rebuildLines() {
	var lines []string

	for _, msg := range v.messages {
		lines = append(lines, v.renderMessage(msg)...)
	}

	// Streaming text
	if v.streaming != "" {
		lines = append(lines, v.renderStreaming(v.streaming)...)
	}

	v.lines = lines
	v.GotoBottom()
}

func (v *MessageViewport) renderMessage(msg ChatMessage) []string {
	var lines []string

	// Role label
	var label, color string
	switch msg.Role {
	case "user":
		label = "You"
		color = "\033[36m" // cyan
	case "assistant":
		label = "π"
		color = "\033[35m" // magenta
	case "system":
		label = "system"
		color = "\033[33m" // yellow
	default:
		label = msg.Role
		color = ""
	}

	ts := msg.Timestamp.Format("15:04")
	reset := ""
	if color != "" {
		reset = "\033[0m"
	}

	header := fmt.Sprintf("%s%s%s [%s]", color, label, reset, ts)
	lines = append(lines, header)

	// Content (Phase 1: plain text, Phase 2 will use glamour)
	for _, line := range strings.Split(msg.Content, "\n") {
		lines = append(lines, line)
	}

	// Tool calls
	for _, tool := range msg.Tools {
		lines = append(lines, v.renderToolCall(tool)...)
	}

	lines = append(lines, "") // blank line separator

	return lines
}

func (v *MessageViewport) renderStreaming(text string) []string {
	var lines []string
	lines = append(lines, "\033[35mπ\033[0m (typing...)")

	for _, line := range strings.Split(text, "\n") {
		lines = append(lines, line)
	}
	return lines
}

func (v *MessageViewport) renderToolCall(tool ToolCallInfo) []string {
	var lines []string

	icon := "🔧"
	status := ""
	if tool.Streaming {
		status = " ⠋ running..."
	} else if tool.IsError {
		icon = "❌"
		status = " error"
	} else {
		status = " ✓"
	}

	collapsed := "▸"
	if !tool.Collapsed {
		collapsed = "▾"
	}

	header := fmt.Sprintf("  %s %s%s%s %s", icon, tool.Name, tool.Args, status, collapsed)
	lines = append(lines, header)

	if !tool.Collapsed && tool.Result != "" {
		for _, line := range strings.Split(tool.Result, "\n") {
			lines = append(lines, "    "+line)
		}
	}

	return lines
}
