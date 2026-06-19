package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
)

// DisplayEvent is a structured representation of an agent event,
// ready for rendering by any UI (CLI, TUI, web).
type DisplayEvent struct {
	Type    DisplayEventType
	Content string // primary content
	Detail  string // secondary detail (error message, result summary, etc.)
	Tool    string // tool name (for tool events)
	IsError bool   // whether this represents an error
}

type DisplayEventType int

const (
	DisplayTextDelta DisplayEventType = iota
	DisplayToolStart
	DisplayToolUpdate
	DisplayToolEnd
	DisplayCompacted
	DisplayDone
	DisplayError
	DisplayConfirmationReq
	DisplayConfirmationRes
	DisplayLoopDetected
)

// Presenter converts agent stream events into display events.
// This decouples event interpretation from rendering.
type Presenter struct {
	w io.Writer
}

// NewPresenter creates a presenter that writes to w.
func NewPresenter(w io.Writer) *Presenter {
	return &Presenter{w: w}
}

// Present converts an AgentStreamEvent to a DisplayEvent and renders it.
func (p *Presenter) Present(event agent.AgentStreamEvent) {
	de := convertEvent(event)
	p.render(de)
}

// convertEvent converts an AgentStreamEvent into a DisplayEvent.
func convertEvent(event agent.AgentStreamEvent) DisplayEvent {
	switch event.Type {
	case agent.StreamEventTextDelta:
		return DisplayEvent{Type: DisplayTextDelta, Content: event.TextDelta}

	case agent.StreamEventToolStart:
		return DisplayEvent{Type: DisplayToolStart, Tool: event.ToolName}

	case agent.StreamEventToolUpdate:
		return DisplayEvent{Type: DisplayToolUpdate, Tool: event.ToolName}

	case agent.StreamEventToolEnd:
		result := fmt.Sprintf("%v", event.ToolResult)
		result = strings.TrimSpace(result)
		if len(result) > 100 {
			result = result[:100] + "..."
		}
		return DisplayEvent{
			Type:    DisplayToolEnd,
			Tool:    event.ToolName,
			Content: result,
			IsError: event.IsError,
		}

	case agent.StreamEventCompacted:
		if event.TrimmedFrom > 0 || event.TrimmedTo > 0 {
			return DisplayEvent{
				Type:    DisplayCompacted,
				Content: fmt.Sprintf("context compacted: %d → %d messages", event.TrimmedFrom, event.TrimmedTo),
			}
		}
		return DisplayEvent{Type: DisplayCompacted, Content: "context compacted"}

	case agent.StreamEventConfirmationReq:
		return DisplayEvent{Type: DisplayConfirmationReq, Content: event.Description}

	case agent.StreamEventConfirmationRes:
		return DisplayEvent{Type: DisplayConfirmationRes, Content: event.Description, IsError: !event.Approved}

	case agent.StreamEventLoopDetected:
		return DisplayEvent{
			Type:    DisplayLoopDetected,
			Tool:    event.ToolName,
			Content: fmt.Sprintf("%d", event.RepeatCount),
		}

	case agent.StreamEventDone:
		return DisplayEvent{Type: DisplayDone}

	case agent.StreamEventError:
		return DisplayEvent{Type: DisplayError, Content: event.Error, IsError: true}

	default:
		return DisplayEvent{Type: DisplayTextDelta, Content: ""}
	}
}

// render writes a DisplayEvent to the output writer using CLI formatting.
func (p *Presenter) render(de DisplayEvent) {
	switch de.Type {
	case DisplayTextDelta:
		fmt.Fprint(p.w, de.Content)

	case DisplayToolStart:
		fmt.Fprintf(p.w, "\n  ▶ %s ", de.Tool)

	case DisplayToolUpdate:
		fmt.Fprint(p.w, ".")

	case DisplayToolEnd:
		if de.IsError {
			result := de.Content
			if len(result) > 120 {
				result = result[:120] + "..."
			}
			fmt.Fprintf(p.w, " ✗\n    error: %s\n", result)
		} else {
			if de.Content != "" {
				fmt.Fprintf(p.w, " ✓\n    → %s\n", de.Content)
			} else {
				fmt.Fprint(p.w, " ✓\n")
			}
		}

	case DisplayCompacted:
		fmt.Fprintf(p.w, "\n  [%s]\n", de.Content)

	case DisplayConfirmationReq:
		fmt.Fprintf(p.w, "\n  ⚠ 需要确认: %s\n", de.Content)

	case DisplayConfirmationRes:
		if de.IsError { // IsError 复用为"是否拒绝"
			msg := "用户拒绝"
			if de.Content != "" {
				msg = de.Content
			}
			fmt.Fprintf(p.w, "  ✗ 已拒绝：%s\n", msg)
		} else {
			fmt.Fprint(p.w, "  ✓ 已确认，继续执行\n")
		}

	case DisplayLoopDetected:
		fmt.Fprintf(p.w, "  ⚠ 检测到循环：%s 连续重复 %s 次\n", de.Tool, de.Content)

	case DisplayDone:
		fmt.Fprintln(p.w)

	case DisplayError:
		fmt.Fprintf(p.w, "\n  error: %s\n", de.Content)
	}
}

// FormatTimestamp formats a Unix timestamp as a human-readable string.
func FormatTimestamp(unix int64) string {
	if unix == 0 {
		return "n/a"
	}
	t := time.Unix(unix, 0)
	// If today, show just time; otherwise show date and time
	now := time.Now()
	if t.Format("2006-01-02") == now.Format("2006-01-02") {
		return t.Format("15:04:05")
	}
	return t.Format("2006-01-02 15:04")
}
