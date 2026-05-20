package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/skill"
	"github.com/earendil-works/pi-go/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSystemPrompt_Default(t *testing.T) {
	prompt := BuildSystemPrompt(Options{})
	assert.Contains(t, prompt, "Pi Go")
	assert.Contains(t, prompt, "Current date:")
}

func TestBuildSystemPrompt_CustomPrompt(t *testing.T) {
	prompt := BuildSystemPrompt(Options{CustomPrompt: "You are a test agent."})
	assert.Contains(t, prompt, "You are a test agent.")
	assert.NotContains(t, prompt, "Pi Go")
}

func TestBuildSystemPrompt_WithCWD(t *testing.T) {
	prompt := BuildSystemPrompt(Options{CWD: "/home/user/project"})
	assert.Contains(t, prompt, "/home/user/project")
}

func TestBuildSystemPrompt_WithTools(t *testing.T) {
	toolList := []agent.Tool{tools.NewBashTool(), tools.NewReadTool()}
	prompt := BuildSystemPrompt(Options{Tools: toolList})
	assert.Contains(t, prompt, "## Available Tools")
	assert.Contains(t, prompt, "### bash")
	assert.Contains(t, prompt, "### read")
	assert.Contains(t, prompt, "## Tool Summary")
	assert.Contains(t, prompt, "Execute shell commands")
	assert.Contains(t, prompt, "Read file contents")
}

func TestBuildSystemPrompt_NoTools(t *testing.T) {
	prompt := BuildSystemPrompt(Options{})
	assert.NotContains(t, prompt, "## Available Tools")
	assert.NotContains(t, prompt, "## Tool Summary")
}

func TestBuildSystemPrompt_ContainsDate(t *testing.T) {
	prompt := BuildSystemPrompt(Options{})
	assert.True(t, strings.Contains(prompt, "Current date: 20"))
}

func TestBuildSystemPrompt_WithGuidelines(t *testing.T) {
	toolList := []agent.Tool{tools.NewBashTool(), tools.NewReadTool()}
	prompt := BuildSystemPrompt(Options{Tools: toolList})
	assert.Contains(t, prompt, "## Guidelines")
	assert.Contains(t, prompt, "Be concise")
}

func TestBuildSystemPrompt_WithContextFiles(t *testing.T) {
	contextFiles := []ContextFile{
		{Path: "/project/CLAUDE.md", Content: "Always use tabs for indentation."},
	}
	prompt := BuildSystemPrompt(Options{ContextFiles: contextFiles})
	assert.Contains(t, prompt, "# Project Context")
	assert.Contains(t, prompt, "/project/CLAUDE.md")
	assert.Contains(t, prompt, "Always use tabs for indentation.")
}

func TestBuildSystemPrompt_WithSkills(t *testing.T) {
	skills := []skill.Skill{
		{
			Name:        "graphify",
			Description: "Convert to knowledge graph",
			FilePath:    "/skills/graphify/SKILL.md",
			BaseDir:     "/skills/graphify",
		},
	}
	prompt := BuildSystemPrompt(Options{Skills: skills})
	assert.Contains(t, prompt, "<available_skills>")
	assert.Contains(t, prompt, "<name>graphify</name>")
}

func TestBuildSystemPrompt_WithAppendSystemPrompt(t *testing.T) {
	prompt := BuildSystemPrompt(Options{AppendSystemPrompt: "Extra instructions here."})
	assert.Contains(t, prompt, "Extra instructions here.")
}

func TestBuildSystemPrompt_ToolGuidelines(t *testing.T) {
	toolList := []agent.Tool{tools.NewEditTool()}
	prompt := BuildSystemPrompt(Options{Tools: toolList})
	assert.Contains(t, prompt, "Use edit for targeted modifications")
}

func TestBuildSystemPrompt_SmartGuidelines_BashWithGrep(t *testing.T) {
	toolList := []agent.Tool{tools.NewBashTool(), tools.NewGrepTool()}
	prompt := BuildSystemPrompt(Options{Tools: toolList})
	assert.Contains(t, prompt, "Prefer grep/find/ls tools over bash for file exploration")
}

func TestBuildSystemPrompt_SmartGuidelines_BashOnly(t *testing.T) {
	toolList := []agent.Tool{tools.NewBashTool()}
	prompt := BuildSystemPrompt(Options{Tools: toolList})
	assert.Contains(t, prompt, "Use bash for file operations")
}

// ─── Context File 加载 ────────────────────────────────────────────────────────

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

	// 根目录有 CLAUDE.md
	require.NoError(t, os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("Root instructions"), 0o644))
	// 中间目录有 AGENTS.md
	require.NoError(t, os.WriteFile(filepath.Join(root, "a", "AGENTS.md"), []byte("Mid instructions"), 0o644))

	files := LoadProjectContextFiles(child, "")
	require.Len(t, files, 2)
	// 从根到叶排列
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
	// 全局的在前
	assert.Contains(t, files[0].Content, "Global")
	assert.Contains(t, files[1].Content, "Local")
}

func TestLoadProjectContextFiles_Dedup(t *testing.T) {
	cwd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "CLAUDE.md"), []byte("Same dir"), 0o644))

	// 同一个目录不应该重复
	files := LoadProjectContextFiles(cwd, cwd)
	assert.Len(t, files, 1)
}

// ─── Git Branch ───────────────────────────────────────────────────────────────

func TestBuildSystemPrompt_GitBranch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/feature/test\n"), 0o644))

	prompt := BuildSystemPrompt(Options{CWD: dir})
	assert.Contains(t, prompt, "feature/test")
}

func TestBuildSystemPrompt_NoGit(t *testing.T) {
	dir := t.TempDir()
	prompt := BuildSystemPrompt(Options{CWD: dir})
	assert.NotContains(t, prompt, "git branch")
}
