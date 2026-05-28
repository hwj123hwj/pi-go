package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/agents/coding"
	"github.com/earendil-works/pi-go/internal/ai/providers"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/earendil-works/pi-go/internal/extensions"
	"github.com/earendil-works/pi-go/internal/operations"
	"github.com/earendil-works/pi-go/internal/runtime"
	"github.com/earendil-works/pi-go/internal/sessionmgr"
	"github.com/earendil-works/pi-go/internal/slashcmd"
)

// App is the thin assembly layer for the agent.
// It assembles dependencies (providers, session manager, runtime registry, etc.)
// but does NOT carry session-specific behavior — that belongs to AgentSession.
type App struct {
	cfg          config.Config
	skillDirs    []string
	sessionMgr   *sessionmgr.Manager
	registry     *providers.Registry
	sessionStore *runtime.SessionRegistry
	extRegistry  *extensions.Registry
	application  runtime.Application
	extraTools   []agent.ExternalToolDef
}

// AppOptions holds the options for creating a new App.
type AppOptions struct {
	Config      config.Config
	SkillDirs   []string
	Application runtime.Application // inject a concrete Application; defaults to CodingApplication
}

// New creates a new App, assembling all shared dependencies.
func New(opts AppOptions) (*App, error) {
	cfg := opts.Config

	// Provider registry
	reg := providers.NewRegistry()
	if err := registerProviders(reg, cfg); err != nil {
		return nil, fmt.Errorf("provider setup: %w", err)
	}

	// Session manager
	mgr := sessionmgr.NewManager(cfg.DataDir)

	// Extension registry
	extReg := extensions.NewRegistry()

	// Session registry (runtime)
	store := runtime.NewSessionRegistry()

	// Use injected Application, or default to CodingApplication
	application := opts.Application
	if application == nil {
		application = coding.NewCodingApplication(cfg)
	}

	return &App{
		cfg:          cfg,
		skillDirs:    opts.SkillDirs,
		sessionMgr:   mgr,
		registry:     reg,
		sessionStore: store,
		extRegistry:  extReg,
		application:  application,
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

// SetExternalTools stores externally registered tool definitions (e.g. from bridge).
func (a *App) SetExternalTools(tools []agent.ExternalToolDef) {
	a.extraTools = tools
}

// deps constructs the Dependencies struct for AgentSession creation.
func (a *App) deps() runtime.Dependencies {
	return runtime.Dependencies{
		Registry:       a.registry,
		SessionMgr:     a.sessionMgr,
		ExtRegistry:    a.extRegistry,
		Application:    a.application,
		ExternalTools:  a.extraTools,
		BuildOperations: func(cfg config.Config, workspace string) *operations.Operations {
			switch cfg.ExecutionMode {
			case "ssh":
				return operations.NewSSHOperations(operations.SSHConfig{
					Host:    cfg.SSHHost,
					Port:    cfg.SSHPort,
					WorkDir: cfg.SSHWorkDir,
				})
			default:
				return operations.NewLocalOperations()
			}
		},
	}
}

// ToolNames returns the names of tools available given the current config and extensions.
// This accounts for EnableBash, AllowedTools, BlockedTools, and extension tools.
func (a *App) ToolNames() []string {
	cfg := a.cfg

	// Try to delegate to Application
	type toolNamer interface {
		ToolNames(enableBash bool) []string
	}
	var baseNames []string
	if tn, ok := a.application.(toolNamer); ok {
		baseNames = tn.ToolNames(cfg.EnableBash)
	} else {
		baseNames = coding.BaseToolNames(cfg.EnableBash)
	}

	// Extension tools
	if a.extRegistry != nil {
		for _, t := range a.extRegistry.Tools() {
			baseNames = append(baseNames, t.Name())
		}
	}

	// External tools (registered via HTTP)
	for _, def := range a.extraTools {
		baseNames = append(baseNames, def.Name)
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
// Returns an error if the configured provider is selected but cannot be initialized
// (e.g. missing API key), rather than silently falling back to mock.
func registerProviders(registry *providers.Registry, cfg config.Config) error {
	// Mock provider is always available
	registry.Register(providers.NewMockProvider())

	switch cfg.Provider {
	case "anthropic":
		if cfg.AnthropicAPIKey != "" {
			registry.Register(providers.NewAnthropicProvider(cfg.AnthropicAPIKey, cfg.AnthropicBaseURL))
			slog.Info("registered anthropic provider", "model", cfg.AnthropicModel, "base_url", cfg.AnthropicBaseURL)
			return nil
		}
		return fmt.Errorf("PI_GO_PROVIDER=anthropic but ANTHROPIC_API_KEY is empty")
	case "openai":
		if cfg.OpenAIAPIKey != "" {
			registry.Register(providers.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL))
			slog.Info("registered openai provider", "model", cfg.OpenAIModel, "base_url", cfg.OpenAIBaseURL)
			return nil
		}
		return fmt.Errorf("PI_GO_PROVIDER=openai but OPENAI_API_KEY is empty")
	case "deepv":
		if cfg.DeepVEnabled {
			workDir := cfg.DeepVWorkDir
			if workDir == "" {
				workDir, _ = os.Getwd()
			}
			registry.Register(providers.NewDeepVProvider(cfg.DeepVServerURL, coding.NewDeepVHeaderProvider(workDir)))
			slog.Info("registered deepv provider", "model", cfg.DeepVModel, "server", cfg.DeepVServerURL)
			return nil
		}
		return fmt.Errorf("PI_GO_PROVIDER=deepv but DEEPV_ENABLED is not true")
	case "mock":
		slog.Info("using mock provider")
		return nil
	default:
		return fmt.Errorf("unknown PI_GO_PROVIDER %q (valid values: mock, anthropic, openai, deepv)", cfg.Provider)
	}
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

// CreateSession creates a new session and returns it as a SessionContext.
// This implements the slashcmd.AppContext interface.
func (a *App) CreateSession(ctx context.Context) (slashcmd.SessionContext, error) {
	return a.NewSession(ctx)
}

// SwitchSession loads an existing session and returns it as a SessionContext.
// This implements the slashcmd.AppContext interface.
func (a *App) SwitchSession(ctx context.Context, sessionID string) (slashcmd.SessionContext, error) {
	return a.LoadSession(ctx, sessionID)
}

// Profiles returns the list of available profile names.
// This implements the slashcmd.AppContext interface.
func (a *App) Profiles() []string {
	type profileLister interface{ Profiles() []string }
	if pl, ok := a.application.(profileLister); ok {
		return pl.Profiles()
	}
	return nil
}

// AvailableModels returns the list of models available for switching.
// This implements the slashcmd.AppContext interface.
func (a *App) AvailableModels() []slashcmd.ModelInfo {
	type modelLister interface{ AvailableModels() []slashcmd.ModelInfo }
	if ml, ok := a.application.(modelLister); ok {
		return ml.AvailableModels()
	}
	return nil
}

// Ensure App implements slashcmd.AppContext at compile time.
var _ slashcmd.AppContext = (*App)(nil)
