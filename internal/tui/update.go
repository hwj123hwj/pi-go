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

// handleKeyPress processes all keyboard input, routing to the appropriate
// context handler based on what overlay/popup is active.
func (m *TuiModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Priority 1: Confirmation dialog ──
	if m.confirmation.IsActive() {
		approved, consumed := m.confirmation.HandleKey(msg)
		if consumed {
			if m.confirmation.resultChan != nil {
				// Dialog was resolved — send result
				m.resolveConfirmation(approved)
			}
			return m, nil
		}
	}

	// ── Priority 2: Completion popup active ──
	if m.completion.IsActive() {
		return m.handleCompletionKey(msg)
	}

	// ── Priority 3: Model selector popup (Ctrl+P) ──
	if m.modelSelect {
		return m.handleModelSelectKey(msg)
	}

	// ── Priority 4: Normal input context ──
	return m.handleInputKey(msg)
}

// handleInputKey handles keys in the normal input context.
func (m *TuiModel) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := DefaultKeyBindings.ResolveInput(msg)

	switch action {

	case ActionCancel: // Ctrl+C
		if m.agentBusy {
			m.streaming = false
			m.agentBusy = false
			m.spinnerOn = false
			m.streamBuf = ""
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case ActionExit: // Ctrl+D
		if m.input.IsEmpty() {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case ActionClearScreen: // Ctrl+L
		m.viewport.Clear()
		return m, nil

	case ActionToggleToolPanel: // Ctrl+O
		// Toggle the last tool panel's collapsed state
		if len(m.messages) > 0 && len(m.messages[len(m.messages)-1].Tools) > 0 {
			idx := len(m.messages) - 1
			lastTool := len(m.messages[idx].Tools) - 1
			m.messages[idx].Tools[lastTool].Collapsed = !m.messages[idx].Tools[lastTool].Collapsed
			m.viewport.SetMessages(m.messages)
		}
		return m, nil

	case ActionOpenModelSelect: // Ctrl+P
		// Build model list from registry
		if m.slashCmds == nil {
			return m, nil
		}
		// Try to get available models from the session's app context
		// For now, use a static list; Phase 3 will wire the real registry
		m.modelSelect = true
		return m, nil

	case ActionSubmit: // Enter
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

	case ActionNewline: // Ctrl+J
		m.input.newLine()
		return m, nil

	case ActionSearchHistory: // Ctrl+R
		// Phase 3: simple history search (cycle backward)
		m.input.navigateHistory(-1)
		return m, nil

	case ActionPageUp: // PgUp
		m.viewport.ScrollUp(m.viewport.height)
		return m, nil

	case ActionPageDown: // PgDn
		m.viewport.ScrollDown(m.viewport.height)
		return m, nil

	case ActionClosePopup: // Esc
		m.completion.Close()
		return m, nil

	default:
		// Delegate to input editor
		m.input.HandleKey(msg)

		// After each keystroke, check if we should trigger autocomplete
		m.checkTriggerCompletion()

		return m, nil
	}
}

// handleCompletionKey handles keys when the autocomplete popup is visible.
func (m *TuiModel) handleCompletionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := DefaultKeyBindings.ResolveCompletion(msg)

	switch action {
	case ActionAcceptCompletion: // Tab or Enter
		item := m.completion.SelectedItem()
		if item != nil {
			m.acceptCompletion(item.InsertText)
		}
		m.completion.Close()
		return m, nil

	case ActionHistoryNext: // Down
		m.completion.Next()
		return m, nil

	case ActionHistoryPrev: // Up
		m.completion.Prev()
		return m, nil

	case ActionClosePopup: // Esc
		m.completion.Close()
		return m, nil

	default:
		// Pass key to input, then re-evaluate completion
		m.input.HandleKey(msg)
		m.checkTriggerCompletion()
		return m, nil
	}
}

// handleModelSelectKey handles keys when the model selector popup is open.
func (m *TuiModel) handleModelSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlP:
		m.modelSelect = false
		return m, nil
	case tea.KeyEnter:
		// TODO: apply selected model
		m.modelSelect = false
		return m, nil
	case tea.KeyUp:
		m.completion.Prev()
		return m, nil
	case tea.KeyDown:
		m.completion.Next()
		return m, nil
	default:
		return m, nil
	}
}

// checkTriggerCompletion evaluates the current input and triggers the
// appropriate completion popup (slash commands or file paths).
func (m *TuiModel) checkTriggerCompletion() {
	if m.agentBusy {
		m.completion.Close()
		return
	}

	input := m.input.Text()
	cursorX := m.input.cursorX

	// Try slash command completion first
	if m.completion.TriggerSlash(input, cursorX, m.slashCmds) {
		return
	}

	// Try file path completion
	if m.completion.TriggerFile(input, cursorX, m.workspace) {
		return
	}

	// No trigger — close popup
	m.completion.Close()
}

// acceptCompletion replaces the trigger text in the input with the completion.
func (m *TuiModel) acceptCompletion(insertText string) {
	// For slash commands: replace from "/" to cursor with the command
	// For file paths: replace from "@" to cursor with the path
	// For model selector: handled separately

	switch m.completion.Kind() {
	case CompletionSlash:
		// Replace the partial /command with the full one
		fullText := m.input.Text()
		beforeCursor := substringBefore(fullText, m.input.cursorX)

		slashIdx := strings.LastIndex(beforeCursor, "/")
		if slashIdx < 0 {
			return
		}

		// Everything before the "/" + the completion + everything after cursor
		afterCursor := string([]rune(fullText)[m.input.cursorX:])
		newText := beforeCursor[:slashIdx] + insertText + " " + afterCursor

		m.input.lines = []string{newText}
		m.input.cursorX = slashIdx + len(insertText) + 1
		m.input.cursorY = 0

	case CompletionFile:
		// Replace from "@" to cursor
		fullText := m.input.Text()
		beforeCursor := substringBefore(fullText, m.input.cursorX)

		atIdx := strings.LastIndex(beforeCursor, "@")
		if atIdx < 0 {
			return
		}

		afterCursor := string([]rune(fullText)[m.input.cursorX:])
		newText := beforeCursor[:atIdx] + insertText + afterCursor

		m.input.lines = []string{newText}
		m.input.cursorX = atIdx + len(insertText)
		m.input.cursorY = 0
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
