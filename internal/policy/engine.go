package policy

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Engine is the policy engine that evaluates tool calls against rules and user decisions.
type Engine struct {
	mu sync.RWMutex

	rules *Rules

	// Session-level always-allow/deny cache (not persisted).
	alwaysAllow map[string]bool
	alwaysDeny  map[string]bool

	// Persistent decisions loaded from policy file.
	allowedTools map[string]bool
	deniedTools  map[string]bool
}

// NewEngine creates a new policy engine.
func NewEngine() *Engine {
	return &Engine{
		rules:        NewRules(),
		alwaysAllow:  make(map[string]bool),
		alwaysDeny:   make(map[string]bool),
		allowedTools: make(map[string]bool),
		deniedTools:  make(map[string]bool),
	}
}

// Rules returns the engine's rule collection for adding rules.
func (e *Engine) Rules() *Rules {
	return e.rules
}

// Check evaluates a tool call and returns the policy decision.
// The args parameter should be the tool's arguments as a map (extracted from JSON).
func (e *Engine) Check(toolName string, args map[string]any) (Decision, *Rule) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 1. Check session-level always-allow cache
	if e.alwaysAllow[toolName] {
		return DecisionAllow, nil
	}

	// 2. Check session-level always-deny cache
	if e.alwaysDeny[toolName] {
		return Deny, nil
	}

	// 3. Check persistent allowed tools
	if e.allowedTools[toolName] {
		return DecisionAllow, nil
	}

	// 4. Check persistent denied tools
	if e.deniedTools[toolName] {
		return Deny, nil
	}

	// 5. Evaluate rules
	return e.rules.Match(toolName, args)
}

// AllowAlways marks a tool as always allowed for the current session.
func (e *Engine) AllowAlways(toolName string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.alwaysAllow[toolName] = true
	delete(e.alwaysDeny, toolName)
}

// DenyAlways marks a tool as always denied for the current session.
func (e *Engine) DenyAlways(toolName string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.alwaysDeny[toolName] = true
	delete(e.alwaysAllow, toolName)
}

// AllowPersist marks a tool as always allowed and persists the decision.
func (e *Engine) AllowPersist(toolName string, policyPath string) error {
	e.mu.Lock()
	e.allowedTools[toolName] = true
	delete(e.deniedTools, toolName)
	e.mu.Unlock()

	return e.persist(policyPath)
}

// DenyPersist marks a tool as always denied and persists the decision.
func (e *Engine) DenyPersist(toolName string, policyPath string) error {
	e.mu.Lock()
	e.deniedTools[toolName] = true
	delete(e.allowedTools, toolName)
	e.mu.Unlock()

	return e.persist(policyPath)
}

// ClearSession clears all session-level decisions.
func (e *Engine) ClearSession() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.alwaysAllow = make(map[string]bool)
	e.alwaysDeny = make(map[string]bool)
}

// ClearAll clears all decisions (session + persistent).
func (e *Engine) ClearAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.alwaysAllow = make(map[string]bool)
	e.alwaysDeny = make(map[string]bool)
	e.allowedTools = make(map[string]bool)
	e.deniedTools = make(map[string]bool)
}

// LoadFromFile loads persistent decisions from a policy file.
func (e *Engine) LoadFromFile(path string) error {
	pf, err := LoadPolicyFile(path)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.allowedTools = make(map[string]bool, len(pf.Allowed))
	for _, name := range pf.Allowed {
		e.allowedTools[name] = true
	}
	e.deniedTools = make(map[string]bool, len(pf.Denied))
	for _, name := range pf.Denied {
		e.deniedTools[name] = true
	}
	return nil
}

// persist writes the current persistent decisions to the policy file.
func (e *Engine) persist(path string) error {
	e.mu.RLock()
	allowed := make([]string, 0, len(e.allowedTools))
	for name := range e.allowedTools {
		allowed = append(allowed, name)
	}
	denied := make([]string, 0, len(e.deniedTools))
	for name := range e.deniedTools {
		denied = append(denied, name)
	}
	e.mu.RUnlock()

	return SavePolicyFile(path, &PolicyFile{
		Allowed: allowed,
		Denied:  denied,
	})
}

// String returns a human-readable summary of the engine state.
func (e *Engine) String() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return fmt.Sprintf(
		"policy engine: %d rule(s), %d session-allow, %d session-deny, %d persistent-allow, %d persistent-deny",
		e.rules.Len(),
		len(e.alwaysAllow), len(e.alwaysDeny),
		len(e.allowedTools), len(e.deniedTools),
	)
}

// UpdatePolicy applies a policy update from a structured payload.
func (e *Engine) UpdatePolicy(payload UpdatePolicyPayload, policyPath string) error {
	switch payload.Decision {
	case DecisionAllow:
		if payload.Scope == "always" {
			return e.AllowPersist(payload.ToolName, policyPath)
		}
		e.AllowAlways(payload.ToolName)
	case Deny:
		if payload.Scope == "always" {
			return e.DenyPersist(payload.ToolName, policyPath)
		}
		e.DenyAlways(payload.ToolName)
	default:
		// AskUser is not a persistent decision
	}
	return nil
}

// UpdatePolicyPayload describes a policy update request.
type UpdatePolicyPayload struct {
	ToolName string   `json:"tool_name"`
	Decision Decision `json:"decision"`
	Scope    string   `json:"scope,omitempty"` // "once" or "always"
}

// MarshalJSON implements json.Marshaler for Engine (for debugging).
func (e *Engine) MarshalJSON() ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sessionAllowed := make([]string, 0, len(e.alwaysAllow))
	for k := range e.alwaysAllow {
		sessionAllowed = append(sessionAllowed, k)
	}
	sessionDenied := make([]string, 0, len(e.alwaysDeny))
	for k := range e.alwaysDeny {
		sessionDenied = append(sessionDenied, k)
	}
	persistAllowed := make([]string, 0, len(e.allowedTools))
	for k := range e.allowedTools {
		persistAllowed = append(persistAllowed, k)
	}
	persistDenied := make([]string, 0, len(e.deniedTools))
	for k := range e.deniedTools {
		persistDenied = append(persistDenied, k)
	}

	return json.Marshal(struct {
		SessionAllowed  []string `json:"session_allowed"`
		SessionDenied   []string `json:"session_denied"`
		PersistAllowed  []string `json:"persist_allowed"`
		PersistDenied   []string `json:"persist_denied"`
		RuleCount       int      `json:"rule_count"`
	}{
		SessionAllowed: sessionAllowed,
		SessionDenied:  sessionDenied,
		PersistAllowed: persistAllowed,
		PersistDenied:  persistDenied,
		RuleCount:      e.rules.Len(),
	})
}
