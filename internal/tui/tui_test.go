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

func TestSeparatorLine(t *testing.T) {
	line := separatorLine(10)
	// ─ is a multi-byte rune (3 bytes in UTF-8), so len() counts bytes.
	// Count runes instead.
	runeCount := len([]rune(line))
	if runeCount != 10 {
		t.Errorf("separatorLine(10) rune count = %d, want 10", runeCount)
	}
}

func TestTuiModel_Init(t *testing.T) {
	m := &TuiModel{
		input:    NewInputModel(),
		viewport: NewMessageViewport(80, 20),
		messages: []ChatMessage{},
	}
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init() should return nil for Phase 1")
	}
}

func TestInputModel_HandleKeyRunes(t *testing.T) {
	im := NewInputModel()
	im.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")})

	if im.Text() != "abc" {
		t.Errorf("after typing 'abc': Text() = %q, want 'abc'", im.Text())
	}
}
