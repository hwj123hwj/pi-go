package extensions

import (
	"context"

	"github.com/hwj123hwj/pi-go/internal/agent"
)

// CommandDef defines a slash command contributed by an extension.
// This mirrors slashcmd.Command but avoids the import cycle.
type CommandDef struct {
	Name        string
	Description string
	Handler     func(args string) (string, error)
}

// Extension is the interface that all extensions must implement.
// MVP: in-process registration only, no .so dynamic loading.
type Extension interface {
	// Name returns the unique name of the extension.
	Name() string

	// Init is called when the extension is registered.
	Init(ctx InitContext) error

	// Tools returns additional tools provided by this extension.
	Tools() []agent.Tool

	// Commands returns additional slash commands provided by this extension.
	Commands() []CommandDef

	// Hooks returns event hooks that this extension wants to listen to.
	Hooks() []Hook
}

// InitContext provides initialization context for extensions.
type InitContext struct {
	Workspace string
	Config    map[string]any
}

// Hook represents an event hook that an extension wants to listen to.
type Hook struct {
	// Event is the event name to listen to.
	Event string

	// Handler is called when the event fires.
	Handler func(ctx context.Context, data any) error
}

// ExtensionWithLifecycle is an optional interface that extensions can implement
// to contribute before/after tool-call hooks. If implemented, the registry
// automatically collects these hooks during Register().
type ExtensionWithLifecycle interface {
	Extension
	BeforeToolCallHooks() []agent.BeforeToolCallHook
	AfterToolCallHooks() []agent.AfterToolCallHook
}

// ExtensionWithSessionHooks is an optional interface for extensions that want
// to observe session start/end. Hooks are non-blocking (observer-only).
// Kept separate from ExtensionWithLifecycle so extensions can opt in granularly.
type ExtensionWithSessionHooks interface {
	Extension
	SessionStartHooks() []agent.SessionStartHook
	SessionEndHooks() []agent.SessionEndHook
}

// ExtensionWithCompressHook is an optional interface for extensions that want
// to observe context compaction. Hook is non-blocking (observer-only).
type ExtensionWithCompressHook interface {
	Extension
	PreCompressHooks() []agent.PreCompressHook
}
