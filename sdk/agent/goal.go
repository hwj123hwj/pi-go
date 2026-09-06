package agent

import "strings"

// goalCompleted checks whether the assistant's response indicates that the
// current goal has been fully achieved. This uses simple keyword matching
// rather than a separate LLM call — the agent is instructed via system prompt
// and follow-up messages to explicitly state when the goal is done.
//
// The detection is intentionally conservative: it only returns true when the
// response clearly signals completion. False negatives are acceptable (the
// agent will just continue for another turn); false positives are not (the
// agent would stop prematurely).
func goalCompleted(responseText string) bool {
	if responseText == "" {
		return false
	}
	text := strings.ToLower(responseText)

	// Strong signals — explicit completion statements
	// NOTE: This is a conservative fallback used only when the LLM evaluator fails.
	// Keep phrases strict to avoid false positives. Phrases like "task complete",
	// "all done" are too vague and can appear in intermediate LLM responses.
	strongPhrases := []string{
		"goal has been achieved",
		"goal has been completed",
		"goal is now complete",
		"objective has been achieved",
		"objective has been completed",
		"all tasks have been completed",
		"目标已达成",
		"目标已实现",
		"全部实施完毕",
	}
	for _, phrase := range strongPhrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}

	return false
}
