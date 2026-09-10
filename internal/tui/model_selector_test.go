package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hwj123hwj/pi-go/internal/app"
	"github.com/hwj123hwj/pi-go/sdk/config"
	"github.com/hwj123hwj/pi-go/sdk/slashcmd"
	"github.com/stretchr/testify/require"
)

type selectorApp struct {
	slashcmd.AppContext
	models []slashcmd.ModelInfo
}

func (a selectorApp) AvailableModels() []slashcmd.ModelInfo { return a.models }

func newSelectorTestModel(t *testing.T) *TuiModel {
	t.Helper()
	cfg := config.Default()
	cfg.Provider, cfg.OpenAIBaseURL, cfg.OpenAIAPIKey, cfg.OpenAIModel = "openai", "", "test-key", "model-00"
	cfg.DataDir, cfg.Workspace = t.TempDir(), t.TempDir()
	a, err := app.New(app.AppOptions{Config: cfg})
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close() })
	session, err := a.LoadOrCreateSession(context.Background(), "")
	require.NoError(t, err)
	reg := slashcmd.NewRegistry()
	reg.Register(slashcmd.Command{Name: "models", Handler: func(slashcmd.Context, string) (slashcmd.CommandResult, error) {
		t.Fatal("TUI must open selector, not print catalog")
		return slashcmd.CommandResult{}, nil
	}})
	m := New(session, reg)
	var models []slashcmd.ModelInfo
	for i := 0; i < 21; i++ {
		models = append(models, slashcmd.ModelInfo{Provider: "openai", ModelID: fmt.Sprintf("model-%02d", i)})
	}
	m.app = selectorApp{AppContext: a, models: models}
	return m
}

func TestModelsOpensNavigableSelector(t *testing.T) {
	m := newSelectorTestModel(t)
	for _, r := range "/models" {
		m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	require.True(t, m.modelSelect)
	require.True(t, m.input.IsEmpty())
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	require.Equal(t, 20, m.completion.SelectedIndex())
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	p, id := m.session.ModelInfo()
	require.Equal(t, "openai", p)
	require.Equal(t, "model-01", id)
	require.Equal(t, id, m.modelID)
	require.False(t, m.modelSelect)
	require.False(t, m.completion.IsActive())
}

func TestSelectorCancelAndScreenBounds(t *testing.T) {
	m := newSelectorTestModel(t)
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 40, Height: 12}} {
		m.Update(size)
		m.handleSlashCommand("/models")
		for i := 0; i < 20; i++ {
			m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
		}
		view := m.View()
		require.LessOrEqual(t, lipgloss.Height(view), size.Height)
		require.LessOrEqual(t, lipgloss.Width(view), size.Width)
		require.Equal(t, 1, strings.Count(view, "openai/model-20"))
		m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
		require.False(t, m.modelSelect)
		_, id := m.session.ModelInfo()
		require.Equal(t, "model-00", id)
	}
}

func TestMouseWheelStaysInTUI(t *testing.T) {
	m := newSelectorTestModel(t)
	m.handleSlashCommand("/models")
	m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	require.Equal(t, 1, m.completion.SelectedIndex())
	m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	require.Equal(t, 0, m.completion.SelectedIndex())
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	m.viewport.SetMessages([]ChatMessage{{Role: "system", Content: strings.Repeat("line\n", 60)}})
	m.viewport.GotoBottom()
	before := m.viewport.scrollOffset
	m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	require.Less(t, m.viewport.scrollOffset, before)
}
