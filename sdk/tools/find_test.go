package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindTool_ByPattern(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("x"), 0o644))

	tool := NewFindTool()
	assert.Equal(t, "find", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + dir + `","pattern":"*.go"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "a.go")
	assert.Contains(t, result.Content, "b.go")
	assert.NotContains(t, result.Content, "c.txt")
}

func TestFindTool_AllFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0o644))

	tool := NewFindTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + dir + `"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "a.txt")
	assert.Contains(t, result.Content, "b.txt")
}

func TestFindTool_TypeFilter(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644))

	tool := NewFindTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + dir + `","type":"dir"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "subdir")
	assert.NotContains(t, result.Content, "file.txt")
}

func TestFindTool_NotDir(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("x"), 0o644))

	tool := NewFindTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `"}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
}

func TestFindTool_Validate(t *testing.T) {
	tool := NewFindTool()

	// 缺少 path
	_, err := tool.Validate([]byte(`{"pattern":"*.go"}`))
	assert.Error(t, err)

	// 无效 type
	_, err = tool.Validate([]byte(`{"path":"/tmp","type":"invalid"}`))
	assert.Error(t, err)
}
