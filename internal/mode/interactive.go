package mode

import (
	codingcli "github.com/hwj123hwj/pi-go/internal/agents/coding/cli"
	"github.com/hwj123hwj/pi-go/internal/app"
	"github.com/hwj123hwj/pi-go/sdk/runtime"
	"github.com/hwj123hwj/pi-go/sdk/slashcmd"
)

// InteractiveMode is the interactive CLI entrypoint.
// Currently wraps the coding-agent CLI; will become mode-agnostic when
// multiple Applications exist.
type InteractiveMode = codingcli.InteractiveMode

// NewInteractiveMode creates a new interactive mode for the coding-agent.
func NewInteractiveMode(session *runtime.AgentSession, cmds *slashcmd.Registry, application *app.App) *InteractiveMode {
	return codingcli.NewInteractiveMode(session, cmds, application)
}
