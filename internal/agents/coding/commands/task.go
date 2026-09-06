package commands

import (
	"fmt"
	"strings"

	"github.com/hwj123hwj/pi-go/internal/handoff"
	"github.com/hwj123hwj/pi-go/sdk/slashcmd"
	"github.com/hwj123hwj/pi-go/sdk/util"
)

// RegisterTaskCommands registers /task slash commands for TASK.md handoff management.
func RegisterTaskCommands(registry *slashcmd.Registry) {
	registry.Register(slashcmd.Command{
		Name:        "task",
		Description: "Manage task handoff context (/task show|clear|done)",
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			workspace := util.CWD()
			args = strings.TrimSpace(args)

			switch {
			case args == "" || args == "show":
				// Show current task
				doc, err := handoff.Load(workspace)
				if err != nil {
					return slashcmd.CommandResult{}, fmt.Errorf("load task: %w", err)
				}
				if doc == nil {
					return slashcmd.CommandResult{Output: "No active task handoff. Use /task <goal> to create one."}, nil
				}
				return slashcmd.CommandResult{Output: doc.Render()}, nil

			case args == "clear":
				// Remove task file
				if err := handoff.Clear(workspace); err != nil {
					return slashcmd.CommandResult{}, fmt.Errorf("clear task: %w", err)
				}
				return slashcmd.CommandResult{Output: "📋 Task handoff cleared."}, nil

			case args == "done" || args == "complete":
				// Mark task as completed
				if err := handoff.MarkComplete(workspace); err != nil {
					return slashcmd.CommandResult{}, fmt.Errorf("mark task complete: %w", err)
				}
				return slashcmd.CommandResult{Output: "✅ Task marked as completed!"}, nil

			case args == "save":
				// Save current session state as task handoff
				// Reads goal from session if available
				goal := ""
				if ctx.Session != nil {
					goal = ctx.Session.Goal()
				}
				if goal == "" {
					goal = "Unspecified task"
				}
				doc := handoff.NewTaskDocument(goal)
				if err := handoff.Save(workspace, doc); err != nil {
					return slashcmd.CommandResult{}, fmt.Errorf("save task: %w", err)
				}
				return slashcmd.CommandResult{Output: fmt.Sprintf("📋 Task handoff saved!\n  Goal: %s\n  Location: %s/%s/%s",
					goal, workspace, handoff.TaskDir, handoff.TaskFileName)}, nil

			default:
				// Treat args as the goal text → create new task
				doc := handoff.NewTaskDocument(args)
				if err := handoff.Save(workspace, doc); err != nil {
					return slashcmd.CommandResult{}, fmt.Errorf("create task: %w", err)
				}
				return slashcmd.CommandResult{
					Output: fmt.Sprintf("📋 Task handoff created!\n  Goal: %s\n  State: %s\n\nThis task will be auto-loaded on session resume.", args, doc.State),
				}, nil
			}
		},
	})
}
