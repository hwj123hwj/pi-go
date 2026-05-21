package app

import (
	"context"
	"log/slog"
	"os"

	"github.com/earendil-works/pi-go/internal/ai/providers"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/earendil-works/pi-go/internal/extensions"
	"github.com/earendil-works/pi-go/internal/slashcmd"
	"github.com/earendil-works/pi-go/internal/runtime"
	"github.com/earendil-works/pi-go/internal/sessionmgr"
)

// App is the thin assembly layer for the coding-agent.
// It assembles dependencies (providers, session manager, runtime registry, etc.)
// but does NOT carry session-specific behavior — that belongs to AgentSession.
type App struct {
	cfg          config.Config
	sessionMgr   *sessionmgr.Manager
	registry     *providers.Registry
	sessionStore *runtime.SessionRegistry
	extRegistry  *extensions.Registry
}

// AppOptions holds the options for creating a new App.
type AppOptions struct {
	Config    config.Config
	SkillDirs []string
}

// New creates a new App, assembling all shared dependencies.
func New(opts AppOptions) (*App, error) {
	cfg := opts.Config

	// Provider registry
	reg := providers.NewRegistry()
	registerProviders(reg, cfg)

	// Session manager
	mgr := sessionmgr.NewManager(cfg.DataDir)

	// Extension registry
	extReg := extensions.NewRegistry()

	// Session registry (runtime)
	store := runtime.NewSessionRegistry()

	return &App{
		cfg:          cfg,
		sessionMgr:   mgr,
		registry:     reg,
		sessionStore: store,
		extRegistry:  extReg,
	}, nil
}

// NewSession creates a new AgentSession with a fresh session.
func (a *App) NewSession(ctx context.Context) (*runtime.AgentSession, error) {
	deps := a.deps()
	opts := runtime.AgentSessionOptions{
		Config:    a.cfg,
		SkillDirs: a.skillDirs(),
	}
	return a.sessionStore.Create(ctx, opts, deps)
}

// LoadSession loads an existing AgentSession by ID.
// If already loaded in the registry, returns the cached instance.
func (a *App) LoadSession(ctx context.Context, sessionID string) (*runtime.AgentSession, error) {
	deps := a.deps()
	opts := runtime.AgentSessionOptions{
		Config:    a.cfg,
		SkillDirs: a.skillDirs(),
	}
	return a.sessionStore.Load(ctx, sessionID, opts, deps)
}

// LoadOrCreateSession loads an existing session or creates a new one.
// If sessionID is empty, creates a new session.
func (a *App) LoadOrCreateSession(ctx context.Context, sessionID string) (*runtime.AgentSession, error) {
	if sessionID == "" {
		return a.NewSession(ctx)
	}
	return a.LoadSession(ctx, sessionID)
}

// SessionManager returns the session persistence manager.
func (a *App) SessionManager() *sessionmgr.Manager {
	return a.sessionMgr
}

// SessionStore returns the runtime session registry.
func (a *App) SessionStore() *runtime.SessionRegistry {
	return a.sessionStore
}

// Config returns the current configuration.
func (a *App) Config() config.Config {
	return a.cfg
}

// ExtRegistry returns the extension registry.
func (a *App) ExtRegistry() *extensions.Registry {
	return a.extRegistry
}

// Close cleans up all resources.
func (a *App) Close() error {
	return a.sessionStore.CloseAll()
}

// deps constructs the Dependencies struct for AgentSession creation.
func (a *App) deps() runtime.Dependencies {
	return runtime.Dependencies{
		Registry:    a.registry,
		SessionMgr:  a.sessionMgr,
		ExtRegistry: a.extRegistry,
	}
}

// skillDirs returns the configured skill directories.
func (a *App) skillDirs() []string {
	// For now, use default skill dirs (the AgentSession handles the logic)
	return nil
}

// registerProviders registers AI providers based on config.
func registerProviders(registry *providers.Registry, cfg config.Config) {
	// Mock provider is always available
	registry.Register(providers.NewMockProvider())

	switch cfg.Provider {
	case "anthropic":
		if cfg.AnthropicAPIKey != "" {
			registry.Register(providers.NewAnthropicProvider(cfg.AnthropicAPIKey, cfg.AnthropicBaseURL))
			slog.Info("registered anthropic provider", "model", cfg.AnthropicModel, "base_url", cfg.AnthropicBaseURL)
		} else {
			slog.Warn("anthropic provider selected but ANTHROPIC_API_KEY is empty, falling back to mock")
		}
	case "openai":
		if cfg.OpenAIAPIKey != "" {
			registry.Register(providers.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL))
			slog.Info("registered openai provider", "model", cfg.OpenAIModel, "base_url", cfg.OpenAIBaseURL)
		} else {
			slog.Warn("openai provider selected but OPENAI_API_KEY is empty, falling back to mock")
		}
	default:
		slog.Info("using mock provider (set PI_GO_PROVIDER=anthropic or openai for real LLM)")
	}
}

// homeDir returns the user's home directory.
func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return ""
}

// ListSessionsInfo implements slashcmd.AppContext.
func (a *App) ListSessionsInfo() ([]slashcmd.SessionInfo, error) {
	mgr := a.sessionMgr
	sessions, err := mgr.List(context.Background())
	if err != nil {
		return nil, err
	}
	result := make([]slashcmd.SessionInfo, len(sessions))
	for i, s := range sessions {
		result[i] = slashcmd.SessionInfo{
			ID:           s.ID,
			MessageCount: s.MessageCount,
			LastActive:   s.LastActive,
		}
	}
	return result, nil
}
