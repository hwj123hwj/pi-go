package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMarkdownThemeSurvivesResize(t *testing.T) {
	original := lipgloss.HasDarkBackground()
	t.Cleanup(func() { lipgloss.SetHasDarkBackground(original) })
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			lipgloss.SetHasDarkBackground(dark)
			renderer := NewMarkdownRenderer(80)
			content := "你好！这是回复正文。\n\n- 列表内容\n\n**重点** 和 `代码`"
			before := renderer.Render(content)
			color := "\x1b[38;5;234m"
			if dark {
				color = "\x1b[38;5;252m"
			}
			require.Contains(t, before, color, "body color must match the terminal background")
			renderer.SetWidth(60)
			renderer.SetWidth(80)
			require.Equal(t, before, renderer.Render(content), "resizing must preserve the terminal theme")
		})
	}
}
