package hooks

import (
	"sort"
	"sync"
	"time"

	"github.com/hwj123hwj/pi-go/internal/agent"
)

// Option configures a hook Entry during registration.
type Option func(*Entry)

// WithPriority sets the hook priority. Lower values run first.
func WithPriority(p int) Option { return func(e *Entry) { e.Priority = p } }

// WithTimeout sets a custom timeout for this hook.
func WithTimeout(d time.Duration) Option { return func(e *Entry) { e.Timeout = d } }

// Registry manages hook registrations with priority-based ordering.
type Registry struct {
	mu      sync.RWMutex
	entries []Entry
}

// NewRegistry creates a new hook registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// AddBeforeToolCall registers a BeforeToolCallHook.
func (r *Registry) AddBeforeToolCall(hook agent.BeforeToolCallHook, opts ...Option) {
	r.addEntry(EventBeforeTool, hook, nil, nil, nil, nil, nil, opts...)
}

// AddAfterToolCall registers an AfterToolCallHook.
func (r *Registry) AddAfterToolCall(hook agent.AfterToolCallHook, opts ...Option) {
	r.addEntry(EventAfterTool, nil, hook, nil, nil, nil, nil, opts...)
}

// AddSessionStart registers a SessionStartHook.
func (r *Registry) AddSessionStart(hook agent.SessionStartHook, opts ...Option) {
	r.addEntry(EventSessionStart, nil, nil, hook, nil, nil, nil, opts...)
}

// AddSessionEnd registers a SessionEndHook.
func (r *Registry) AddSessionEnd(hook agent.SessionEndHook, opts ...Option) {
	r.addEntry(EventSessionEnd, nil, nil, nil, hook, nil, nil, opts...)
}

// AddPreCompress registers a PreCompressHook.
func (r *Registry) AddPreCompress(hook agent.PreCompressHook, opts ...Option) {
	r.addEntry(EventPreCompress, nil, nil, nil, nil, hook, nil, opts...)
}

// AddGeneric registers a generic hook function.
func (r *Registry) AddGeneric(event Event, hook Hook, opts ...Option) {
	r.addEntry(event, nil, nil, nil, nil, nil, hook, opts...)
}

// addEntry is the internal method that stores a hook entry with all its typed fields.
func (r *Registry) addEntry(
	event Event,
	beforeToolCall agent.BeforeToolCallHook,
	afterToolCall agent.AfterToolCallHook,
	sessionStart agent.SessionStartHook,
	sessionEnd agent.SessionEndHook,
	preCompress agent.PreCompressHook,
	generic Hook,
	opts ...Option,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := Entry{
		Event:          event,
		Priority:       DefaultPriority,
		Timeout:        DefaultHookTimeout,
		BeforeToolCall: beforeToolCall,
		AfterToolCall:  afterToolCall,
		SessionStart:   sessionStart,
		SessionEnd:     sessionEnd,
		PreCompress:    preCompress,
		Generic:        generic,
	}
	for _, opt := range opts {
		opt(&entry)
	}
	r.entries = append(r.entries, entry)
}

// GetBeforeToolCallHooks returns BeforeToolCall hooks sorted by priority (lowest first).
func (r *Registry) GetBeforeToolCallHooks() []agent.BeforeToolCallHook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []Entry
	for _, e := range r.entries {
		if e.Event == EventBeforeTool && e.BeforeToolCall != nil {
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Priority < filtered[j].Priority })

	hooks := make([]agent.BeforeToolCallHook, len(filtered))
	for i, e := range filtered {
		hooks[i] = e.BeforeToolCall
	}
	return hooks
}

// GetAfterToolCallHooks returns AfterToolCall hooks sorted by priority (lowest first).
func (r *Registry) GetAfterToolCallHooks() []agent.AfterToolCallHook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []Entry
	for _, e := range r.entries {
		if e.Event == EventAfterTool && e.AfterToolCall != nil {
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Priority < filtered[j].Priority })

	hooks := make([]agent.AfterToolCallHook, len(filtered))
	for i, e := range filtered {
		hooks[i] = e.AfterToolCall
	}
	return hooks
}

// GetSessionStartHooks returns SessionStart hooks sorted by priority.
func (r *Registry) GetSessionStartHooks() []agent.SessionStartHook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []Entry
	for _, e := range r.entries {
		if e.Event == EventSessionStart && e.SessionStart != nil {
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Priority < filtered[j].Priority })

	hooks := make([]agent.SessionStartHook, len(filtered))
	for i, e := range filtered {
		hooks[i] = e.SessionStart
	}
	return hooks
}

// GetSessionEndHooks returns SessionEnd hooks sorted by priority.
func (r *Registry) GetSessionEndHooks() []agent.SessionEndHook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []Entry
	for _, e := range r.entries {
		if e.Event == EventSessionEnd && e.SessionEnd != nil {
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Priority < filtered[j].Priority })

	hooks := make([]agent.SessionEndHook, len(filtered))
	for i, e := range filtered {
		hooks[i] = e.SessionEnd
	}
	return hooks
}

// GetPreCompressHooks returns PreCompress hooks sorted by priority.
func (r *Registry) GetPreCompressHooks() []agent.PreCompressHook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []Entry
	for _, e := range r.entries {
		if e.Event == EventPreCompress && e.PreCompress != nil {
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Priority < filtered[j].Priority })

	hooks := make([]agent.PreCompressHook, len(filtered))
	for i, e := range filtered {
		hooks[i] = e.PreCompress
	}
	return hooks
}

// GetByEvent returns all entries for a specific event, sorted by priority.
func (r *Registry) GetByEvent(event Event) []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []Entry
	for _, e := range r.entries {
		if e.Event == event {
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Priority < filtered[j].Priority })
	return filtered
}

// Len returns the total number of registered hooks.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// Clear removes all registered hooks.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = nil
}
