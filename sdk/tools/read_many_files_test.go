package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadManyFilesTool_Basic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("content A"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("content B"), 0o644))

	tool := NewReadManyFilesTool(WithReadManyFilesWorkspace(dir))
	assert.Equal(t, "read_many_files", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"paths":["a.txt","b.txt"]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "content A")
	assert.Contains(t, result.Content, "content B")
	assert.Contains(t, result.Content, "--- ")
}

func TestReadManyFilesTool_Glob(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "util.go"), []byte("package util"), 0o644))

	tool := NewReadManyFilesTool(WithReadManyFilesWorkspace(dir))
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"paths":["src/*.go"]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "package main")
	assert.Contains(t, result.Content, "package util")
}

func TestReadManyFilesTool_NoPaths(t *testing.T) {
	tool := NewReadManyFilesTool()
	_, err := tool.Validate([]byte(`{"paths":[]}`))
	assert.Error(t, err)
}

func TestReadManyFilesTool_NonExistent(t *testing.T) {
	tool := NewReadManyFilesTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"paths":["/tmp/nonexistent_dir_xyz/file.txt"]}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "No matching files")
}

func TestReadManyFilesTool_IsConcurrencySafe(t *testing.T) {
	tool := NewReadManyFilesTool()
	assert.True(t, tool.IsConcurrencySafe(nil))
}
