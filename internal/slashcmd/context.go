package slashcmd

// Context provides the runtime context that slash command handlers need.
// It uses interfaces to avoid import cycles with app/runtime packages.
type Context struct {
	// Session provides session-level operations.
	Session SessionContext

	// App provides app-level operations.
	App AppContext
}

// SessionContext is the interface for session operations available to slash commands.
type SessionContext interface {
	SessionID() string
	ModelInfo() (provider string, modelID string)
}

// AppContext is the interface for app operations available to slash commands.
type AppContext interface {
	ListSessionsInfo() ([]SessionInfo, error)
}

// SessionInfo holds session metadata for listing.
type SessionInfo struct {
	ID           string
	MessageCount int
	LastActive   int64
}
