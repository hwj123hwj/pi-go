package slashcmd

import (
	"fmt"
	"sort"
	"strings"
)

// Command defines a slash command.
type Command struct {
	Name        string
	Description string
	Handler     func(ctx Context, args string) (string, error)
}

// Registry manages slash commands.
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates a new empty slash command registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
	}
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd Command) {
	r.commands[cmd.Name] = cmd
}

// Execute parses and executes a slash command.
// input should be the full input string (e.g., "/help", "/compact reason").
func (r *Registry) Execute(cmdCtx Context, input string) (string, error) {
	name, args := ParseSlashCommand(input)
	cmd, ok := r.commands[name]
	if !ok {
		return "", fmt.Errorf("unknown command: %s", name)
	}
	return cmd.Handler(cmdCtx, args)
}

// IsSlashCommand returns true if the input starts with "/".
func IsSlashCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), "/")
}

// Help returns a formatted help string listing all commands.
func (r *Registry) Help() string {
	var names []string
	for name := range r.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("Available commands:\n")
	for _, name := range names {
		cmd := r.commands[name]
		b.WriteString(fmt.Sprintf("  %-12s %s\n", name, cmd.Description))
	}
	return b.String()
}

// Names returns all registered command names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Command returns the command with the given name.
// Returns a zero-value Command if not found.
func (r *Registry) Command(name string) Command {
	return r.commands[name]
}

// ParseSlashCommand splits input into command name and args.
// Exported for callers that need to inspect the command name before execution.
func ParseSlashCommand(input string) (string, string) {
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "/")
	parts := strings.SplitN(input, " ", 2)
	name := parts[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return name, args
}
