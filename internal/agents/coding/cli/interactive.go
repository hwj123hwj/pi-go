package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/earendil-works/pi-go/internal/app"
	"github.com/earendil-works/pi-go/internal/runtime"
	"github.com/earendil-works/pi-go/internal/slashcmd"
	"github.com/earendil-works/pi-go/internal/ui"
)

// InteractiveMode is the coding-agent CLI conversation loop.
// It owns coding-specific command handling and terminal presentation behavior.
type InteractiveMode struct {
	session   *runtime.AgentSession
	slashCmds *slashcmd.Registry
	app       *app.App
	presenter *ui.Presenter
}

// NewInteractiveMode creates a coding-agent interactive CLI.
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
	fmt.Println(ui.FormatSessionStatus(m.session.SessionID(), provider, modelID, cwd))
	fmt.Println()

	for {
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

		if slashcmd.IsSlashCommand(input) {
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

		m.runPrompt(ctx, input)
	}
}

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

func (m *InteractiveMode) handleNewSession(ctx context.Context) error {
	if m.app == nil {
		fmt.Println("error: app not available — cannot create new session")
		return nil
	}

	newSession, err := m.app.NewSession(ctx)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	oldID := m.session.SessionID()
	_ = m.session.Close()
	m.session = newSession

	provider, modelID := m.session.ModelInfo()
	cwd, _ := os.Getwd()
	fmt.Printf("Created new session: %s (previous: %s)\n", m.session.SessionID(), oldID)
	fmt.Println(ui.FormatSessionStatus(m.session.SessionID(), provider, modelID, cwd))
	fmt.Println()
	return nil
}
