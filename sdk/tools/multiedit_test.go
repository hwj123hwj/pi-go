package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiEditTool_Basic(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\n"), 0o644))

	tool := NewMultiEditTool(WithMultiEditWorkspace(dir))
	assert.Equal(t, "multiedit", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"file_path":"` + testFile + `","edits":[{"old_string":"line2","new_string":"REPLACED2"},{"old_string":"line4","new_string":"REPLACED4"}]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "2 edits")

	data, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "line1\nREPLACED2\nline3\nREPLACED4\n", string(data))
}

func TestMultiEditTool_TransactionalRollback(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	original := "line1\nline2\nline3\n"
	require.NoError(t, os.WriteFile(testFile, []byte(original), 0o644))

	tool := NewMultiEditTool(WithMultiEditWorkspace(dir))
	ctx := context.Background()

	// First edit succeeds, second fails (old_string not found)
	validated, err := tool.Validate([]byte(`{"file_path":"` + testFile + `","edits":[{"old_string":"line1","new_string":"REPLACED"},{"old_string":"NOT_EXIST","new_string":"x"}]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
	assert.True(t, result.IsError)

	// Verify file was rolled back to original
	data, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, original, string(data))
}

func TestMultiEditTool_MultiFile(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "a.txt")
	file2 := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(file1, []byte("hello world"), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte("foo bar"), 0o644))

	tool := NewMultiEditTool(WithMultiEditWorkspace(dir))
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"file_path":"` + file1 + `","edits":[{"old_string":"hello","new_string":"hi"},{"file_path":"` + file2 + `","old_string":"bar","new_string":"baz"}]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data1, _ := os.ReadFile(file1)
	data2, _ := os.ReadFile(file2)
	assert.Equal(t, "hi world", string(data1))
	assert.Equal(t, "foo baz", string(data2))
}

func TestMultiEditTool_EmptyEdits(t *testing.T) {
	tool := NewMultiEditTool()
	_, err := tool.Validate([]byte(`{"file_path":"/tmp/test","edits":[]}`))
	assert.Error(t, err)
}

func TestMultiEditTool_MissingFilePath(t *testing.T) {
	tool := NewMultiEditTool()
	_, err := tool.Validate([]byte(`{"edits":[{"old_string":"a","new_string":"b"}]}`))
	assert.Error(t, err)
}
