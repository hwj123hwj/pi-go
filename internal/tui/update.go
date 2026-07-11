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
		if len(m.messages) > 0 && len(m.messages[len(m.messages)-1].Tools) > 0 {
			idx := len(m.messages) - 1
			lastTool := len(m.messages[idx].Tools) - 1
			m.messages[idx].Tools[lastTool].Collapsed = !m.messages[idx].Tools[lastTool].Collapsed
			m.viewport.SetMessages(m.messages)
		}
		return m, nil

	case ActionOpenModelSelect: // Ctrl+P
		if m.slashCmds == nil {
			return m, nil
		}
		m.modelSelect = true
		return m, nil

	case ActionSubmit: // Enter
		input := m.input.Text()
		if input == "" {
			return m, nil
		}
		if slashcmd.IsSlashCommand(input) {
			return m.handleSlashCommand(input)
		}
		lower := strings.ToLower(strings.TrimSpace(input))
		if lower == "exit" || lower == "quit" {
			m.quitting = true
			return m, tea.Quit
		}
		return m.sendMessage(input)

	case ActionNewline: // Ctrl+J
		m.input.newLine()
		// Resize viewport to account for new input line
		m.viewport.Resize(m.width, m.height-m.inputHeight()-m.statusBarHeight())
		return m, nil

	case ActionSearchHistory: // Ctrl+R
		m.input.navigateHistory(-1)
		return m, nil

	case ActionPageUp:
		m.viewport.ScrollUp(m.viewport.height)
		return m, nil

	case ActionPageDown:
		m.viewport.ScrollDown(m.viewport.height)
		return m, nil

	case ActionClosePopup:
		m.completion.Close()
		return m, nil

	default:
		m.input.HandleKey(msg)
		m.checkTriggerCompletion()
		return m, nil
	}
}

// handleCompletionKey handles keys when the autocomplete popup is visible.
func (m *TuiModel) handleCompletionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := DefaultKeyBindings.ResolveCompletion(msg)

	switch action {
	case ActionAcceptCompletion:
		item := m.completion.SelectedItem()
		if item != nil {
			m.acceptCompletion(item.InsertText)
		}
		m.completion.Close()
		return m, nil

	case ActionHistoryNext:
		m.completion.Next()
		return m, nil

	case ActionHistoryPrev:
		m.completion.Prev()
		return m, nil

	case ActionClosePopup:
		m.completion.Close()
		return m, nil

	default:
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
// appropriate completion popup.
func (m *TuiModel) checkTriggerCompletion() {
	if m.agentBusy {
		m.completion.Close()
		return
	}
	input := m.input.Text()
	cursorX := m.input.cursorX

	if m.completion.TriggerSlash(input, cursorX, m.slashCmds) {
		return
	}
	if m.completion.TriggerFile(input, cursorX, m.workspace) {
		return
	}
	m.completion.Close()
}

// acceptCompletion replaces the trigger text in the input with the completion.
func (m *TuiModel) acceptCompletion(insertText string) {
	switch m.completion.Kind() {
	case CompletionSlash:
		fullText := m.input.Text()
		beforeCursor := substringBefore(fullText, m.input.cursorX)
		slashIdx := strings.LastIndex(beforeCursor, "/")
		if slashIdx < 0 {
			return
		}
		afterCursor := string([]rune(fullText)[m.input.cursorX:])
		newText := beforeCursor[:slashIdx] + insertText + " " + afterCursor
		m.input.lines = []string{newText}
		m.input.cursorX = slashIdx + len(insertText) + 1
		m.input.cursorY = 0

	case CompletionFile:
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
	m.input.AddHistory(input)
	m.input.Reset()

	// Start agent streaming
	m.streaming = true
	m.agentBusy = true
	m.spinnerOn = true
	m.streamBuf = ""

	m.viewport.SetMessages(m.messages)
	m.viewport.GotoBottom()

	return m, m.startAgentStream(input)
}

// startAgentStream runs the agent stream loop in a goroutine.
// CRITICAL: We must NOT mutate model fields from inside this goroutine.
// Instead, we send each event to the Bubble Tea program via program.Send(),
// which safely delivers it to the Update() function on the main goroutine.
func (m *TuiModel) startAgentStream(input string) tea.Cmd {
	program := m.program // capture before goroutine starts
	return func() tea.Msg {
		ctx := context.Background()
		stream, err := m.session.PromptStream(ctx, input)
		if err != nil {
			return AgentErrorMsg{Err: err}
		}

		for event := range stream {
			var msg tea.Msg
			switch event.Type {
			case agent.StreamEventTextDelta:
				msg = StreamTextMsg{Delta: event.TextDelta}

			case agent.StreamEventToolStart:
				msg = ToolStartMsg{
					ID:   event.ToolCallID,
					Name: event.ToolName,
					Args: event.ToolArgs,
				}

			case agent.StreamEventToolUpdate:
				msg = ToolUpdateMsg{
					ID:     event.ToolCallID,
					Name:   event.ToolName,
					Result: event.ToolResult,
				}

			case agent.StreamEventToolEnd:
				msg = ToolEndMsg{
					ID:      event.ToolCallID,
					Name:    event.ToolName,
					Result:  event.ToolResult,
					IsError: event.IsError,
				}

			case agent.StreamEventCompacted:
				msg = CompactionMsg{Summary: event.Summary}

			case agent.StreamEventMicroCompacted:
				msg = CompactionMsg{Summary: "micro-compact: " + event.Summary}

			case agent.StreamEventLoopDetected:
				msg = LoopDetectedMsg{Tool: event.ToolName, Count: event.RepeatCount}

			case agent.StreamEventError:
				msg = AgentErrorMsg{Err: fmt.Errorf("%s", event.Error)}

			case agent.StreamEventDone:
				msg = StreamDoneMsg{}

			case agent.StreamEventTurnEnd:
				// Turn ended but stream may continue (multi-turn)
				continue
			}

			// Send each event immediately to the TUI for live updates.
			// This is the standard Bubble Tea pattern for async streaming.
			if program != nil && msg != nil {
				program.Send(msg)
			}
		}

		// Signal completion
		return StreamDoneMsg{}
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
