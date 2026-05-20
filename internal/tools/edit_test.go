package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditTool_Replace(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0o644))

	tool := NewEditTool()
	assert.Equal(t, "edit", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","old_string":"line2","new_string":"replaced"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "edited")
	assert.False(t, result.IsError)

	// 验证文件内容
	data, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "line1\nreplaced\nline3\n", string(data))
}

func TestEditTool_NotFound(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0o644))

	tool := NewEditTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","old_string":"not exist","new_string":"x"}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
}

func TestEditTool_NotUnique(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("aaa bbb aaa"), 0o644))

	tool := NewEditTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","old_string":"aaa","new_string":"x"}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not unique")
}

func TestEditTool_CreateNewFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "sub", "new.txt")

	tool := NewEditTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","old_string":"","new_string":"new content"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "created")

	data, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(data))
}

func TestEditTool_Validate(t *testing.T) {
	tool := NewEditTool()

	// 缺少 path
	_, err := tool.Validate([]byte(`{"old_string":"a","new_string":"b"}`))
	assert.Error(t, err)

	// 无效 JSON
	_, err = tool.Validate([]byte(`invalid`))
	assert.Error(t, err)
}
