package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
)

// ANSI color codes
const (
	ColorReset   = "\033[0m"
	ColorBold    = "\033[1m"
	ColorDim     = "\033[2m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"
	ColorGray    = "\033[90m"
)

// Spinner characters for progress indication
var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ToolStatus represents the current state of a tool execution
type ToolStatus int

const (
	ToolStatusRunning ToolStatus = iota
	ToolStatusDone
	ToolStatusError
)

// ToolCall tracks an active tool execution
type ToolCall struct {
	Name      string
	StartTime time.Time
	Status    ToolStatus
	Result    string
	IsError   bool
}

// EnhancedPresenter provides rich terminal output with colors and formatting
type EnhancedPresenter struct {
	w             io.Writer
	mu            sync.Mutex
	activeTools   map[string]*ToolCall
	spinnerIdx    int
	spinnerDone   chan struct{}
	streaming     bool
	streamBuf     strings.Builder
	turnStartTime time.Time
}

// NewEnhancedPresenter creates a presenter with rich formatting
func NewEnhancedPresenter(w io.Writer) *EnhancedPresenter {
	return &EnhancedPresenter{
		w:           w,
		activeTools: make(map[string]*ToolCall),
		spinnerDone: make(chan struct{}),
	}
}

// Present converts an AgentStreamEvent to rich terminal output
func (p *EnhancedPresenter) Present(event agent.AgentStreamEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch event.Type {
	case agent.StreamEventTextDelta:
		p.handleTextDelta(event.TextDelta)

	case agent.StreamEventToolStart:
		p.handleToolStart(event.ToolCallID, event.ToolName)

	case agent.StreamEventToolUpdate:
		// Tool updates are handled by spinner
		_ = event.ToolName

	case agent.StreamEventToolEnd:
		p.handleToolEnd(event.ToolCallID, event.ToolName, event.ToolResult, event.IsError)

	case agent.StreamEventCompacted:
		p.handleCompacted(event.TrimmedFrom, event.TrimmedTo)

	case agent.StreamEventDone:
		p.handleDone()

	case agent.StreamEventError:
		p.handleError(event.Error)
	}
}

func (p *EnhancedPresenter) handleTextDelta(delta string) {
	if !p.streaming {
		p.streaming = true
		p.turnStartTime = time.Now()
		p.streamBuf.Reset()
	}
	p.streamBuf.WriteString(delta)
	fmt.Fprint(p.w, delta)
}

func (p *EnhancedPresenter) handleToolStart(toolCallID, toolName string) {
	// Flush any pending text
	if p.streaming {
		p.streaming = false
	}

	tool := &ToolCall{
		Name:      toolName,
		StartTime: time.Now(),
		Status:    ToolStatusRunning,
	}
	p.activeTools[toolCallID] = tool

	icon := getToolIcon(toolName)
	fmt.Fprintf(p.w, "\n%s  ▶ %s%s %s%s",
		ColorCyan, icon, ColorBold, toolName, ColorReset)

	// Start spinner in background
	go p.runSpinner(toolCallID)
}

func (p *EnhancedPresenter) handleToolEnd(toolCallID, toolName string, result any, isError bool) {
	tool, exists := p.activeTools[toolCallID]
	if !exists {
		return
	}

	tool.Status = ToolStatusDone
	tool.IsError = isError
	if result != nil {
		tool.Result = fmt.Sprintf("%v", result)
	}

	elapsed := time.Since(tool.StartTime)
	delete(p.activeTools, toolCallID)

	// Move cursor to beginning of line and clear
	fmt.Fprint(p.w, "\r\033[K")

	if isError {
		fmt.Fprintf(p.w, "  %s✗ %s%s%s (%s)%s",
			ColorRed, ColorBold, toolName, ColorReset, formatDuration(elapsed), ColorRed)
		if tool.Result != "" {
			resultStr := truncate(tool.Result, 120)
			fmt.Fprintf(p.w, "\n    %serror: %s%s", ColorDim, resultStr, ColorReset)
		}
		fmt.Fprint(p.w, ColorReset)
	} else {
		fmt.Fprintf(p.w, "  %s✓ %s%s%s (%s)%s",
			ColorGreen, ColorBold, toolName, ColorReset, formatDuration(elapsed), ColorReset)

		// Format output based on tool type
		if tool.Result != "" {
			if toolName == "bash" {
				// For bash, show output in a box
				output := tool.Result
				if len(output) > 300 {
					output = output[:300] + "\n... (truncated)"
				}
				fmt.Fprintf(p.w, "\n%s%s┌─%s\n", ColorGray, ColorDim, ColorReset)
				lines := strings.Split(output, "\n")
				for _, line := range lines {
					fmt.Fprintf(p.w, "%s%s│ %s%s\n", ColorGray, ColorDim, line, ColorReset)
				}
				fmt.Fprintf(p.w, "%s%s└─%s", ColorGray, ColorDim, ColorReset)
			} else {
				// For other tools, show result inline
				resultStr := truncate(tool.Result, 100)
				fmt.Fprintf(p.w, "\n    %s→ %s%s", ColorGray, resultStr, ColorReset)
			}
		}
	}
	fmt.Fprintln(p.w)
}

func (p *EnhancedPresenter) handleCompacted(trimmedFrom, trimmedTo int) {
	fmt.Fprintf(p.w, "\n%s  [context compacted: %d → %d messages]%s\n",
		ColorYellow, trimmedFrom, trimmedTo, ColorReset)
}

func (p *EnhancedPresenter) handleDone() {
	p.streaming = false
	if p.turnStartTime.IsZero() {
		return
	}
	elapsed := time.Since(p.turnStartTime)
	fmt.Fprintf(p.w, "\n%s%s%s\n", ColorDim, formatDuration(elapsed), ColorReset)
	p.turnStartTime = time.Time{}
}

func (p *EnhancedPresenter) handleError(err string) {
	p.streaming = false
	fmt.Fprintf(p.w, "\n%s  error: %s%s\n", ColorRed, err, ColorReset)
}

func (p *EnhancedPresenter) runSpinner(toolCallID string) {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.spinnerDone:
			return
		case <-ticker.C:
			p.mu.Lock()
			tool, exists := p.activeTools[toolCallID]
			if !exists || tool.Status != ToolStatusRunning {
				p.mu.Unlock()
				return
			}

			elapsed := time.Since(tool.StartTime)
			spinner := spinnerChars[p.spinnerIdx%len(spinnerChars)]
			p.spinnerIdx++

			// Move to beginning of line, clear, and redraw
			fmt.Fprintf(p.w, "\r\033[K  %s%s %s%s %s%s",
				ColorCyan, spinner, ColorBold, tool.Name, ColorReset, formatDuration(elapsed))
			p.mu.Unlock()
		}
	}
}

// getToolIcon returns a colored icon for the tool
func getToolIcon(toolName string) string {
	icons := map[string]string{
		"bash":  "⌨️ ",
		"read":  "📖",
		"write": "✏️ ",
		"edit":  "📝",
		"grep":  "🔍",
		"find":  "📂",
		"ls":    "📋",
	}
	if icon, ok := icons[toolName]; ok {
		return icon
	}
	return "🔧"
}

// formatDuration formats a duration as human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

// truncate truncates a string to maxLen and appends "..." if needed
func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FormatTable renders a markdown table with box-drawing characters
func (p *EnhancedPresenter) FormatTable(header []string, rows [][]string) string {
	if len(header) == 0 {
		return ""
	}

	// Calculate column widths
	colWidths := make([]int, len(header))
	for i, h := range header {
		colWidths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	var sb strings.Builder

	// Top border
	sb.WriteString(ColorGray)
	sb.WriteString("┌")
	for i, w := range colWidths {
		sb.WriteString(strings.Repeat("─", w+2))
		if i < len(colWidths)-1 {
			sb.WriteString("┬")
		}
	}
	sb.WriteString("┐")
	sb.WriteString(ColorReset)
	sb.WriteString("\n")

	// Header
	sb.WriteString(ColorGray)
	sb.WriteString("│")
	sb.WriteString(ColorReset)
	for i, h := range header {
		sb.WriteString(ColorBold)
		sb.WriteString(fmt.Sprintf(" %-*s ", colWidths[i], h))
		sb.WriteString(ColorReset)
		sb.WriteString(ColorGray)
		sb.WriteString("│")
		sb.WriteString(ColorReset)
	}
	sb.WriteString("\n")

	// Header separator
	sb.WriteString(ColorGray)
	sb.WriteString("├")
	for i, w := range colWidths {
		sb.WriteString(strings.Repeat("─", w+2))
		if i < len(colWidths)-1 {
			sb.WriteString("┼")
		}
	}
	sb.WriteString("┤")
	sb.WriteString(ColorReset)
	sb.WriteString("\n")

	// Rows
	for _, row := range rows {
		sb.WriteString(ColorGray)
		sb.WriteString("│")
		sb.WriteString(ColorReset)
		for i, cell := range row {
			if i < len(colWidths) {
				sb.WriteString(fmt.Sprintf(" %-*s ", colWidths[i], cell))
			}
			sb.WriteString(ColorGray)
			sb.WriteString("│")
			sb.WriteString(ColorReset)
		}
		sb.WriteString("\n")
	}

	// Bottom border
	sb.WriteString(ColorGray)
	sb.WriteString("└")
	for i, w := range colWidths {
		sb.WriteString(strings.Repeat("─", w+2))
		if i < len(colWidths)-1 {
			sb.WriteString("┴")
		}
	}
	sb.WriteString("┘")
	sb.WriteString(ColorReset)

	return sb.String()
}

// FormatBashOutput formats bash command output with better styling
func (p *EnhancedPresenter) FormatBashOutput(command string, output string, exitCode int, elapsed time.Duration) string {
	var sb strings.Builder

	// Command header
	sb.WriteString(fmt.Sprintf("\n%s%s$ %s%s\n", ColorDim, ColorBold, command, ColorReset))

	// Output with subtle border
	if output != "" {
		lines := strings.Split(output, "\n")
		sb.WriteString(ColorGray)
		sb.WriteString("┌─")
		sb.WriteString(ColorReset)
		sb.WriteString("\n")
		for _, line := range lines {
			sb.WriteString(ColorGray)
			sb.WriteString("│ ")
			sb.WriteString(ColorReset)
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString(ColorGray)
		sb.WriteString("└─")
		sb.WriteString(ColorReset)
	}

	// Status line
	if exitCode == 0 {
		sb.WriteString(fmt.Sprintf("  %s✓ %s%s\n", ColorGreen, formatDuration(elapsed), ColorReset))
	} else {
		sb.WriteString(fmt.Sprintf("  %s✗ exit %d%s %s%s\n", ColorRed, exitCode, ColorReset, formatDuration(elapsed), ColorReset))
	}

	return sb.String()
}

// FormatSection formats a section with a title and content
func (p *EnhancedPresenter) FormatSection(title string, content string) string {
	return fmt.Sprintf("\n%s%s── %s ──%s\n%s\n", ColorCyan, ColorBold, title, ColorReset, content)
}

// FormatKeyValue formats a key-value pair
func (p *EnhancedPresenter) FormatKeyValue(key string, value string) string {
	return fmt.Sprintf("  %s%-15s%s %s", ColorBold, key+":", ColorReset, value)
}

// FormatSessionStatus returns a rich formatted status line
func FormatSessionStatus(sessionID, provider, modelID, profile, cwd string) string {
	dir := cwd
	if dir != "" {
		parts := strings.Split(dir, "/")
		if len(parts) > 0 {
			dir = parts[len(parts)-1]
		}
	}

	// Truncate session ID if longer than 12 chars
	displayID := sessionID
	if len(sessionID) > 12 {
		displayID = sessionID[:12] + "..."
	}

	return fmt.Sprintf("%s%sSession:%s %s %s│%s %sModel:%s %s/%s %s│%s %sProfile:%s %s %s│%s %sCWD:%s %s%s",
		ColorDim, ColorBold, ColorReset, displayID,
		ColorDim, ColorReset,
		ColorBold, ColorReset, provider, modelID,
		ColorDim, ColorReset,
		ColorBold, ColorReset, profile,
		ColorDim, ColorReset,
		ColorBold, ColorReset, dir,
		ColorReset)
}

// FormatInputPrompt returns a styled input prompt
func FormatInputPrompt() string {
	return fmt.Sprintf("%s%sYou>%s ", ColorBlue, ColorBold, ColorReset)
}

// FormatAssistantPrompt returns a styled assistant prompt
func FormatAssistantPrompt() string {
	return fmt.Sprintf("%s%sPi>%s ", ColorGreen, ColorBold, ColorReset)
}

// PrintBanner prints the startup banner
func PrintBanner(w io.Writer) {
	fmt.Fprintf(w, "\n%s%s╔════════════════════════════════════════════════════════════╗%s\n", ColorCyan, ColorBold, ColorReset)
	fmt.Fprintf(w, "%s%s║                    ⚡ Pi-Go Agent                        ║%s\n", ColorCyan, ColorBold, ColorReset)
	fmt.Fprintf(w, "%s%s╚════════════════════════════════════════════════════════════╝%s\n\n", ColorCyan, ColorBold, ColorReset)
}

// PrintHelp prints the help message
func PrintHelp(w io.Writer) {
	fmt.Fprintf(w, "%sCommands:%s\n", ColorBold, ColorReset)
	fmt.Fprintf(w, "  %s/new%s          New session\n", ColorCyan, ColorReset)
	fmt.Fprintf(w, "  %s/compact%s      Compact context\n", ColorCyan, ColorReset)
	fmt.Fprintf(w, "  %s/sessions%s     List sessions\n", ColorCyan, ColorReset)
	fmt.Fprintf(w, "  %s/switch%s       Switch session\n", ColorCyan, ColorReset)
	fmt.Fprintf(w, "  %s/model%s        Switch model\n", ColorCyan, ColorReset)
	fmt.Fprintf(w, "  %s/profile%s      Switch profile\n", ColorCyan, ColorReset)
	fmt.Fprintf(w, "  %s/goal%s         Set goal\n", ColorCyan, ColorReset)
	fmt.Fprintf(w, "  %s/tools%s        List tools\n", ColorCyan, ColorReset)
	fmt.Fprintf(w, "  %s/clear%s        Clear screen\n", ColorCyan, ColorReset)
	fmt.Fprintf(w, "  %s/help%s         Show this help\n", ColorCyan, ColorReset)
	fmt.Fprintf(w, "  %s/exit%s         Exit\n\n", ColorCyan, ColorReset)
}
