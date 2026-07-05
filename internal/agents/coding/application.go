package coding

import (
	"github.com/hwj123hwj/pi-go/internal/agent"
	"github.com/hwj123hwj/pi-go/internal/agents/coding/commands"
	codingprofile "github.com/hwj123hwj/pi-go/internal/agents/coding/profile"
	codingprompt "github.com/hwj123hwj/pi-go/internal/agents/coding/prompt"
	codingtools "github.com/hwj123hwj/pi-go/internal/agents/coding/tools"
	"github.com/hwj123hwj/pi-go/internal/config"
	modelsreg "github.com/hwj123hwj/pi-go/internal/models"
	"github.com/hwj123hwj/pi-go/internal/runtime"
	"github.com/hwj123hwj/pi-go/internal/slashcmd"
)

// CodingApplication implements runtime.Application for the coding-agent.
// It is the concrete application that gets injected into the Platform layer.
type CodingApplication struct {
	Cfg         config.Config
	modelReg    *modelsreg.Registry
}

// NewCodingApplication creates a new CodingApplication with the given config.
func NewCodingApplication(cfg config.Config) CodingApplication {
	modelConfigPath := modelsreg.ResolveConfigPath(cfg.DataDir)
	return CodingApplication{
		Cfg:      cfg,
		modelReg: modelsreg.NewDefaultRegistry(modelConfigPath),
	}
}

// BuildTools assembles the coding-agent toolset based on the provided options.
func (a CodingApplication) BuildTools(opts runtime.ToolBuildOptions) []agent.Tool {
	mutationQueue := codingtools.NewFileMutationQueue()
	backupMgr := commands.GetUndoManager()
	return codingtools.BuildList(codingtools.ListOptions{
		Workspace:          opts.Workspace,
		MaxOutputLen:       opts.MaxOutputLen,
		EnableBash:         a.Cfg.EnableBash,
		BashOps:            opts.BashOps,
		EnableWeb:          a.Cfg.EnableWeb,
		WebTimeoutSeconds:  a.Cfg.WebTimeoutSeconds,
		EnableWebSearch:    a.Cfg.EnableWebSearch,
		FileOps:            opts.FileOps,
		ExtensionTools:     opts.ExtensionTools,
		AllowedTools:       opts.AllowedTools,
		BlockedTools:       opts.BlockedTools,
		FileMutationQueue:  mutationQueue,
		BackupManager:      backupMgr,
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
	return codingprofile.All()
}

// AvailableModels returns the list of models available for switching.
// Uses the config-driven model registry, falling back to defaults.
func (a CodingApplication) AvailableModels() []slashcmd.ModelInfo {
	all := a.modelReg.List()
	result := make([]slashcmd.ModelInfo, 0, len(all))
	for _, m := range all {
		result = append(result, slashcmd.ModelInfo{
			Provider: m.Provider,
			ModelID:  m.ID,
		})
	}
	return result
}

// ToolNames returns the canonical coding-agent tool names.
func (CodingApplication) ToolNames(enableBash bool) []string {
	return codingtools.BaseToolNames(enableBash)
}

// Verify interface compliance at compile time.
var _ runtime.Application = CodingApplication{}
