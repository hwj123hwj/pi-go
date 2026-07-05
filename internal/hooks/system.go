package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hwj123hwj/pi-go/internal/agent"
)

// System is the main hook system entry point that coordinates
// registry, runner, and aggregator.
type System struct {
	registry   *Registry
	runner     *Runner
	aggregator *Aggregator
}

// NewSystem creates a new hook system.
func NewSystem() *System {
	return &System{
		registry:   NewRegistry(),
		runner:     NewRunner(),
		aggregator: NewAggregator(),
	}
}

// Registry returns the hook registry for adding hooks.
func (s *System) Registry() *Registry { return s.registry }

// AddBeforeToolCall registers a BeforeToolCallHook in the system.
func (s *System) AddBeforeToolCall(hook agent.BeforeToolCallHook, opts ...Option) {
	s.registry.AddBeforeToolCall(hook, opts...)
}

// AddAfterToolCall registers an AfterToolCallHook in the system.
func (s *System) AddAfterToolCall(hook agent.AfterToolCallHook, opts ...Option) {
	s.registry.AddAfterToolCall(hook, opts...)
}

// AddSessionStart registers a SessionStartHook in the system.
func (s *System) AddSessionStart(hook agent.SessionStartHook, opts ...Option) {
	s.registry.AddSessionStart(hook, opts...)
}

// AddSessionEnd registers a SessionEndHook in the system.
func (s *System) AddSessionEnd(hook agent.SessionEndHook, opts ...Option) {
	s.registry.AddSessionEnd(hook, opts...)
}

// AddPreCompress registers a PreCompressHook in the system.
func (s *System) AddPreCompress(hook agent.PreCompressHook, opts ...Option) {
	s.registry.AddPreCompress(hook, opts...)
}

// RunBefore runs all before-tool-call hooks (both registered and existing lifecycle hooks)
// in priority order. Returns the (possibly modified) ToolCallContext or an error to block execution.
func (s *System) RunBefore(
	ctx context.Context,
	existing []agent.BeforeToolCallHook,
	call agent.ToolCallContext,
) (agent.ToolCallContext, error) {
	// Merge registered hooks with existing lifecycle hooks.
	// Registered hooks run first (by priority), then existing hooks (in original order).
	registered := s.registry.GetBeforeToolCallHooks()
	all := make([]agent.BeforeToolCallHook, 0, len(registered)+len(existing))
	all = append(all, registered...)
	all = append(all, existing...)

	if len(all) == 0 {
		return call, nil
	}

	contexts := make([]agent.ToolCallContext, 0, len(all))
	results := make([]Result, 0, len(all))

	current := call
	for _, hook := range all {
		if hook == nil {
			continue
		}
		result := s.runner.Run(ctx, DefaultHookTimeout, func(runCtx context.Context) error {
			var err error
			current, err = hook(runCtx, current)
			return err
		})
		contexts = append(contexts, current)
		results = append(results, result)

		if result.Err != nil {
			slog.Debug("before-hook blocked execution",
				"tool", call.ToolName,
				"error", result.Err,
				"duration", result.Duration,
			)
			return current, result.Err
		}
	}

	return s.aggregator.AggregateBeforeTool(results, contexts)
}

// RunAfter runs all after-tool-call hooks.
// Returns the (possibly modified) ToolResult or an error.
func (s *System) RunAfter(
	ctx context.Context,
	existing []agent.AfterToolCallHook,
	call agent.ToolCallContext,
	result agent.ToolResult,
) (agent.ToolResult, error) {
	registered := s.registry.GetAfterToolCallHooks()
	all := make([]agent.AfterToolCallHook, 0, len(registered)+len(existing))
	all = append(all, registered...)
	all = append(all, existing...)

	if len(all) == 0 {
		return result, nil
	}

	toolResults := make([]agent.ToolResult, 0, len(all))
	results := make([]Result, 0, len(all))

	current := result
	for _, hook := range all {
		if hook == nil {
			continue
		}
		hookResult := s.runner.Run(ctx, DefaultHookTimeout, func(runCtx context.Context) error {
			var err error
			current, err = hook(runCtx, call, current)
			return err
		})
		toolResults = append(toolResults, current)
		results = append(results, hookResult)

		if hookResult.Err != nil {
			slog.Debug("after-hook failed",
				"tool", call.ToolName,
				"error", hookResult.Err,
				"duration", hookResult.Duration,
			)
		}
	}

	return s.aggregator.AggregateAfterTool(results, toolResults)
}

// RunSessionStart runs all session-start hooks (non-blocking: errors are logged, not propagated).
func (s *System) RunSessionStart(ctx context.Context, existing []agent.SessionStartHook, e agent.SessionStartEvent) {
	registered := s.registry.GetSessionStartHooks()
	all := make([]agent.SessionStartHook, 0, len(registered)+len(existing))
	all = append(all, registered...)
	all = append(all, existing...)

	for _, hook := range all {
		if hook == nil {
			continue
		}
		result := s.runner.Run(ctx, DefaultHookTimeout, func(runCtx context.Context) error {
			return hook(runCtx, e)
		})
		if result.Err != nil {
			slog.Warn("SessionStart hook failed (non-blocking)", "error", result.Err)
		}
	}
}

// RunSessionEnd runs all session-end hooks (non-blocking: errors are logged, not propagated).
func (s *System) RunSessionEnd(ctx context.Context, existing []agent.SessionEndHook, e agent.SessionEndEvent) {
	registered := s.registry.GetSessionEndHooks()
	all := make([]agent.SessionEndHook, 0, len(registered)+len(existing))
	all = append(all, registered...)
	all = append(all, existing...)

	for _, hook := range all {
		if hook == nil {
			continue
		}
		result := s.runner.Run(ctx, DefaultHookTimeout, func(runCtx context.Context) error {
			return hook(runCtx, e)
		})
		if result.Err != nil {
			slog.Warn("SessionEnd hook failed (non-blocking)", "error", result.Err)
		}
	}
}

// RunPreCompress runs all pre-compress hooks (non-blocking: errors are logged, not propagated).
func (s *System) RunPreCompress(ctx context.Context, existing []agent.PreCompressHook, e agent.PreCompressEvent) {
	registered := s.registry.GetPreCompressHooks()
	all := make([]agent.PreCompressHook, 0, len(registered)+len(existing))
	all = append(all, registered...)
	all = append(all, existing...)

	for _, hook := range all {
		if hook == nil {
			continue
		}
		result := s.runner.Run(ctx, DefaultHookTimeout, func(runCtx context.Context) error {
			return hook(runCtx, e)
		})
		if result.Err != nil {
			slog.Warn("PreCompress hook failed (non-blocking)", "error", result.Err)
		}
	}
}

// ToLifecycleHooks converts the system's registered hooks into a LifecycleHooks struct
// that can be used directly with agent.Options. This creates snapshot copies of
// currently registered hooks.
func (s *System) ToLifecycleHooks() agent.LifecycleHooks {
	return agent.LifecycleHooks{
		Before:      s.registry.GetBeforeToolCallHooks(),
		After:       s.registry.GetAfterToolCallHooks(),
		SessionStart: s.registry.GetSessionStartHooks(),
		SessionEnd:  s.registry.GetSessionEndHooks(),
		PreCompress: s.registry.GetPreCompressHooks(),
	}
}

// Status returns the hook system status for debugging.
func (s *System) Status() Status {
	return Status{
		TotalHooks: s.registry.Len(),
	}
}

// Status holds hook system status information.
type Status struct {
	TotalHooks int
}

// FromLifecycleHooks creates a System pre-loaded with hooks from existing LifecycleHooks.
// This provides backward compatibility with the existing hook mechanism.
func FromLifecycleHooks(hooks agent.LifecycleHooks) *System {
	s := NewSystem()
	for _, h := range hooks.Before {
		s.registry.AddBeforeToolCall(h, WithPriority(PriorityUser))
	}
	for _, h := range hooks.After {
		s.registry.AddAfterToolCall(h, WithPriority(PriorityUser))
	}
	for _, h := range hooks.SessionStart {
		s.registry.AddSessionStart(h, WithPriority(PriorityUser))
	}
	for _, h := range hooks.SessionEnd {
		s.registry.AddSessionEnd(h, WithPriority(PriorityUser))
	}
	for _, h := range hooks.PreCompress {
		s.registry.AddPreCompress(h, WithPriority(PriorityUser))
	}
	return s
}

// FormatStatus returns a human-readable status string.
func FormatStatus(s *System) string {
	st := s.Status()
	return fmt.Sprintf("hooks system: %d registered hook(s)", st.TotalHooks)
}

// DefaultSessionTimeout is the default timeout for session hooks.
const DefaultSessionTimeout = 30 * time.Second
