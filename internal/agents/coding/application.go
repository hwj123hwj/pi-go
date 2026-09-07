package coding

import (
	"context"
	"github.com/hwj123hwj/pi-go/internal/agents/coding/commands"
	codingprofile "github.com/hwj123hwj/pi-go/internal/agents/coding/profile"
	codingprompt "github.com/hwj123hwj/pi-go/internal/agents/coding/prompt"
	codingtools "github.com/hwj123hwj/pi-go/internal/agents/coding/tools"
	modelsreg "github.com/hwj123hwj/pi-go/internal/models"
	"github.com/hwj123hwj/pi-go/sdk/agent"
	"github.com/hwj123hwj/pi-go/sdk/config"
	"github.com/hwj123hwj/pi-go/sdk/runtime"
	"github.com/hwj123hwj/pi-go/sdk/slashcmd"
	"time"
)

// CodingApplication implements runtime.Application for the coding-agent.
// It is the concrete application that gets injected into the Platform layer.
type CodingApplication struct {
	Cfg      config.Config
	modelReg *modelsreg.Registry
}

// NewCodingApplication creates a new CodingApplication with the given config.
func NewCodingApplication(cfg config.Config) CodingApplication {
	modelConfigPath := modelsreg.ResolveConfigPath(cfg.DataDir)
	reg := modelsreg.NewDefaultRegistry(modelConfigPath)

	// 配置了 OpenAI 兼容网关时，拉取 /models 合并进注册表：
	// 网关是可用性的真实来源，本地清单只提供元数据（上下文窗口等）。
	// 拉取失败静默降级为本地清单——网关不在线不应阻塞启动。
	if cfg.OpenAIBaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if ids, err := modelsreg.FetchGatewayModels(ctx, cfg.OpenAIBaseURL, cfg.OpenAIAPIKey); err == nil {
			reg.MergeGateway("openai", ids)
		}
		cancel()
	}

	return CodingApplication{
		Cfg:      cfg,
		modelReg: reg,
	}
}

// BuildTools assembles the coding-agent toolset based on the provided options.
func (a CodingApplication) BuildTools(opts runtime.ToolBuildOptions) []agent.Tool {
	mutationQueue := codingtools.NewFileMutationQueue()
	backupMgr := commands.GetUndoManager()
	return codingtools.BuildList(codingtools.ListOptions{
		Workspace:         opts.Workspace,
		MaxOutputLen:      opts.MaxOutputLen,
		EnableBash:        a.Cfg.EnableBash,
		BashOps:           opts.BashOps,
		EnableWeb:         a.Cfg.EnableWeb,
		WebTimeoutSeconds: a.Cfg.WebTimeoutSeconds,
		EnableWebSearch:   a.Cfg.EnableWebSearch,
		FileOps:           opts.FileOps,
		ExtensionTools:    opts.ExtensionTools,
		AllowedTools:      opts.AllowedTools,
		BlockedTools:      opts.BlockedTools,
		FileMutationQueue: mutationQueue,
		BackupManager:     backupMgr,
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
