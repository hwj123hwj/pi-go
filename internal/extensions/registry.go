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

	// Lifecycle hooks for tool execution.
	beforeHooks []agent.BeforeToolCallHook
	afterHooks  []agent.AfterToolCallHook

	// Observer hooks for session lifecycle and compaction (non-blocking).
	sessionStartHooks []agent.SessionStartHook
	sessionEndHooks   []agent.SessionEndHook
	preCompressHooks  []agent.PreCompressHook
}

// NewRegistry creates a new empty extension registry.
func NewRegistry() *Registry {
	return &Registry{
		extensions: make([]Extension, 0),
		hooks:      make(map[string][]func(context.Context, any) error),
	}
}

// Register adds an extension to the registry.
// If the extension implements ExtensionWithLifecycle, its before/after hooks
// are automatically collected.
func (r *Registry) Register(ext Extension) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.extensions = append(r.extensions, ext)
	for _, hook := range ext.Hooks() {
		r.hooks[hook.Event] = append(r.hooks[hook.Event], hook.Handler)
	}

	// Auto-collect lifecycle hooks if extension implements ExtensionWithLifecycle
	if le, ok := ext.(ExtensionWithLifecycle); ok {
		r.beforeHooks = append(r.beforeHooks, le.BeforeToolCallHooks()...)
		r.afterHooks = append(r.afterHooks, le.AfterToolCallHooks()...)
	}

	// Auto-collect session observer hooks (non-blocking).
	if se, ok := ext.(ExtensionWithSessionHooks); ok {
		r.sessionStartHooks = append(r.sessionStartHooks, se.SessionStartHooks()...)
		r.sessionEndHooks = append(r.sessionEndHooks, se.SessionEndHooks()...)
	}

	// Auto-collect compaction observer hooks (non-blocking).
	if ce, ok := ext.(ExtensionWithCompressHook); ok {
		r.preCompressHooks = append(r.preCompressHooks, ce.PreCompressHooks()...)
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

// RegisterBeforeToolCallHook adds a before-tool-call lifecycle hook.
// Hooks are called in registration order.
func (r *Registry) RegisterBeforeToolCallHook(hook agent.BeforeToolCallHook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.beforeHooks = append(r.beforeHooks, hook)
}

// RegisterAfterToolCallHook adds an after-tool-call lifecycle hook.
// Hooks are called in registration order.
func (r *Registry) RegisterAfterToolCallHook(hook agent.AfterToolCallHook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.afterHooks = append(r.afterHooks, hook)
}

// LifecycleHooks returns the aggregated lifecycle hooks from all registered
// extension hooks. This is consumed by the runtime to inject into the agent.
func (r *Registry) LifecycleHooks() agent.LifecycleHooks {
	r.mu.RLock()
	defer r.mu.RUnlock()

	before := make([]agent.BeforeToolCallHook, len(r.beforeHooks))
	copy(before, r.beforeHooks)
	after := make([]agent.AfterToolCallHook, len(r.afterHooks))
	copy(after, r.afterHooks)
	sessionStart := make([]agent.SessionStartHook, len(r.sessionStartHooks))
	copy(sessionStart, r.sessionStartHooks)
	sessionEnd := make([]agent.SessionEndHook, len(r.sessionEndHooks))
	copy(sessionEnd, r.sessionEndHooks)
	preCompress := make([]agent.PreCompressHook, len(r.preCompressHooks))
	copy(preCompress, r.preCompressHooks)

	return agent.LifecycleHooks{
		Before:       before,
		After:        after,
		SessionStart: sessionStart,
		SessionEnd:   sessionEnd,
		PreCompress:  preCompress,
	}
}
