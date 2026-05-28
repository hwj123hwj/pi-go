package mode

import (
	"github.com/earendil-works/pi-go/internal/app"
	"github.com/earendil-works/pi-go/internal/server"
	"github.com/earendil-works/pi-go/internal/slashcmd"
)

// ServeMode handles the HTTP server mode.
// It uses App to support multi-session routing.
type ServeMode struct {
	app       *app.App
	slashCmds *slashcmd.Registry
}

// NewServeMode creates a new serve mode.
func NewServeMode(application *app.App, slashCmds *slashcmd.Registry) *ServeMode {
	return &ServeMode{
		app:       application,
		slashCmds: slashCmds,
	}
}

// Run starts the HTTP server.
func (m *ServeMode) Run(listenAddr string) error {
	srv := server.New(m.app, m.slashCmds)
	return srv.ListenAndServe(listenAddr)
}
