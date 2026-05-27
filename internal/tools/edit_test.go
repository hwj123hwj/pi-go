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

func TestEditTool_MultiEdit_Success(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\nline5\n"), 0o644))

	tool := NewEditTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","edits":[{"old_string":"line2","new_string":"REPLACED2"},{"old_string":"line4","new_string":"REPLACED4"}]}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "2 edits applied")

	data, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "line1\nREPLACED2\nline3\nREPLACED4\nline5\n", string(data))
}

func TestEditTool_MultiEdit_NotFound(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("line1\nline2\n"), 0o644))

	tool := NewEditTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","edits":[{"old_string":"line1","new_string":"x"},{"old_string":"NOT_EXIST","new_string":"y"}]}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "edits[1]: old_string not found")

	// 文件应未被修改（回滚）
	data, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "line1\nline2\n", string(data))
}

func TestEditTool_MultiEdit_Overlapping(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("abc\n"), 0o644))

	tool := NewEditTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","edits":[{"old_string":"abc","new_string":"x"},{"old_string":"bc","new_string":"y"}]}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "overlapping")
}

func TestEditTool_MultiEdit_NotUnique(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("aaa\nbbb\naaa\n"), 0o644))

	tool := NewEditTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","edits":[{"old_string":"aaa","new_string":"x"}]}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be unique")
}

func TestEditTool_MultiEdit_MutuallyExclusive(t *testing.T) {
	tool := NewEditTool()
	_, err := tool.Validate([]byte(`{"path":"/tmp/test","old_string":"a","new_string":"b","edits":[{"old_string":"c","new_string":"d"}]}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot provide both")
}

func TestApplyEdits(t *testing.T) {
	// 纯函数测试，无需文件系统
	content := "hello world\nfoo bar\nbaz qux\n"

	result, err := applyEdits(content, []EditEntry{
		{OldString: "hello", NewString: "hi"},
		{OldString: "baz qux", NewString: "BAZ QUX"},
	})
	require.NoError(t, err)
	assert.Equal(t, "hi world\nfoo bar\nBAZ QUX\n", result)
}
