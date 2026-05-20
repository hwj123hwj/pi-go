package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrepTool_SearchFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc hello() {\n\treturn\n}\n"
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))

	tool := NewGrepTool()
	assert.Equal(t, "grep", tool.Name())

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"pattern":"func","path":"` + testFile + `"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "func hello()")
	assert.Contains(t, result.Content, "1 match")
}

func TestGrepTool_SearchDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\nfunc a() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\nfunc b() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("no match here\n"), 0o644))

	tool := NewGrepTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"pattern":"func","path":"` + dir + `","include":"*.go"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "a.go")
	assert.Contains(t, result.Content, "b.go")
	assert.NotContains(t, result.Content, "c.txt")
}

func TestGrepTool_NoMatch(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0o644))

	tool := NewGrepTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"pattern":"notfound","path":"` + testFile + `"}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "no matches")
}

func TestGrepTool_IgnoreCase(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("Hello World"), 0o644))

	tool := NewGrepTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"pattern":"hello","path":"` + testFile + `","ignore_case":true}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "Hello World")
}

func TestGrepTool_InvalidRegex(t *testing.T) {
	tool := NewGrepTool()
	_, err := tool.Validate([]byte(`{"pattern":"[invalid","path":"."}`))
	assert.Error(t, err)
}

func TestGrepTool_Validate(t *testing.T) {
	tool := NewGrepTool()

	// 缺少 pattern
	_, err := tool.Validate([]byte(`{"path":"."}`))
	assert.Error(t, err)

	// 缺少 path
	_, err = tool.Validate([]byte(`{"pattern":"test"}`))
	assert.Error(t, err)
}
