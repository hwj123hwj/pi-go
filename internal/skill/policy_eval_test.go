package skill

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPolicyEval_GuizangBranchesAndAmbiguousRequest(t *testing.T) {
	sk := makePolicyEvalGuizangSkill(t)

	summary := RunPolicyEval([]PolicyEvalCase{
		{
			Name:         "guizang magazine branch",
			Skill:        sk,
			Args:         "做一份机器学习 PPT，风格 A 电子杂志",
			ExpectBranch: "magazine",
			ExpectAllowed: []string{
				"assets/template.html",
				"references/themes.md",
				"references/layouts.md",
				"references/checklist.md",
			},
			ExpectBlocked: []string{
				"assets/template-swiss.html",
				"references/themes-swiss.md",
				"references/layouts-swiss.md",
				"references/swiss-layout-lock.md",
			},
			ExpectNotAllowed: []string{
				"assets/template-swiss.html",
				"references/themes-swiss.md",
				"references/layouts-swiss.md",
			},
			ExpectNotBlocked: []string{
				"references/checklist.md",
			},
		},
		{
			Name:         "guizang swiss branch",
			Skill:        sk,
			Args:         "继续换成 Swiss Style / 瑞士国际主义",
			ExpectBranch: "swiss",
			ExpectAllowed: []string{
				"assets/template-swiss.html",
				"references/themes-swiss.md",
				"references/layouts-swiss.md",
				"references/swiss-layout-lock.md",
				"scripts/validate-swiss-deck.mjs",
				"references/checklist.md",
			},
			ExpectBlocked: []string{
				"assets/template.html",
				"references/themes.md",
				"references/layouts.md",
			},
			ExpectNotAllowed: []string{
				"assets/template.html",
				"references/themes.md",
				"references/layouts.md",
			},
			ExpectNotBlocked: []string{
				"references/checklist.md",
			},
		},
		{
			Name:         "guizang ambiguous branch",
			Skill:        sk,
			Args:         "做一份机器学习 PPT",
			ExpectBranch: "",
			ExpectAllowed: []string{
				"references/checklist.md",
			},
			ExpectBlocked: []string{
				"assets/template.html",
				"assets/template-swiss.html",
				"references/themes.md",
				"references/themes-swiss.md",
			},
			ExpectNotAllowed: []string{
				"assets/template.html",
				"assets/template-swiss.html",
			},
			ExpectNotBlocked: []string{
				"references/checklist.md",
			},
		},
	})

	require.Equal(t, 3, summary.Total)
	assert.Equal(t, 3, summary.Passed)
	assert.Equal(t, 0, summary.Failed)
	assert.Equal(t, 0, summary.FalsePositive)
	assert.Equal(t, 0, summary.FalseNegative)
	for _, result := range summary.Results {
		assert.Truef(t, result.Passed, "%s failures: %v", result.Name, result.Failures)
	}
}

func TestRunPolicyEval_ClassifiesFalsePositiveAndFalseNegative(t *testing.T) {
	sk := makePolicyEvalGuizangSkill(t)

	summary := RunPolicyEval([]PolicyEvalCase{{
		Name:             "intentional mismatch",
		Skill:            sk,
		Args:             "风格 B",
		ExpectBranch:     "magazine",
		ExpectAllowed:    []string{"assets/template.html"},
		ExpectNotAllowed: []string{"assets/template-swiss.html"},
	}})

	require.Equal(t, 1, summary.Total)
	assert.Equal(t, 0, summary.Passed)
	assert.Equal(t, 1, summary.Failed)
	assert.Greater(t, summary.FalsePositive, 0)
	assert.Greater(t, summary.FalseNegative, 0)
	assert.False(t, summary.Results[0].Passed)
	assert.NotEmpty(t, summary.Results[0].Failures)
}

func TestRunPolicyEval_CoversAllowedToolsPathsAndForkContext(t *testing.T) {
	sk := makePolicyEvalDocsMaintainerSkill()

	summary := RunPolicyEval([]PolicyEvalCase{{
		Name:                   "docs maintainer constrained policy",
		Skill:                  sk,
		Args:                   "同步 docs/README.md 和 internal/skill/skill.go",
		ExpectExecutionContext: "fork",
		ExpectAllowedTools:     []string{"read", "edit", "bash"},
		RejectAllowedTools:     []string{"write", "ls", "grep"},
		ExpectAllowedToolSpecs: []string{"read", "edit", "Bash(go test ./internal/skill:*)"},
		ExpectPathPatterns:     []string{"docs/**", "internal/skill/**"},
	}})

	require.Equal(t, 1, summary.Total)
	assert.Equal(t, 1, summary.Passed)
	assert.Equal(t, 0, summary.FalsePositive)
	assert.Equal(t, 0, summary.FalseNegative)
	require.Len(t, summary.Results, 1)
	assert.Equal(t, []string{"read", "edit", "bash"}, summary.Results[0].AllowedTools)
	assert.Equal(t, "fork", summary.Results[0].ExecutionContext)
}

func TestBuildExecutionPolicy_ContinuationRequestOverridesPreviousBranch(t *testing.T) {
	sk := makePolicyEvalGuizangSkill(t)

	policy := BuildExecutionPolicy(sk, "Previous invocation args:\n风格 A 机器学习 PPT\n\nPrevious selected branch: magazine\n\nCurrent continuation request:\n继续把刚才那份换成瑞士风")

	assert.Equal(t, "swiss", policy.Branch)
	assert.Contains(t, policy.AllowedSkillPaths, filepath.Join(sk.BaseDir, "assets/template-swiss.html"))
	assert.NotContains(t, policy.AllowedSkillPaths, filepath.Join(sk.BaseDir, "assets/template.html"))
	assert.Contains(t, policy.BlockedSkillPaths, filepath.Join(sk.BaseDir, "assets/template.html"))
}

func TestBuildExecutionPolicy_ContinuationFallsBackToPreviousBranch(t *testing.T) {
	sk := makePolicyEvalGuizangSkill(t)

	policy := BuildExecutionPolicy(sk, "Previous invocation args:\n风格 A 机器学习 PPT\n\nPrevious selected branch: magazine\n\nCurrent continuation request:\n继续扩写内容")

	assert.Equal(t, "magazine", policy.Branch)
	assert.Contains(t, policy.AllowedSkillPaths, filepath.Join(sk.BaseDir, "assets/template.html"))
	assert.NotContains(t, policy.AllowedSkillPaths, filepath.Join(sk.BaseDir, "assets/template-swiss.html"))
}

func makePolicyEvalGuizangSkill(t *testing.T) Skill {
	t.Helper()
	dir := t.TempDir()
	for _, rel := range []string{
		"assets/template.html",
		"assets/template-swiss.html",
		"references/themes.md",
		"references/themes-swiss.md",
		"references/layouts.md",
		"references/layouts-swiss.md",
		"references/swiss-layout-lock.md",
		"references/checklist.md",
		"scripts/validate-swiss-deck.mjs",
	} {
		path := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(rel), 0o644))
	}
	return Skill{
		Name:    "guizang-ppt-skill",
		BaseDir: dir,
		Branches: []SkillBranch{
			{
				Name:    "magazine",
				Aliases: []string{"风格 A", "电子杂志", "magazine"},
				Paths: []string{
					"assets/template.html",
					"references/themes.md",
					"references/layouts.md",
				},
			},
			{
				Name:    "swiss",
				Aliases: []string{"风格 B", "瑞士", "Swiss Style"},
				Paths: []string{
					"assets/template-swiss.html",
					"references/themes-swiss.md",
					"references/layouts-swiss.md",
					"references/swiss-layout-lock.md",
					"scripts/validate-swiss-deck.mjs",
				},
			},
		},
		Content: "Shared checklist: `references/checklist.md`.",
	}
}

func makePolicyEvalDocsMaintainerSkill() Skill {
	return Skill{
		Name:             "docs-maintainer",
		Description:      "Keep docs synchronized with code.",
		BaseDir:          "/repo/.claude/skills/docs-maintainer",
		AllowedTools:     []string{"read", "edit", "Bash(go test ./internal/skill:*)"},
		Paths:            []string{"docs/**", "internal/skill/**"},
		ExecutionContext: "fork",
		Content:          "Update docs only for changed code paths.",
	}
}
