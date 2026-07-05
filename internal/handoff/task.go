// Package handoff implements TASK.md-based task context persistence.
//
// Design philosophy (inspired by hwjcode):
// "Documents are the most reliable context transfer mechanism."
// When a long-running agent task is interrupted (timeout, crash, user switch),
// the agent can write its progress to TASK.md. On the next session resume,
// TASK.md is automatically loaded into the system prompt so the agent picks up
// exactly where it left off — no lost context, no repeated work.
package handoff

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// TaskFileName is the canonical task handoff file name.
	TaskFileName = "TASK.md"
	// TaskDir is the subdirectory inside the workspace for task files.
	TaskDir = ".easycode"
)

// TaskState represents the current state of a task.
type TaskState string

const (
	StateInProgress TaskState = "in-progress"
	StateBlocked    TaskState = "blocked"
	StateCompleted  TaskState = "completed"
	StatePaused     TaskState = "paused"
)

// TaskDocument represents the structured content of a TASK.md file.
type TaskDocument struct {
	Title       string     `json:"title"`
	Goal        string     `json:"goal"`
	State       TaskState  `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Progress    []string   `json:"progress"`
	NextSteps   []string   `json:"next_steps"`
	Blockers    []string   `json:"blockers"`
	FilesChanged []string  `json:"files_changed"`
	Notes       string     `json:"notes"`
}

// NewTaskDocument creates a new TaskDocument for the given goal.
func NewTaskDocument(goal string) *TaskDocument {
	now := time.Now()
	return &TaskDocument{
		Title:     "Task Handoff",
		Goal:      goal,
		State:     StateInProgress,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Render serializes the TaskDocument to Markdown format.
func (td *TaskDocument) Render() string {
	var b strings.Builder
	b.WriteString("# 📋 Task Handoff\n\n")
	b.WriteString(fmt.Sprintf("**Goal:** %s\n\n", td.Goal))
	b.WriteString(fmt.Sprintf("**State:** %s\n\n", td.State))
	b.WriteString(fmt.Sprintf("**Created:** %s\n", td.CreatedAt.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("**Updated:** %s\n\n", td.UpdatedAt.Format("2006-01-02 15:04:05")))

	if len(td.Progress) > 0 {
		b.WriteString("## ✅ Progress\n\n")
		for _, item := range td.Progress {
			b.WriteString(fmt.Sprintf("- %s\n", item))
		}
		b.WriteString("\n")
	}

	if len(td.NextSteps) > 0 {
		b.WriteString("## ➡️ Next Steps\n\n")
		for _, item := range td.NextSteps {
			b.WriteString(fmt.Sprintf("- [ ] %s\n", item))
		}
		b.WriteString("\n")
	}

	if len(td.Blockers) > 0 {
		b.WriteString("## 🚫 Blockers\n\n")
		for _, item := range td.Blockers {
			b.WriteString(fmt.Sprintf("- %s\n", item))
		}
		b.WriteString("\n")
	}

	if len(td.FilesChanged) > 0 {
		b.WriteString("## 📁 Files Changed\n\n")
		for _, f := range td.FilesChanged {
			b.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		b.WriteString("\n")
	}

	if td.Notes != "" {
		b.WriteString("## 📝 Notes\n\n")
		b.WriteString(td.Notes)
		b.WriteString("\n")
	}

	return b.String()
}

// taskFilePath returns the full path to the TASK.md file in the workspace.
func taskFilePath(workspace string) string {
	return filepath.Join(workspace, TaskDir, TaskFileName)
}

// Save writes the TaskDocument to TASK.md in the workspace.
func Save(workspace string, doc *TaskDocument) error {
	dir := filepath.Join(workspace, TaskDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create task dir: %w", err)
	}

	doc.UpdatedAt = time.Now()
	content := doc.Render()
	path := taskFilePath(workspace)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write task file: %w", err)
	}

	return nil
}

// Load reads the TASK.md from the workspace.
// Returns nil, nil if the file does not exist (no active task).
func Load(workspace string) (*TaskDocument, error) {
	path := taskFilePath(workspace)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No task file — not an error
		}
		return nil, fmt.Errorf("read task file: %w", err)
	}

	return parseTaskDocument(string(data)), nil
}

// LoadAsPrompt loads the TASK.md content as a system prompt injection.
// Returns empty string if no task file exists.
func LoadAsPrompt(workspace string) string {
	doc, err := Load(workspace)
	if err != nil || doc == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## 📋 Task Handoff (Resumed)\n\n")
	b.WriteString("You are resuming a task from a previous session. Here is the context:\n\n")
	b.WriteString(doc.Render())
	b.WriteString("\n**Continue working on this task from where you left off.**\n")
	return b.String()
}

// Clear removes the TASK.md file from the workspace.
// No-op if the file doesn't exist.
func Clear(workspace string) error {
	path := taskFilePath(workspace)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove task file: %w", err)
	}
	return nil
}

// parseTaskDocument parses a rendered TASK.md back into a TaskDocument.
// This is a best-effort parser — it extracts the key sections.
func parseTaskDocument(content string) *TaskDocument {
	doc := &TaskDocument{}

	lines := strings.Split(content, "\n")
	currentSection := ""
	var progress, nextSteps, blockers, filesChanged []string
	var notesLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Parse sections
		switch {
		case strings.HasPrefix(trimmed, "**Goal:**"):
			doc.Goal = strings.TrimPrefix(trimmed, "**Goal:**")
			doc.Goal = strings.TrimSpace(doc.Goal)
		case strings.HasPrefix(trimmed, "**State:**"):
			stateStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "**State:**"))
			doc.State = TaskState(stateStr)
		case strings.Contains(line, "## ✅ Progress"):
			currentSection = "progress"
		case strings.Contains(line, "## ➡️ Next Steps"):
			currentSection = "next"
		case strings.Contains(line, "## 🚫 Blockers"):
			currentSection = "blockers"
		case strings.Contains(line, "## 📁 Files Changed"):
			currentSection = "files"
		case strings.Contains(line, "## 📝 Notes"):
			currentSection = "notes"
		case strings.HasPrefix(trimmed, "- "):
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			item = strings.TrimPrefix(item, "[ ] ") // remove checkbox prefix
			switch currentSection {
			case "progress":
				progress = append(progress, item)
			case "next":
				nextSteps = append(nextSteps, item)
			case "blockers":
				blockers = append(blockers, item)
			case "files":
				item = strings.Trim(item, "`")
				filesChanged = append(filesChanged, item)
			}
		case currentSection == "notes" && trimmed != "":
			notesLines = append(notesLines, line)
		}
	}

	doc.Progress = progress
	doc.NextSteps = nextSteps
	doc.Blockers = blockers
	doc.FilesChanged = filesChanged
	doc.Notes = strings.Join(notesLines, "\n")

	return doc
}

// AddProgress adds a progress item and saves.
func AddProgress(workspace string, item string) error {
	doc, err := Load(workspace)
	if err != nil {
		return err
	}
	if doc == nil {
		doc = NewTaskDocument("Unknown task")
	}
	doc.Progress = append(doc.Progress, item)
	return Save(workspace, doc)
}

// AddNextStep adds a next step and saves.
func AddNextStep(workspace string, item string) error {
	doc, err := Load(workspace)
	if err != nil {
		return err
	}
	if doc == nil {
		doc = NewTaskDocument("Unknown task")
	}
	doc.NextSteps = append(doc.NextSteps, item)
	return Save(workspace, doc)
}

// MarkComplete marks the task as completed and saves.
func MarkComplete(workspace string) error {
	doc, err := Load(workspace)
	if err != nil {
		return err
	}
	if doc == nil {
		return nil // Nothing to mark
	}
	doc.State = StateCompleted
	return Save(workspace, doc)
}
