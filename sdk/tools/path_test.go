package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolvePath_RelativeWithWorkspace(t *testing.T) {
	result := ResolvePath("/workspace", "src/main.go")
	expected := filepath.Clean("/workspace/src/main.go")
	assert.Equal(t, expected, result)
}

func TestResolvePath_AbsolutePath(t *testing.T) {
	result := ResolvePath("/workspace", "/tmp/test.go")
	assert.Equal(t, "/tmp/test.go", result)
}

func TestResolvePath_EmptyWorkspace(t *testing.T) {
	result := ResolvePath("", "test.go")
	assert.True(t, filepath.IsAbs(result))
}

func TestResolvePath_DotPath(t *testing.T) {
	result := ResolvePath("/workspace", ".")
	expected := filepath.Clean("/workspace")
	assert.Equal(t, expected, result)
}

func TestResolvePath_ParentTraversal(t *testing.T) {
	result := ResolvePath("/workspace", "../../etc/passwd")
	// Should resolve but not necessarily be safe
	assert.True(t, filepath.IsAbs(result) || result != "")
}

func TestIsPathSafe_WithinWorkspace(t *testing.T) {
	assert.True(t, IsPathSafe("/workspace", "src/main.go"))
	assert.True(t, IsPathSafe("/workspace", "src/../test.go"))
}

func TestIsPathSafe_EscapeAttempt(t *testing.T) {
	assert.False(t, IsPathSafe("/workspace", "../../etc/passwd"))
	assert.False(t, IsPathSafe("/workspace", "/etc/passwd"))
}

func TestIsPathSafe_EmptyWorkspace(t *testing.T) {
	// Empty workspace means no restrictions
	assert.True(t, IsPathSafe("", "/etc/passwd"))
	assert.True(t, IsPathSafe("", "../../anything"))
}

func TestIsPathSafe_ExactWorkspace(t *testing.T) {
	assert.True(t, IsPathSafe("/workspace", "."))
}

func TestIsPathSafe_SymlinkEscape(t *testing.T) {
	tmp := t.TempDir()

	// Create workspace structure
	workspace := filepath.Join(tmp, "workspace")
	assert.NoError(t, os.Mkdir(workspace, 0o755))
	assert.NoError(t, os.MkdirAll(filepath.Join(workspace, "safe"), 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(workspace, "safe", "file.txt"), []byte("hi"), 0o644))

	// Create an outside file
	outside := filepath.Join(tmp, "outside.txt")
	assert.NoError(t, os.WriteFile(outside, []byte("secret"), 0o644))

	// Create a symlink inside workspace pointing outside
	symlinkPath := filepath.Join(workspace, "escape_link")
	assert.NoError(t, os.Symlink(outside, symlinkPath))

	// Path that exists and is a symlink -> should be blocked
	assert.False(t, IsPathSafe(workspace, "escape_link"))

	// Normal safe path -> should pass
	assert.True(t, IsPathSafe(workspace, "safe/file.txt"))

	// Path that doesn't exist yet but is within workspace -> should pass
	assert.True(t, IsPathSafe(workspace, "safe/new_file.txt"))

	// Symlink to a directory that escapes
	escapedDir := filepath.Join(tmp, "escaped_dir")
	assert.NoError(t, os.Mkdir(escapedDir, 0o755))
	dirSymlink := filepath.Join(workspace, "escape_dir")
	assert.NoError(t, os.Symlink(escapedDir, dirSymlink))

	assert.False(t, IsPathSafe(workspace, "escape_dir/file.txt"))
}
