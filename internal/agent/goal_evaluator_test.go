package agent

import (
	"context"
	"testing"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/ai/providers"
)

// mockEvalProvider returns a fixed text response for testing goal evaluation.
type mockEvalProvider struct {
	response string // the text to return
}

func (p *mockEvalProvider) Name() string { return "mock-eval" }

func (p *mockEvalProvider) StreamSimple(ctx context.Context, req ai.SimpleStreamRequest) (*ai.EventStream, error) {
	return p.Stream(ctx, ai.StreamRequest{
		Model:    req.Model,
		Messages: req.Messages,
		System:   req.System,
		Tools:    req.Tools,
	})
}

func (p *mockEvalProvider) Stream(ctx context.Context, req ai.StreamRequest) (*ai.EventStream, error) {
	stream := ai.NewEventStream(8)
	go func() {
		defer stream.Close()
		partial := ai.StreamAssistantMessage{
			Text:       p.response,
			StopReason: ai.StopReasonStop,
		}
		_ = stream.Push(ctx, ai.EventStart{Partial: partial})
		_ = stream.Push(ctx, ai.EventTextDelta{ContentIndex: 0, Delta: p.response, Partial: partial})
		_ = stream.Push(ctx, ai.EventTextEnd{ContentIndex: 0, Text: p.response, Partial: partial})
		_ = stream.Push(ctx, ai.EventDone{Reason: ai.StopReasonStop, Message: partial})
		stream.SetResult(partial, nil)
	}()
	return stream, nil
}

func TestEvaluateGoalCompletion_LLMTrue(t *testing.T) {
	registry := providers.NewRegistry()
	registry.Register(&mockEvalProvider{response: `{"ok": true}`})

	model := ai.Model{ID: "test", Provider: "mock-eval"}

	done, reason := evaluateGoalCompletion(
		context.Background(),
		registry,
		model,
		"All optimizations have been applied and verified.",
		"Optimize error handling in app.go",
	)

	if !done {
		t.Errorf("expected done=true, got false (reason: %s)", reason)
	}
}

func TestEvaluateGoalCompletion_LLMFalse(t *testing.T) {
	registry := providers.NewRegistry()
	registry.Register(&mockEvalProvider{response: `{"ok": false, "reason": "still need to verify the changes"}`})

	model := ai.Model{ID: "test", Provider: "mock-eval"}

	done, reason := evaluateGoalCompletion(
		context.Background(),
		registry,
		model,
		"I've applied some changes to app.go.",
		"Optimize error handling in app.go",
	)

	if done {
		t.Error("expected done=false, got true")
	}
	if reason == "" {
		t.Error("expected a reason for not done")
	}
}

func TestEvaluateGoalCompletion_LLMWithMarkdownFence(t *testing.T) {
	registry := providers.NewRegistry()
	registry.Register(&mockEvalProvider{response: "```json\n{\"ok\": true}\n```"})

	model := ai.Model{ID: "test", Provider: "mock-eval"}

	done, _ := evaluateGoalCompletion(
		context.Background(),
		registry,
		model,
		"All tasks completed and verified.",
		"Fix all bugs",
	)

	if !done {
		t.Error("expected done=true with markdown fence wrapping")
	}
}

func TestEvaluateGoalCompletion_NoProvider_Fallback(t *testing.T) {
	registry := providers.NewRegistry() // empty, no provider registered

	model := ai.Model{ID: "test", Provider: "nonexistent"}

	// Response contains "goal has been achieved" keyword → fallback should detect it
	done, reason := evaluateGoalCompletion(
		context.Background(),
		registry,
		model,
		"The goal has been achieved. All optimizations are done.",
		"Optimize error handling",
	)

	if !done {
		t.Errorf("expected done=true via keyword fallback, got false (reason: %s)", reason)
	}
}

func TestEvaluateGoalCompletion_EmptyResponse(t *testing.T) {
	registry := providers.NewRegistry()
	registry.Register(&mockEvalProvider{response: `{"ok": true}`})

	model := ai.Model{ID: "test", Provider: "mock-eval"}

	done, _ := evaluateGoalCompletion(
		context.Background(),
		registry,
		model,
		"", // empty response
		"do something",
	)

	if done {
		t.Error("empty response should never be done")
	}
}

func TestParseGoalEvalResult(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected bool
	}{
		{"simple true", `{"ok": true}`, true},
		{"simple false", `{"ok": false, "reason": "not done"}`, false},
		{"with markdown fence", "```json\n{\"ok\": true}\n```", true},
		{"with plain fence", "```\n{\"ok\": false}\n```", false},
		{"whitespace padded", "  \n{\"ok\": true}\n  ", true},
		{"invalid JSON triggers fallback", "not json at all", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done, _ := parseGoalEvalResult(tt.raw, "I made some changes.")
			if done != tt.expected {
				t.Errorf("parseGoalEvalResult(%q) = %v, want %v", tt.raw, done, tt.expected)
			}
		})
	}
}

func TestBuildGoalEvalPrompt(t *testing.T) {
	prompt := buildGoalEvalPrompt("optimize code", "I made some changes")

	if !contains(prompt, "optimize code") {
		t.Error("prompt should contain the goal")
	}
	if !contains(prompt, "I made some changes") {
		t.Error("prompt should contain the assistant response")
	}
	if !contains(prompt, "<goal-objective>") {
		t.Error("prompt should have goal XML tags")
	}
	if !contains(prompt, "<assistant-response>") {
		t.Error("prompt should have response XML tags")
	}
}

func TestBuildGoalEvalPrompt_Truncation(t *testing.T) {
	longResponse := make([]byte, 5000)
	for i := range longResponse {
		longResponse[i] = 'x'
	}

	prompt := buildGoalEvalPrompt("goal", string(longResponse))

	// Should contain the truncated version, not the full 5000 chars
	if contains(prompt, string(longResponse)) {
		t.Error("long response should be truncated in prompt")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
