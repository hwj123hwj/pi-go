package mode

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/earendil-works/pi-go/internal/app"
	"github.com/earendil-works/pi-go/internal/runtime"
	"github.com/earendil-works/pi-go/internal/slashcmd"
	"github.com/earendil-works/pi-go/internal/ui"
)

// InteractiveMode handles the interactive chat UI loop.
// It only handles stdin/stdout presentation — no session business logic.
type InteractiveMode struct {
	session   *runtime.AgentSession
	slashCmds *slashcmd.Registry
	app       *app.App
	presenter *ui.Presenter
}

// NewInteractiveMode creates a new interactive mode.
func NewInteractiveMode(session *runtime.AgentSession, cmds *slashcmd.Registry, application *app.App) *InteractiveMode {
	return &InteractiveMode{
		session:   session,
		slashCmds: cmds,
		app:       application,
		presenter: ui.NewPresenter(os.Stdout),
	}
}

// Run starts the interactive chat loop.
func (m *InteractiveMode) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	provider, modelID := m.session.ModelInfo()
	cwd, _ := os.Getwd()

	fmt.Println("pi-go chat mode. Type your message and press Enter. Ctrl-D to exit.")
	fmt.Printf("Session: %s | Model: %s/%s\n", m.session.SessionID(), provider, modelID)
	fmt.Printf("CWD: %s\n\n", cwd)

	for {
		// Show prompt with model indicator
		fmt.Print("You> ")
		if !scanner.Scan() {
			return nil
		}
		input := scanner.Text()
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			return nil
		}

		// Handle slash commands
		if slashcmd.IsSlashCommand(input) {
			// Special handling for /new which needs to replace the session
			cmdName, _ := slashcmd.ParseSlashCommand(input)
			if cmdName == "new" {
				if err := m.handleNewSession(ctx); err != nil {
					fmt.Printf("error: %s\n", err)
				}
				continue
			}

			cmdCtx := slashcmd.Context{
				Ctx:     ctx,
				Session: m.session,
				App:     m.app,
			}
			output, err := m.slashCmds.Execute(cmdCtx, input)
			if err != nil {
				fmt.Printf("error: %s\n", err)
			} else {
				fmt.Println(output)
			}
			continue
		}

		// Normal prompt via streaming
		m.runPrompt(ctx, input)
	}
}

// runPrompt executes a streaming prompt and renders events.
func (m *InteractiveMode) runPrompt(ctx context.Context, input string) {
	promptCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	stream, err := m.session.PromptStream(promptCtx, input)
	if err != nil {
		fmt.Printf("error: %s\n", err)
		return
	}

	fmt.Print("Pi> ")
	for event := range stream {
		m.presenter.Present(event)
	}
	fmt.Println()
}

// handleNewSession creates a new session and switches to it.
func (m *InteractiveMode) handleNewSession(ctx context.Context) error {
	if m.app == nil {
		fmt.Println("error: app not available — cannot create new session")
		return nil
	}

	newSession, err := m.app.NewSession(ctx)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Close old session
	oldID := m.session.SessionID()
	m.session.Close()

	// Switch to new session
	m.session = newSession

	provider, modelID := m.session.ModelInfo()
	cwd, _ := os.Getwd()
	fmt.Printf("Created new session: %s (previous: %s)\n", m.session.SessionID(), oldID)
	fmt.Printf("Session: %s | Model: %s/%s | CWD: %s\n\n", m.session.SessionID(), provider, modelID, filepath.Base(cwd))
	return nil
}
