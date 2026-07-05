package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTodoTool_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &TodoTool{filePath: filepath.Join(tmpDir, "todo.json")}

	assert.Equal(t, "todo_write", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"todos":[{"id":"task_1","content":"Fix bug","status":"pending","priority":"high"}]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "1 total")

	// Verify file was created
	todos, err := tool.GetTodos()
	require.NoError(t, err)
	require.Len(t, todos, 1)
	assert.Equal(t, "task_1", todos[0].ID)
	assert.Equal(t, "Fix bug", todos[0].Content)
	assert.Equal(t, "pending", todos[0].Status)
	assert.Equal(t, "high", todos[0].Priority)
}

func TestTodoTool_ReplaceAll(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &TodoTool{filePath: filepath.Join(tmpDir, "todo.json")}
	ctx := context.Background()

	// Set initial todos
	validated, err := tool.Validate([]byte(`{"todos":[{"id":"1","content":"A","status":"pending","priority":"low"}]}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	require.NoError(t, err)

	// Replace with new todos
	validated, err = tool.Validate([]byte(`{"todos":[{"id":"2","content":"B","status":"completed","priority":"high"},{"id":"3","content":"C","status":"in_progress","priority":"medium"}]}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	require.NoError(t, err)

	todos, err := tool.GetTodos()
	require.NoError(t, err)
	assert.Len(t, todos, 2)
	assert.Equal(t, "2", todos[0].ID)
	assert.Equal(t, "3", todos[1].ID)
}

func TestTodoTool_InvalidStatus(t *testing.T) {
	tool := NewTodoTool()
	_, err := tool.Validate([]byte(`{"todos":[{"id":"1","content":"A","status":"bad","priority":"low"}]}`))
	assert.Error(t, err)
}

func TestTodoTool_InvalidPriority(t *testing.T) {
	tool := NewTodoTool()
	_, err := tool.Validate([]byte(`{"todos":[{"id":"1","content":"A","status":"pending","priority":"urgent"}]}`))
	assert.Error(t, err)
}

func TestTodoTool_EmptyID(t *testing.T) {
	tool := NewTodoTool()
	_, err := tool.Validate([]byte(`{"todos":[{"id":"","content":"A","status":"pending","priority":"low"}]}`))
	assert.Error(t, err)
}

func TestTodoTool_Display(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &TodoTool{filePath: filepath.Join(tmpDir, "todo.json")}
	ctx := context.Background()

	validated, err := tool.Validate([]byte(`{"todos":[` +
		`{"id":"1","content":"Done task","status":"completed","priority":"high"},` +
		`{"id":"2","content":"Active task","status":"in_progress","priority":"medium"},` +
		`{"id":"3","content":"Pending task","status":"pending","priority":"low"}` +
		`]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)

	// Check output has all items
	assert.Contains(t, result.Content, "Done task")
	assert.Contains(t, result.Content, "Active task")
	assert.Contains(t, result.Content, "Pending task")
}

func TestTodoTool_JSONPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "todo.json")

	// Create and save
	tool := &TodoTool{filePath: filePath}
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"todos":[{"id":"persist_1","content":"Persist me","status":"pending","priority":"high"}]}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	require.NoError(t, err)

	// Verify JSON file content directly
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var todos []TodoItem
	err = json.Unmarshal(data, &todos)
	require.NoError(t, err)
	require.Len(t, todos, 1)
	assert.Equal(t, "persist_1", todos[0].ID)
}
