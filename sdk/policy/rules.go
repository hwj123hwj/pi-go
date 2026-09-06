package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Decision represents a policy decision.
type Decision int

const (
	// DecisionAllow permits the tool call to proceed.
	DecisionAllow Decision = iota
	// DecisionAskUser asks the user for confirmation before proceeding.
	DecisionAskUser
	// DecisionDeny blocks the tool call entirely.
	Deny
)

// String returns a human-readable representation of the decision.
func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionAskUser:
		return "ask_user"
	case Deny:
		return "deny"
	default:
		return "unknown"
	}
}

// MarshalJSON implements json.Marshaler.
func (d Decision) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Decision) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "allow":
		*d = DecisionAllow
	case "ask_user":
		*d = DecisionAskUser
	case "deny":
		*d = Deny
	default:
		return fmt.Errorf("unknown decision: %s", s)
	}
	return nil
}

// Rule defines a matching pattern and its associated policy decision.
type Rule struct {
	// Name is a human-readable identifier for this rule.
	Name string `json:"name"`
	// ToolPattern is a glob pattern to match tool names (e.g., "bash", "file_*", "*").
	// Empty or "*" matches all tools.
	ToolPattern string `json:"tool_pattern,omitempty"`
	// ArgPath is the JSON path within tool arguments to match (e.g., "file", "path").
	// Empty means no argument matching.
	ArgPath string `json:"arg_path,omitempty"`
	// ArgPattern is a glob pattern to match the extracted argument value.
	ArgPattern string `json:"arg_pattern,omitempty"`
	// Decision is the policy decision for matching calls.
	Decision Decision `json:"decision"`
	// Priority controls rule evaluation order. Lower values are evaluated first.
	Priority int `json:"priority"`
}

// DefaultRulePriority is the default priority for rules.
const DefaultRulePriority = 100

// Rules manages a collection of policy rules.
type Rules struct {
	mu    sync.RWMutex
	rules []Rule
}

// NewRules creates a new rules collection.
func NewRules() *Rules {
	return &Rules{}
}

// Add adds a rule to the collection. Rules are kept sorted by priority.
func (r *Rules) Add(rule Rule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rule.Priority == 0 {
		rule.Priority = DefaultRulePriority
	}
	r.rules = append(r.rules, rule)
}

// Match returns the decision for the first matching rule, or DecisionAskUser if no rule matches.
func (r *Rules) Match(toolName string, args map[string]any) (Decision, *Rule) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Sort by priority (we assume rules are added in priority order; re-sort for safety).
	sorted := make([]Rule, len(r.rules))
	copy(sorted, r.rules)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Priority < sorted[i].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	for i := range sorted {
		if r.matchesRule(&sorted[i], toolName, args) {
			return sorted[i].Decision, &sorted[i]
		}
	}

	// Default: ask user
	return DecisionAskUser, nil
}

// matchesRule checks if a tool call matches a rule.
func (r *Rules) matchesRule(rule *Rule, toolName string, args map[string]any) bool {
	// Match tool name pattern
	if rule.ToolPattern != "" && rule.ToolPattern != "*" {
		matched, err := filepath.Match(rule.ToolPattern, toolName)
		if err != nil || !matched {
			return false
		}
	}

	// Match argument value (if ArgPath is specified)
	if rule.ArgPath != "" && rule.ArgPattern != "" {
		val, ok := extractArgValue(args, rule.ArgPath)
		if !ok {
			return false
		}
		strVal, ok := val.(string)
		if !ok {
			return false
		}
		// Match against both the full value and the base name.
		// This allows patterns like "*.key" to match "/etc/secret.key".
		matched, err := filepath.Match(rule.ArgPattern, strVal)
		if !matched || err != nil {
			matched, err = filepath.Match(rule.ArgPattern, filepath.Base(strVal))
			if err != nil || !matched {
				return false
			}
		}
	}

	return true
}

// extractArgValue extracts a value from a map by key path.
// Supports simple key lookup (e.g., "file" -> args["file"]).
func extractArgValue(args map[string]any, path string) (any, bool) {
	if args == nil {
		return nil, false
	}
	val, ok := args[path]
	return val, ok
}

// Len returns the number of rules.
func (r *Rules) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.rules)
}

// Clear removes all rules.
func (r *Rules) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules = nil
}

// ─── Persistence ───────────────────────────────────────────────────────────────

// PolicyFile represents the on-disk format of the policy file.
type PolicyFile struct {
	Allowed []string `json:"allowed"`
	Denied  []string `json:"denied"`
}

// LoadPolicyFile reads a policy file from disk.
func LoadPolicyFile(path string) (*PolicyFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &PolicyFile{}, nil
		}
		return nil, fmt.Errorf("read policy file: %w", err)
	}

	var pf PolicyFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parse policy file: %w", err)
	}
	return &pf, nil
}

// SavePolicyFile writes a policy file to disk.
func SavePolicyFile(path string, pf *PolicyFile) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create policy dir: %w", err)
	}

	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write policy file: %w", err)
	}
	return nil
}
