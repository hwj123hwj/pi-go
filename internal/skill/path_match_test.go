package skill

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchByPath_MatchesRelativePathPattern(t *testing.T) {
	skills := []Skill{
		{Name: "docs-skill", Paths: []string{"docs/**"}},
		{Name: "src-skill", Paths: []string{"src/**/*.go"}},
	}

	matched := MatchByPath(skills, "/repo", "please update `docs/guide/intro.md`")
	assert.Len(t, matched, 1)
	assert.Equal(t, "docs-skill", matched[0].Name)
}

func TestMatchByPath_MatchesAbsoluteWorkspacePath(t *testing.T) {
	workspace := filepath.Join(string(filepath.Separator), "repo")
	skills := []Skill{{Name: "docs-skill", Paths: []string{"docs/**"}}}

	matched := MatchByPath(skills, workspace, "check "+filepath.Join(workspace, "docs", "guide.md"))
	assert.Len(t, matched, 1)
	assert.Equal(t, "docs-skill", matched[0].Name)
}

func TestMatchByPath_SkipsHiddenInvocationDisabledSkills(t *testing.T) {
	skills := []Skill{{
		Name:                   "hidden",
		Paths:                  []string{"docs/**"},
		DisableModelInvocation: true,
	}}

	assert.Empty(t, MatchByPath(skills, "/repo", "docs/guide.md"))
}

func TestMatchByExplicitPaths(t *testing.T) {
	skills := []Skill{{Name: "go-skill", Paths: []string{"src/**/*.go"}}}

	matched := MatchByExplicitPaths(skills, "/repo", []string{"src/app/main.go"})
	assert.Len(t, matched, 1)
	assert.Equal(t, "go-skill", matched[0].Name)
}

func TestMatchByPath_MatchesBasenameAndAbsoluteSuffixGlobs(t *testing.T) {
	workspace := filepath.Join(string(filepath.Separator), "repo")
	skills := []Skill{
		{Name: "deck-skill", Paths: []string{"*.pptx"}},
		{Name: "docs-skill", Paths: []string{"docs/*.md"}},
	}

	matched := MatchByPath(skills, workspace, "restyle "+filepath.Join(workspace, "decks", "ml.pptx"))
	assert.Len(t, matched, 1)
	assert.Equal(t, "deck-skill", matched[0].Name)

	matched = MatchByPath(skills, workspace, "update "+filepath.Join(workspace, "docs", "guide.md"))
	assert.Len(t, matched, 1)
	assert.Equal(t, "docs-skill", matched[0].Name)

	assert.Empty(t, MatchByPath(skills, workspace, "update "+filepath.Join(workspace, "archive", "guide.md")))
}
