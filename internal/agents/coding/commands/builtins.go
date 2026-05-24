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
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			return slashcmd.CommandResult{Output: formatHelp(registry)}, nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "compact",
		Description: "Manually trigger context compaction",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			return slashcmd.CommandResult{Output: "Context compaction runs automatically when the conversation grows too long.\nManual compaction is not yet implemented — it will be available in a future update."}, nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "sessions",
		Description: "List all sessions",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			if ctx.App == nil {
				return slashcmd.CommandResult{Output: "app not available"}, nil
			}
			sessions, err := ctx.App.ListSessionsInfo()
			if err != nil {
				return slashcmd.CommandResult{}, fmt.Errorf("list sessions: %w", err)
			}
			if len(sessions) == 0 {
				return slashcmd.CommandResult{Output: "no sessions found"}, nil
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
			return slashcmd.CommandResult{Output: b.String()}, nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "session",
		Description: "Show current session info",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			if ctx.Session == nil {
				return slashcmd.CommandResult{Output: "no active session"}, nil
			}
			provider, modelID := ctx.Session.ModelInfo()
			tools := ctx.Session.ToolNames()
			profile := ctx.Session.Profile()
			return slashcmd.CommandResult{Output: fmt.Sprintf("Session:  %s\nModel:    %s/%s\nProfile:  %s\nTools:    %s",
				ctx.Session.SessionID(),
				provider, modelID,
				profile,
				strings.Join(tools, ", "))}, nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "branch",
		Description: "Switch to a specific entry (branch navigation) [planned]",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			return slashcmd.CommandResult{Output: "Branch navigation is not yet implemented. It will be available in a future update."}, nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "new",
		Description: "Create a new session",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			if ctx.App == nil {
				return slashcmd.CommandResult{Output: "app not available — cannot create new session"}, nil
			}
			newSession, err := ctx.App.CreateSession(ctx.Ctx)
			if err != nil {
				return slashcmd.CommandResult{}, fmt.Errorf("create session: %w", err)
			}
			provider, modelID := newSession.ModelInfo()
			output := fmt.Sprintf("Created new session: %s\nModel: %s/%s\nReady for input.", newSession.SessionID(), provider, modelID)
			return slashcmd.CommandResult{
				Output:          output,
				SessionSwitchTo: newSession,
			}, nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "switch",
		Description: "Switch to an existing session by ID",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			sessionID := strings.TrimSpace(args)
			if sessionID == "" {
				return slashcmd.CommandResult{Output: "Usage: /switch <session-id>"}, nil
			}
			if ctx.App == nil {
				return slashcmd.CommandResult{Output: "app not available — cannot switch session"}, nil
			}
			targetSession, err := ctx.App.SwitchSession(ctx.Ctx, sessionID)
			if err != nil {
				return slashcmd.CommandResult{}, fmt.Errorf("switch session: %w", err)
			}
			provider, modelID := targetSession.ModelInfo()
			output := fmt.Sprintf("Switched to session: %s\nModel: %s/%s", targetSession.SessionID(), provider, modelID)
			return slashcmd.CommandResult{
				Output:          output,
				SessionSwitchTo: targetSession,
			}, nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "tools",
		Description: "Show available tools",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			if ctx.Session == nil {
				return slashcmd.CommandResult{Output: "no active session"}, nil
			}
			tools := ctx.Session.ToolNames()
			if len(tools) == 0 {
				return slashcmd.CommandResult{Output: "no tools available"}, nil
			}
			return slashcmd.CommandResult{Output: "Available tools:\n  " + strings.Join(tools, "\n  ")}, nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "model",
		Description: "Show or switch model (/model [provider:]model_name)",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			if ctx.Session == nil {
				return slashcmd.CommandResult{Output: "no active session"}, nil
			}
			provider, modelID := ctx.Session.ModelInfo()

			newInput := strings.TrimSpace(args)
			if newInput == "" {
				return slashcmd.CommandResult{Output: fmt.Sprintf("Current model: %s/%s", provider, modelID)}, nil
			}

			var newModel, newProvider string
			if parts := strings.SplitN(newInput, ":", 2); len(parts) == 2 {
				newProvider = parts[0]
				newModel = parts[1]
			} else {
				newModel = newInput
			}

			if err := ctx.Session.SwitchModel(ctx.Ctx, newModel, newProvider); err != nil {
				return slashcmd.CommandResult{}, fmt.Errorf("switch model: %w", err)
			}
			newP, newM := ctx.Session.ModelInfo()
			return slashcmd.CommandResult{Output: fmt.Sprintf("Switched: %s/%s → %s/%s", provider, modelID, newP, newM)}, nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "profiles",
		Description: "List all available profiles",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			if ctx.App == nil {
				return slashcmd.CommandResult{Output: "app not available"}, nil
			}
			profiles := ctx.App.Profiles()
			currentProfile := ""
			if ctx.Session != nil {
				currentProfile = ctx.Session.Profile()
			}
			var b strings.Builder
			b.WriteString("Profiles:\n")
			for _, p := range profiles {
				marker := "  "
				if p == currentProfile {
					marker = "→ "
				}
				b.WriteString(fmt.Sprintf("%s%s\n", marker, p))
			}
			return slashcmd.CommandResult{Output: b.String()}, nil
		},
	})

	registry.Register(slashcmd.Command{
		Name:        "profile",
		Description: "Show or switch profile (/profile [name])",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			if ctx.Session == nil {
				return slashcmd.CommandResult{Output: "no active session"}, nil
			}
			name := strings.TrimSpace(args)
			if name == "" {
				return slashcmd.CommandResult{Output: fmt.Sprintf("Current profile: %s", ctx.Session.Profile())}, nil
			}
			oldProfile := ctx.Session.Profile()
			if err := ctx.Session.SwitchProfile(ctx.Ctx, name); err != nil {
				return slashcmd.CommandResult{}, err
			}
			return slashcmd.CommandResult{Output: fmt.Sprintf("Switched profile: %s → %s", oldProfile, ctx.Session.Profile())}, nil
		},
	})
}

func formatHelp(registry *slashcmd.Registry) string {
	names := registry.Names()

	var sessionCmds, infoCmds, actionCmds []string
	for _, name := range names {
		switch name {
		case "sessions", "session", "new", "switch", "branch":
			sessionCmds = append(sessionCmds, name)
		case "help", "tools", "model", "profiles", "profile":
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
			desc := cmd.Description
			b.WriteString(fmt.Sprintf("    %-14s %s\n", "/"+name, desc))
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
