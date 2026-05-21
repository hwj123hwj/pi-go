package mode

import (
	"github.com/earendil-works/pi-go/internal/app"
	"github.com/earendil-works/pi-go/internal/server"
)

// ServeMode handles the HTTP server mode.
// It uses App to support multi-session routing.
type ServeMode struct {
	app *app.App
}

// NewServeMode creates a new serve mode.
func NewServeMode(application *app.App) *ServeMode {
	return &ServeMode{
		app: application,
	}
}

// Run starts the HTTP server.
func (m *ServeMode) Run(listenAddr string) error {
	srv := server.New(m.app)
	return srv.ListenAndServe(listenAddr)
}
