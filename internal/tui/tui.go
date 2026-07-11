package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hwj123hwj/pi-go/internal/agent"
	"github.com/hwj123hwj/pi-go/internal/runtime"
	"github.com/hwj123hwj/pi-go/internal/slashcmd"
)

// TuiModel is the root Bubble Tea model for pi-go's interactive TUI.
type TuiModel struct {
	// Dimensions
	width  int
	height int

	// Sub-components
	input      InputModel
	viewport   MessageViewport
	spinnerOn  bool
	spinnerIdx int

	// State
	messages   []ChatMessage
	streaming  bool // LLM is generating text
	agentBusy  bool // agent is running tools or thinking
	streamBuf  string // accumulating text from LLM
	quitting   bool

	// Session
	session   *runtime.AgentSession
	slashCmds *slashcmd.Registry

	// Agent communication
	confirmCh chan ConfirmationResultMsg // user's confirmation reply
	err       error

	// UI metadata
	provider  string
	modelID   string
	workspace string
}

// New creates a new TuiModel.
func New(session *runtime.AgentSession, cmds *slashcmd.Registry) *TuiModel {
	provider, modelID := session.ModelInfo()
	m := &TuiModel{
		width:     80,
		height:    24,
		input:     NewInputModel(),
		viewport:  NewMessageViewport(80, 20),
		messages:  []ChatMessage{},
		session:   session,
		slashCmds: cmds,
		provider:  provider,
		modelID:   modelID,
		confirmCh: make(chan ConfirmationResultMsg, 1),
	}

	// Wire confirmation callback
	session.SetConfirmFunc(func(ctx context.Context, req agent.ConfirmationRequest) agent.ConfirmDecision {
		return m.handleConfirmation(ctx, req)
	})

	return m
}

// SetWorkspace sets the workspace path for display.
func (m *TuiModel) SetWorkspace(ws string) {
	m.workspace = ws
}

// Init implements tea.Model.
func (m *TuiModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *TuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Terminal resize ──
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Resize(msg.Width, m.inputHeight()+m.statusBarHeight())
		return m, nil

	// ── Key press ──
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	// ── Agent events ──
	case StreamTextMsg:
		m.streamBuf += msg.Delta
		m.agentBusy = true
		m.viewport.SetStreaming(m.streamBuf)
		return m, nil

	case ToolStartMsg:
		m.agentBusy = true
		m.spinnerOn = true
		if len(m.messages) > 0 {
			idx := len(m.messages) - 1
			m.messages[idx].Tools = append(m.messages[idx].Tools, ToolCallInfo{
				Name:      msg.Name,
				Args:      fmt.Sprintf("%v", msg.Args),
				Streaming: true,
				Collapsed: true,
			})
		}
		m.viewport.SetMessages(m.messages)
		return m, m.spinnerTick()

	case ToolUpdateMsg:
		return m, nil

	case ToolEndMsg:
		m.spinnerOn = false
		if len(m.messages) > 0 {
			idx := len(m.messages) - 1
			for i := range m.messages[idx].Tools {
				if m.messages[idx].Tools[i].Name == msg.Name && m.messages[idx].Tools[i].Streaming {
					m.messages[idx].Tools[i].Streaming = false
					m.messages[idx].Tools[i].Result = fmt.Sprintf("%v", msg.Result)
					m.messages[idx].Tools[i].IsError = msg.IsError
					break
				}
			}
		}
		m.viewport.SetMessages(m.messages)
		return m, nil

	case StreamDoneMsg:
		if m.streamBuf != "" {
			m.messages = append(m.messages, ChatMessage{
				Role:      "assistant",
				Content:   m.streamBuf,
				Timestamp: time.Now(),
			})
			m.streamBuf = ""
		}
		m.streaming = false
		m.agentBusy = false
		m.spinnerOn = false
		m.viewport.SetStreaming("")
		m.viewport.SetMessages(m.messages)
		return m, nil

	case AgentErrorMsg:
		m.streaming = false
		m.agentBusy = false
		m.spinnerOn = false
		m.err = msg.Err
		m.messages = append(m.messages, ChatMessage{
			Role:      "system",
			Content:   fmt.Sprintf("❌ Error: %v", msg.Err),
			Timestamp: time.Now(),
		})
		m.viewport.SetMessages(m.messages)
		return m, nil

	case ConfirmationMsg:
		// Show confirmation prompt in viewport
		m.messages = append(m.messages, ChatMessage{
			Role:      "system",
			Content:   fmt.Sprintf("⚠️ Confirm: %s (y/n)", msg.Req.Description),
			Timestamp: time.Now(),
		})
		m.viewport.SetMessages(m.messages)
		return m, nil

	case CompactionMsg:
		m.messages = append(m.messages, ChatMessage{
			Role:      "system",
			Content:   fmt.Sprintf("📦 Context compacted: %s", msg.Summary),
			Timestamp: time.Now(),
		})
		m.viewport.SetMessages(m.messages)
		return m, nil

	case LoopDetectedMsg:
		m.messages = append(m.messages, ChatMessage{
			Role:      "system",
			Content:   fmt.Sprintf("⚠️ Loop detected: %s (%d repeats)", msg.Tool, msg.Count),
			Timestamp: time.Now(),
		})
		m.viewport.SetMessages(m.messages)
		return m, nil

	case TickMsg:
		if m.spinnerOn {
			m.spinnerIdx++
			return m, m.spinnerTick()
		}
		return m, nil
	}

	return m, nil
}

// View implements tea.Model.
func (m *TuiModel) View() string {
	if m.quitting {
		return "Goodbye! 👋\n"
	}

	var buf string

	// Message viewport
	buf += m.viewport.View()
	buf += "\n"

	// Separator line
	buf += separatorLine(m.width)
	buf += "\n"

	// Input area
	buf += m.input.View()

	// Status bar
	buf += "\n"
	buf += m.statusBar()

	return buf
}

// ── Helpers ───────────────────────────────────────────────────────────────

func (m *TuiModel) inputHeight() int {
	return 3 // input box height (tunable)
}

func (m *TuiModel) statusBarHeight() int {
	return 2 // status bar + separator
}

func (m *TuiModel) statusBar() string {
	status := "● ready"
	if m.agentBusy {
		status = spinnerChars[m.spinnerIdx%len(spinnerChars)] + " working"
	}
	return fmt.Sprintf(" %s │ %s/%s │ %s", status, m.provider, m.modelID, m.workspace)
}

func separatorLine(width int) string {
	if width <= 0 {
		width = 80
	}
	return strings.Repeat("─", width)
}

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *TuiModel) spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

// handleConfirmation is called by the agent when a dangerous tool needs user approval.
// It sends the request to the TUI and blocks until the user responds.
func (m *TuiModel) handleConfirmation(ctx context.Context, req agent.ConfirmationRequest) agent.ConfirmDecision {
	slog.Info("confirmation requested", "tool", req.ToolName, "desc", req.Description)

	// In Phase 1, we auto-approve since we can't show a popup yet.
	// Phase 3 will implement a proper confirmation dialog.
	return agent.ConfirmDecision{Approved: true}
}
