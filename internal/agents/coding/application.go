package coding

import (
	"github.com/earendil-works/pi-go/internal/agent"
	codingprompt "github.com/earendil-works/pi-go/internal/agents/coding/prompt"
	codingtools "github.com/earendil-works/pi-go/internal/agents/coding/tools"
	"github.com/earendil-works/pi-go/internal/runtime"
)

// CodingApplication implements runtime.Application for the coding-agent.
// It is the concrete application that gets injected into the Platform layer.
type CodingApplication struct{}

// BuildTools assembles the coding-agent toolset based on the provided options.
func (CodingApplication) BuildTools(opts runtime.ToolBuildOptions) []agent.Tool {
	return codingtools.BuildList(codingtools.ListOptions{
		Workspace:      opts.Workspace,
		MaxOutputLen:   opts.MaxOutputLen,
		EnableBash:     opts.EnableBash,
		BashOps:        opts.BashOps,
		FileOps:        opts.FileOps,
		ExtensionTools: opts.ExtensionTools,
		AllowedTools:   opts.AllowedTools,
		BlockedTools:   opts.BlockedTools,
	})
}

// BuildPrompt constructs the coding-agent system prompt based on the provided options.
func (CodingApplication) BuildPrompt(opts runtime.PromptBuildOptions) string {
	return codingprompt.BuildSystemPrompt(codingprompt.Options{
		CustomPrompt: opts.CustomPrompt,
		CWD:          opts.CWD,
		Tools:        opts.Tools,
		ContextFiles: opts.ContextFiles,
		Skills:       opts.Skills,
		Profile:      opts.Profile,
		Goal:         opts.Goal,
	})
}

// Verify interface compliance at compile time.
var _ runtime.Application = CodingApplication{}
