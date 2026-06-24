package mode

import (
	"context"
	"fmt"

	"github.com/hwj123hwj/pi-go/internal/runtime"
)

// PrintMode handles one-shot prompt execution (non-interactive).
type PrintMode struct {
	session *runtime.AgentSession
}

// NewPrintMode creates a new print mode.
func NewPrintMode(session *runtime.AgentSession) *PrintMode {
	return &PrintMode{
		session: session,
	}
}

// Run executes a single prompt and prints the result.
func (m *PrintMode) Run(ctx context.Context, prompt string) error {
	assistant, err := m.session.Prompt(ctx, prompt)
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	fmt.Println(assistant.Text)
	return nil
}
