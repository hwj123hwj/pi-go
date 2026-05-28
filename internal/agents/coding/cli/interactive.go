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

// clearScreen clears the terminal display using ANSI escape sequences.
func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

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
	fmt.Println(ui.FormatSessionStatus(m.session.SessionID(), provider, modelID, m.session.Profile(), cwd))
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
			cmdCtx := slashcmd.Context{
				Ctx:     ctx,
				Session: m.session,
				App:     m.app,
			}
			result, err := m.slashCmds.Execute(cmdCtx, input)
			if err != nil {
				fmt.Printf("error: %s\n", err)
			} else {
				// Handle clear screen first
				if result.ClearScreen {
					clearScreen()
					continue
				}
				// Handle session switch if the command requests it.
				// We only swap the pointer; the SessionRegistry owns the
				// lifecycle (including Close) for all registered sessions.
				if result.SessionSwitchTo != nil {
					oldID := m.session.SessionID()
					m.session = result.SessionSwitchTo.(*runtime.AgentSession)
					provider, modelID := m.session.ModelInfo()
					fmt.Printf("Session switched: %s → %s\n", oldID, m.session.SessionID())
					fmt.Println(ui.FormatSessionStatus(m.session.SessionID(), provider, modelID, m.session.Profile(), cwd))
					fmt.Println()
				}
				if result.Output != "" {
					fmt.Println(result.Output)
				}
			}
				// Handle ShouldQuery: auto-trigger agent execution
				if result.ShouldQuery {
					m.runPrompt(ctx, "Start working on the goal.")
					continue
				}
			continue
		}

		m.runPrompt(ctx, input)
	}
}

func (m *InteractiveMode) runPrompt(ctx context.Context, input string) {
	// Goal-driven prompts need much more time: use 30 minutes instead of 5.
	timeout := 5 * time.Minute
	if m.session != nil && m.session.Goal() != "" {
		timeout = 30 * time.Minute
	}
	promptCtx, cancel := context.WithTimeout(ctx, timeout)
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
