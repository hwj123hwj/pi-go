package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hwj123hwj/pi-go/internal/scheduler"
	"github.com/hwj123hwj/pi-go/internal/slashcmd"
)

// RegisterLoopCommands registers /loop and /crontab slash commands.
// It requires a LoopManager to be passed from the app layer.
func RegisterLoopCommands(registry *slashcmd.Registry, loopMgr *scheduler.LoopManager) {
	registerLoopCommand(registry, loopMgr)
}

func registerLoopCommand(registry *slashcmd.Registry, loopMgr *scheduler.LoopManager) {
	registry.Register(slashcmd.Command{
		Name:        "loop",
		Description: "Schedule a recurring task (e.g. /loop 5m run tests)",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			if ctx.Session == nil {
				return slashcmd.CommandResult{Output: "no active session"}, nil
			}
			sessionID := ctx.Session.SessionID()
			args = strings.TrimSpace(args)

			// ── /loop clear ──
			if args == "clear" || args == "stop" {
				if loopMgr.Get(sessionID) == nil {
					return slashcmd.CommandResult{Output: "No active loop to clear."}, nil
				}
				loopMgr.Stop(sessionID)
				return slashcmd.CommandResult{Output: "🔄 Loop stopped."}, nil
			}

			// ── /loop (no args) — show status ──
			if args == "" {
				lc := loopMgr.Get(sessionID)
				if lc == nil {
					return slashcmd.CommandResult{Output: loopHelp()}, nil
				}
				remaining := lc.Remaining()
				if remaining < 0 {
					remaining = 0
				}
				lastRun := "never"
				if !lc.LastRunAt.IsZero() {
					lastRun = lc.LastRunAt.Format("15:04:05")
				}
				return slashcmd.CommandResult{Output: fmt.Sprintf(
					"🔄 Active Watchdog Loop:\n  Prompt: %q\n  Interval: %s\n  Status: Running (expires in %s)\n  Started: %s\n  Last Run: %s\n\nTo stop: /loop clear",
					lc.Prompt,
					scheduler.FormatDuration(lc.Interval),
					scheduler.FormatDuration(remaining),
					lc.StartedAt.Format("15:04:05"),
					lastRun,
				)}, nil
			}

			// ── /loop <interval> <prompt> [--expires <duration>] ──
			// Parse --expires option
			var expiresTTL time.Duration
			expiresRegex := "--expires"
			cleanArgs := args
			if idx := strings.Index(args, expiresRegex); idx >= 0 {
				rest := strings.TrimSpace(args[idx+len(expiresRegex):])
				if len(rest) > 0 {
					parts := strings.Fields(rest)
					if len(parts) > 0 {
						parsed, ok := scheduler.ParseDuration(parts[0])
						if !ok {
							return slashcmd.CommandResult{Output: fmt.Sprintf(
								"Invalid expires duration %q. Use e.g. \"1h\", \"30m\".", parts[0])}, nil
						}
						expiresTTL = parsed
						cleanArgs = strings.TrimSpace(args[:idx])
					}
				}
			}

			// Parse interval + prompt
			argParts := strings.Fields(cleanArgs)
			if len(argParts) < 2 {
				return slashcmd.CommandResult{Output: loopHelp()}, nil
			}

			intervalStr := argParts[0]
			interval, ok := scheduler.ParseDuration(intervalStr)
			if !ok {
				return slashcmd.CommandResult{Output: fmt.Sprintf(
					"Invalid interval %q. Use e.g. \"5m\", \"1h\".", intervalStr)}, nil
			}

			if interval < time.Minute {
				return slashcmd.CommandResult{Output: "Minimum loop interval is 1 minute (60s / 1m) to prevent rate limiting."}, nil
			}

			promptText := strings.Join(argParts[1:], " ")
			if promptText == "" {
				return slashcmd.CommandResult{Output: "Prompt cannot be empty."}, nil
			}

			// Start the loop with a no-op trigger. The actual prompt injection
			// is wired by the serve/feishu mode layer via a TriggerResolver.
			_, err := loopMgr.Start(sessionID, promptText, interval, expiresTTL, func(ctx context.Context, _ string) error {
				return nil
			})
			if err != nil {
				return slashcmd.CommandResult{}, fmt.Errorf("start loop: %w", err)
			}

			return slashcmd.CommandResult{
				Output: fmt.Sprintf(
					"🔄 Loop scheduled! Will run %q every %s.\nTo stop: /loop clear",
					promptText,
					scheduler.FormatDuration(interval),
				),
				ShouldQuery: false,
			}, nil
		},
	})
}

func loopHelp() string {
	return strings.TrimSpace(`
🔄 /loop — Schedule a recurring task in the current session

Usage:
  /loop <interval> <prompt>           Start a watchdog loop
  /loop clear                          Stop the active loop
  /loop                                Show current loop status

Options:
  --expires <duration>   Set max lifetime (default: 3 days)

Supported intervals: s (seconds), m (minutes), h (hours)
Minimum interval: 1 minute

Examples:
  /loop 5m run go test ./...
  /loop 1h check CI status
  /loop 30m review new pull requests --expires 8h
`)
}
