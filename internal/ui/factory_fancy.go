//go:build fancy

package ui

import "io"

// NewTUI creates the fancy TUI renderer using Bubble Tea + Lipgloss.
// Built with -tags fancy.
func NewTUI(w io.Writer) TUIRenderer {
	return NewFancyPresenter(w)
}

// IsFancyMode reports whether the binary was built with -tags fancy.
func IsFancyMode() bool { return true }
