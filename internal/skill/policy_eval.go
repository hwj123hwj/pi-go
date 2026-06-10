package skill

import (
	"fmt"
	"path/filepath"
)

// PolicyEvalCase describes an expected skill execution policy decision.
// Paths may be absolute or relative to Skill.BaseDir.
type PolicyEvalCase struct {
	Name                   string
	Skill                  Skill
	Args                   string
	ExpectBranch           string
	ExpectExecutionContext string
	ExpectAllowedTools     []string
	RejectAllowedTools     []string
	ExpectAllowedToolSpecs []string
	ExpectPathPatterns     []string
	ExpectAllowed          []string
	ExpectBlocked          []string
	ExpectNotAllowed       []string
	ExpectNotBlocked       []string
}

type PolicyEvalResult struct {
	Name             string
	Passed           bool
	FalsePositive    int
	FalseNegative    int
	Failures         []string
	AllowedTools     []string
	AllowedToolSpecs []string
	PathPatterns     []string
	AllowedPaths     []string
	BlockedPaths     []string
	SelectedBranch   string
	ExecutionContext string
}

type PolicyEvalSummary struct {
	Total         int
	Passed        int
	Failed        int
	FalsePositive int
	FalseNegative int
	Results       []PolicyEvalResult
}

// RunPolicyEval evaluates skill policy expectations and classifies misses.
func RunPolicyEval(cases []PolicyEvalCase) PolicyEvalSummary {
	summary := PolicyEvalSummary{Total: len(cases), Results: make([]PolicyEvalResult, 0, len(cases))}
	for _, tc := range cases {
		result := runPolicyEvalCase(tc)
		if result.Passed {
			summary.Passed++
		} else {
			summary.Failed++
		}
		summary.FalsePositive += result.FalsePositive
		summary.FalseNegative += result.FalseNegative
		summary.Results = append(summary.Results, result)
	}
	return summary
}

func runPolicyEvalCase(tc PolicyEvalCase) PolicyEvalResult {
	policy := BuildExecutionPolicy(tc.Skill, tc.Args)
	result := PolicyEvalResult{
		Name:             tc.Name,
		AllowedTools:     append([]string(nil), policy.AllowedTools...),
		AllowedToolSpecs: append([]string(nil), policy.AllowedToolSpecs...),
		PathPatterns:     append([]string(nil), policy.PathPatterns...),
		AllowedPaths:     append([]string(nil), policy.AllowedSkillPaths...),
		BlockedPaths:     append([]string(nil), policy.BlockedSkillPaths...),
		SelectedBranch:   policy.Branch,
		ExecutionContext: policy.ExecutionContext,
	}
	allowed := stringSet(policy.AllowedSkillPaths)
	blocked := stringSet(policy.BlockedSkillPaths)
	allowedTools := valueSet(policy.AllowedTools)
	allowedToolSpecs := valueSet(policy.AllowedToolSpecs)
	pathPatterns := valueSet(policy.PathPatterns)

	if tc.ExpectBranch != policy.Branch {
		result.FalseNegative++
		result.Failures = append(result.Failures, fmt.Sprintf("branch = %q, want %q", policy.Branch, tc.ExpectBranch))
	}
	if tc.ExpectExecutionContext != "" && tc.ExpectExecutionContext != policy.ExecutionContext {
		result.FalseNegative++
		result.Failures = append(result.Failures, fmt.Sprintf("execution context = %q, want %q", policy.ExecutionContext, tc.ExpectExecutionContext))
	}
	for _, tool := range tc.ExpectAllowedTools {
		if _, ok := allowedTools[tool]; !ok {
			result.FalseNegative++
			result.Failures = append(result.Failures, fmt.Sprintf("expected allowed tool missing: %s", tool))
		}
	}
	for _, tool := range tc.RejectAllowedTools {
		if _, ok := allowedTools[tool]; ok {
			result.FalsePositive++
			result.Failures = append(result.Failures, fmt.Sprintf("tool was allowed unexpectedly: %s", tool))
		}
	}
	for _, spec := range tc.ExpectAllowedToolSpecs {
		if _, ok := allowedToolSpecs[spec]; !ok {
			result.FalseNegative++
			result.Failures = append(result.Failures, fmt.Sprintf("expected allowed tool spec missing: %s", spec))
		}
	}
	for _, pattern := range tc.ExpectPathPatterns {
		if _, ok := pathPatterns[pattern]; !ok {
			result.FalseNegative++
			result.Failures = append(result.Failures, fmt.Sprintf("expected path pattern missing: %s", pattern))
		}
	}
	for _, path := range tc.ExpectAllowed {
		abs := evalPath(tc.Skill.BaseDir, path)
		if _, ok := allowed[abs]; !ok {
			result.FalseNegative++
			result.Failures = append(result.Failures, fmt.Sprintf("expected allowed path missing: %s", abs))
		}
	}
	for _, path := range tc.ExpectBlocked {
		abs := evalPath(tc.Skill.BaseDir, path)
		if _, ok := blocked[abs]; !ok {
			result.FalseNegative++
			result.Failures = append(result.Failures, fmt.Sprintf("expected blocked path missing: %s", abs))
		}
	}
	for _, path := range tc.ExpectNotAllowed {
		abs := evalPath(tc.Skill.BaseDir, path)
		if _, ok := allowed[abs]; ok {
			result.FalsePositive++
			result.Failures = append(result.Failures, fmt.Sprintf("path was allowed unexpectedly: %s", abs))
		}
	}
	for _, path := range tc.ExpectNotBlocked {
		abs := evalPath(tc.Skill.BaseDir, path)
		if _, ok := blocked[abs]; ok {
			result.FalsePositive++
			result.Failures = append(result.Failures, fmt.Sprintf("path was blocked unexpectedly: %s", abs))
		}
	}
	result.Passed = len(result.Failures) == 0
	return result
}

func evalPath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[filepath.Clean(value)] = struct{}{}
	}
	return out
}

func valueSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}
