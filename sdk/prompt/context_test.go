package prompt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadContextFiles_Found(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Instructions\nUse Go."), 0o644))

	files := LoadContextFiles(dir)
	require.Len(t, files, 1)
	assert.Equal(t, "# Instructions\nUse Go.", files[0].Content)
	assert.Contains(t, files[0].Path, "CLAUDE.md")
}

func TestLoadContextFiles_NotFound(t *testing.T) {
	dir := t.TempDir()
	files := LoadContextFiles(dir)
	assert.Empty(t, files)
}

func TestLoadContextFiles_Empty(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(""), 0o644))

	files := LoadContextFiles(dir)
	assert.Empty(t, files)
}

func TestLoadContextFiles_Priority(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("Claude"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("Agents"), 0o644))

	files := LoadContextFiles(dir)
	require.Len(t, files, 1)
	assert.Equal(t, "Claude", files[0].Content)
}

func TestLoadProjectContextFiles_AncestorTraversal(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(child, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("Root instructions"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "a", "AGENTS.md"), []byte("Mid instructions"), 0o644))

	files := LoadProjectContextFiles(child, "")
	require.Len(t, files, 2)
	assert.Contains(t, files[0].Content, "Root")
	assert.Contains(t, files[1].Content, "Mid")
}

func TestLoadProjectContextFiles_WithAgentDir(t *testing.T) {
	agentDir := t.TempDir()
	cwd := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("Global instructions"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "CLAUDE.md"), []byte("Local instructions"), 0o644))

	files := LoadProjectContextFiles(cwd, agentDir)
	require.Len(t, files, 2)
	assert.Contains(t, files[0].Content, "Global")
	assert.Contains(t, files[1].Content, "Local")
}

func TestLoadProjectContextFiles_Dedup(t *testing.T) {
	cwd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "CLAUDE.md"), []byte("Same dir"), 0o644))

	files := LoadProjectContextFiles(cwd, cwd)
	assert.Len(t, files, 1)
}
