package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryTool_Basic(t *testing.T) {
	// Use a temp directory for the memory file
	tmpDir := t.TempDir()
	tool := &MemoryTool{filePath: filepath.Join(tmpDir, "AGENTS.md")}

	assert.Equal(t, "save_memory", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"fact":"My favorite color is blue"}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "My favorite color is blue")

	// Verify file was created with the memory
	data, err := os.ReadFile(filepath.Join(tmpDir, "AGENTS.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "## Easy Code Added Memories")
	assert.Contains(t, string(data), "- My favorite color is blue")
}

func TestMemoryTool_AppendMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &MemoryTool{filePath: filepath.Join(tmpDir, "AGENTS.md")}
	ctx := context.Background()

	// Save first fact
	validated, err := tool.Validate([]byte(`{"fact":"First fact"}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	require.NoError(t, err)

	// Save second fact
	validated, err = tool.Validate([]byte(`{"fact":"Second fact"}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	require.NoError(t, err)

	// Verify both facts are in the file
	data, err := os.ReadFile(filepath.Join(tmpDir, "AGENTS.md"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "- First fact")
	assert.Contains(t, content, "- Second fact")
}

func TestMemoryTool_EmptyFact(t *testing.T) {
	tool := NewMemoryTool()
	_, err := tool.Validate([]byte(`{"fact":""}`))
	assert.Error(t, err)
}

func TestMemoryTool_MissingFact(t *testing.T) {
	tool := NewMemoryTool()
	_, err := tool.Validate([]byte(`{}`))
	assert.Error(t, err)
}
