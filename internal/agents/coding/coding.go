package coding

import (
	codingtools "github.com/hwj123hwj/pi-go/internal/agents/coding/tools"
	codingdeepv "github.com/hwj123hwj/pi-go/internal/agents/coding/deepv"
	codingcommands "github.com/hwj123hwj/pi-go/internal/agents/coding/commands"
	"github.com/hwj123hwj/pi-go/internal/slashcmd"
)

func BaseToolNames(enableBash bool) []string {
	return codingtools.BaseToolNames(enableBash)
}

func RegisterCommands(registry *slashcmd.Registry) {
	codingcommands.RegisterBuiltins(registry)
	codingcommands.RegisterWikiCommands(registry)
}

func NewDeepVHeaderProvider(workDir string) interface{ Headers() map[string]string } {
	return codingdeepv.HeaderProvider{WorkDir: workDir}
}
