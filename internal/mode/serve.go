package mode

import (
	"net/http"

	"github.com/hwj123hwj/pi-go/internal/app"
	"github.com/hwj123hwj/pi-go/internal/server"
	"github.com/hwj123hwj/pi-go/internal/slashcmd"
)

// ServeMode handles the HTTP server mode.
// It uses App to support multi-session routing.
type ServeMode struct {
	app       *app.App
	slashCmds *slashcmd.Registry
	extraMux  *http.ServeMux // optional extra routes (e.g. music audio proxy)
}

// NewServeMode creates a new serve mode.
func NewServeMode(application *app.App, slashCmds *slashcmd.Registry) *ServeMode {
	return &ServeMode{
		app:       application,
		slashCmds: slashCmds,
	}
}

// SetExtraRoutes sets an additional ServeMux to be merged into the server's routes.
// This is used by music-agent to register audio proxy endpoints without modifying server.go.
func (m *ServeMode) SetExtraRoutes(mux *http.ServeMux) {
	m.extraMux = mux
}

// Run starts the HTTP server.
func (m *ServeMode) Run(listenAddr string) error {
	srv := server.New(m.app, m.slashCmds)
	srv.SetVersion(Version)
	if m.extraMux != nil {
		srv.SetExtraRoutes(m.extraMux)
	}
	// Enable API key auth if configured
	if m.app.Config().APIKey != "" {
		srv.SetAPIKey(m.app.Config().APIKey)
	}
	return srv.ListenAndServe(listenAddr)
}

// Version is set by main via SetVersion.
var Version = "dev"

// SetVersion sets the build version.
func SetVersion(v string) {
	Version = v
}
