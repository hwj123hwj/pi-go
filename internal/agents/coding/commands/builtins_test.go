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
	profile   string
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

func (m *mockSession) Profile() string {
	if m.profile == "" {
		return "coding"
	}
	return m.profile
}

func (m *mockSession) SwitchProfile(_ context.Context, profile string) error {
	switch profile {
	case "coding", "review":
		m.profile = profile
		return nil
	default:
		return assert.AnError
	}
}

type mockApp struct {
	sessions     []slashcmd.SessionInfo
	newSession   slashcmd.SessionContext
	switchTarget slashcmd.SessionContext
	newErr       error
	switchErr    error
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

func (m *mockApp) SwitchSession(ctx context.Context, sessionID string) (slashcmd.SessionContext, error) {
	if m.switchErr != nil {
		return nil, m.switchErr
	}
	return m.switchTarget, nil
}

func (m *mockApp) Profiles() []string {
	return []string{"coding", "review"}
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
	assert.Contains(t, reg.Names(), "switch")
	assert.Contains(t, reg.Names(), "tools")
	assert.Contains(t, reg.Names(), "model")
	assert.Contains(t, reg.Names(), "profiles")
	assert.Contains(t, reg.Names(), "profile")
}

func TestHelp_Formatted(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "test"},
		App:     &mockApp{},
	}
	result, err := reg.Execute(cmdCtx, "/help")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Session management")
	assert.Contains(t, result.Output, "Information")
	assert.Contains(t, result.Output, "/help")
	assert.Contains(t, result.Output, "/tools")
	assert.Contains(t, result.Output, "/new")
	assert.Contains(t, result.Output, "/switch")
	assert.Contains(t, result.Output, "/profiles")
	assert.Contains(t, result.Output, "/profile")
}

func TestTools_UsesRealToolList(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{toolNames: []string{"bash", "read", "grep", "custom_tool"}},
		App:     &mockApp{},
	}
	result, err := reg.Execute(cmdCtx, "/tools")
	require.NoError(t, err)

	assert.Contains(t, result.Output, "bash")
	assert.Contains(t, result.Output, "read")
	assert.Contains(t, result.Output, "grep")
	assert.Contains(t, result.Output, "custom_tool")
}

func TestSession_ShowsFullInfo(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "sess_abc123", provider: "anthropic", modelID: "claude-3.5-sonnet"},
		App:     &mockApp{},
	}
	result, err := reg.Execute(cmdCtx, "/session")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "sess_abc123")
	assert.Contains(t, result.Output, "anthropic")
	assert.Contains(t, result.Output, "claude-3.5-sonnet")
	assert.Contains(t, result.Output, "Tools:")
	assert.Contains(t, result.Output, "Profile:")
}

func TestModel_SwitchWithProvider(t *testing.T) {
	reg := newRegistry()

	sess := &mockSession{provider: "anthropic", modelID: "claude-3.5-sonnet"}
	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: sess,
		App:     &mockApp{},
	}
	result, err := reg.Execute(cmdCtx, "/model openai:gpt-4o")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Switched:")
	assert.Contains(t, result.Output, "openai")
	assert.Contains(t, result.Output, "gpt-4o")
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
	result, err := reg.Execute(cmdCtx, "/sessions")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "sess_current")
	assert.Contains(t, result.Output, "sess_other")
	assert.Contains(t, result.Output, "→")
}

func TestNew_CreatesSession(t *testing.T) {
	reg := newRegistry()

	newSess := &mockSession{sessionID: "sess_new", provider: "mock", modelID: "mock"}
	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "sess_old"},
		App:     &mockApp{newSession: newSess},
	}
	result, err := reg.Execute(cmdCtx, "/new")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "sess_new")
	assert.Contains(t, result.Output, "Created new session")
}

func TestNew_ReturnsSessionSwitchTo(t *testing.T) {
	reg := newRegistry()

	newSess := &mockSession{sessionID: "sess_new", provider: "mock", modelID: "mock"}
	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "sess_old"},
		App:     &mockApp{newSession: newSess},
	}
	result, err := reg.Execute(cmdCtx, "/new")
	require.NoError(t, err)
	assert.NotNil(t, result.SessionSwitchTo)
	assert.Equal(t, "sess_new", result.SessionSwitchTo.SessionID())
}

func TestSwitch_LoadsSession(t *testing.T) {
	reg := newRegistry()

	targetSess := &mockSession{sessionID: "sess_target", provider: "anthropic", modelID: "claude-3.5-sonnet"}
	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "sess_current"},
		App:     &mockApp{switchTarget: targetSess},
	}
	result, err := reg.Execute(cmdCtx, "/switch sess_target")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "sess_target")
	assert.Contains(t, result.Output, "Switched to session")
	assert.NotNil(t, result.SessionSwitchTo)
	assert.Equal(t, "sess_target", result.SessionSwitchTo.SessionID())
}

func TestSwitch_NoArgs(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "sess_current"},
		App:     &mockApp{},
	}
	result, err := reg.Execute(cmdCtx, "/switch")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Usage:")
}

func TestProfiles_ListsProfiles(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "test", profile: "coding"},
		App:     &mockApp{},
	}
	result, err := reg.Execute(cmdCtx, "/profiles")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "coding")
	assert.Contains(t, result.Output, "review")
	assert.Contains(t, result.Output, "→ coding")
}

func TestProfile_ShowCurrent(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "test", profile: "coding"},
		App:     &mockApp{},
	}
	result, err := reg.Execute(cmdCtx, "/profile")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Current profile:")
	assert.Contains(t, result.Output, "coding")
}

func TestProfile_SwitchToReview(t *testing.T) {
	reg := newRegistry()

	sess := &mockSession{sessionID: "test", profile: "coding"}
	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: sess,
		App:     &mockApp{},
	}
	result, err := reg.Execute(cmdCtx, "/profile review")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "Switched profile:")
	assert.Contains(t, result.Output, "review")
	assert.Equal(t, "review", sess.Profile())
}

func TestProfile_SwitchInvalid(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "test"},
		App:     &mockApp{},
	}
	_, err := reg.Execute(cmdCtx, "/profile nonexistent")
	assert.Error(t, err)
}

func TestBranch_IsPlaceholder(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "test"},
		App:     &mockApp{},
	}
	result, err := reg.Execute(cmdCtx, "/branch")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "not yet implemented")
}

func TestSession_IncludesProfile(t *testing.T) {
	reg := newRegistry()

	cmdCtx := slashcmd.Context{
		Ctx:     context.Background(),
		Session: &mockSession{sessionID: "sess_abc123", provider: "anthropic", modelID: "claude-3.5-sonnet", profile: "review"},
		App:     &mockApp{},
	}
	result, err := reg.Execute(cmdCtx, "/session")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "review")
}
