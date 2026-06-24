package kb

import (
	"context"
	"log/slog"

	"github.com/hwj123hwj/pi-go/internal/runtime"
)

// KBSessionExt implements runtime.SessionExt for the kb-agent.
type KBSessionExt struct {
	rebuild func() error
	goal    string
}

// NewKBSessionExt creates a new KBSessionExt.
func NewKBSessionExt() *KBSessionExt {
	return &KBSessionExt{}
}

// SetRebuild sets the rebuild callback. Called by AgentSession after creation.
func (e *KBSessionExt) SetRebuild(fn func() error) {
	e.rebuild = fn
}

// Profile returns a fixed profile name (kb agent has a single profile).
func (e *KBSessionExt) Profile() string { return "default" }

// SwitchProfile is a no-op for kb agent.
func (e *KBSessionExt) SwitchProfile(_ context.Context, _ string) error {
	return nil
}

func (e *KBSessionExt) Goal() string { return e.goal }

func (e *KBSessionExt) SetGoal(goal string) {
	e.goal = goal
	if e.rebuild != nil {
		if err := e.rebuild(); err != nil {
			slog.Error("failed to rebuild agent after goal set", "error", err)
		}
	}
}

func (e *KBSessionExt) ClearGoal() {
	e.goal = ""
	if e.rebuild != nil {
		if err := e.rebuild(); err != nil {
			slog.Error("failed to rebuild agent after goal clear", "error", err)
		}
	}
}

// Compile-time check.
var _ runtime.SessionExt = (*KBSessionExt)(nil)
