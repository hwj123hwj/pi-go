package coding

import (
	"github.com/earendil-works/pi-go/internal/agent"
	codingprompt "github.com/earendil-works/pi-go/internal/agents/coding/prompt"
	codingtools "github.com/earendil-works/pi-go/internal/agents/coding/tools"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/earendil-works/pi-go/internal/runtime"
	"github.com/earendil-works/pi-go/internal/slashcmd"
	"github.com/earendil-works/pi-go/internal/agents/coding/profile"
)

// CodingApplication implements runtime.Application for the coding-agent.
// It is the concrete application that gets injected into the Platform layer.
type CodingApplication struct {
	Cfg config.Config
}

// NewCodingApplication creates a new CodingApplication with the given config.
func NewCodingApplication(cfg config.Config) CodingApplication {
	return CodingApplication{Cfg: cfg}
}

// BuildTools assembles the coding-agent toolset based on the provided options.
func (a CodingApplication) BuildTools(opts runtime.ToolBuildOptions) []agent.Tool {
	mutationQueue := codingtools.NewFileMutationQueue()
	return codingtools.BuildList(codingtools.ListOptions{
		Workspace:         opts.Workspace,
		MaxOutputLen:      opts.MaxOutputLen,
		EnableBash:        a.Cfg.EnableBash,
		BashOps:           opts.BashOps,
		FileOps:           opts.FileOps,
		ExtensionTools:    opts.ExtensionTools,
		AllowedTools:      opts.AllowedTools,
		BlockedTools:      opts.BlockedTools,
		FileMutationQueue: mutationQueue,
	})
}

// BuildPrompt constructs the coding-agent system prompt based on the provided options.
func (a CodingApplication) BuildPrompt(opts runtime.PromptBuildOptions, profileName, goal string) string {
	return codingprompt.BuildSystemPrompt(codingprompt.Options{
		CustomPrompt: opts.CustomPrompt,
		CWD:          opts.CWD,
		Tools:        opts.Tools,
		ContextFiles: opts.ContextFiles,
		Skills:       opts.Skills,
		Profile:      profileName,
		Goal:         goal,
	})
}

// NewSessionExt creates a per-session CodingSessionExt.
// The rebuild callback is nil initially; AgentSession injects it later via SetRebuild.
func (a CodingApplication) NewSessionExt() runtime.SessionExt {
	return NewCodingSessionExt(nil)
}

// Profiles returns the list of available profile names.
func (CodingApplication) Profiles() []string {
	return profile.All()
}

// AvailableModels returns the list of models available for switching.
func (CodingApplication) AvailableModels() []slashcmd.ModelInfo {
	return []slashcmd.ModelInfo{
		{Provider: "anthropic", ModelID: "claude-sonnet-4-6"},
		{Provider: "anthropic", ModelID: "claude-sonnet-4-5"},
		{Provider: "anthropic", ModelID: "claude-sonnet-4"},
		{Provider: "openai", ModelID: "gpt-4o"},
		{Provider: "openai", ModelID: "gpt-4o-mini"},
		{Provider: "deepv", ModelID: "glm-5"},
		{Provider: "deepv", ModelID: "deepseek-v4-flash"},
		{Provider: "deepv", ModelID: "deepseek-v4-pro"},
		{Provider: "deepv", ModelID: "kimi-k2.6"},
		{Provider: "mock", ModelID: "mock"},
	}
}

// ToolNames returns the canonical coding-agent tool names.
func (CodingApplication) ToolNames(enableBash bool) []string {
	return codingtools.BaseToolNames(enableBash)
}

// Verify interface compliance at compile time.
var _ runtime.Application = CodingApplication{}
