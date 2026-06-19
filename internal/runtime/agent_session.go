package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/ai/models"
	"github.com/earendil-works/pi-go/internal/ai/providers"
	"github.com/earendil-works/pi-go/internal/compaction"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/earendil-works/pi-go/internal/extensions"
	"github.com/earendil-works/pi-go/internal/operations"
	"github.com/earendil-works/pi-go/internal/prompt"
	"github.com/earendil-works/pi-go/internal/session"
	"github.com/earendil-works/pi-go/internal/sessionmgr"
	"github.com/earendil-works/pi-go/internal/skill"
	"github.com/earendil-works/pi-go/internal/util"
)

// Dependencies holds the shared dependencies needed to create an AgentSession.
// These are provided by the App layer and shared across sessions.
type Dependencies struct {
	Registry         *providers.Registry
	SessionMgr       *sessionmgr.Manager
	ExtRegistry      *extensions.Registry
	Application      Application
	ExternalTools    []agent.ExternalToolDef
	BuildOperations  func(cfg config.Config, workspace string) *operations.Operations
}

// AgentSessionOptions holds the options for creating a new AgentSession.
type AgentSessionOptions struct {
	SessionID string
	Config    config.Config
	SkillDirs []string
}

// AgentSession is the central runtime abstraction for agent sessions.
// It unifies agent lifecycle, session, events, compaction, and branch navigation.
// All modes (interactive, print, serve) depend on this object.
// Application-specific state (profile, goal) is delegated to SessionExt.
type AgentSession struct {
	agent       *agent.Agent
	session     *session.Session
	sessionID   string
	sessionMgr  *sessionmgr.Manager
	cfg         config.Config
	extRegistry *extensions.Registry
	sessionPath string
	deps        Dependencies
	skillDirs   []string
	application Application
	ext         SessionExt
}

// NewAgentSession creates a new AgentSession.
// If sessionID is empty, a new session is created.
// If sessionID is provided, an existing session is loaded.
func NewAgentSession(ctx context.Context, opts AgentSessionOptions, deps Dependencies) (*AgentSession, error) {
	s := &AgentSession{
		cfg:         opts.Config,
		sessionMgr:  deps.SessionMgr,
		extRegistry: deps.ExtRegistry,
		deps:        deps,
		skillDirs:   opts.SkillDirs,
	}

	s.application = deps.Application

	// Create per-session extension (holds application-specific state like profile/goal)
	if deps.Application != nil {
		s.ext = deps.Application.NewSessionExt()
		if cse, ok := s.ext.(interface{ SetRebuild(func() error) }); ok {
			cse.SetRebuild(func() error {
				// Use Background instead of the creation context: profile/goal
					// changes can happen long after the request that created this
					// session has completed and its context been canceled.
					_, err := s.rebuildAgent(context.Background(), s.deps.Registry, s.skillDirs)
				return err
			})
		}
	}

	var err error

	if opts.SessionID != "" && s.sessionMgr.Exists(opts.SessionID) {
		// Load existing session
		s.sessionID = opts.SessionID
		s.session, s.sessionPath, err = s.sessionMgr.Open(ctx, opts.SessionID)
		if err != nil {
			return nil, fmt.Errorf("open session %q: %w", opts.SessionID, err)
		}
		slog.Info("loaded existing session", "id", opts.SessionID)
	} else {
		// Create new session
		s.sessionID, s.sessionPath, err = s.sessionMgr.Create(ctx)
		if err != nil {
			return nil, fmt.Errorf("create session: %w", err)
		}
		// Re-open to get Session object
		s.session, s.sessionPath, err = s.sessionMgr.Open(ctx, s.sessionID)
		if err != nil {
			return nil, fmt.Errorf("open new session: %w", err)
		}
		slog.Info("created new session", "id", s.sessionID)
	}

	// Build the agent
	ag, err := s.buildAgent(ctx, deps.Registry, opts.SkillDirs)
	if err != nil {
		return nil, fmt.Errorf("build agent: %w", err)
	}
	s.agent = ag

	return s, nil
}

// Prompt sends a message and waits for the complete response.
func (s *AgentSession) Prompt(ctx context.Context, input string) (ai.AssistantMessage, error) {
	return s.agent.Prompt(ctx, ai.NewTextUserMessage(input))
}

// PromptStream sends a message and returns an event channel for streaming.
func (s *AgentSession) PromptStream(ctx context.Context, input string) (<-chan agent.AgentStreamEvent, error) {
	return s.agent.PromptStream(ctx, ai.NewTextUserMessage(input))
}

// SessionID returns the current session ID.
func (s *AgentSession) SessionID() string {
	return s.sessionID
}

// Session returns the underlying session object.
func (s *AgentSession) Session() *session.Session {
	return s.session
}

// Agent returns the underlying agent.
func (s *AgentSession) Agent() *agent.Agent {
	return s.agent
}

// Config returns the current config.
func (s *AgentSession) Config() config.Config {
	return s.cfg
}

// Workspace returns the session's working directory.
func (s *AgentSession) Workspace() string {
	return s.cfg.Workspace
}

// ModelInfo returns the provider name and model ID.
func (s *AgentSession) ModelInfo() (string, string) {
	modelID := s.cfg.AnthropicModel
	providerName := s.cfg.Provider
	if providerName == "openai" {
		modelID = s.cfg.OpenAIModel
	}
	if providerName == "deepv" {
		modelID = s.cfg.DeepVModel
	}
	if providerName == "mock" || modelID == "" {
		modelID = "mock"
		providerName = "mock"
	}
	return providerName, modelID
}

// ToolNames returns the names of tools available in the current session.
func (s *AgentSession) ToolNames() []string {
	if s.agent == nil {
		return nil
	}
	return s.agent.ToolNames()
}

// SwitchModel changes the model (and optionally the provider) at runtime
// and rebuilds the agent. The change takes effect for subsequent prompts.
// If provider is non-empty, both provider and model are switched;
// otherwise only the model field for the current provider is updated.
func (s *AgentSession) SwitchModel(ctx context.Context, modelID string, provider string) error {
	// If provider is specified, switch to it
	if provider != "" && provider != s.cfg.Provider {
		s.cfg.Provider = provider
	}

	providerName := s.cfg.Provider
	switch providerName {
	case "openai":
		s.cfg.OpenAIModel = modelID
	case "deepv":
		s.cfg.DeepVModel = modelID
	case "anthropic":
		s.cfg.AnthropicModel = modelID
	default:
		s.cfg.Provider = "deepv"
		s.cfg.DeepVModel = modelID
		providerName = "deepv"
	}

	// Rebuild agent with new model
	ag, err := s.buildAgent(ctx, s.deps.Registry, s.skillDirs)
	if err != nil {
		return fmt.Errorf("rebuild agent with model %q: %w", modelID, err)
	}
	s.agent = ag

	slog.Info("switched model", "provider", providerName, "model", modelID)
	return nil
}

// Compact manually triggers context compaction.
// It generates an LLM summary of older messages, persists it to session storage,
// and returns the summary along with trimming stats.
func (s *AgentSession) Compact(ctx context.Context, customInstructions string) (string, int, int, error) {
	if s.agent == nil {
		return "", 0, 0, fmt.Errorf("no active agent")
	}
	return s.agent.CompactNow(ctx, customInstructions)
}

// Profile returns the current profile name.
// Delegates to SessionExt if available.
func (s *AgentSession) Profile() string {
	if s.ext != nil {
		return s.ext.Profile()
	}
	return ""
}

// SwitchProfile changes the active profile and rebuilds the agent
// so that the new profile's system prompt takes effect immediately.
// Delegates to SessionExt if available.
func (s *AgentSession) SwitchProfile(ctx context.Context, profile string) error {
	if s.ext == nil {
		return fmt.Errorf("profile switching not supported")
	}
	return s.ext.SwitchProfile(ctx, profile)
}

// Goal returns the current session goal.
// Delegates to SessionExt if available.
func (s *AgentSession) Goal() string {
	if s.ext != nil {
		return s.ext.Goal()
	}
	return ""
}

// SetGoal sets the current session goal and rebuilds the agent
// so the goal is injected into the system prompt immediately.
// Delegates to SessionExt if available.
func (s *AgentSession) SetGoal(goal string) {
	if s.ext == nil {
		return
	}
	s.ext.SetGoal(goal)
}

// ClearGoal clears the current session goal and rebuilds the agent.
// Delegates to SessionExt if available.
func (s *AgentSession) ClearGoal() {
	if s.ext == nil {
		return
	}
	s.ext.ClearGoal()
}

// MoveTo navigates the session to a specific entry (branch navigation).
func (s *AgentSession) MoveTo(ctx context.Context, entryID string, summary string) error {
	if s.session == nil {
		return fmt.Errorf("no active session")
	}
	return s.session.MoveTo(ctx, entryID, summary)
}

// Close cleans up resources.
func (s *AgentSession) Close() error {
	if s.session != nil && s.session.Storage() != nil {
		return s.session.Storage().Close()
	}
	return nil
}

// rebuildAgent rebuilds the agent with the current session state.
// This is called after profile/goal changes via SessionExt's rebuild callback.
func (s *AgentSession) rebuildAgent(ctx context.Context, registry *providers.Registry, skillDirs []string) (*agent.Agent, error) {
	ag, err := s.buildAgent(ctx, registry, skillDirs)
	if err != nil {
		return nil, err
	}
	s.agent = ag
	return ag, nil
}

// buildAgent constructs the agent.Agent for this session.
// Tool and prompt assembly is delegated to the injected Application.
func (s *AgentSession) buildAgent(ctx context.Context, registry *providers.Registry, skillDirs []string) (*agent.Agent, error) {
	cfg := s.cfg
	cwd := util.CWD()

	// Build tools via Application interface
	toolList := s.application.BuildTools(s.toolBuildOptions(cwd))

	// Determine model
	modelID := cfg.AnthropicModel
	providerName := cfg.Provider
	if providerName == "openai" {
		modelID = cfg.OpenAIModel
	}
	if providerName == "deepv" {
		modelID = cfg.DeepVModel
	}
	if providerName == "mock" || modelID == "" {
		modelID = "mock"
		providerName = "mock"
	}

	model := ai.Model{
		ID:            modelID,
		Name:          modelID,
		Provider:      providerName,
		ContextWindow: models.ContextWindow(modelID),
		MaxTokens:     4096,
	}

	// Compaction settings
	compactionSettings := compaction.DefaultSettings()
	var summarizeFunc compaction.SummarizeFunc
	if providerName != "mock" {
		summarizeFunc = compaction.LLMSummarizer(registry, model)
	}

	// Load context files
	contextFiles := prompt.LoadProjectContextFiles(cwd, "")

	// Load skills
	var skills []skill.Skill
	if len(skillDirs) == 0 {
		// Default skill dirs
		skillDirs = []string{}
		defaultSkillDir := filepath.Join(cwd, ".claude", "skills")
		if fi, err := os.Stat(defaultSkillDir); err == nil && fi.IsDir() {
			skillDirs = append(skillDirs, defaultSkillDir)
		}
		homeSkillDir := filepath.Join(util.HomeDir(), ".claude", "skills")
		if fi, err := os.Stat(homeSkillDir); err == nil && fi.IsDir() {
			skillDirs = append(skillDirs, homeSkillDir)
		}
	}
	if len(skillDirs) > 0 {
		result := skill.LoadFromDirs(skillDirs...)
		skills = result.Skills
		for _, diag := range result.Diagnostics {
			slog.Warn("skill diagnostic", "code", diag.Code, "message", diag.Message, "path", diag.Path)
		}
	}

	// Read profile/goal from SessionExt
	var profileName, goal string
	if s.ext != nil {
		profileName = s.ext.Profile()
		goal = s.ext.Goal()
	}

	// Build system prompt via Application interface
	systemPrompt := s.application.BuildPrompt(PromptBuildOptions{
		CustomPrompt: cfg.PromptTemplate,
		CWD:          cwd,
		Tools:        toolList,
		ContextFiles: contextFiles,
		Skills:       skills,
	}, profileName, goal)

	// Aggregate lifecycle hooks from extension registry
	var lifecycleHooks agent.LifecycleHooks
	if s.extRegistry != nil {
		lifecycleHooks = s.extRegistry.LifecycleHooks()
	}

	return agent.New(agent.Options{
		Model:              model,
		Registry:           registry,
		System:             systemPrompt,
		Tools:              toolList,
		MaxTurns:           cfg.MaxTurns,
		Goal:               goal,
		Session:            s.session,
		CompactionSettings: compactionSettings,
		SummarizeFunc:      summarizeFunc,
		LifecycleHooks:     lifecycleHooks,
	}), nil
}

// toolBuildOptions constructs ToolBuildOptions from the current config and session state.
func (s *AgentSession) toolBuildOptions(cwd string) ToolBuildOptions {
	cfg := s.cfg

	// Resolve workspace: prefer config, fallback to cwd
	workspace := cfg.Workspace
	if workspace == "" {
		workspace = cwd
	}

	// Build operations backend via Dependencies callback
	var ops *operations.Operations
	if s.deps.BuildOperations != nil {
		ops = s.deps.BuildOperations(cfg, workspace)
	} else {
		ops = operations.NewLocalOperations()
	}

	// In SSH mode, override workspace with remote working directory
	if cfg.ExecutionMode == "ssh" && cfg.SSHWorkDir != "" {
		workspace = cfg.SSHWorkDir
	}

	// Extension tools
	var extTools []agent.Tool
	if s.extRegistry != nil {
		extTools = s.extRegistry.Tools()
	}

	// External tools (registered via HTTP callback)
	for _, def := range s.deps.ExternalTools {
		if t, err := agent.NewExternalTool(def); err == nil {
			extTools = append(extTools, t)
		} else {
			slog.Warn("skip invalid external tool", "name", def.Name, "error", err)
		}
	}

	return ToolBuildOptions{
		Workspace:      workspace,
		MaxOutputLen:   cfg.MaxOutputLen,
		BashOps:        ops.Bash,
		FileOps:        ops.Files,
		ExtensionTools: extTools,
		AllowedTools:   cfg.AllowedTools,
		BlockedTools:   cfg.BlockedTools,
	}
}
