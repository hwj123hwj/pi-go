//go:build fancy

package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/earendil-works/pi-go/internal/agent"
)

// Lipgloss styles — defined once, reused for performance.
// Adapts to terminal background automatically.
var (
	fancyStyleBorder = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#0969DA", Dark: "#58A6FF"}).
				Bold(true)

	fancyStyleTool = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0969DA", Dark: "#58A6FF"}).
			Bold(true)

	fancyStyleSuccess = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#1A7F37", Dark: "#3FB950"})

	fancyStyleError = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#CF222E", Dark: "#F85149"}).
			Bold(true)

	fancyStyleWarn = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#D29922"}).
			Bold(true)

	fancyStyleDim = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#656D76", Dark: "#8B949E"})

	fancyStyleAdded = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3FB950"))

	fancyStyleRemoved = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F85149"))

	fancyStyleCodeBlock = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#D0D7DE", Dark: "#30363D"}).
				Padding(0, 1)
)

// FancyPresenter is the Bubble Tea-powered TUI renderer.
// Only compiled when -tags fancy is used.
type FancyPresenter struct {
	w             io.Writer
	mu            sync.Mutex
	activeTools   map[string]*ToolCall
	streaming     bool
	streamBuf     strings.Builder
	turnStartTime time.Time
	diffRenderer  *DiffRenderer
}

// NewFancyPresenter creates a Lipgloss-based renderer.
func NewFancyPresenter(w io.Writer) *FancyPresenter {
	return &FancyPresenter{
		w:            w,
		activeTools:  make(map[string]*ToolCall),
		diffRenderer: NewDiffRenderer(),
	}
}

// Present implements TUIRenderer.
func (p *FancyPresenter) Present(event agent.AgentStreamEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch event.Type {
	case agent.StreamEventTextDelta:
		if !p.streaming {
			p.streaming = true
			p.turnStartTime = time.Now()
			p.streamBuf.Reset()
		}
		p.streamBuf.WriteString(event.TextDelta)

	case agent.StreamEventToolStart:
		if p.streaming {
			p.streaming = false
		}
		p.activeTools[event.ToolCallID] = &ToolCall{
			Name:      event.ToolName,
			StartTime: time.Now(),
			Status:    ToolStatusRunning,
		}
		fmt.Fprintln(p.w)
		fmt.Fprintln(p.w, fancyStyleTool.Render(fmt.Sprintf("  ▶ %s", event.ToolName)))

	case agent.StreamEventToolEnd:
		tool, ok := p.activeTools[event.ToolCallID]
		if !ok {
			return
		}
		elapsed := time.Since(tool.StartTime)
		delete(p.activeTools, event.ToolCallID)

		if event.IsError {
			fmt.Fprintln(p.w, fancyStyleError.Render(
				fmt.Sprintf("  ✗ %s (%s)", event.ToolName, formatDuration(elapsed))))
			if event.ToolResult != nil {
				resultStr := truncate(fmt.Sprintf("%v", event.ToolResult), 120)
				fmt.Fprintln(p.w, fancyStyleDim.Render("    error: "+resultStr))
			}
		} else {
			fmt.Fprintln(p.w, fancyStyleSuccess.Render(
				fmt.Sprintf("  ✓ %s (%s)", event.ToolName, formatDuration(elapsed))))
			if event.ToolResult != nil {
				resultStr := fmt.Sprintf("%v", event.ToolResult)
				if event.ToolName == "bash" {
					p.renderBashOutput(resultStr)
				} else {
					fmt.Fprintln(p.w, fancyStyleDim.Render("    → "+truncate(resultStr, 80)))
				}
			}
		}

	case agent.StreamEventCompacted:
		fmt.Fprintln(p.w, fancyStyleDim.Render(
			fmt.Sprintf("\n  [context compacted: %d → %d messages]", event.TrimmedFrom, event.TrimmedTo)))

	case agent.StreamEventConfirmationReq:
		fmt.Fprintln(p.w)
		fmt.Fprintln(p.w, fancyStyleWarn.Render(fmt.Sprintf("  ⚠ 需要确认: %s", event.Description)))

	case agent.StreamEventConfirmationRes:
		if event.Approved {
			fmt.Fprintln(p.w, fancyStyleSuccess.Render("  ✓ 已确认，继续执行"))
		} else {
			msg := "用户拒绝"
			if event.Description != "" {
				msg = event.Description
			}
			fmt.Fprintln(p.w, fancyStyleError.Render(fmt.Sprintf("  ✗ 已拒绝：%s", msg)))
		}

	case agent.StreamEventLoopDetected:
		fmt.Fprintln(p.w, fancyStyleWarn.Render(fmt.Sprintf("  ⚠ 检测到循环：%s 连续重复 %d 次", event.ToolName, event.RepeatCount)))

	case agent.StreamEventMicroCompacted:
		fmt.Fprintln(p.w, fancyStyleDim.Render(fmt.Sprintf("  ↘ 已清理 %d 个旧工具结果以节省上下文", event.ClearedCount)))

	case agent.StreamEventDone:
		if p.streamBuf.Len() > 0 {
			rendered := RenderMarkdown(p.streamBuf.String())
			fmt.Fprint(p.w, rendered)
			p.streamBuf.Reset()
		}
		if !p.turnStartTime.IsZero() {
			elapsed := time.Since(p.turnStartTime)
			fmt.Fprintln(p.w, fancyStyleDim.Render(formatDuration(elapsed)))
			p.turnStartTime = time.Time{}
		}
		p.streaming = false

	case agent.StreamEventError:
		p.streaming = false
		fmt.Fprintln(p.w, fancyStyleError.Render("  error: "+event.Error))
	}
}

func (p *FancyPresenter) renderBashOutput(output string) {
	lines := strings.Split(output, "\n")
	// Remove trailing empty lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 5 {
		lines = lines[:5]
		lines = append(lines, fancyStyleDim.Render("... (truncated)"))
	}
	for i, line := range lines {
		if len(line) > 100 {
			lines[i] = line[:100] + "..."
		}
	}
	box := fancyStyleCodeBlock.Render(strings.Join(lines, "\n"))
	fmt.Fprintln(p.w, box)
}

// Verify implementation
var _ TUIRenderer = (*FancyPresenter)(nil)
