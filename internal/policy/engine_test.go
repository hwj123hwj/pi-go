package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Rule Matching Tests ──────────────────────────────────────────────────────

func TestRules_MatchToolName_ExactMatch(t *testing.T) {
	rules := NewRules()
	rules.Add(Rule{
		Name:        "allow bash",
		ToolPattern: "bash",
		Decision:    DecisionAllow,
		Priority:    100,
	})

	decision, rule := rules.Match("bash", nil)
	assert.Equal(t, DecisionAllow, decision)
	assert.NotNil(t, rule)
	assert.Equal(t, "allow bash", rule.Name)
}

func TestRules_MatchToolName_NoMatch(t *testing.T) {
	rules := NewRules()
	rules.Add(Rule{
		Name:        "allow bash",
		ToolPattern: "bash",
		Decision:    DecisionAllow,
		Priority:    100,
	})

	decision, _ := rules.Match("python", nil)
	assert.Equal(t, DecisionAskUser, decision, "should default to ask_user when no rule matches")
}

func TestRules_MatchToolName_Wildcard(t *testing.T) {
	rules := NewRules()
	rules.Add(Rule{
		Name:        "allow all file tools",
		ToolPattern: "file_*",
		Decision:    DecisionAllow,
		Priority:    100,
	})

	decision, _ := rules.Match("file_read", nil)
	assert.Equal(t, DecisionAllow, decision)

	decision, _ = rules.Match("file_write", nil)
	assert.Equal(t, DecisionAllow, decision)

	decision, _ = rules.Match("bash", nil)
	assert.Equal(t, DecisionAskUser, decision)
}

func TestRules_MatchToolName_AllWildcard(t *testing.T) {
	rules := NewRules()
	rules.Add(Rule{
		Name:        "deny all",
		ToolPattern: "*",
		Decision:    Deny,
		Priority:    100,
	})

	decision, _ := rules.Match("bash", nil)
	assert.Equal(t, Deny, decision)

	decision, _ = rules.Match("python", nil)
	assert.Equal(t, Deny, decision)
}

func TestRules_MatchWithArgPath(t *testing.T) {
	rules := NewRules()
	rules.Add(Rule{
		Name:        "deny sensitive files",
		ToolPattern: "file_*",
		ArgPath:     "file",
		ArgPattern:  "*.key",
		Decision:    Deny,
		Priority:    50,
	})

	// Matches tool pattern and arg pattern
	decision, _ := rules.Match("file_read", map[string]any{"file": "/etc/secret.key"})
	assert.Equal(t, Deny, decision)

	// Matches tool pattern but not arg pattern
	decision, _ = rules.Match("file_read", map[string]any{"file": "/tmp/data.txt"})
	assert.Equal(t, DecisionAskUser, decision)

	// Doesn't match tool pattern
	decision, _ = rules.Match("bash", map[string]any{"command": "cat secret.key"})
	assert.Equal(t, DecisionAskUser, decision)
}

func TestRules_MatchWithArgPath_MissingArg(t *testing.T) {
	rules := NewRules()
	rules.Add(Rule{
		Name:        "deny config writes",
		ToolPattern: "file_write",
		ArgPath:     "file",
		ArgPattern:  "*.json",
		Decision:    Deny,
		Priority:    50,
	})

	// Tool matches but arg path not present → rule doesn't match
	decision, _ := rules.Match("file_write", map[string]any{"content": "data"})
	assert.Equal(t, DecisionAskUser, decision)
}

func TestRules_PriorityOrdering(t *testing.T) {
	rules := NewRules()
	// Lower priority = evaluated first
	rules.Add(Rule{
		Name:        "deny all file tools",
		ToolPattern: "file_*",
		Decision:    Deny,
		Priority:    10,
	})
	rules.Add(Rule{
		Name:        "allow file_read",
		ToolPattern: "file_read",
		Decision:    DecisionAllow,
		Priority:    200,
	})

	// First matching rule wins (priority 10 = deny)
	decision, rule := rules.Match("file_read", nil)
	assert.Equal(t, Deny, decision)
	assert.Equal(t, "deny all file tools", rule.Name)
}

func TestRules_PriorityOrdering_Reversed(t *testing.T) {
	rules := NewRules()
	// Higher priority = evaluated first (lower number)
	rules.Add(Rule{
		Name:        "allow file_read",
		ToolPattern: "file_read",
		Decision:    DecisionAllow,
		Priority:    10,
	})
	rules.Add(Rule{
		Name:        "deny all file tools",
		ToolPattern: "file_*",
		Decision:    Deny,
		Priority:    200,
	})

	// First matching rule wins (priority 10 = allow)
	decision, rule := rules.Match("file_read", nil)
	assert.Equal(t, DecisionAllow, decision)
	assert.Equal(t, "allow file_read", rule.Name)
}

func TestRules_LenAndClear(t *testing.T) {
	rules := NewRules()
	assert.Equal(t, 0, rules.Len())

	rules.Add(Rule{Name: "test1", Decision: DecisionAllow})
	rules.Add(Rule{Name: "test2", Decision: Deny})
	assert.Equal(t, 2, rules.Len())

	rules.Clear()
	assert.Equal(t, 0, rules.Len())
}

// ─── Engine Tests ─────────────────────────────────────────────────────────────

func TestEngine_Check_SessionAllow(t *testing.T) {
	engine := NewEngine()

	// Before allow: default ask_user
	decision, _ := engine.Check("bash", nil)
	assert.Equal(t, DecisionAskUser, decision)

	// Allow for session
	engine.AllowAlways("bash")
	decision, _ = engine.Check("bash", nil)
	assert.Equal(t, DecisionAllow, decision)
}

func TestEngine_Check_SessionDeny(t *testing.T) {
	engine := NewEngine()

	engine.DenyAlways("bash")
	decision, _ := engine.Check("bash", nil)
	assert.Equal(t, Deny, decision)
}

func TestEngine_Check_RuleBasedDecision(t *testing.T) {
	engine := NewEngine()

	engine.Rules().Add(Rule{
		Name:        "allow read-only tools",
		ToolPattern: "file_read",
		Decision:    DecisionAllow,
		Priority:    100,
	})
	engine.Rules().Add(Rule{
		Name:        "ask for write tools",
		ToolPattern: "file_write",
		Decision:    DecisionAskUser,
		Priority:    100,
	})

	decision, _ := engine.Check("file_read", nil)
	assert.Equal(t, DecisionAllow, decision)

	decision, _ = engine.Check("file_write", nil)
	assert.Equal(t, DecisionAskUser, decision)

	// Unknown tool: default ask_user
	decision, _ = engine.Check("bash", nil)
	assert.Equal(t, DecisionAskUser, decision)
}

func TestEngine_Check_SessionOverrideRule(t *testing.T) {
	engine := NewEngine()

	// Rule says deny
	engine.Rules().Add(Rule{
		Name:        "deny bash",
		ToolPattern: "bash",
		Decision:    Deny,
	})

	decision, _ := engine.Check("bash", nil)
	assert.Equal(t, Deny, decision)

	// Session allow overrides rule
	engine.AllowAlways("bash")
	decision, _ = engine.Check("bash", nil)
	assert.Equal(t, DecisionAllow, decision)
}

func TestEngine_ClearSession(t *testing.T) {
	engine := NewEngine()

	engine.AllowAlways("bash")
	decision, _ := engine.Check("bash", nil)
	assert.Equal(t, DecisionAllow, decision)

	engine.ClearSession()
	decision, _ = engine.Check("bash", nil)
	assert.Equal(t, DecisionAskUser, decision, "should revert to default after clear")
}

func TestEngine_ClearAll(t *testing.T) {
	engine := NewEngine()

	engine.AllowAlways("bash")
	engine.DenyAlways("rm")

	engine.ClearAll()

	// Session decisions cleared, back to default (ask_user)
	decision, _ := engine.Check("bash", nil)
	assert.Equal(t, DecisionAskUser, decision, "session allow should be cleared")

	decision, _ = engine.Check("rm", nil)
	assert.Equal(t, DecisionAskUser, decision, "session deny should be cleared")
}

func TestEngine_AllowDenySwitch(t *testing.T) {
	engine := NewEngine()

	engine.AllowAlways("bash")
	decision, _ := engine.Check("bash", nil)
	assert.Equal(t, DecisionAllow, decision)

	// Switching from allow to deny
	engine.DenyAlways("bash")
	decision, _ = engine.Check("bash", nil)
	assert.Equal(t, Deny, decision)

	// Switching back to allow
	engine.AllowAlways("bash")
	decision, _ = engine.Check("bash", nil)
	assert.Equal(t, DecisionAllow, decision)
}

// ─── Persistence Tests ────────────────────────────────────────────────────────

func TestEngine_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, ".pi-go", "policy.json")

	engine := NewEngine()
	err := engine.AllowPersist("bash", policyPath)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(policyPath)
	require.NoError(t, err)

	// Load into new engine
	engine2 := NewEngine()
	err = engine2.LoadFromFile(policyPath)
	require.NoError(t, err)

	decision, _ := engine2.Check("bash", nil)
	assert.Equal(t, DecisionAllow, decision)
}

func TestEngine_Persistence_Deny(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, ".pi-go", "policy.json")

	engine := NewEngine()
	err := engine.DenyPersist("rm", policyPath)
	require.NoError(t, err)

	engine2 := NewEngine()
	err = engine2.LoadFromFile(policyPath)
	require.NoError(t, err)

	decision, _ := engine2.Check("rm", nil)
	assert.Equal(t, Deny, decision)
}

func TestPolicyFile_LoadNonExistent(t *testing.T) {
	pf, err := LoadPolicyFile("/nonexistent/path.json")
	require.NoError(t, err)
	assert.Empty(t, pf.Allowed)
	assert.Empty(t, pf.Denied)
}

func TestPolicyFile_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-policy.json")

	pf := &PolicyFile{
		Allowed: []string{"bash", "python"},
		Denied:  []string{"rm"},
	}

	err := SavePolicyFile(path, pf)
	require.NoError(t, err)

	loaded, err := LoadPolicyFile(path)
	require.NoError(t, err)

	assert.ElementsMatch(t, pf.Allowed, loaded.Allowed)
	assert.ElementsMatch(t, pf.Denied, loaded.Denied)
}

func TestDecision_JSON(t *testing.T) {
	tests := []struct {
		decision Decision
		json     string
	}{
		{DecisionAllow, `"allow"`},
		{DecisionAskUser, `"ask_user"`},
		{Deny, `"deny"`},
	}

	for _, tt := range tests {
		data, err := json.Marshal(tt.decision)
		require.NoError(t, err)
		assert.Equal(t, tt.json, string(data))

		var d Decision
		err = json.Unmarshal(data, &d)
		require.NoError(t, err)
		assert.Equal(t, tt.decision, d)
	}
}

func TestEngine_MarshalJSON(t *testing.T) {
	engine := NewEngine()
	engine.AllowAlways("bash")
	engine.DenyAlways("rm")

	data, err := json.Marshal(engine)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	sessionAllowed := result["session_allowed"].([]any)
	sessionDenied := result["session_denied"].([]any)
	assert.Contains(t, sessionAllowed, "bash")
	assert.Contains(t, sessionDenied, "rm")
}

// ─── Updater Tests ────────────────────────────────────────────────────────────

func TestUpdater_AllowDenyTool(t *testing.T) {
	engine := NewEngine()
	tmpDir := t.TempDir()
	updater := NewUpdater(engine, filepath.Join(tmpDir, "policy.json"))

	updater.AllowTool("bash")
	decision, _ := engine.Check("bash", nil)
	assert.Equal(t, DecisionAllow, decision)

	updater.DenyTool("bash")
	decision, _ = engine.Check("bash", nil)
	assert.Equal(t, Deny, decision)
}

func TestUpdater_ApplyDecision(t *testing.T) {
	engine := NewEngine()
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "policy.json")
	updater := NewUpdater(engine, policyPath)

	// Approve without persist
	err := updater.ApplyDecision("bash", true, false)
	require.NoError(t, err)
	decision, _ := engine.Check("bash", nil)
	assert.Equal(t, DecisionAllow, decision)

	// Deny with persist
	err = updater.ApplyDecision("rm", false, true)
	require.NoError(t, err)
	decision, _ = engine.Check("rm", nil)
	assert.Equal(t, Deny, decision)

	// Verify persisted
	engine2 := NewEngine()
	err = engine2.LoadFromFile(policyPath)
	require.NoError(t, err)
	decision, _ = engine2.Check("rm", nil)
	assert.Equal(t, Deny, decision)
}

func TestUpdater_ClearSession(t *testing.T) {
	engine := NewEngine()
	updater := NewUpdater(engine, "")

	updater.AllowTool("bash")
	decision, _ := engine.Check("bash", nil)
	assert.Equal(t, DecisionAllow, decision)

	updater.ClearSession()
	decision, _ = engine.Check("bash", nil)
	assert.Equal(t, DecisionAskUser, decision)
}

func TestDefaultPolicyPath(t *testing.T) {
	path := DefaultPolicyPath("/home/user/project")
	assert.Equal(t, "/home/user/project/.pi-go/policy.json", path)
}

func TestUpdater_LoadFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, ".pi-go", "policy.json")

	// Write a policy file
	pf := &PolicyFile{Allowed: []string{"git"}}
	err := SavePolicyFile(policyPath, pf)
	require.NoError(t, err)

	// Create updater and load
	engine := NewEngine()
	updater := NewUpdater(engine, policyPath)
	err = updater.LoadFromDisk()
	require.NoError(t, err)

	decision, _ := engine.Check("git", nil)
	assert.Equal(t, DecisionAllow, decision)
}

func TestUpdater_String(t *testing.T) {
	engine := NewEngine()
	s := engine.String()
	assert.Contains(t, s, "policy engine")
	assert.Contains(t, s, "0 rule")
}
