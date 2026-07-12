package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInputModel_InsertAndText(t *testing.T) {
	im := NewInputModel()
	im.insertString("hello")
	im.insertString(" world")

	got := im.Text()
	want := "hello world"
	if got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestInputModel_Backspace(t *testing.T) {
	im := NewInputModel()
	im.insertString("hello")
	im.backspace()

	got := im.Text()
	want := "hell"
	if got != want {
		t.Errorf("after backspace: Text() = %q, want %q", got, want)
	}
}

func TestInputModel_NewLine(t *testing.T) {
	im := NewInputModel()
	im.insertString("line1")
	im.newLine()
	im.insertString("line2")

	got := im.Text()
	want := "line1\nline2"
	if got != want {
		t.Errorf("after newLine: Text() = %q, want %q", got, want)
	}

	if im.cursorY != 1 {
		t.Errorf("cursorY = %d, want 1", im.cursorY)
	}
}

func TestInputModel_Undo(t *testing.T) {
	im := NewInputModel()
	im.insertString("hello")
	im.saveUndo()
	im.insertString(" world")
	im.undo()

	got := im.Text()
	want := "hello"
	if got != want {
		t.Errorf("after undo: Text() = %q, want %q", got, want)
	}
}

func TestInputModel_DeleteWordBackward(t *testing.T) {
	im := NewInputModel()
	im.insertString("hello world")
	// cursor is at end (11)
	im.deleteWordBackward()

	got := im.Text()
	want := "hello "
	if got != want {
		t.Errorf("after deleteWordBackward: Text() = %q, want %q", got, want)
	}
}

func TestInputModel_KillToEnd(t *testing.T) {
	im := NewInputModel()
	im.insertString("hello world")
	// Move cursor to position 5
	im.cursorX = 5
	im.killToEnd()

	got := im.Text()
	want := "hello"
	if got != want {
		t.Errorf("after killToEnd: Text() = %q, want %q", got, want)
	}
}

func TestInputModel_History(t *testing.T) {
	im := NewInputModel()
	im.AddHistory("first")
	im.AddHistory("second")

	// Navigate up (older)
	im.navigateHistory(-1)
	if im.Text() != "second" {
		t.Errorf("history[0] = %q, want 'second'", im.Text())
	}

	// Navigate up again (oldest)
	im.navigateHistory(-1)
	if im.Text() != "first" {
		t.Errorf("history[1] = %q, want 'first'", im.Text())
	}
}

func TestInputModel_IsEmpty(t *testing.T) {
	im := NewInputModel()
	if !im.IsEmpty() {
		t.Error("new input should be empty")
	}
	im.insertString("x")
	if im.IsEmpty() {
		t.Error("input with text should not be empty")
	}
}

func TestInputModel_Reset(t *testing.T) {
	im := NewInputModel()
	im.insertString("hello")
	im.Reset()
	if !im.IsEmpty() {
		t.Error("after Reset(), input should be empty")
	}
}

func TestMessageViewport_SetMessages(t *testing.T) {
	vp := NewMessageViewport(80, 10)
	vp.SetMessages([]ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	})

	if len(vp.lines) == 0 {
		t.Error("viewport should have rendered lines")
	}
}

func TestMessageViewport_Streaming(t *testing.T) {
	vp := NewMessageViewport(80, 10)
	vp.SetStreaming("partial response...")

	if len(vp.lines) == 0 {
		t.Error("viewport should have rendered streaming text")
	}
}

func TestStatusBarRender(t *testing.T) {
	sb := NewStatusBar()
	result := sb.Render(80, "ready", 0, "openai", "gpt-4o", "/home/user/pi-go", false, 0, 0)
	if result == "" {
		t.Error("StatusBar.Render() should not be empty")
	}
}

func TestTuiModel_Init(t *testing.T) {
	m := &TuiModel{
		input:    NewInputModel(),
		viewport: NewMessageViewport(80, 20),
		statusBar: *NewStatusBar(),
		messages:  []ChatMessage{},
		theme:     DefaultTheme(),
	}
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestInputModel_HandleKeyRunes(t *testing.T) {
	im := NewInputModel()
	im.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")})

	if im.Text() != "abc" {
		t.Errorf("after typing 'abc': Text() = %q, want 'abc'", im.Text())
	}
}

func TestToolPanelRender(t *testing.T) {
	tp := NewToolPanel(ToolCallInfo{
		Name:      "bash",
		Args:      "go test",
		Result:    "ok  pkg 0.1s",
		Collapsed: true,
	}, 60)
	lines := tp.Render()
	if len(lines) == 0 {
		t.Error("ToolPanel.Render() should produce output")
	}
}

func TestRenderDiff(t *testing.T) {
	theme := DefaultTheme()
	diff := "+added line\n-removed line\n context"
	lines := RenderDiff(diff, theme)
	if len(lines) != 3 {
		t.Errorf("RenderDiff should return 3 lines, got %d", len(lines))
	}
}

func TestMarkdownRender(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	result := mr.Render("**bold text**")
	if result == "" {
		t.Error("Markdown render should not be empty")
	}
}

func TestCompletionSlash(t *testing.T) {
	cm := NewCompletionState()
	// Simulate typing "/he"
	// We can't use a real registry here, so just test the state machine
	cm.kind = CompletionSlash
	cm.visible = true
	cm.items = []CompletionItem{
		{Label: "/help", Description: "Show help"},
		{Label: "/history", Description: "View history"},
	}

	if !cm.IsActive() {
		t.Error("completion should be active")
	}
	if len(cm.Items()) != 2 {
		t.Errorf("expected 2 items, got %d", len(cm.Items()))
	}

	// Test navigation
	cm.Next()
	if cm.SelectedIndex() != 1 {
		t.Errorf("after Next, selected = %d, want 1", cm.SelectedIndex())
	}
	cm.Prev()
	if cm.SelectedIndex() != 0 {
		t.Errorf("after Prev, selected = %d, want 0", cm.SelectedIndex())
	}

	// Test close
	cm.Close()
	if cm.IsActive() {
		t.Error("completion should be inactive after Close()")
	}
}

func TestConfirmationState(t *testing.T) {
	cs := NewConfirmationState()
	if cs.IsActive() {
		t.Error("new confirmation should be inactive")
	}

	cs.Show("tool-1", "bash", "Run: rm -rf /tmp/cache")
	if !cs.IsActive() {
		t.Error("confirmation should be active after Show()")
	}

	if cs.Selected() != 0 {
		t.Errorf("default selected = %d, want 0 (Yes)", cs.Selected())
	}

	cs.Hide()
	if cs.IsActive() {
		t.Error("confirmation should be inactive after Hide()")
	}
}

func TestKeyBindings(t *testing.T) {
	kb := NewKeyBindingTable()

	// Test input context
	action := kb.ResolveInput(tea.KeyMsg{Type: tea.KeyEnter})
	if action != ActionSubmit {
		t.Errorf("Enter should resolve to ActionSubmit, got %v", action)
	}

	action = kb.ResolveInput(tea.KeyMsg{Type: tea.KeyCtrlL})
	if action != ActionClearScreen {
		t.Errorf("Ctrl+L should resolve to ActionClearScreen, got %v", action)
	}

	// Test completion context
	action = kb.ResolveCompletion(tea.KeyMsg{Type: tea.KeyTab})
	if action != ActionAcceptCompletion {
		t.Errorf("Tab in completion should resolve to ActionAcceptCompletion, got %v", action)
	}

	// Test confirmation context
	action = kb.ResolveConfirmation(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if action != ActionSelectYes {
		t.Errorf("'y' in confirmation should resolve to ActionSelectYes, got %v", action)
	}
}

func TestToolPanelToggle(t *testing.T) {
	tp := NewToolPanel(ToolCallInfo{
		Name:      "edit",
		Args:      "test.go",
		Result:    "ok",
		Collapsed: true,
	}, 60)
	if !tp.info.Collapsed {
		t.Error("should start collapsed")
	}
	tp.ToggleCollapsed()
	if tp.info.Collapsed {
		t.Error("should be expanded after toggle")
	}
}
