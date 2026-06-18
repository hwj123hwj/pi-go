//go:build !fancy

package ui

import "io"

// NewTUI creates the lightweight TUI renderer (default mode).
// Build with -tags fancy to use Bubble Tea instead.
func NewTUI(w io.Writer) TUIRenderer {
	return NewEnhancedPresenter(w)
}

// IsFancyMode reports whether the binary was built with -tags fancy.
func IsFancyMode() bool { return false }
