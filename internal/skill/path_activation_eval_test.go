package skill

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPathActivationEval_ClassifiesAutoSteerAndNoMatch(t *testing.T) {
	workspace := filepath.Join(string(filepath.Separator), "repo")
	skills := []Skill{
		{Name: "docs-skill", Paths: []string{"docs/**"}},
		{Name: "deck-skill", Paths: []string{"*.pptx"}},
		{Name: "other-deck-skill", Paths: []string{"*.pptx"}},
		{Name: "guizang-ppt-skill", Paths: []string{"decks/**/*.html", "slides/**/*.pptx"}},
		{Name: "hidden-docs", Paths: []string{"docs/**"}, DisableModelInvocation: true},
	}

	summary := RunPathActivationEval([]PathActivationEvalCase{
		{
			Name:         "single docs path auto invokes",
			Skills:       skills,
			Workspace:    workspace,
			Input:        "update docs/guide.md",
			ExpectAction: PathActivationAutoInvoke,
			ExpectSkills: []string{"docs-skill"},
			RejectSkills: []string{"hidden-docs", "deck-skill"},
		},
		{
			Name:         "ambiguous deck path steers",
			Skills:       skills,
			Workspace:    workspace,
			Input:        "restyle " + filepath.Join(workspace, "slides", "ml.pptx"),
			ExpectAction: PathActivationSteer,
			ExpectSkills: []string{"deck-skill", "other-deck-skill"},
			RejectSkills: []string{"docs-skill"},
		},
		{
			Name:         "plain prose does not trigger",
			Skills:       skills,
			Workspace:    workspace,
			Input:        "please write a concise summary about docs quality",
			ExpectAction: PathActivationNone,
			RejectSkills: []string{"docs-skill", "deck-skill", "other-deck-skill", "guizang-ppt-skill", "hidden-docs"},
		},
		{
			Name:         "outside docs folder does not trigger docs skill",
			Skills:       skills,
			Workspace:    workspace,
			Input:        "update " + filepath.Join(workspace, "archive", "guide.md"),
			ExpectAction: PathActivationNone,
			RejectSkills: []string{"docs-skill"},
		},
		{
			Name:         "specific deck html path auto invokes guizang",
			Skills:       skills,
			Workspace:    workspace,
			Input:        "update decks/ml-deck/index.html",
			ExpectAction: PathActivationAutoInvoke,
			ExpectSkills: []string{"guizang-ppt-skill"},
			RejectSkills: []string{"deck-skill", "other-deck-skill", "docs-skill"},
		},
		{
			Name:         "pptx prose without path does not trigger",
			Skills:       skills,
			Workspace:    workspace,
			Input:        "做一个机器学习 PPTX 风格的讲稿",
			ExpectAction: PathActivationNone,
			RejectSkills: []string{"deck-skill", "other-deck-skill", "guizang-ppt-skill"},
		},
	})

	require.Equal(t, 6, summary.Total)
	assert.Equal(t, 6, summary.Passed)
	assert.Equal(t, 0, summary.Failed)
	assert.Equal(t, 0, summary.FalsePositive)
	assert.Equal(t, 0, summary.FalseNegative)
	for _, result := range summary.Results {
		assert.Truef(t, result.Passed, "%s failures: %v", result.Name, result.Failures)
	}
}

func TestRunPathActivationEval_ClassifiesFalsePositiveAndFalseNegative(t *testing.T) {
	skills := []Skill{{Name: "docs-skill", Paths: []string{"docs/**"}}}

	summary := RunPathActivationEval([]PathActivationEvalCase{
		{
			Name:         "unexpected match",
			Skills:       skills,
			Workspace:    "/repo",
			Input:        "update docs/guide.md",
			ExpectAction: PathActivationNone,
			RejectSkills: []string{"docs-skill"},
		},
		{
			Name:         "missing expected match",
			Skills:       skills,
			Workspace:    "/repo",
			Input:        "write a summary",
			ExpectAction: PathActivationAutoInvoke,
			ExpectSkills: []string{"docs-skill"},
		},
	})

	require.Equal(t, 2, summary.Total)
	assert.Equal(t, 0, summary.Passed)
	assert.Equal(t, 2, summary.Failed)
	assert.Greater(t, summary.FalsePositive, 0)
	assert.Greater(t, summary.FalseNegative, 0)
}
