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
	skillDirs    []string
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
		skillDirs:    opts.SkillDirs,
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
		SkillDirs: a.skillDirs,
	}
	return a.sessionStore.Create(ctx, opts, deps)
}

// LoadSession loads an existing AgentSession by ID.
// If already loaded in the registry, returns the cached instance.
func (a *App) LoadSession(ctx context.Context, sessionID string) (*runtime.AgentSession, error) {
	deps := a.deps()
	opts := runtime.AgentSessionOptions{
		Config:    a.cfg,
		SkillDirs: a.skillDirs,
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

// ToolNames returns the names of tools available given the current config and extensions.
// This accounts for EnableBash, AllowedTools, BlockedTools, and extension tools.
func (a *App) ToolNames() []string {
	cfg := a.cfg

	baseNames := []string{"read", "write", "edit", "grep", "find", "ls"}
	if cfg.EnableBash {
		baseNames = append([]string{"bash"}, baseNames...)
	}

	// Extension tools
	if a.extRegistry != nil {
		for _, t := range a.extRegistry.Tools() {
			baseNames = append(baseNames, t.Name())
		}
	}

	// Apply filtering
	if len(cfg.AllowedTools) == 0 && len(cfg.BlockedTools) == 0 {
		return baseNames
	}

	allowedSet := make(map[string]bool, len(cfg.AllowedTools))
	for _, name := range cfg.AllowedTools {
		allowedSet[name] = true
	}
	blockedSet := make(map[string]bool, len(cfg.BlockedTools))
	for _, name := range cfg.BlockedTools {
		blockedSet[name] = true
	}

	var filtered []string
	for _, name := range baseNames {
		if len(cfg.AllowedTools) > 0 && !allowedSet[name] {
			continue
		}
		if blockedSet[name] {
			continue
		}
		filtered = append(filtered, name)
	}
	return filtered
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
	case "deepv":
		if cfg.DeepVEnabled {
			workDir := cfg.DeepVWorkDir
			if workDir == "" {
				workDir, _ = os.Getwd()
			}
			registry.Register(providers.NewDeepVProvider(cfg.DeepVServerURL, workDir))
			slog.Info("registered deepv provider", "model", cfg.DeepVModel, "server", cfg.DeepVServerURL)
		} else {
			slog.Warn("deepv provider selected but DEEPV_ENABLED is not true, falling back to mock")
		}
	default:
		slog.Info("using mock provider (set PI_GO_PROVIDER=anthropic, openai, or deepv for real LLM)")
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
