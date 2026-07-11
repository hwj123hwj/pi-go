package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hwj123hwj/pi-go/internal/runtime"
	"github.com/hwj123hwj/pi-go/internal/slashcmd"
	"github.com/hwj123hwj/pi-go/internal/ui"
)

// Run starts the Bubble Tea TUI program.
// This is the main entry point for the new TUI interactive mode.
func Run(session *runtime.AgentSession, cmds *slashcmd.Registry) error {
	m := New(session, cmds)

	// Set workspace for display
	if cwd, err := os.Getwd(); err == nil {
		m.SetWorkspace(cwd)
	}

	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Print banner before entering alt screen
	ui.PrintBanner(os.Stdout)

	_, err := p.Run()
	return err
}

// BannerText returns the banner string for the TUI.
func BannerText() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("╔══════════════════════════════════════════╗\n")
	b.WriteString("║     π-go Interactive TUI (Bubble Tea)    ║\n")
	b.WriteString("║                                          ║\n")
	b.WriteString("║  Enter: Send  │  Shift+Enter: Newline    ║\n")
	b.WriteString("║  Ctrl+C: Cancel/Exit  │  Ctrl+L: Clear    ║\n")
	b.WriteString("║  ↑↓: History  │  Ctrl+Z: Undo             ║\n")
	b.WriteString("╚══════════════════════════════════════════╝\n")
	b.WriteString("\n")
	return b.String()
}

// QuitMsg is a message that triggers program exit.
type QuitMsg struct{}

// String returns a human-readable description of the model state (for debugging).
func (m *TuiModel) String() string {
	return fmt.Sprintf("TuiModel{msgs:%d streaming:%v busy:%v input:%q}",
		len(m.messages), m.streaming, m.agentBusy, m.input.Text())
}
