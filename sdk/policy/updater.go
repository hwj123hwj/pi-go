package policy

import (
	"log/slog"
	"path/filepath"
)

// Updater provides high-level operations to update and manage the policy engine.
type Updater struct {
	engine     *Engine
	policyPath string
}

// NewUpdater creates a new policy updater.
// policyPath is the path to the persistent policy file (e.g., ".pi-go/policy.json").
func NewUpdater(engine *Engine, policyPath string) *Updater {
	return &Updater{
		engine:     engine,
		policyPath: policyPath,
	}
}

// AllowTool allows a tool for the current session only.
func (u *Updater) AllowTool(toolName string) {
	u.engine.AllowAlways(toolName)
	slog.Info("policy: allowed tool for session", "tool", toolName)
}

// DenyTool denies a tool for the current session only.
func (u *Updater) DenyTool(toolName string) {
	u.engine.DenyAlways(toolName)
	slog.Info("policy: denied tool for session", "tool", toolName)
}

// AllowToolPersist allows a tool and persists the decision to disk.
func (u *Updater) AllowToolPersist(toolName string) error {
	if err := u.engine.AllowPersist(toolName, u.policyPath); err != nil {
		slog.Error("policy: failed to persist allow decision", "tool", toolName, "error", err)
		return err
	}
	slog.Info("policy: allowed tool (persisted)", "tool", toolName, "path", u.policyPath)
	return nil
}

// DenyToolPersist denies a tool and persists the decision to disk.
func (u *Updater) DenyToolPersist(toolName string) error {
	if err := u.engine.DenyPersist(toolName, u.policyPath); err != nil {
		slog.Error("policy: failed to persist deny decision", "tool", toolName, "error", err)
		return err
	}
	slog.Info("policy: denied tool (persisted)", "tool", toolName, "path", u.policyPath)
	return nil
}

// ApplyDecision applies a user's confirmation decision to the policy engine.
// This is called when the user responds to a confirmation prompt.
func (u *Updater) ApplyDecision(toolName string, approved bool, persist bool) error {
	if approved {
		if persist {
			return u.AllowToolPersist(toolName)
		}
		u.AllowTool(toolName)
		return nil
	}

	if persist {
		return u.DenyToolPersist(toolName)
	}
	u.DenyTool(toolName)
	return nil
}

// ClearSession clears all session-level policy decisions.
func (u *Updater) ClearSession() {
	u.engine.ClearSession()
	slog.Info("policy: cleared session decisions")
}

// ClearAll clears all policy decisions (session + persistent).
func (u *Updater) ClearAll() {
	u.engine.ClearAll()
	slog.Info("policy: cleared all decisions")
}

// LoadFromDisk loads persistent policy decisions from the policy file.
func (u *Updater) LoadFromDisk() error {
	return u.engine.LoadFromFile(u.policyPath)
}

// PolicyPath returns the path to the persistent policy file.
func (u *Updater) PolicyPath() string {
	return u.policyPath
}

// DefaultPolicyPath returns the default policy file path relative to a project root.
func DefaultPolicyPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".pi-go", "policy.json")
}
