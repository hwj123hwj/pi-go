package tui

import (
	"testing"

	"github.com/hwj123hwj/easyagent/sdk/slashcmd"
)

func subTestRegistry() *slashcmd.Registry {
	reg := slashcmd.NewRegistry()
	reg.Register(slashcmd.Command{
		Name:        "feishu",
		Description: "feishu",
		Subcommands: []slashcmd.Subcommand{
			{Name: "setup", Description: "setup"},
			{Name: "start", Description: "start"},
			{Name: "stop", Description: "stop"},
			{Name: "status", Description: "status"},
			{Name: "logout", Description: "logout"},
		},
	})
	reg.Register(slashcmd.Command{Name: "help", Description: "help"})
	return reg
}

func TestTriggerSub(t *testing.T) {
	reg := subTestRegistry()
	cm := NewCompletionState()

	// Bare "/feishu" belongs to TriggerSlash.
	if cm.TriggerSub("/feishu", 7, reg) {
		t.Error("bare command should not trigger sub completion")
	}

	// Right after the space: all subcommands offered.
	if !cm.TriggerSub("/feishu ", 8, reg) {
		t.Fatal("/feishu<space> should trigger sub completion")
	}
	if len(cm.Items()) != 5 {
		t.Errorf("items = %d, want 5", len(cm.Items()))
	}

	// Prefix filter "st" → start/stop/status.
	if !cm.TriggerSub("/feishu st", 10, reg) {
		t.Fatal("/feishu st should trigger sub completion")
	}
	if len(cm.Items()) != 3 {
		t.Errorf("items for \"st\" = %d, want 3", len(cm.Items()))
	}
	if cm.queryStart != 8 {
		t.Errorf("queryStart = %d, want 8", cm.queryStart)
	}

	// No match → no popup.
	if cm.TriggerSub("/feishu zz", 10, reg) {
		t.Error("/feishu zz should not trigger sub completion")
	}

	// Past the first argument slot (flags/args) → quiet.
	if cm.TriggerSub("/feishu setup ", 14, reg) {
		t.Error("second argument slot should not trigger sub completion")
	}
	if cm.TriggerSub("/feishu setup --manual a b", 25, reg) {
		t.Error("flag args should not trigger sub completion")
	}

	// Commands without subcommands → quiet.
	if cm.TriggerSub("/help ", 6, reg) {
		t.Error("command without subcommands should not trigger")
	}
}

func TestAcceptSubCompletion(t *testing.T) {
	reg := subTestRegistry()
	m := TuiModel{input: NewInputModel(), completion: NewCompletionState()}

	m.input.insertString("/feishu st")
	if !m.completion.TriggerSub(m.input.Text(), m.input.cursorX, reg) {
		t.Fatal("expected sub completion popup")
	}

	m.acceptCompletion("start")

	want := "/feishu start "
	if got := m.input.Text(); got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
	if m.input.cursorX != len([]rune(want)) {
		t.Errorf("cursorX = %d, want %d", m.input.cursorX, len([]rune(want)))
	}
}
