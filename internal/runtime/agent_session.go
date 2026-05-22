package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/ai/providers"
	"github.com/earendil-works/pi-go/internal/compaction"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/earendil-works/pi-go/internal/extensions"
	"github.com/earendil-works/pi-go/internal/operations"
	"github.com/earendil-works/pi-go/internal/prompt"
	"github.com/earendil-works/pi-go/internal/session"
	"github.com/earendil-works/pi-go/internal/sessionmgr"
	"github.com/earendil-works/pi-go/internal/skill"
	"github.com/earendil-works/pi-go/internal/tools"
	"github.com/earendil-works/pi-go/internal/util"
)

// Dependencies holds the shared dependencies needed to create an AgentSession.
// These are provided by the App layer and shared across sessions.
type Dependencies struct {
	Registry    *providers.Registry
	SessionMgr  *sessionmgr.Manager
	ExtRegistry *extensions.Registry
}

// AgentSessionOptions holds the options for creating a new AgentSession.
type AgentSessionOptions struct {
	SessionID string
	Config    config.Config
	SkillDirs []string
}

// AgentSession is the central runtime abstraction for the coding-agent.
// It unifies agent lifecycle, session, events, compaction, and branch navigation.
// All modes (interactive, print, serve) depend on this object.
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
func (s *AgentSession) Compact(ctx context.Context, reason string) error {
	slog.Info("manual compaction triggered", "reason", reason, "session", s.sessionID)
	// Compaction happens automatically in the agent loop;
	// this is a placeholder for manual compaction if we want to pre-emptively compact.
	return nil
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

// buildAgent constructs the agent.Agent for this session.
// This is the logic that was previously in main.go's buildAgent().
func (s *AgentSession) buildAgent(ctx context.Context, registry *providers.Registry, skillDirs []string) (*agent.Agent, error) {
	cfg := s.cfg
	cwd := util.CWD()

	// Build tools
	toolList := s.buildToolList(cwd)

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
		ContextWindow: contextWindowForModel(modelID),
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

	// Build system prompt
	systemPrompt := prompt.BuildSystemPrompt(prompt.Options{
		CustomPrompt: cfg.PromptTemplate,
		CWD:          cwd,
		Tools:        toolList,
		ContextFiles: contextFiles,
		Skills:       skills,
	})

	return agent.New(agent.Options{
		Model:              model,
		Registry:           registry,
		System:             systemPrompt,
		Tools:              toolList,
		MaxTurns:           cfg.MaxTurns,
		Session:            s.session,
		CompactionSettings: compactionSettings,
		SummarizeFunc:      summarizeFunc,
	}), nil
}

// buildToolList constructs the list of tools based on config.
func (s *AgentSession) buildToolList(cwd string) []agent.Tool {
	cfg := s.cfg

	// Resolve workspace: prefer config, fallback to cwd
	// For SSH mode, workspace must be the remote working directory
	workspace := cfg.Workspace
	if workspace == "" {
		workspace = cwd
	}

	// Build operations backend based on execution mode
	ops := s.buildOperations(workspace)

	// In SSH mode, override workspace with remote working directory
	// so tools resolve paths against the remote filesystem
	if cfg.ExecutionMode == "ssh" && cfg.SSHWorkDir != "" {
		workspace = cfg.SSHWorkDir
	}

	// Get base tools (all tools receive workspace for path resolution)
	toolList := []agent.Tool{}

	// Bash tool respects EnableBash config
	if cfg.EnableBash {
		toolList = append(toolList, tools.NewBashTool(
			tools.WithBashWorkspace(workspace),
			tools.WithBashMaxOutputLen(cfg.MaxOutputLen),
			tools.WithBashOperations(ops.Bash),
		))
	}

	toolList = append(toolList,
		tools.NewReadTool(
			tools.WithReadWorkspace(workspace),
			tools.WithReadMaxOutputLen(cfg.MaxOutputLen),
			tools.WithReadOperations(ops.Files),
		),
		tools.NewWriteTool(
			tools.WithWriteWorkspace(workspace),
			tools.WithWriteOperations(ops.Files),
		),
		tools.NewEditTool(
			tools.WithEditWorkspace(workspace),
			tools.WithEditOperations(ops.Files),
		),
		tools.NewGrepTool(
			tools.WithGrepWorkspace(workspace),
			tools.WithGrepMaxOutputLen(cfg.MaxOutputLen),
			tools.WithGrepOperations(ops.Files),
		),
		tools.NewFindTool(
			tools.WithFindWorkspace(workspace),
			tools.WithFindOperations(ops.Files),
		),
		tools.NewLsTool(
			tools.WithLsWorkspace(workspace),
			tools.WithLsMaxOutputLen(cfg.MaxOutputLen),
			tools.WithLsOperations(ops.Files),
		),
	)

	// Add extension tools
	if s.extRegistry != nil {
		extTools := s.extRegistry.Tools()
		toolList = append(toolList, extTools...)
	}

	// Apply AllowedTools/BlockedTools filtering
	toolList = filterTools(toolList, cfg.AllowedTools, cfg.BlockedTools)

	return toolList
}

// buildOperations creates the Operations container based on config.ExecutionMode.
func (s *AgentSession) buildOperations(workspace string) *operations.Operations {
	switch s.cfg.ExecutionMode {
	case "ssh":
		return operations.NewSSHOperations(operations.SSHConfig{
			Host:    s.cfg.SSHHost,
			Port:    s.cfg.SSHPort,
			WorkDir: s.cfg.SSHWorkDir,
		})
	default:
		return operations.NewLocalOperations()
	}
}

// filterTools applies allowed/blocked tool filters from config.
func filterTools(tools []agent.Tool, allowed []string, blocked []string) []agent.Tool {
	if len(allowed) == 0 && len(blocked) == 0 {
		return tools
	}

	allowedSet := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		allowedSet[name] = true
	}
	blockedSet := make(map[string]bool, len(blocked))
	for _, name := range blocked {
		blockedSet[name] = true
	}

	var filtered []agent.Tool
	for _, tool := range tools {
		name := tool.Name()
		if len(allowed) > 0 && !allowedSet[name] {
			continue
		}
		if blockedSet[name] {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

// contextWindowForModel returns the context window size for a given model ID.
func contextWindowForModel(modelID string) int {
	windows := map[string]int{
		"claude-3-5-sonnet":   200000,
		"claude-3-5-haiku":    200000,
		"claude-3-opus":       200000,
		"claude-sonnet-4":     200000,
		"claude-sonnet-4-5":   200000,
		"gpt-4o":              128000,
		"gpt-4o-mini":         128000,
		"gpt-4-turbo":         128000,
		"gpt-4":               8192,
		"o1":                  200000,
		"o1-mini":             128000,
		"o3-mini":             200000,
		"claude-sonnet-4-6":   200000,
		"glm-5":               128000,
		"deepseek-v4-flash":   128000,
	}
	if w, ok := windows[modelID]; ok {
		return w
	}
	return 128000
}
