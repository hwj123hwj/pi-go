package extensions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/earendil-works/pi-go/internal/agent"
)

// Registry manages registered extensions and provides aggregate access
// to their tools, commands, and event hooks.
type Registry struct {
	mu         sync.RWMutex
	extensions []Extension
	hooks      map[string][]func(context.Context, any) error
}

// NewRegistry creates a new empty extension registry.
func NewRegistry() *Registry {
	return &Registry{
		extensions: make([]Extension, 0),
		hooks:      make(map[string][]func(context.Context, any) error),
	}
}

// Register adds an extension to the registry and initializes it.
func (r *Registry) Register(ext Extension) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.extensions = append(r.extensions, ext)
	for _, hook := range ext.Hooks() {
		r.hooks[hook.Event] = append(r.hooks[hook.Event], hook.Handler)
	}
	return nil
}

// Tools returns all tools from all registered extensions.
func (r *Registry) Tools() []agent.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []agent.Tool
	for _, ext := range r.extensions {
		tools = append(tools, ext.Tools()...)
	}
	return tools
}

// Commands returns all command definitions from all registered extensions.
func (r *Registry) Commands() []CommandDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var cmds []CommandDef
	for _, ext := range r.extensions {
		cmds = append(cmds, ext.Commands()...)
	}
	return cmds
}

// EmitHook fires an event to all registered hooks for that event.
func (r *Registry) EmitHook(ctx context.Context, event string, data any) error {
	r.mu.RLock()
	handlers := append([]func(context.Context, any) error(nil), r.hooks[event]...)
	r.mu.RUnlock()

	var firstErr error
	for _, handler := range handlers {
		if err := handler(ctx, data); err != nil {
			slog.Error("extension hook error", "event", event, "error", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("hook for event %q failed: %w", event, err)
			}
		}
	}
	return firstErr
}

// Names returns the names of all registered extensions.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.extensions))
	for i, ext := range r.extensions {
		names[i] = ext.Name()
	}
	return names
}
