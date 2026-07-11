package tui

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

// InputModel is a multi-line text input editor.
// Supports cursor movement, word deletion, undo, and history.
type InputModel struct {
	lines    []string
	cursorX  int
	cursorY  int
	undoStack []InputSnapshot
	history  []string
	histIdx  int
	prompt   string
}

// InputSnapshot captures input state for undo.
type InputSnapshot struct {
	lines   []string
	cursorX int
	cursorY int
}

// NewInputModel creates a new input editor.
func NewInputModel() InputModel {
	return InputModel{
		lines:    []string{""},
		cursorX:  0,
		cursorY:  0,
		undoStack: nil,
		history:  nil,
		histIdx:  -1,
		prompt:   "> ",
	}
}

// Text returns the full input text as a single string.
func (im *InputModel) Text() string {
	return strings.Join(im.lines, "\n")
}

// IsEmpty returns true if there's no text.
func (im *InputModel) IsEmpty() bool {
	return len(im.lines) == 1 && im.lines[0] == ""
}

// Reset clears the input.
func (im *InputModel) Reset() {
	im.saveUndo()
	im.lines = []string{""}
	im.cursorX = 0
	im.cursorY = 0
}

// HandleKey processes a key event.
func (im *InputModel) HandleKey(msg tea.KeyMsg) {
	switch msg.Type {

	case tea.KeyBackspace:
		im.backspace()

	case tea.KeyDelete:
		im.deleteForward()

	case tea.KeyLeft:
		im.cursorLeft()

	case tea.KeyRight:
		im.cursorRight()

	case tea.KeyUp:
		if im.cursorY == 0 {
			im.navigateHistory(-1)
		} else {
			im.cursorUp()
		}

	case tea.KeyDown:
		if im.cursorY == len(im.lines)-1 {
			im.navigateHistory(1)
		} else {
			im.cursorDown()
		}

	case tea.KeyHome, tea.KeyCtrlA:
		im.cursorX = 0

	case tea.KeyEnd, tea.KeyCtrlE:
		im.cursorX = utf8.RuneCountInString(im.lines[im.cursorY])

	case tea.KeyCtrlK:
		im.killToEnd()

	case tea.KeyCtrlU:
		im.killToStart()

	case tea.KeyCtrlW:
		im.deleteWordBackward()

	case tea.KeyCtrlZ:
		im.undo()

	case tea.KeyCtrlJ:
		im.newLine()

	default:
		// Regular character input
		if msg.Type == tea.KeyRunes || (msg.Alt && len(msg.Runes) > 0) {
			im.insertString(string(msg.Runes))
		}
	}
}

// View renders the input area.
func (im *InputModel) View() string {
	var buf strings.Builder

	for i, line := range im.lines {
		if i == 0 {
			buf.WriteString(im.prompt)
		} else {
			buf.WriteString("  ")
		}
		buf.WriteString(line)

		// Show cursor on the current line
		if i == im.cursorY {
			// Cursor position indicator (Phase 1: no cursor, just highlight line)
		}

		if i < len(im.lines)-1 {
			buf.WriteString("\n")
		}
	}

	// Cursor blink indicator at the end
	if im.cursorY == len(im.lines)-1 {
		buf.WriteString("▏")
	}

	return buf.String()
}

// ── Cursor movement ───────────────────────────────────────────────────────

func (im *InputModel) cursorLeft() {
	if im.cursorX > 0 {
		im.cursorX--
	} else if im.cursorY > 0 {
		im.cursorY--
		im.cursorX = utf8.RuneCountInString(im.lines[im.cursorY])
	}
}

func (im *InputModel) cursorRight() {
	lineLen := utf8.RuneCountInString(im.lines[im.cursorY])
	if im.cursorX < lineLen {
		im.cursorX++
	} else if im.cursorY < len(im.lines)-1 {
		im.cursorY++
		im.cursorX = 0
	}
}

func (im *InputModel) cursorUp() {
	if im.cursorY > 0 {
		im.cursorY--
		lineLen := utf8.RuneCountInString(im.lines[im.cursorY])
		if im.cursorX > lineLen {
			im.cursorX = lineLen
		}
	}
}

func (im *InputModel) cursorDown() {
	if im.cursorY < len(im.lines)-1 {
		im.cursorY++
		lineLen := utf8.RuneCountInString(im.lines[im.cursorY])
		if im.cursorX > lineLen {
			im.cursorX = lineLen
		}
	}
}

// ── Text editing ──────────────────────────────────────────────────────────

func (im *InputModel) insertString(s string) {
	im.saveUndo()
	line := im.lines[im.cursorY]
	runes := []rune(line)
	insertAt := im.cursorX
	newRunes := append(runes[:insertAt], append([]rune(s), runes[insertAt:]...)...)
	im.lines[im.cursorY] = string(newRunes)
	im.cursorX += utf8.RuneCountInString(s)
}

func (im *InputModel) backspace() {
	if im.cursorX == 0 && im.cursorY == 0 {
		return
	}
	im.saveUndo()
	if im.cursorX == 0 {
		// Merge with previous line
		prevLine := im.lines[im.cursorY-1]
		im.cursorX = utf8.RuneCountInString(prevLine)
		im.lines[im.cursorY-1] = prevLine + im.lines[im.cursorY]
		im.lines = append(im.lines[:im.cursorY], im.lines[im.cursorY+1:]...)
		im.cursorY--
	} else {
		runes := []rune(im.lines[im.cursorY])
		im.lines[im.cursorY] = string(append(runes[:im.cursorX-1], runes[im.cursorX:]...))
		im.cursorX--
	}
}

func (im *InputModel) deleteForward() {
	lineLen := utf8.RuneCountInString(im.lines[im.cursorY])
	if im.cursorX < lineLen {
		im.saveUndo()
		runes := []rune(im.lines[im.cursorY])
		im.lines[im.cursorY] = string(append(runes[:im.cursorX], runes[im.cursorX+1:]...))
	} else if im.cursorY < len(im.lines)-1 {
		im.saveUndo()
		im.lines[im.cursorY] = im.lines[im.cursorY] + im.lines[im.cursorY+1]
		im.lines = append(im.lines[:im.cursorY+1], im.lines[im.cursorY+2:]...)
	}
}

func (im *InputModel) killToEnd() {
	if im.cursorX < utf8.RuneCountInString(im.lines[im.cursorY]) {
		im.saveUndo()
		runes := []rune(im.lines[im.cursorY])
		im.lines[im.cursorY] = string(runes[:im.cursorX])
	}
}

func (im *InputModel) killToStart() {
	if im.cursorX > 0 {
		im.saveUndo()
		runes := []rune(im.lines[im.cursorY])
		im.lines[im.cursorY] = string(runes[im.cursorX:])
		im.cursorX = 0
	}
}

func (im *InputModel) deleteWordBackward() {
	if im.cursorX == 0 {
		im.backspace()
		return
	}
	im.saveUndo()
	runes := []rune(im.lines[im.cursorY])
	pos := im.cursorX - 1
	// Skip trailing spaces
	for pos > 0 && runes[pos] == ' ' {
		pos--
	}
	// Skip word
	for pos > 0 && runes[pos-1] != ' ' {
		pos--
	}
	im.lines[im.cursorY] = string(append(runes[:pos], runes[im.cursorX:]...))
	im.cursorX = pos
}

func (im *InputModel) newLine() {
	im.saveUndo()
	line := im.lines[im.cursorY]
	runes := []rune(line)
	beforeCursor := string(runes[:im.cursorX])
	afterCursor := string(runes[im.cursorX:])
	im.lines[im.cursorY] = beforeCursor
	// Insert new line
	newLines := make([]string, 0, len(im.lines)+1)
	newLines = append(newLines, im.lines[:im.cursorY+1]...)
	newLines = append(newLines, afterCursor)
	newLines = append(newLines, im.lines[im.cursorY+1:]...)
	im.lines = newLines
	im.cursorY++
	im.cursorX = 0
}

// ── History ───────────────────────────────────────────────────────────────

func (im *InputModel) navigateHistory(dir int) {
	if len(im.history) == 0 {
		return
	}
	if im.histIdx < 0 {
		// First time navigating — save current input
		im.histIdx = len(im.history)
	}
	im.histIdx += dir
	if im.histIdx < 0 {
		im.histIdx = 0
	}
	if im.histIdx >= len(im.history) {
		im.histIdx = len(im.history) - 1
	}
	im.lines = []string{im.history[im.histIdx]}
	im.cursorX = utf8.RuneCountInString(im.lines[0])
	im.cursorY = 0
}

// AddHistory adds a submitted input to history.
func (im *InputModel) AddHistory(text string) {
	if text == "" {
		return
	}
	// Don't add consecutive duplicates
	if len(im.history) > 0 && im.history[len(im.history)-1] == text {
		return
	}
	im.history = append(im.history, text)
	im.histIdx = -1
}

// ── Undo ──────────────────────────────────────────────────────────────────

func (im *InputModel) saveUndo() {
	snap := InputSnapshot{
		lines:   make([]string, len(im.lines)),
		cursorX: im.cursorX,
		cursorY: im.cursorY,
	}
	copy(snap.lines, im.lines)
	im.undoStack = append(im.undoStack, snap)
	// Limit undo stack to 50 entries
	if len(im.undoStack) > 50 {
		im.undoStack = im.undoStack[1:]
	}
}

func (im *InputModel) undo() {
	if len(im.undoStack) == 0 {
		return
	}
	snap := im.undoStack[len(im.undoStack)-1]
	im.undoStack = im.undoStack[:len(im.undoStack)-1]
	im.lines = snap.lines
	im.cursorX = snap.cursorX
	im.cursorY = snap.cursorY
}
