package slashcmd

import (
	"fmt"
	"strings"
)

// RegisterBuiltins registers the built-in slash commands into the given registry.
func RegisterBuiltins(registry *Registry) {
	registry.Register(Command{
		Name:        "help",
		Description: "List all commands",
		Handler: func(ctx Context, args string) (string, error) {
			return registry.Help(), nil
		},
	})

	registry.Register(Command{
		Name:        "compact",
		Description: "Manually trigger context compaction",
		Handler: func(ctx Context, args string) (string, error) {
			return "compaction triggered (runs automatically when needed)", nil
		},
	})

	registry.Register(Command{
		Name:        "sessions",
		Description: "List all sessions",
		Handler: func(ctx Context, args string) (string, error) {
			if ctx.App == nil {
				return "app not available", nil
			}
			sessions, err := ctx.App.ListSessionsInfo()
			if err != nil {
				return "", fmt.Errorf("list sessions: %w", err)
			}
			if len(sessions) == 0 {
				return "no sessions", nil
			}
			var b strings.Builder
			for _, s := range sessions {
				b.WriteString(fmt.Sprintf("  %s  messages=%d  last_active=%d\n", s.ID, s.MessageCount, s.LastActive))
			}
			return b.String(), nil
		},
	})

	registry.Register(Command{
		Name:        "session",
		Description: "Show current session info",
		Handler: func(ctx Context, args string) (string, error) {
			if ctx.Session == nil {
				return "no active session", nil
			}
			return fmt.Sprintf("session: %s", ctx.Session.SessionID()), nil
		},
	})

	registry.Register(Command{
		Name:        "branch",
		Description: "Switch to a specific entry (branch navigation)",
		Handler: func(ctx Context, args string) (string, error) {
			entryID := strings.TrimSpace(args)
			if entryID == "" {
				return "usage: /branch <entry_id>", nil
			}
			return fmt.Sprintf("branch to entry %s (not yet implemented in slash commands)", entryID), nil
		},
	})

	registry.Register(Command{
		Name:        "new",
		Description: "Create a new session",
		Handler: func(ctx Context, args string) (string, error) {
			return "use Ctrl-D to exit and restart with a new session", nil
		},
	})

	registry.Register(Command{
		Name:        "tools",
		Description: "Show available tools",
		Handler: func(ctx Context, args string) (string, error) {
			return "Built-in tools: bash, read, write, edit, grep, find, ls", nil
		},
	})

	registry.Register(Command{
		Name:        "model",
		Description: "Show or switch model (/model [model_name])",
		Handler: func(ctx Context, args string) (string, error) {
			if ctx.Session == nil {
				return "no active session", nil
			}
			provider, modelID := ctx.Session.ModelInfo()

			newModel := strings.TrimSpace(args)
			if newModel == "" {
				// 无参数：显示当前模型
				return fmt.Sprintf("provider: %s, model: %s", provider, modelID), nil
			}

			// 有参数：切换模型
			if err := ctx.Session.SwitchModel(ctx.Ctx, newModel); err != nil {
				return "", fmt.Errorf("switch model: %w", err)
			}
			newProvider, newModelID := ctx.Session.ModelInfo()
			return fmt.Sprintf("switched: %s/%s -> %s/%s", provider, modelID, newProvider, newModelID), nil
		},
	})
}
