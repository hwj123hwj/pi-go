package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hwj123hwj/pi-go/internal/agent"
	"github.com/hwj123hwj/pi-go/internal/slashcmd"
)

// handleKeyPress processes all keyboard input.
func (m *TuiModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {

	// ── Global keys ──
	case tea.KeyCtrlC:
		if m.agentBusy {
			// Cancel current agent execution
			m.streaming = false
			m.agentBusy = false
			m.spinnerOn = false
			m.streamBuf = ""
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case tea.KeyCtrlD:
		if m.input.IsEmpty() {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyCtrlL:
		// Clear screen
		m.viewport.Clear()
		return m, nil

	// ── Submit message ──
	case tea.KeyEnter:
		input := m.input.Text()
		if input == "" {
			return m, nil
		}
		// Handle slash commands locally
		if slashcmd.IsSlashCommand(input) {
			return m.handleSlashCommand(input)
		}
		// Handle special commands
		lower := strings.ToLower(strings.TrimSpace(input))
		if lower == "exit" || lower == "quit" {
			m.quitting = true
			return m, tea.Quit
		}
		// Send to agent
		return m.sendMessage(input)

	// ── Input editing (delegate to input model) ──
	default:
		m.input.HandleKey(msg)
		return m, nil
	}
}

// sendMessage dispatches user input to the agent and starts streaming.
func (m *TuiModel) sendMessage(input string) (tea.Model, tea.Cmd) {
	// Add user message to history
	m.messages = append(m.messages, ChatMessage{
		Role:      "user",
		Content:   input,
		Timestamp: time.Now(),
	})

	// Add to input history
	m.input.AddHistory(input)

	// Reset input
	m.input.Reset()

	// Start agent streaming
	m.streaming = true
	m.agentBusy = true
	m.spinnerOn = true
	m.streamBuf = ""

	// Update viewport
	m.viewport.SetMessages(m.messages)
	m.viewport.GotoBottom()

	// Run agent in goroutine, stream events as tea.Msg
	return m, m.startAgentStream(input)
}

// startAgentStream runs the agent stream loop in a goroutine,
// converting agent events into tea.Msg via tea.Cmd.
func (m *TuiModel) startAgentStream(input string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		stream, err := m.session.PromptStream(ctx, input)
		if err != nil {
			return AgentErrorMsg{Err: err}
		}

		var lastMsg tea.Msg = StreamDoneMsg{}

		for event := range stream {
			switch event.Type {
			case agent.StreamEventTextDelta:
				// Text deltas need to be sent live, but tea.Cmd can only return
				// one msg. We'll accumulate in the model and send the final state.
				m.streamBuf += event.TextDelta
				lastMsg = StreamTextMsg{Delta: event.TextDelta}

			case agent.StreamEventToolStart:
				lastMsg = ToolStartMsg{
					ID:   event.ToolCallID,
					Name: event.ToolName,
					Args: event.ToolArgs,
				}

			case agent.StreamEventToolEnd:
				lastMsg = ToolEndMsg{
					ID:      event.ToolCallID,
					Name:    event.ToolName,
					Result:  event.ToolResult,
					IsError: event.IsError,
				}

			case agent.StreamEventCompacted:
				lastMsg = CompactionMsg{Summary: event.Summary}

			case agent.StreamEventLoopDetected:
				lastMsg = LoopDetectedMsg{Tool: event.ToolName, Count: event.RepeatCount}

			case agent.StreamEventError:
				lastMsg = AgentErrorMsg{Err: fmt.Errorf("%s", event.Error)}

			case agent.StreamEventDone:
				// Stream complete
				return StreamDoneMsg{}
			}
		}

		return lastMsg
	}
}

// handleSlashCommand processes /commands locally.
func (m *TuiModel) handleSlashCommand(input string) (tea.Model, tea.Cmd) {
	m.input.Reset()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: m.session,
		App:     nil,
	}
	result, err := m.slashCmds.Execute(cmdCtx, input)
	if err != nil {
		m.messages = append(m.messages, ChatMessage{
			Role:    "system",
			Content: fmt.Sprintf("❌ Command error: %v", err),
		})
	} else {
		m.messages = append(m.messages, ChatMessage{
			Role:    "user",
			Content: input,
		})
		if result.Output != "" {
			m.messages = append(m.messages, ChatMessage{
				Role:    "system",
				Content: result.Output,
			})
		}
	}

	m.viewport.SetMessages(m.messages)
	m.viewport.GotoBottom()
	return m, nil
}
