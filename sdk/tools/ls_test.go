package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLsTool_Directory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))

	tool := NewLsTool()
	assert.Equal(t, "ls", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + dir + `"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "a.txt")
	assert.Contains(t, result.Content, "subdir")
}

func TestLsTool_SingleFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello"), 0o644))

	tool := NewLsTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "test.txt")
}

func TestLsTool_ShowHidden(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("x"), 0o644))

	tool := NewLsTool()
	ctx := context.Background()

	// 不显示隐藏
	validated, err := tool.Validate([]byte(`{"path":"` + dir + `"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.NotContains(t, result.Content, ".hidden")
	assert.Contains(t, result.Content, "visible.txt")

	// 显示隐藏
	validated2, err := tool.Validate([]byte(`{"path":"` + dir + `","all":true}`))
	require.NoError(t, err)
	result2, err := tool.Execute(ctx, validated2, nil)
	require.NoError(t, err)
	assert.Contains(t, result2.Content, ".hidden")
	assert.Contains(t, result2.Content, "visible.txt")
}

func TestLsTool_NotExist(t *testing.T) {
	tool := NewLsTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"/nonexistent/path"}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
}

func TestLsTool_Validate(t *testing.T) {
	tool := NewLsTool()
	_, err := tool.Validate([]byte(`{}`))
	assert.Error(t, err)
}
