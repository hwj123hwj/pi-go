package slashcmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock implementations ────────────────────────────────────────────────────

type mockSession struct {
	sessionID   string
	provider    string
	modelID     string
	toolNames   []string
	switchErr   error
}

func (m *mockSession) SessionID() string { return m.sessionID }
func (m *mockSession) ModelInfo() (string, string) {
	if m.provider == "" && m.modelID == "" {
		return "mock", "mock"
	}
	return m.provider, m.modelID
}
func (m *mockSession) SwitchModel(ctx context.Context, modelID string, provider string) error {
	if m.switchErr != nil {
		return m.switchErr
	}
	m.modelID = modelID
	if provider != "" {
		m.provider = provider
	}
	return nil
}
func (m *mockSession) ToolNames() []string {
	if m.toolNames == nil {
		return []string{"bash", "read", "write", "edit"}
	}
	return m.toolNames
}

type mockApp struct {
	sessions    []SessionInfo
	newSession  SessionContext
	newErr      error
}

func (m *mockApp) ListSessionsInfo() ([]SessionInfo, error) {
	return m.sessions, nil
}
func (m *mockApp) CreateSession(ctx context.Context) (SessionContext, error) {
	if m.newErr != nil {
		return nil, m.newErr
	}
	return m.newSession, nil
}

// ─── /help tests ─────────────────────────────────────────────────────────────

func TestHelp_Formatted(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "test"},
		App:     &mockApp{},
	}
	output, err := reg.Execute(cmdCtx, "/help")
	require.NoError(t, err)

	// Should have grouped sections
	assert.Contains(t, output, "Session management")
	assert.Contains(t, output, "Information")
	assert.Contains(t, output, "/help")
	assert.Contains(t, output, "/tools")
	assert.Contains(t, output, "/new")
}

// ─── /tools tests ────────────────────────────────────────────────────────────

func TestTools_UsesRealToolList(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: &mockSession{toolNames: []string{"bash", "read", "grep", "custom_tool"}},
		App:     &mockApp{},
	}
	output, err := reg.Execute(cmdCtx, "/tools")
	require.NoError(t, err)

	assert.Contains(t, output, "bash")
	assert.Contains(t, output, "read")
	assert.Contains(t, output, "grep")
	assert.Contains(t, output, "custom_tool")
	assert.NotContains(t, output, "Built-in tools:") // should not be hardcoded anymore
}

func TestTools_NoSession(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: nil,
		App:     &mockApp{},
	}
	output, err := reg.Execute(cmdCtx, "/tools")
	require.NoError(t, err)
	assert.Equal(t, "no active session", output)
}

// ─── /session tests ──────────────────────────────────────────────────────────

func TestSession_ShowsFullInfo(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "sess_abc123", provider: "anthropic", modelID: "claude-3.5-sonnet"},
		App:     &mockApp{},
	}
	output, err := reg.Execute(cmdCtx, "/session")
	require.NoError(t, err)

	assert.Contains(t, output, "sess_abc123")
	assert.Contains(t, output, "anthropic")
	assert.Contains(t, output, "claude-3.5-sonnet")
	assert.Contains(t, output, "Tools:")
}

// ─── /model tests ────────────────────────────────────────────────────────────

func TestModel_ShowCurrent(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: &mockSession{provider: "openai", modelID: "gpt-4o"},
		App:     &mockApp{},
	}
	output, err := reg.Execute(cmdCtx, "/model")
	require.NoError(t, err)
	assert.Contains(t, output, "openai/gpt-4o")
}

func TestModel_SwitchWithProvider(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	sess := &mockSession{provider: "anthropic", modelID: "claude-3.5-sonnet"}
	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: sess,
		App:     &mockApp{},
	}
	output, err := reg.Execute(cmdCtx, "/model openai:gpt-4o")
	require.NoError(t, err)
	assert.Contains(t, output, "Switched:")
	assert.Contains(t, output, "openai")
	assert.Contains(t, output, "gpt-4o")
}

func TestModel_SwitchWithoutProvider(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	sess := &mockSession{provider: "anthropic", modelID: "claude-3.5-sonnet"}
	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: sess,
		App:     &mockApp{},
	}
	output, err := reg.Execute(cmdCtx, "/model claude-sonnet-4")
	require.NoError(t, err)
	assert.Contains(t, output, "Switched:")
}

func TestModel_SwitchError(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	sess := &mockSession{provider: "mock", modelID: "mock", switchErr: assert.AnError}
	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: sess,
		App:     &mockApp{},
	}
	_, err := reg.Execute(cmdCtx, "/model gpt-4o")
	assert.Error(t, err)
}

// ─── /sessions tests ─────────────────────────────────────────────────────────

func TestSessions_ListWithCurrentMarked(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "sess_current"},
		App: &mockApp{
			sessions: []SessionInfo{
				{ID: "sess_current", MessageCount: 5, LastActive: 1700000000},
				{ID: "sess_other", MessageCount: 3, LastActive: 1699990000},
			},
		},
	}
	output, err := reg.Execute(cmdCtx, "/sessions")
	require.NoError(t, err)

	assert.Contains(t, output, "sess_current")
	assert.Contains(t, output, "sess_other")
	assert.Contains(t, output, "→") // current marker
	assert.Contains(t, output, "5") // message count
}

func TestSessions_NoSessions(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "test"},
		App:     &mockApp{sessions: []SessionInfo{}},
	}
	output, err := reg.Execute(cmdCtx, "/sessions")
	require.NoError(t, err)
	assert.Contains(t, output, "no sessions")
}

// ─── /compact tests ──────────────────────────────────────────────────────────

func TestCompact_HonestMessage(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "test"},
		App:     &mockApp{},
	}
	output, err := reg.Execute(cmdCtx, "/compact")
	require.NoError(t, err)
	assert.Contains(t, output, "automatically")
	assert.Contains(t, output, "not yet implemented")
}

// ─── /branch tests ───────────────────────────────────────────────────────────

func TestBranch_NotImplemented(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "test"},
		App:     &mockApp{},
	}
	output, err := reg.Execute(cmdCtx, "/branch abc123")
	require.NoError(t, err)
	assert.Contains(t, output, "not yet implemented")
}

// ─── /new tests ──────────────────────────────────────────────────────────────

func TestNew_CreatesSession(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	newSess := &mockSession{sessionID: "sess_new", provider: "mock", modelID: "mock"}
	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "sess_old"},
		App:     &mockApp{newSession: newSess},
	}
	output, err := reg.Execute(cmdCtx, "/new")
	require.NoError(t, err)
	assert.Contains(t, output, "sess_new")
	assert.Contains(t, output, "Created new session")
}

func TestNew_NoApp(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmdCtx := Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "test"},
		App:     nil,
	}
	output, err := reg.Execute(cmdCtx, "/new")
	require.NoError(t, err)
	assert.Contains(t, output, "app not available")
}

// ─── Registry tests ──────────────────────────────────────────────────────────

func TestRegistry_Command(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Command("help")
	assert.Equal(t, "help", cmd.Name)
	assert.Equal(t, "List all commands", cmd.Description)

	cmd = reg.Command("nonexistent")
	assert.Equal(t, "", cmd.Name)
}

func TestRegistry_ParseSlashCommand(t *testing.T) {
	name, args := ParseSlashCommand("/help")
	assert.Equal(t, "help", name)
	assert.Equal(t, "", args)

	name, args = ParseSlashCommand("/model gpt-4o")
	assert.Equal(t, "model", name)
	assert.Equal(t, "gpt-4o", args)

	name, args = ParseSlashCommand("  /compact reason here  ")
	assert.Equal(t, "compact", name)
	assert.Equal(t, "reason here", args)
}
