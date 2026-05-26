package mode

import (
	codingcli "github.com/earendil-works/pi-go/internal/agents/coding/cli"
	"github.com/earendil-works/pi-go/internal/app"
	"github.com/earendil-works/pi-go/internal/runtime"
	"github.com/earendil-works/pi-go/internal/slashcmd"
)

// InteractiveMode is the interactive CLI entrypoint.
// Currently wraps the coding-agent CLI; will become mode-agnostic when
// multiple Applications exist.
type InteractiveMode = codingcli.InteractiveMode

// NewInteractiveMode creates a new interactive mode for the coding-agent.
func NewInteractiveMode(session *runtime.AgentSession, cmds *slashcmd.Registry, application *app.App) *InteractiveMode {
	return codingcli.NewInteractiveMode(session, cmds, application)
}
