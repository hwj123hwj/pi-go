package music

import (
	"context"
	"log/slog"

	"github.com/hwj123hwj/pi-go/internal/runtime"
)

// MusicSessionExt implements runtime.SessionExt for the music-agent.
// Phase 1: minimal implementation with rebuild support.
type MusicSessionExt struct {
	rebuild func() error
	goal    string
}

// NewMusicSessionExt creates a new MusicSessionExt.
func NewMusicSessionExt() *MusicSessionExt {
	return &MusicSessionExt{}
}

// SetRebuild sets the rebuild callback. Called by AgentSession after creation.
func (e *MusicSessionExt) SetRebuild(fn func() error) {
	e.rebuild = fn
}

// Profile returns a fixed profile name (music agent has a single profile for now).
func (e *MusicSessionExt) Profile() string { return "default" }

// SwitchProfile is a no-op for Phase 1 (single profile).
func (e *MusicSessionExt) SwitchProfile(_ context.Context, _ string) error {
	return nil
}

func (e *MusicSessionExt) Goal() string { return e.goal }

func (e *MusicSessionExt) SetGoal(goal string) {
	e.goal = goal
	if e.rebuild != nil {
		if err := e.rebuild(); err != nil {
			slog.Error("failed to rebuild agent after goal set", "error", err)
		}
	}
}

func (e *MusicSessionExt) ClearGoal() {
	e.goal = ""
	if e.rebuild != nil {
		if err := e.rebuild(); err != nil {
			slog.Error("failed to rebuild agent after goal clear", "error", err)
		}
	}
}

// Compile-time check.
var _ runtime.SessionExt = (*MusicSessionExt)(nil)
