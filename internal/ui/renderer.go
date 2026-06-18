package ui

import (
	"github.com/earendil-works/pi-go/internal/agent"
)

// TUIRenderer is an abstraction for terminal UI rendering.
// Two implementations exist:
//   - EnhancedPresenter (default, lightweight, ~7.6MB binary)
//   - FancyPresenter (build with -tags fancy, ~11MB, uses Bubble Tea)
type TUIRenderer interface {
	Present(event agent.AgentStreamEvent)
}

// Verify EnhancedPresenter implements TUIRenderer at compile time.
var _ TUIRenderer = (*EnhancedPresenter)(nil)
