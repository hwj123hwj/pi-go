package commands

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/earendil-works/pi-go/internal/slashcmd"
)

// RegisterBuiltins registers coding-agent slash commands into the shared framework registry.
func RegisterBuiltins(registry *slashcmd.Registry) {
	registry.Register(slashcmd.Command{
		Name:        "help",
		Description: "List all commands",
		Handler: func(ctx slashcmd.Context, args string) (string, error) {
			return formatHelp(registry), nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "compact",
		Description: "Manually trigger context compaction",
		Handler: func(ctx slashcmd.Context, args string) (string, error) {
			return "Context compaction runs automatically when the conversation grows too long.\nManual compaction is not yet implemented — it will be available in a future update.", nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "sessions",
		Description: "List all sessions",
		Handler: func(ctx slashcmd.Context, args string) (string, error) {
			if ctx.App == nil {
				return "app not available", nil
			}
			sessions, err := ctx.App.ListSessionsInfo()
			if err != nil {
				return "", fmt.Errorf("list sessions: %w", err)
			}
			if len(sessions) == 0 {
				return "no sessions found", nil
			}

			currentID := ""
			if ctx.Session != nil {
				currentID = ctx.Session.SessionID()
			}

			var b strings.Builder
			b.WriteString("Sessions:\n")
			for _, s := range sessions {
				marker := "  "
				if s.ID == currentID {
					marker = "→ "
				}
				lastActive := time.Unix(s.LastActive, 0).Format("2006-01-02 15:04")
				b.WriteString(fmt.Sprintf("%s%-20s  messages=%-4d  last_active=%s\n", marker, s.ID, s.MessageCount, lastActive))
			}
			return b.String(), nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "session",
		Description: "Show current session info",
		Handler: func(ctx slashcmd.Context, args string) (string, error) {
			if ctx.Session == nil {
				return "no active session", nil
			}
			provider, modelID := ctx.Session.ModelInfo()
			tools := ctx.Session.ToolNames()
			return fmt.Sprintf("Session:  %s\nModel:    %s/%s\nTools:    %s",
				ctx.Session.SessionID(),
				provider, modelID,
				strings.Join(tools, ", ")), nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "branch",
		Description: "Switch to a specific entry (branch navigation)",
		Handler: func(ctx slashcmd.Context, args string) (string, error) {
			return "Branch navigation is not yet implemented. It will be available in a future update.", nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "new",
		Description: "Create a new session",
		Handler: func(ctx slashcmd.Context, args string) (string, error) {
			if ctx.App == nil {
				return "app not available — cannot create new session", nil
			}
			newSession, err := ctx.App.CreateSession(ctx.Ctx)
			if err != nil {
				return "", fmt.Errorf("create session: %w", err)
			}
			ctx.Session = newSession
			provider, modelID := newSession.ModelInfo()
			return fmt.Sprintf("Created new session: %s\nModel: %s/%s\nReady for input.", newSession.SessionID(), provider, modelID), nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "tools",
		Description: "Show available tools",
		Handler: func(ctx slashcmd.Context, args string) (string, error) {
			if ctx.Session == nil {
				return "no active session", nil
			}
			tools := ctx.Session.ToolNames()
			if len(tools) == 0 {
				return "no tools available", nil
			}
			return "Available tools:\n  " + strings.Join(tools, "\n  "), nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "model",
		Description: "Show or switch model (/model [provider:]model_name)",
		Handler: func(ctx slashcmd.Context, args string) (string, error) {
			if ctx.Session == nil {
				return "no active session", nil
			}
			provider, modelID := ctx.Session.ModelInfo()

			newInput := strings.TrimSpace(args)
			if newInput == "" {
				return fmt.Sprintf("Current model: %s/%s", provider, modelID), nil
			}

			var newModel, newProvider string
			if parts := strings.SplitN(newInput, ":", 2); len(parts) == 2 {
				newProvider = parts[0]
				newModel = parts[1]
			} else {
				newModel = newInput
			}

			if err := ctx.Session.SwitchModel(ctx.Ctx, newModel, newProvider); err != nil {
				return "", fmt.Errorf("switch model: %w", err)
			}
			newP, newM := ctx.Session.ModelInfo()
			return fmt.Sprintf("Switched: %s/%s → %s/%s", provider, modelID, newP, newM), nil
		},
	})
}

func formatHelp(registry *slashcmd.Registry) string {
	names := registry.Names()

	var sessionCmds, infoCmds, actionCmds []string
	for _, name := range names {
		switch name {
		case "sessions", "session", "new", "branch":
			sessionCmds = append(sessionCmds, name)
		case "help", "tools", "model":
			infoCmds = append(infoCmds, name)
		default:
			actionCmds = append(actionCmds, name)
		}
	}

	sort.Strings(sessionCmds)
	sort.Strings(infoCmds)
	sort.Strings(actionCmds)

	var b strings.Builder
	b.WriteString("Available commands:\n\n")

	if len(sessionCmds) > 0 {
		b.WriteString("  Session management:\n")
		for _, name := range sessionCmds {
			cmd := registry.Command(name)
			b.WriteString(fmt.Sprintf("    %-14s %s\n", "/"+name, cmd.Description))
		}
		b.WriteString("\n")
	}

	if len(infoCmds) > 0 {
		b.WriteString("  Information:\n")
		for _, name := range infoCmds {
			cmd := registry.Command(name)
			b.WriteString(fmt.Sprintf("    %-14s %s\n", "/"+name, cmd.Description))
		}
		b.WriteString("\n")
	}

	if len(actionCmds) > 0 {
		b.WriteString("  Actions:\n")
		for _, name := range actionCmds {
			cmd := registry.Command(name)
			b.WriteString(fmt.Sprintf("    %-14s %s\n", "/"+name, cmd.Description))
		}
		b.WriteString("\n")
	}

	return b.String()
}
