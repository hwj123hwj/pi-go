package skill

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Frontmatter 解析 ─────────────────────────────────────────────────────────

func TestParseFrontmatter_WithMetadata(t *testing.T) {
	content := `---
name: example
description: Example skill
---
Use this skill.
`
	fm, body := parseFrontmatter(content)
	assert.Equal(t, "example", fm["name"])
	assert.Equal(t, "Example skill", fm["description"])
	assert.Equal(t, "Use this skill.", body)
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "Just a skill body."
	fm, body := parseFrontmatter(content)
	assert.Nil(t, fm)
	assert.Equal(t, "Just a skill body.", body)
}

func TestParseFrontmatter_Unterminated(t *testing.T) {
	content := "---\nname: test\nNo closing"
	fm, body := parseFrontmatter(content)
	assert.Nil(t, fm)
	assert.Equal(t, content, body)
}

func TestParseFrontmatter_DisableModelInvocation(t *testing.T) {
	content := `---
name: hidden
description: Hidden skill
disable-model-invocation: true
---
Hidden content.
`
	fm, body := parseFrontmatter(content)
	assert.Equal(t, "true", fm["disable-model-invocation"])
	assert.Equal(t, "Hidden content.", body)
}

// ─── 技能加载 ─────────────────────────────────────────────────────────────────

func TestLoadFromDirs_SKILLMD(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	content := `---
name: my-skill
description: A test skill
---
Do something useful.
`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644))

	result := LoadFromDirs(dir)
	require.Len(t, result.Skills, 1)
	assert.Equal(t, "my-skill", result.Skills[0].Name)
	assert.Equal(t, "A test skill", result.Skills[0].Description)
	assert.Equal(t, "Do something useful.", result.Skills[0].Content)
	assert.Equal(t, skillDir, result.Skills[0].BaseDir)
	assert.False(t, result.Skills[0].DisableModelInvocation)
}

func TestLoadFromDirs_PlainMD(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: plain
description: Plain skill
---
Plain body.
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.md"), []byte(content), 0o644))

	result := LoadFromDirs(dir)
	require.Len(t, result.Skills, 1)
	assert.Equal(t, "plain", result.Skills[0].Name)
}

func TestLoadFromDirs_NestedDirectories(t *testing.T) {
	dir := t.TempDir()

	// 创建嵌套结构
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "skills", "tool-a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "skills", "tool-b"), 0o755))

	contentA := "---\nname: tool-a\ndescription: Tool A\n---\nTool A content.\n"
	contentB := "---\nname: tool-b\ndescription: Tool B\n---\nTool B content.\n"

	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills", "tool-a", "SKILL.md"), []byte(contentA), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills", "tool-b", "SKILL.md"), []byte(contentB), 0o644))

	result := LoadFromDirs(filepath.Join(dir, "skills"))
	assert.Len(t, result.Skills, 2)

	names := make(map[string]bool)
	for _, s := range result.Skills {
		names[s.Name] = true
	}
	assert.True(t, names["tool-a"])
	assert.True(t, names["tool-b"])
}

func TestLoadFromDirs_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()

	// 创建隐藏目录中的技能
	hiddenDir := filepath.Join(dir, ".hidden-skill")
	require.NoError(t, os.MkdirAll(hiddenDir, 0o755))
	hiddenContent := "---\nname: hidden\ndescription: Hidden\n---\nHidden.\n"
	require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "SKILL.md"), []byte(hiddenContent), 0o644))

	// 创建正常目录中的技能
	normalDir := filepath.Join(dir, "visible-skill")
	require.NoError(t, os.MkdirAll(normalDir, 0o755))
	normalContent := "---\nname: visible-skill\ndescription: Visible\n---\nVisible.\n"
	require.NoError(t, os.WriteFile(filepath.Join(normalDir, "SKILL.md"), []byte(normalContent), 0o644))

	result := LoadFromDirs(dir)
	assert.Len(t, result.Skills, 1)
	assert.Equal(t, "visible-skill", result.Skills[0].Name)
}

func TestLoadFromDirs_NonexistentDir(t *testing.T) {
	result := LoadFromDirs("/nonexistent/path")
	assert.Empty(t, result.Skills)
	// 不存在的目录不产生诊断（被静默跳过）
	hasNonExist := false
	for _, d := range result.Diagnostics {
		if d.Code == DiagFileInfoFailed {
			hasNonExist = true
		}
	}
	assert.False(t, hasNonExist)
}

func TestLoadFromDirs_SkipsNodeModules(t *testing.T) {
	dir := t.TempDir()

	nmDir := filepath.Join(dir, "node_modules", "some-pkg")
	require.NoError(t, os.MkdirAll(nmDir, 0o755))
	content := "---\nname: nm\ndescription: NM\n---\nNM.\n"
	require.NoError(t, os.WriteFile(filepath.Join(nmDir, "SKILL.md"), []byte(content), 0o644))

	result := LoadFromDirs(dir)
	assert.Empty(t, result.Skills)
}

func TestLoadFromDirs_DisableModelInvocation(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "internal-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	content := `---
name: internal-skill
description: Internal use
disable-model-invocation: true
---
Internal.
`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644))

	result := LoadFromDirs(dir)
	require.Len(t, result.Skills, 1)
	assert.True(t, result.Skills[0].DisableModelInvocation)
}

// ─── 格式化 ───────────────────────────────────────────────────────────────────

func TestFormatForSystemPrompt(t *testing.T) {
	skills := []Skill{
		{
			Name:        "graphify",
			Description: "Convert input to knowledge graph",
			FilePath:    "/home/user/.skills/graphify/SKILL.md",
			BaseDir:     "/home/user/.skills/graphify",
		},
		{
			Name:                  "internal",
			Description:           "Internal tool",
			FilePath:              "/home/user/.skills/internal/SKILL.md",
			DisableModelInvocation: true,
		},
	}

	output := FormatForSystemPrompt(skills)
	assert.Contains(t, output, "<available_skills>")
	assert.Contains(t, output, "<name>graphify</name>")
	assert.Contains(t, output, "<description>Convert input to knowledge graph</description>")
	assert.Contains(t, output, "<location>/home/user/.skills/graphify/SKILL.md</location>")
	assert.Contains(t, output, "</available_skills>")
	// internal 被隐藏
	assert.NotContains(t, output, "internal")
}

func TestFormatForSystemPrompt_Empty(t *testing.T) {
	assert.Equal(t, "", FormatForSystemPrompt(nil))
	assert.Equal(t, "", FormatForSystemPrompt([]Skill{}))
}

func TestFormatInvocation(t *testing.T) {
	skill := Skill{
		Name:        "test",
		Description: "Test skill",
		Content:     "Do the thing.",
		FilePath:    "/skills/test/SKILL.md",
		BaseDir:     "/skills/test",
	}

	result := FormatInvocation(skill, "Also do this extra thing.")
	assert.Contains(t, result, `<skill name="test" location="/skills/test/SKILL.md">`)
	assert.Contains(t, result, "References are relative to /skills/test.")
	assert.Contains(t, result, "Do the thing.")
	assert.Contains(t, result, "</skill>")
	assert.Contains(t, result, "Also do this extra thing.")
}

func TestFormatInvocation_NoAdditional(t *testing.T) {
	skill := Skill{
		Name:     "solo",
		Content:  "Solo content.",
		FilePath: "/solo/SKILL.md",
		BaseDir:  "/solo",
	}

	result := FormatInvocation(skill, "")
	assert.Contains(t, result, "<skill name=\"solo\"")
	assert.Contains(t, result, "Solo content.")
	assert.Contains(t, result, "</skill>")
	// 没有额外的指令追加在末尾
	assert.True(t, len(result) > 0)
}

// ─── 查找 ─────────────────────────────────────────────────────────────────────

func TestFindByName(t *testing.T) {
	skills := []Skill{
		{Name: "alpha", Description: "A"},
		{Name: "beta", Description: "B"},
	}

	found := FindByName(skills, "beta")
	require.NotNil(t, found)
	assert.Equal(t, "beta", found.Name)

	assert.Nil(t, FindByName(skills, "nonexistent"))
}

// ─── XML 转义 ─────────────────────────────────────────────────────────────────

func TestEscapeXML(t *testing.T) {
	assert.Equal(t, "&lt;tag&gt;", escapeXML("<tag>"))
	assert.Equal(t, "&amp;", escapeXML("&"))
	assert.Equal(t, "&quot;hi&quot;", escapeXML(`"hi"`))
	assert.Equal(t, "&apos;s", escapeXML("'s"))
}

// ─── Name 自动推断 ────────────────────────────────────────────────────────────

func TestParseSkillFile_NameFromDirectory(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "auto-named")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	// SKILL.md 没有 name frontmatter
	content := "---\ndescription: Auto-named skill\n---\nContent here.\n"
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644))

	result := LoadFromDirs(dir)
	require.Len(t, result.Skills, 1)
	assert.Equal(t, "auto-named", result.Skills[0].Name)
}

func TestParseSkillFile_NameFromFileName(t *testing.T) {
	dir := t.TempDir()
	content := "---\ndescription: File-named skill\n---\nContent.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "my-tool.md"), []byte(content), 0o644))

	result := LoadFromDirs(dir)
	require.Len(t, result.Skills, 1)
	assert.Equal(t, "my-tool", result.Skills[0].Name)
}

// ─── 无 Frontmatter ──────────────────────────────────────────────────────────

func TestLoadFromDirs_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "basic")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Just plain text content."), 0o644))

	result := LoadFromDirs(dir)
	require.Len(t, result.Skills, 1)
	assert.Equal(t, "basic", result.Skills[0].Name)
	assert.Equal(t, "Just plain text content.", result.Skills[0].Content)
}

// ─── 多目录加载 ───────────────────────────────────────────────────────────────

func TestLoadFromDirs_MultipleDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir1, "skill-a"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "skill-a", "SKILL.md"), []byte("---\nname: skill-a\ndescription: A\n---\nA"), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(dir2, "skill-b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "skill-b", "SKILL.md"), []byte("---\nname: skill-b\ndescription: B\n---\nB"), 0o644))

	result := LoadFromDirs(dir1, dir2)
	assert.Len(t, result.Skills, 2)
}

// ─── 诊断信息 ─────────────────────────────────────────────────────────────────

func TestLoadFromDirs_InvalidName(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "BAD NAME!")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	content := "---\nname: BAD NAME!\ndescription: Bad name\n---\nContent.\n"
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644))

	result := LoadFromDirs(dir)
	// 技能仍然被加载（诊断是警告，不阻止加载）
	require.Len(t, result.Skills, 1)
	assert.Equal(t, "BAD NAME!", result.Skills[0].Name)

	// 但有诊断信息
	hasInvalidName := false
	for _, d := range result.Diagnostics {
		if d.Code == DiagInvalidName {
			hasInvalidName = true
		}
	}
	assert.True(t, hasInvalidName)
}
