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

func TestPatchTool_Basic(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world\n"), 0o644))

	tool := NewPatchTool(WithPatchWorkspace(dir))
	assert.Equal(t, "patch", tool.Name())

	patchText := `--- a/hello.txt
+++ b/hello.txt
@@ -1 +1 @@
-hello world
+hello universe
`

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"patchText":"` + escapeJSON(patchText) + `"}`))
	require.NoError(t, err)

	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "Patch applied")
}

func TestPatchTool_EmptyPatch(t *testing.T) {
	tool := NewPatchTool()
	_, err := tool.Validate([]byte(`{"patchText":""}`))
	assert.Error(t, err)
}

func TestParseUnifiedDiff_Basic(t *testing.T) {
	patch := `--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,3 @@
 line1
-old
+new
 line3
`
	hunks := parseUnifiedDiff(patch)
	require.Len(t, hunks, 1)
	assert.Equal(t, "file.txt", hunks[0].Path)
	assert.Equal(t, "update", hunks[0].Type)
}

func TestParseUnifiedDiff_NewFile(t *testing.T) {
	patch := `--- /dev/null
+++ b/newfile.txt
@@ -0,0 +1 @@
+new content
`
	hunks := parseUnifiedDiff(patch)
	require.Len(t, hunks, 1)
	assert.Equal(t, "newfile.txt", hunks[0].Path)
	assert.Equal(t, "add", hunks[0].Type)
	assert.Contains(t, hunks[0].Content, "new content")
}

func TestPatchTool_WithBackup(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "backup_test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("original\n"), 0o644))

	bm := NewBackupManager()
	tool := NewPatchTool(WithPatchWorkspace(dir), WithPatchBackupManager(bm))

	patchText := `--- a/backup_test.txt
+++ b/backup_test.txt
@@ -1 +1 @@
-original
+modified
`

	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"patchText":"` + escapeJSON(patchText) + `"}`))
	require.NoError(t, err)

	_, err = tool.Execute(ctx, validated, nil)
	require.NoError(t, err)

	// Verify backup exists
	assert.True(t, bm.HasBackup(testFile))
}

// escapeJSON escapes a string for use in a JSON string value.
func escapeJSON(s string) string {
	b, _ := (&jsonValue{s}).MarshalJSON()
	// Strip the surrounding quotes
	return string(b[1 : len(b)-1])
}

type jsonValue struct{ s string }

func (v *jsonValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.s)
}
