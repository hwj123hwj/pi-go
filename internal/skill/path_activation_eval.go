package skill

import "fmt"

type PathActivationAction string

const (
	PathActivationNone       PathActivationAction = "none"
	PathActivationAutoInvoke PathActivationAction = "auto_invoke"
	PathActivationSteer      PathActivationAction = "steer"
)

type PathActivationEvalCase struct {
	Name         string
	Skills       []Skill
	Workspace    string
	Input        string
	ExpectAction PathActivationAction
	ExpectSkills []string
	RejectSkills []string
}

type PathActivationEvalResult struct {
	Name          string
	Passed        bool
	FalsePositive int
	FalseNegative int
	Failures      []string
	Action        PathActivationAction
	MatchedSkills []string
}

type PathActivationEvalSummary struct {
	Total         int
	Passed        int
	Failed        int
	FalsePositive int
	FalseNegative int
	Results       []PathActivationEvalResult
}

func RunPathActivationEval(cases []PathActivationEvalCase) PathActivationEvalSummary {
	summary := PathActivationEvalSummary{Total: len(cases), Results: make([]PathActivationEvalResult, 0, len(cases))}
	for _, tc := range cases {
		result := runPathActivationEvalCase(tc)
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

func runPathActivationEvalCase(tc PathActivationEvalCase) PathActivationEvalResult {
	matches := MatchByPath(tc.Skills, tc.Workspace, tc.Input)
	names := make([]string, 0, len(matches))
	for _, sk := range matches {
		names = append(names, sk.Name)
	}
	action := pathActivationActionForMatches(matches)
	result := PathActivationEvalResult{
		Name:          tc.Name,
		Action:        action,
		MatchedSkills: names,
	}
	if action != tc.ExpectAction {
		switch {
		case action == PathActivationNone:
			result.FalseNegative++
		case tc.ExpectAction == PathActivationNone:
			result.FalsePositive++
		default:
			result.FalsePositive++
			result.FalseNegative++
		}
		result.Failures = append(result.Failures, fmt.Sprintf("action = %q, want %q", action, tc.ExpectAction))
	}
	matchedSet := nameSet(names)
	for _, name := range tc.ExpectSkills {
		if _, ok := matchedSet[name]; !ok {
			result.FalseNegative++
			result.Failures = append(result.Failures, fmt.Sprintf("expected skill missing: %s", name))
		}
	}
	for _, name := range tc.RejectSkills {
		if _, ok := matchedSet[name]; ok {
			result.FalsePositive++
			result.Failures = append(result.Failures, fmt.Sprintf("skill matched unexpectedly: %s", name))
		}
	}
	result.Passed = len(result.Failures) == 0
	return result
}

func pathActivationActionForMatches(matches []Skill) PathActivationAction {
	switch len(matches) {
	case 0:
		return PathActivationNone
	case 1:
		return PathActivationAutoInvoke
	default:
		return PathActivationSteer
	}
}

func nameSet(names []string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		out[name] = struct{}{}
	}
	return out
}
