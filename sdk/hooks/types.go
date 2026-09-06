package hooks

import (
	"time"

	"github.com/hwj123hwj/pi-go/sdk/agent"
)

// Event types for the hook system.
type Event int

const (
	EventBeforeTool  Event = iota // Before tool execution
	EventAfterTool                // After tool execution
	EventSessionStart             // Session start
	EventSessionEnd               // Session end
	EventPreCompress              // Before context compression
)

// Priority levels for hook execution order. Lower values run first.
const (
	PrioritySystem    = 10  // System-level hooks (highest priority)
	PriorityPolicy    = 50  // Policy engine hooks
	PriorityUser      = 100 // User-defined hooks
	PriorityExtension = 200 // Extension hooks (lowest priority)
)

// DefaultPriority is the default hook execution priority.
const DefaultPriority = PriorityUser

// DefaultHookTimeout is the default timeout for hook execution (60 seconds).
const DefaultHookTimeout = 60 * time.Second

// Hook is a generic hook function that can handle any event type via switch.
// Use specific typed hooks (BeforeToolCallHook, etc.) for better type safety.
type Hook func(event Event, data any) error

// Entry represents a registered hook with metadata.
type Entry struct {
	Event    Event
	Priority int
	Timeout  time.Duration
	// BeforeToolCall is a typed before-tool-call hook.
	BeforeToolCall agent.BeforeToolCallHook
	// AfterToolCall is a typed after-tool-call hook.
	AfterToolCall agent.AfterToolCallHook
	// SessionStart is a typed session-start hook.
	SessionStart agent.SessionStartHook
	// SessionEnd is a typed session-end hook.
	SessionEnd agent.SessionEndHook
	// PreCompress is a typed pre-compress hook.
	PreCompress agent.PreCompressHook
	// Generic is a generic hook function (lower priority than typed hooks).
	Generic Hook
}

// Result holds the outcome of a single hook execution.
type Result struct {
	Duration time.Duration
	Err      error
}
