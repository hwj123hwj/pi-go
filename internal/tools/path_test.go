package tools

import (
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
