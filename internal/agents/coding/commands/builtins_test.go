package commands

import (
	"context"
	"testing"

	"github.com/earendil-works/pi-go/internal/slashcmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSession struct {
	sessionID string
	provider  string
	modelID   string
	toolNames []string
	switchErr error
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
	sessions   []slashcmd.SessionInfo
	newSession slashcmd.SessionContext
	newErr     error
}

func (m *mockApp) ListSessionsInfo() ([]slashcmd.SessionInfo, error) {
	return m.sessions, nil
}

func (m *mockApp) CreateSession(ctx context.Context) (slashcmd.SessionContext, error) {
	if m.newErr != nil {
		return nil, m.newErr
	}
	return m.newSession, nil
}

func newRegistry() *slashcmd.Registry {
	reg := slashcmd.NewRegistry()
	RegisterBuiltins(reg)
	return reg
}

func TestRegisterBuiltins(t *testing.T) {
	reg := newRegistry()

	assert.Contains(t, reg.Names(), "help")
	assert.Contains(t, reg.Names(), "compact")
	assert.Contains(t, reg.Names(), "sessions")
	assert.Contains(t, reg.Names(), "session")
	assert.Contains(t, reg.Names(), "branch")
	assert.Contains(t, reg.Names(), "new")
	assert.Contains(t, reg.Names(), "tools")
	assert.Contains(t, reg.Names(), "model")
}

func TestHelp_Formatted(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "test"},
		App:     &mockApp{},
	}
	output, err := reg.Execute(cmdCtx, "/help")
	require.NoError(t, err)
	assert.Contains(t, output, "Session management")
	assert.Contains(t, output, "Information")
	assert.Contains(t, output, "/help")
	assert.Contains(t, output, "/tools")
	assert.Contains(t, output, "/new")
}

func TestTools_UsesRealToolList(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
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
}

func TestSession_ShowsFullInfo(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
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

func TestModel_SwitchWithProvider(t *testing.T) {
	reg := newRegistry()

	sess := &mockSession{provider: "anthropic", modelID: "claude-3.5-sonnet"}
	cmdCtx := slashcmd.Context{
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

func TestSessions_ListWithCurrentMarked(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "sess_current"},
		App: &mockApp{
			sessions: []slashcmd.SessionInfo{
				{ID: "sess_current", MessageCount: 5, LastActive: 1700000000},
				{ID: "sess_other", MessageCount: 3, LastActive: 1699990000},
			},
		},
	}
	output, err := reg.Execute(cmdCtx, "/sessions")
	require.NoError(t, err)
	assert.Contains(t, output, "sess_current")
	assert.Contains(t, output, "sess_other")
	assert.Contains(t, output, "→")
}

func TestNew_CreatesSession(t *testing.T) {
	reg := newRegistry()

	newSess := &mockSession{sessionID: "sess_new", provider: "mock", modelID: "mock"}
	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "sess_old"},
		App:     &mockApp{newSession: newSess},
	}
	output, err := reg.Execute(cmdCtx, "/new")
	require.NoError(t, err)
	assert.Contains(t, output, "sess_new")
	assert.Contains(t, output, "Created new session")
}
