package profile

import "strings"

// Profile represents a coding-agent role configuration.
type Profile string

const (
	// ProfileCoding is the default coding mode — full tool access, write/edit enabled.
	ProfileCoding Profile = "coding"
	// ProfileReview is the review mode — read-only focus, analyzes risks and gaps.
	ProfileReview Profile = "review"
)

// Valid returns true if the given profile name is recognized.
func Valid(p string) bool {
	switch Profile(p) {
	case ProfileCoding, ProfileReview:
		return true
	default:
		return false
	}
}

// All returns the list of available profile names.
func All() []string {
	return []string{string(ProfileCoding), string(ProfileReview)}
}

// PromptFor returns the system prompt base for the given profile.
// This is the profile-specific portion that replaces the default prompt.
func PromptFor(p Profile) string {
	switch p {
	case ProfileReview:
		return reviewPrompt
	case ProfileCoding:
		fallthrough
	default:
		return ""
	}
}

// PromptAppendFor returns additional prompt instructions for the given profile.
// This is appended after the base prompt.
func PromptAppendFor(p Profile) string {
	switch p {
	case ProfileReview:
		return reviewAppend
	case ProfileCoding:
		fallthrough
	default:
		return ""
	}
}

const reviewPrompt = `You are Pi Go in Code Review mode. You are a senior engineer performing a careful, thorough code review.

You operate inside an agent loop:
1. Receive a user message (usually code or a diff to review)
2. Think about potential issues, risks, and improvements
3. Use read-only tools to examine relevant code context
4. Return your review findings

Your primary goals are to:
- Identify bugs, security vulnerabilities, and logic errors
- Spot missing error handling and edge cases
- Check for test coverage gaps
- Assess code readability and maintainability
- Flag performance concerns

Important constraints:
- You are in REVIEW mode. Do NOT modify, edit, or write files.
- Use read, grep, find, and ls to understand the codebase.
- If you need to run commands, only use read-only operations.
- When you find issues, clearly describe the problem and suggest fixes.
- Be specific: reference file paths, line numbers, and code snippets.`

const reviewAppend = `

## Review Mode Active

You are currently in review mode. Your role is strictly analytical:
- READ files to understand context
- ANALYZE code for issues
- REPORT findings clearly
- Do NOT edit, write, or execute modifying commands
`

// FormatList returns a human-readable list of available profiles.
func FormatList(current string) string {
	var b strings.Builder
	b.WriteString("Profiles:\n")
	for _, p := range All() {
		marker := "  "
		if p == current {
			marker = "→ "
		}
		b.WriteString(marker + p + "\n")
	}
	return b.String()
}
