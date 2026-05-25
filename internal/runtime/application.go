package runtime

import (
	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/operations"
	"github.com/earendil-works/pi-go/internal/prompt"
	"github.com/earendil-works/pi-go/internal/skill"
)

// Application is the interface that an agent application must implement.
// The runtime (Platform layer) depends on this interface, not on any
// concrete application (e.g. coding-agent). The App layer is responsible
// for choosing and injecting the concrete implementation.
type Application interface {
	// BuildTools assembles the tool list for the agent session.
	BuildTools(opts ToolBuildOptions) []agent.Tool

	// BuildPrompt constructs the system prompt for the agent session.
	BuildPrompt(opts PromptBuildOptions) string
}

// ToolBuildOptions contains the context needed by an Application to build its tool list.
type ToolBuildOptions struct {
	Workspace      string
	MaxOutputLen   int
	EnableBash     bool
	BashOps        operations.BashOperations
	FileOps        operations.FileOperations
	ExtensionTools []agent.Tool
	AllowedTools   []string
	BlockedTools   []string
}

// PromptBuildOptions contains the context needed by an Application to build its system prompt.
type PromptBuildOptions struct {
	CustomPrompt string
	CWD          string
	Tools        []agent.Tool
	ContextFiles []prompt.ContextFile
	Skills       []skill.Skill
	Profile      string
	Goal         string
}
