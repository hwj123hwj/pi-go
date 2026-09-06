package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/hwj123hwj/pi-go/sdk/agent"
)

const (
	todoDirName  = ".pi-go"
	todoFileName = "todo.json"
)

// TodoItem represents a single todo entry.
type TodoItem struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Status   string `json:"status"`   // "pending", "in_progress", "completed"
	Priority string `json:"priority"` // "high", "medium", "low"
}

// TodoTool manages a todo list with JSON persistence to ~/.pi-go/todo.json.
// Each call replaces the entire list (wholesale update, matching the TS pattern).
type TodoTool struct {
	mu       sync.Mutex
	filePath string // resolved lazily
}

type TodoParams struct {
	Todos []TodoItem `json:"todos"`
}

func NewTodoTool() *TodoTool {
	return &TodoTool{}
}

func (t *TodoTool) Name() string { return "todo_write" }

func (t *TodoTool) Description() string {
	return `Manage todo items by providing a complete list of todos. The tool will update the entire todo list with the provided items. Use this for creating, updating, or managing all your todo items.

CRITICAL: When you want to use this tool, DO NOT write text descriptions or explanations. Directly call the function with proper JSON parameters.

The "todos" parameter must be a JSON array of objects (NOT an array of strings). Each object must have: "id" (string), "content" (string), "status" ("pending"|"in_progress"|"completed"), "priority" ("high"|"medium"|"low").

Example: [{"id": "task_1", "content": "My task", "status": "pending", "priority": "high"}]`
}

func (t *TodoTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"todos": map[string]any{
				"description": "Complete list of todo items. Provide the full todo list that will replace the existing one.",
				"type":        "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":       map[string]any{"type": "string", "description": "Todo item unique identifier"},
						"content":  map[string]any{"type": "string", "description": "Todo item content"},
						"status":   map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}, "description": "Todo status"},
						"priority": map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}, "description": "Todo priority"},
					},
					"required": []string{"id", "content", "status", "priority"},
				},
			},
		},
		"required": []string{"todos"},
	}
}

func (t *TodoTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params TodoParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	for i, item := range params.Todos {
		if strings.TrimSpace(item.ID) == "" {
			return nil, fmt.Errorf("todos[%d]: id is required", i)
		}
		if strings.TrimSpace(item.Content) == "" {
			return nil, fmt.Errorf("todos[%d]: content is required", i)
		}
		switch item.Status {
		case "pending", "in_progress", "completed":
		default:
			return nil, fmt.Errorf("todos[%d]: status must be one of pending, in_progress, completed (got %q)", i, item.Status)
		}
		switch item.Priority {
		case "high", "medium", "low":
		default:
			return nil, fmt.Errorf("todos[%d]: priority must be one of high, medium, low (got %q)", i, item.Priority)
		}
	}
	return json.Marshal(params)
}

func (t *TodoTool) Execute(_ context.Context, raw json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var params TodoParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	if err := t.saveTodos(params.Todos); err != nil {
		slog.Error("todo_write failed", "error", err)
		return agent.ToolResult{IsError: true, Content: fmt.Sprintf("Failed to save todos: %v", err)}, err
	}

	// Build stats
	pending, inProgress, completed := 0, 0, 0
	for _, item := range params.Todos {
		switch item.Status {
		case "pending":
			pending++
		case "in_progress":
			inProgress++
		case "completed":
			completed++
		}
	}

	// Build display
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Todo list updated: %d total (%d pending, %d in_progress, %d completed)\n\n",
		len(params.Todos), pending, inProgress, completed))

	// Sort: completed first, then in_progress, then pending
	sorted := make([]TodoItem, len(params.Todos))
	copy(sorted, params.Todos)
	sort.Slice(sorted, func(i, j int) bool {
		statusOrder := map[string]int{"completed": 0, "in_progress": 1, "pending": 2}
		return statusOrder[sorted[i].Status] < statusOrder[sorted[j].Status]
	})

	for _, item := range sorted {
		icon := "☐"
		if item.Status == "completed" {
			icon = "☑"
		} else if item.Status == "in_progress" {
			icon = "⊡"
		}
		b.WriteString(fmt.Sprintf("  %s [%s] %s (%s)\n", icon, item.Priority, item.Content, item.ID))
	}

	return agent.ToolResult{Content: b.String()}, nil
}

func (t *TodoTool) getFilePath() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.filePath != "" {
		return t.filePath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	t.filePath = filepath.Join(home, todoDirName, todoFileName)
	return t.filePath, nil
}

func (t *TodoTool) saveTodos(todos []TodoItem) error {
	filePath, err := t.getFilePath()
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("create todo dir: %w", err)
	}

	data, err := json.MarshalIndent(todos, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal todos: %w", err)
	}

	return os.WriteFile(filePath, data, 0o644)
}

// GetTodos returns the current in-memory todo list (for testing).
func (t *TodoTool) GetTodos() ([]TodoItem, error) {
	filePath, err := t.getFilePath()
	if err != nil {
		return nil, err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var todos []TodoItem
	if err := json.Unmarshal(data, &todos); err != nil {
		return nil, err
	}
	return todos, nil
}
