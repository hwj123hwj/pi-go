package mode

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/app"
	"github.com/earendil-works/pi-go/internal/runtime"
	"github.com/earendil-works/pi-go/internal/slashcmd"
)

// InteractiveMode handles the interactive chat UI loop.
// It only handles stdin/stdout presentation — no session business logic.
type InteractiveMode struct {
	session   *runtime.AgentSession
	slashCmds *slashcmd.Registry
	app       *app.App
}

// NewInteractiveMode creates a new interactive mode.
func NewInteractiveMode(session *runtime.AgentSession, cmds *slashcmd.Registry, application *app.App) *InteractiveMode {
	return &InteractiveMode{
		session:   session,
		slashCmds: cmds,
		app:       application,
	}
}

// Run starts the interactive chat loop.
func (m *InteractiveMode) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("pi-go chat mode. Type your message and press Enter. Ctrl-D to exit.")
	fmt.Printf("Session: %s\n\n", m.session.SessionID())

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

		// Handle slash commands
		if slashcmd.IsSlashCommand(input) {
			cmdCtx := slashcmd.Context{
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
		promptCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)

		stream, err := m.session.PromptStream(promptCtx, input)
		if err != nil {
			fmt.Println("error:", err)
			cancel()
			continue
		}

		fmt.Print("Pi> ")
		for event := range stream {
			switch event.Type {
			case agent.StreamEventTextDelta:
				fmt.Print(event.TextDelta)
			case agent.StreamEventToolStart:
				fmt.Printf("\n[tool:%s] ", event.ToolName)
			case agent.StreamEventToolEnd:
				fmt.Print("✓ ")
			case agent.StreamEventCompacted:
				fmt.Printf("\n[compacted: %d chars trimmed] ", len(event.Summary))
			case agent.StreamEventDone:
				fmt.Println()
			case agent.StreamEventError:
				fmt.Printf("\nerror: %s\n", event.Error)
			}
		}

		cancel()
		fmt.Println()
	}
}
