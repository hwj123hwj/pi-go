package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hwj123hwj/pi-go/internal/agent"
	platformprompt "github.com/hwj123hwj/pi-go/internal/prompt"
	"github.com/hwj123hwj/pi-go/internal/skill"
	basetools "github.com/hwj123hwj/pi-go/internal/tools"
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
	toolList := []agent.Tool{basetools.NewBashTool(), basetools.NewReadTool()}
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
	toolList := []agent.Tool{basetools.NewBashTool(), basetools.NewReadTool()}
	prompt := BuildSystemPrompt(Options{Tools: toolList})
	assert.Contains(t, prompt, "## Guidelines")
	assert.Contains(t, prompt, "Be concise")
}

func TestBuildSystemPrompt_WithContextFiles(t *testing.T) {
	contextFiles := []platformprompt.ContextFile{
		{Path: "/project/CLAUDE.md", Content: "Always use tabs for indentation."},
	}
	prompt := BuildSystemPrompt(Options{ContextFiles: contextFiles})
	assert.Contains(t, prompt, "# Project Context")
	assert.Contains(t, prompt, "/project/CLAUDE.md")
	assert.Contains(t, prompt, "Always use tabs for indentation.")
}

func TestBuildSystemPrompt_WithSkills(t *testing.T) {
	skills := []skill.Skill{{
		Name:        "graphify",
		Description: "Convert to knowledge graph",
		FilePath:    "/skills/graphify/SKILL.md",
		BaseDir:     "/skills/graphify",
	}}
	prompt := BuildSystemPrompt(Options{Skills: skills})
	assert.Contains(t, prompt, "<available_skills>")
	assert.Contains(t, prompt, "<name>graphify</name>")
}

func TestBuildSystemPrompt_WithAppendSystemPrompt(t *testing.T) {
	prompt := BuildSystemPrompt(Options{AppendSystemPrompt: "Extra instructions here."})
	assert.Contains(t, prompt, "Extra instructions here.")
}

func TestBuildSystemPrompt_ToolGuidelines(t *testing.T) {
	toolList := []agent.Tool{basetools.NewEditTool()}
	prompt := BuildSystemPrompt(Options{Tools: toolList})
	assert.Contains(t, prompt, "Use edit for targeted modifications")
}

func TestBuildSystemPrompt_SmartGuidelines_BashWithGrep(t *testing.T) {
	toolList := []agent.Tool{basetools.NewBashTool(), basetools.NewGrepTool()}
	prompt := BuildSystemPrompt(Options{Tools: toolList})
	assert.Contains(t, prompt, "Prefer grep/find/ls tools over bash for file exploration")
}

func TestBuildSystemPrompt_SmartGuidelines_BashOnly(t *testing.T) {
	toolList := []agent.Tool{basetools.NewBashTool()}
	prompt := BuildSystemPrompt(Options{Tools: toolList})
	assert.Contains(t, prompt, "Use bash for file operations")
}

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

func TestBuildSystemPrompt_CodingProfile(t *testing.T) {
	prompt := BuildSystemPrompt(Options{Profile: "coding"})
	assert.Contains(t, prompt, "Pi Go")
	assert.Contains(t, prompt, "server-side coding agent")
}

func TestBuildSystemPrompt_ReviewProfile(t *testing.T) {
	prompt := BuildSystemPrompt(Options{Profile: "review"})
	assert.Contains(t, prompt, "Code Review")
	assert.Contains(t, prompt, "REVIEW mode")
	assert.Contains(t, prompt, "Review Mode Active")
	assert.NotContains(t, prompt, "server-side coding agent")
}

func TestBuildSystemPrompt_ReviewProfile_WithCustomPrompt(t *testing.T) {
	prompt := BuildSystemPrompt(Options{CustomPrompt: "Custom prompt", Profile: "review"})
	assert.Contains(t, prompt, "Custom prompt")
	// Custom prompt overrides profile-specific prompt
	assert.NotContains(t, prompt, "Code Review")
}

func TestBuildSystemPrompt_CodingProfile_NoExtraAppend(t *testing.T) {
	prompt := BuildSystemPrompt(Options{Profile: "coding"})
	assert.NotContains(t, prompt, "Review Mode Active")
}
