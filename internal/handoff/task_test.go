package handoff

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskDocumentRender(t *testing.T) {
	doc := &TaskDocument{
		Title:    "Task Handoff",
		Goal:     "Implement /loop feature",
		State:    StateInProgress,
		Progress: []string{"Created scheduler package", "Wrote tests"},
		NextSteps: []string{"Wire into server", "Add slash command"},
		Blockers: []string{"Waiting on API review"},
		FilesChanged: []string{"internal/scheduler/loop.go"},
		Notes:    "Need to handle concurrent access carefully.",
	}

	md := doc.Render()

	// Verify key sections are present
	checks := []string{
		"# 📋 Task Handoff",
		"**Goal:** Implement /loop feature",
		"**State:** in-progress",
		"## ✅ Progress",
		"Created scheduler package",
		"## ➡️ Next Steps",
		"Wire into server",
		"## 🚫 Blockers",
		"Waiting on API review",
		"## 📁 Files Changed",
		"`internal/scheduler/loop.go`",
		"## 📝 Notes",
		"Need to handle concurrent access",
	}

	for _, check := range checks {
		if !contains(md, check) {
			t.Errorf("Render() output missing %q", check)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	doc := NewTaskDocument("Test task goal")
	doc.Progress = []string{"Step 1 done", "Step 2 done"}
	doc.NextSteps = []string{"Step 3", "Step 4"}
	doc.FilesChanged = []string{"file1.go", "file2.go"}

	// Save
	if err := Save(tmpDir, doc); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists in .pi-go directory
	path := filepath.Join(tmpDir, ".pi-go", TaskFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("TASK.md was not created in .pi-go directory")
	}

	// Load
	loaded, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}

	// Verify content
	if loaded.Goal != "Test task goal" {
		t.Errorf("Goal = %q, want %q", loaded.Goal, "Test task goal")
	}
	if loaded.State != StateInProgress {
		t.Errorf("State = %q, want %q", loaded.State, StateInProgress)
	}
	if len(loaded.Progress) != 2 {
		t.Errorf("Progress count = %d, want 2", len(loaded.Progress))
	}
	if len(loaded.NextSteps) != 2 {
		t.Errorf("NextSteps count = %d, want 2", len(loaded.NextSteps))
	}
	if len(loaded.FilesChanged) != 2 {
		t.Errorf("FilesChanged count = %d, want 2", len(loaded.FilesChanged))
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	doc, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load should not error on missing file: %v", err)
	}
	if doc != nil {
		t.Fatal("Load should return nil for non-existent file")
	}
}

func TestLoadAsPrompt(t *testing.T) {
	tmpDir := t.TempDir()

	// No task file: should return empty
	prompt := LoadAsPrompt(tmpDir)
	if prompt != "" {
		t.Error("LoadAsPrompt should return empty when no task file exists")
	}

	// Save a task and verify prompt content
	doc := NewTaskDocument("Test prompt injection")
	doc.Progress = []string{"Did something"}
	Save(tmpDir, doc)

	prompt = LoadAsPrompt(tmpDir)
	if prompt == "" {
		t.Fatal("LoadAsPrompt should return non-empty when task file exists")
	}
	if !contains(prompt, "Test prompt injection") {
		t.Error("LoadAsPrompt should contain the goal")
	}
	if !contains(prompt, "Task Handoff (Resumed)") {
		t.Error("LoadAsPrompt should contain handoff header")
	}
}

func TestClear(t *testing.T) {
	tmpDir := t.TempDir()

	// Save a task
	doc := NewTaskDocument("To be cleared")
	Save(tmpDir, doc)

	// Verify it exists
	path := filepath.Join(tmpDir, TaskDir, TaskFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("TASK.md should exist before Clear")
	}

	// Clear it
	if err := Clear(tmpDir); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("TASK.md should not exist after Clear")
	}

	// Clear again should not error
	if err := Clear(tmpDir); err != nil {
		t.Fatalf("Clear on non-existent file should not error: %v", err)
	}
}

func TestParseTaskDocument(t *testing.T) {
	content := `# 📋 Task Handoff

**Goal:** Implement feature X

**State:** in-progress

**Created:** 2026-07-05 08:00:00
**Updated:** 2026-07-05 09:00:00

## ✅ Progress

- Created files
- Wrote tests

## ➡️ Next Steps

- [ ] Deploy
- [ ] Write docs

## 📁 Files Changed

- ` + "`main.go`" + `
- ` + "`test.go`" + `

## 📝 Notes

Important context here.
`

	doc := parseTaskDocument(content)

	if doc.Goal != "Implement feature X" {
		t.Errorf("Goal = %q, want %q", doc.Goal, "Implement feature X")
	}
	if doc.State != StateInProgress {
		t.Errorf("State = %q, want %q", doc.State, StateInProgress)
	}
	if len(doc.Progress) != 2 {
		t.Errorf("Progress count = %d, want 2", len(doc.Progress))
	}
	if len(doc.NextSteps) != 2 {
		t.Errorf("NextSteps count = %d, want 2", len(doc.NextSteps))
	}
	if doc.NextSteps[0] != "Deploy" {
		t.Errorf("NextSteps[0] = %q, want %q", doc.NextSteps[0], "Deploy")
	}
	if len(doc.FilesChanged) != 2 {
		t.Errorf("FilesChanged count = %d, want 2", len(doc.FilesChanged))
	}
	if doc.FilesChanged[0] != "main.go" {
		t.Errorf("FilesChanged[0] = %q, want %q", doc.FilesChanged[0], "main.go")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
