package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hwj123hwj/pi-go/sdk/ai"
	"github.com/hwj123hwj/pi-go/sdk/ai/providers"
)

// GoalEvalResult is the structured output from the goal completion evaluator.
type GoalEvalResult struct {
	Done   bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

var (
	goalLogFile *os.File
	goalLogMu   sync.Mutex
)

func goalLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)

	// Only log to file, not stderr (keeps TUI clean)
	goalLogMu.Lock()
	defer goalLogMu.Unlock()
	if goalLogFile == nil {
		path := filepath.Join(os.TempDir(), "pi-goal-debug.log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return
		}
		goalLogFile = f
	}
	fmt.Fprintf(goalLogFile, "[%s] %s", time.Now().Format("15:04:05.000"), msg)
}

// evaluateGoalCompletion uses a lightweight LLM call to determine whether
// the agent's latest response shows that the goal has been fully achieved.
//
// This mirrors CC's createGoalPromptHook approach: a focused evaluator prompt
// that only judges completion, never executes the goal itself.
//
// On any error (provider unavailable, parse failure, timeout), it falls back
// to the keyword-based goalCompleted() function.
func evaluateGoalCompletion(
	ctx context.Context,
	registry *providers.Registry,
	model ai.Model,
	assistantText string,
	goal string,
) (bool, string) {
	// Short-circuit: if response is empty, definitely not done
	if assistantText == "" {
		return false, "empty response"
	}

	slog.Info("goal evaluator: starting evaluation", "goal", goal, "responseLen", len(assistantText), "provider", model.Provider)

	provider, ok := registry.Get(model.Provider)
	if !ok {
		slog.Warn("goal evaluator: provider not available, using keyword fallback", "provider", model.Provider)
		return goalCompleted(assistantText), "keyword fallback: provider unavailable"
	}

	evalPrompt := buildGoalEvalPrompt(goal, assistantText)

	stream, err := provider.Stream(ctx, ai.StreamRequest{
		Model: model,
		Messages: []ai.Message{
			ai.NewTextUserMessage(evalPrompt),
		},
		System:   "You are a goal completion evaluator. Return only valid JSON. No markdown, no prose, no explanation.",
		MaxTokens: intPtr(256),
	})
	if err != nil {
		slog.Warn("goal evaluator: stream failed, using keyword fallback", "error", err)
		return goalCompleted(assistantText), "keyword fallback: stream error"
	}

	// Collect the full response from the stream
	var result strings.Builder
	for event := range stream.Events() {
		switch e := event.(type) {
		case ai.EventTextDelta:
			result.WriteString(e.Delta)
		case ai.EventError:
			slog.Warn("goal evaluator: stream error, using keyword fallback", "error", e.Error)
			return goalCompleted(assistantText), "keyword fallback: stream error"
		case ai.EventDone:
			// stream complete
		}
	}

	goalLog("[goal-debug] Evaluator LLM raw response: %s\n", result.String())
	return parseGoalEvalResult(result.String(), assistantText)
}

// buildGoalEvalPrompt constructs the evaluator prompt, modeled after CC's
// createGoalPromptHook.
func buildGoalEvalPrompt(goal, assistantText string) string {
	// Truncate assistant text to avoid excessive token usage in the eval call.
	// Keep last ~3000 runes — completion signals are usually in the tail.
	// Use runes (not bytes) to avoid splitting multi-byte UTF-8 characters.
	truncated := assistantText
	runes := []rune(truncated)
	if len(runes) > 3000 {
		truncated = string(runes[len(runes)-3000:])
	}

	return fmt.Sprintf(`You are a goal completion evaluator. Determine whether the assistant has completed the stated objective.

RULES:
- If the assistant is still actively working (reading files, exploring code, running commands, waiting for tool results), return {"ok": false}.
- If the assistant has produced a final response that addresses the objective (summary, report, analysis, list, answer, etc.), return {"ok": true}.
- A response with conclusions, recommendations, or a structured output counts as done — even if more could theoretically be said.
- If the assistant explicitly says it is done, finished, or has completed the task, return {"ok": true}.
- When in doubt, consider whether a human reading the response would consider the task finished.

<goal-objective>
%s
</goal-objective>

<assistant-response>
%s
</assistant-response>

Return {"ok": true} if the objective is satisfied, or {"ok": false, "reason": "..."} if not.
Return only the JSON object. No markdown, no prose.`, goal, truncated)
}

// parseGoalEvalResult parses the LLM evaluator response.
// Falls back to keyword matching on any parse failure.
func parseGoalEvalResult(raw string, assistantText string) (bool, string) {
	// Clean up: strip markdown code fences if present
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var result GoalEvalResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		slog.Warn("goal evaluator: JSON parse failed, using keyword fallback", "raw", cleaned, "error", err)
		return goalCompleted(assistantText), "keyword fallback: parse error"
	}

	goalLog("[goal-debug] Parsed eval result: done=%v reason=%q\n", result.Done, result.Reason)
	return result.Done, result.Reason
}

func intPtr(v int) *int {
	return &v
}
