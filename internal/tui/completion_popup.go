package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// CompletionPopup renders the autocomplete dropdown as a floating panel.
// It sits just above the input area.
//
//   ┌──────────────────────────┐
//   │ /help    Show this help  │ ← highlighted (selected)
//   │ /history  View history   │
//   └──────────────────────────┘
type CompletionPopup struct {
	theme *Theme
}

// NewCompletionPopup creates a new popup renderer.
func NewCompletionPopup() *CompletionPopup {
	return &CompletionPopup{theme: DefaultTheme()}
}

// Render produces the popup string from a CompletionState.
func (cp *CompletionPopup) Render(cm *CompletionState, width int) string {
	if !cm.IsActive() {
		return ""
	}

	items := cm.Items()
	selected := cm.SelectedIndex()

	// Calculate column widths
	maxLabel := 0
	for _, item := range items {
		w := lipgloss.Width(item.Label)
		if w > maxLabel {
			maxLabel = w
		}
	}

	// Build popup lines
	var lines []string
	for i, item := range items {
		label := item.Label
		desc := item.Description

		// Pad label to align descriptions
		padded := label + strings.Repeat(" ", maxInt(1, maxLabel-lipgloss.Width(label)+2))

		// Truncate description to fit width
		availDesc := width - lipgloss.Width(padded) - 4
		if availDesc > 0 && lipgloss.Width(desc) > availDesc {
			desc = truncateRunes(desc, availDesc-1) + "…"
		}

		var line string
		if i == selected {
			// Highlighted row — reverse video / accent background
			line = cp.theme.StatusAccent.Render(padded) +
				cp.theme.StatusDim.Render(desc)
			// Full-width highlight
			fullContent := padded + desc
			padW := width - 2 - lipgloss.Width(fullContent)
			if padW > 0 {
				line = line + strings.Repeat(" ", padW)
			}
			line = "\x1b[7m" + line + "\x1b[0m"
		} else {
			line = cp.theme.ToolHeader.Render(padded) +
				cp.theme.StatusDim.Render(desc)
		}

		lines = append(lines, line)
	}

	// Wrap in border
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#0969DA", Dark: "#58A6FF"}).
		Padding(0, 0).
		Width(width - 2)

	content := strings.Join(lines, "\n")
	return popupStyle.Render(content)
}

// ConfirmationPopup renders a yes/no confirmation dialog.
//
//   ┌─ ⚠️ Confirm ──────────────────────────────────┐
//   │ Run: rm -rf /tmp/cache                         │
//   │                                                │
//   │   [Y] Yes    [N] No    [Esc] Cancel           │
//   └────────────────────────────────────────────────┘
type ConfirmationPopup struct {
	theme *Theme
}

// NewConfirmationPopup creates a new confirmation dialog renderer.
func NewConfirmationPopup() *ConfirmationPopup {
	return &ConfirmationPopup{theme: DefaultTheme()}
}

// Render produces the confirmation dialog.
// `selected` is 0=Yes, 1=No.
func (cp *ConfirmationPopup) Render(description string, selected int, width int) string {
	title := cp.theme.WarnText.Render("⚠️  Confirm")

	// Wrap description to fit width
	descWidth := width - 6
	descLines := wrapText(description, descWidth)

	var bodyLines []string
	bodyLines = append(bodyLines, title)
	bodyLines = append(bodyLines, "")
	for _, dl := range descLines {
		bodyLines = append(bodyLines, cp.theme.ToolBody.Render(dl))
	}
	bodyLines = append(bodyLines, "")

	// Buttons
	yesLabel := "  Y  Yes  "
	noLabel := "  N  No  "
	escLabel := cp.theme.StatusDim.Render("  Esc  Cancel  ")

	if selected == 0 {
		yesLabel = "\x1b[7m" + cp.theme.SuccessText.Render("▶ Y  Yes ") + "\x1b[0m"
		noLabel = cp.theme.StatusDim.Render("  N  No  ")
	} else {
		yesLabel = cp.theme.StatusDim.Render("  Y  Yes  ")
		noLabel = "\x1b[7m" + cp.theme.ErrorText.Render("▶ N  No ") + "\x1b[0m"
	}

	buttons := yesLabel + "   " + noLabel + "   " + escLabel
	bodyLines = append(bodyLines, buttons)

	content := strings.Join(bodyLines, "\n")

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#D29922", Dark: "#D29922"}).
		Padding(0, 1).
		Width(width - 2)

	return dialogStyle.Render(content)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// wrapText wraps text to fit within maxWidth runes per line.
func wrapText(text string, maxWidth int) []string {
	if maxWidth < 10 {
		maxWidth = 10
	}
	var result []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			if lipgloss.Width(line)+1+lipgloss.Width(word) <= maxWidth {
				line += " " + word
			} else {
				result = append(result, line)
				line = word
			}
		}
		result = append(result, line)
	}
	if len(result) == 0 {
		return []string{""}
	}
	return result
}

// RenderModelPopup renders the Ctrl+P model selector popup.
func (cp *CompletionPopup) RenderModelPopup(cm *CompletionState, width int) string {
	if !cm.IsActive() || cm.Kind() != CompletionModel {
		return ""
	}
	return cp.Render(cm, width)
}

// RenderConfirmationPopup renders a confirmation dialog overlay.
func RenderConfirmationPopup(desc string, selected int, width int) string {
	cp := NewConfirmationPopup()
	return cp.Render(desc, selected, width)
}

// fmtImportGuard ensures fmt is used (for future debugging).
var _ = fmt.Sprintf
