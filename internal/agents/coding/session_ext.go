package coding

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/earendil-works/pi-go/internal/agents/coding/profile"
	"github.com/earendil-works/pi-go/internal/runtime"
)

// CodingSessionExt implements runtime.SessionExt for the coding-agent.
// It holds per-session application state (profile, goal).
type CodingSessionExt struct {
	profile string
	goal    string
	rebuild func() error
}

// NewCodingSessionExt creates a new CodingSessionExt with default profile "coding".
func NewCodingSessionExt(rebuild func() error) *CodingSessionExt {
	return &CodingSessionExt{
		profile: string(profile.ProfileCoding),
		rebuild: rebuild,
	}
}

// SetRebuild sets the rebuild callback. Called by AgentSession after creation
// to inject the rebuild function (avoids circular dependency in constructor).
// NOTE: Parameter must be `func() error` (not a named type) so that the
// interface assertion in AgentSession (`interface{ SetRebuild(func() error) }`)
// succeeds. Go treats named types as distinct from their underlying types.
func (e *CodingSessionExt) SetRebuild(fn func() error) {
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
	slog.Info("CodingSessionExt.SetGoal called", "goal", goal, "rebuildIsNil", e.rebuild == nil)
	if e.rebuild != nil {
		slog.Info("CodingSessionExt.SetGoal: calling rebuild")
		if err := e.rebuild(); err != nil {
			slog.Error("failed to rebuild agent after goal set", "error", err)
		}
		slog.Info("CodingSessionExt.SetGoal: rebuild done")
	} else {
		slog.Warn("CodingSessionExt.SetGoal: rebuild is nil, agent NOT rebuilt with goal!")
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
