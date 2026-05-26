package coding

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/earendil-works/pi-go/internal/agents/coding/profile"
	"github.com/earendil-works/pi-go/internal/runtime"
)

// RebuildFunc is a callback to trigger agent rebuild after state change.
type RebuildFunc func() error

// CodingSessionExt implements runtime.SessionExt for the coding-agent.
// It holds per-session application state (profile, goal).
type CodingSessionExt struct {
	profile string
	goal    string
	rebuild RebuildFunc
}

// NewCodingSessionExt creates a new CodingSessionExt with default profile "coding".
func NewCodingSessionExt(rebuild RebuildFunc) *CodingSessionExt {
	return &CodingSessionExt{
		profile: string(profile.ProfileCoding),
		rebuild: rebuild,
	}
}

// SetRebuild sets the rebuild callback. Called by AgentSession after creation
// to inject the rebuild function (avoids circular dependency in constructor).
func (e *CodingSessionExt) SetRebuild(fn RebuildFunc) {
	e.rebuild = fn
}

func (e *CodingSessionExt) Profile() string { return e.profile }

func (e *CodingSessionExt) SwitchProfile(ctx context.Context, p string) error {
	if !profile.Valid(p) {
		return fmt.Errorf("unknown profile: %q (available: %v)", p, profile.All())
	}
	e.profile = p
	if e.rebuild != nil {
		if err := e.rebuild(); err != nil {
			return fmt.Errorf("rebuild agent with profile %q: %w", p, err)
		}
	}
	return nil
}

func (e *CodingSessionExt) Goal() string { return e.goal }

func (e *CodingSessionExt) SetGoal(goal string) {
	e.goal = goal
	if e.rebuild != nil {
		if err := e.rebuild(); err != nil {
			slog.Error("failed to rebuild agent after goal set", "error", err)
		}
	}
}

func (e *CodingSessionExt) ClearGoal() {
	e.goal = ""
	if e.rebuild != nil {
		if err := e.rebuild(); err != nil {
			slog.Error("failed to rebuild agent after goal clear", "error", err)
		}
	}
}

// Compile-time check.
var _ runtime.SessionExt = (*CodingSessionExt)(nil)
