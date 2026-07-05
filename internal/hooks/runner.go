package hooks

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"
)

// Runner executes individual hooks with timeout and panic recovery.
type Runner struct{}

// NewRunner creates a new hook runner.
func NewRunner() *Runner { return &Runner{} }

// Run executes a hook function with timeout and panic recovery.
// Returns a Result with timing information and any error.
func (r *Runner) Run(ctx context.Context, timeout time.Duration, fn func(context.Context) error) Result {
	start := time.Now()

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	err := r.runSafe(ctx, fn)
	return Result{
		Duration: time.Since(start),
		Err:      err,
	}
}

// runSafe executes fn with panic recovery. Panics are converted to errors.
func (r *Runner) runSafe(ctx context.Context, fn func(context.Context) error) (retErr error) {
	defer func() {
		if p := recover(); p != nil {
			stack := string(debug.Stack())
			slog.Error("hook panicked", "panic", p, "stack", stack)
			retErr = &PanicError{Value: p, Stack: stack}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return fn(ctx)
}

// PanicError wraps a recovered panic value with its stack trace.
type PanicError struct {
	Value any
	Stack string
}

func (e *PanicError) Error() string {
	return "hook panicked"
}
