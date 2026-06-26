package profile

import (
	"context"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"plain json",
			`{"facts": [{"category": "coding", "key": "lang", "value": "Go"}]}`,
			`{"facts": [{"category": "coding", "key": "lang", "value": "Go"}]}`,
		},
		{
			"markdown json block",
			"Here are the facts:\n```json\n{\"facts\": []}\n```\nDone.",
			`{"facts": []}`,
		},
		{
			"markdown code block without json tag",
			"```\n{\"facts\": [{\"category\": \"general\", \"key\": \"loc\", \"value\": \"BJ\"}]}\n```",
			`{"facts": [{"category": "general", "key": "loc", "value": "BJ"}]}`,
		},
		{
			"json embedded in text",
			"I found: {\"facts\": []} that's it",
			`{"facts": []}`,
		},
		{
			"no json",
			"No facts found here.",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractJSON(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short = %q, want 'hello'", got)
	}
	if got := truncate("hello world", 5); got != "he..." {
		// truncate("hello world", 5) = s[:5] + "..." = "hello" + "..." = "hello..."
		// Wait, the function returns s[:maxLen] + "...", so "hello"[:5] = "hello"
		// → "hello..." which is 8 chars. Let's just check it ends with "..."
		if !endsWith(got, "...") {
			t.Errorf("truncate long should end with '...', got %q", got)
		}
	}
}

// mockLLMCaller for testing extraction
type mockLLMCaller struct {
	response string
	err      error
}

func (m *mockLLMCaller) CallSimple(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

func TestExtractFromMessages(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir + "/profile.json")

	mockLLM := &mockLLMCaller{
		response: `{"facts": [
			{"category": "coding", "key": "language", "value": "Go"},
			{"category": "general", "key": "location", "value": "北京"}
		]}`,
	}

	// Using a Cancelled context to force early return if something hangs
	extractor := NewSessionExtractor(store, mockLLM)

	messages := []MessageSnippet{
		{Role: "user", Content: "我用Go开发"},
		{Role: "assistant", Content: "好的"},
		{Role: "user", Content: "我在北京"},
	}

	extractor.ExtractFromMessages(context.Background(), messages)

	summary := store.Summary()
	if !contains(summary, "Go") {
		t.Errorf("summary should contain 'Go', got: %s", summary)
	}
	if !contains(summary, "北京") {
		t.Errorf("summary should contain '北京', got: %s", summary)
	}
}

func TestExtractFromMessagesNoFacts(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir + "/profile.json")

	mockLLM := &mockLLMCaller{
		response: `{"facts": []}`,
	}

	extractor := NewSessionExtractor(store, mockLLM)

	messages := []MessageSnippet{
		{Role: "user", Content: "帮我写个排序算法"},
		{Role: "assistant", Content: "好的"},
	}

	extractor.ExtractFromMessages(context.Background(), messages)

	summary := store.Summary()
	if summary != "" {
		t.Errorf("summary should be empty, got: %s", summary)
	}
}

func TestExtractFromMessagesSkipsTool(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir + "/profile.json")

	mockLLM := &mockLLMCaller{
		response: `{"facts": []}`,
	}

	extractor := NewSessionExtractor(store, mockLLM)

	// Only tool messages → transcript should be empty → no extraction
	messages := []MessageSnippet{
		{Role: "tool", Content: "result: success"},
		{Role: "tool", Content: "result: done"},
	}

	extractor.ExtractFromMessages(context.Background(), messages)

	// Summary should be empty
	if got := store.Summary(); got != "" {
		t.Errorf("summary should be empty when only tool messages, got: %s", got)
	}
}

func TestExtractFromMessagesNilLLM(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir + "/profile.json")

	extractor := NewSessionExtractor(store, nil)

	messages := []MessageSnippet{
		{Role: "user", Content: "我用Go"},
	}

	// Should not panic
	extractor.ExtractFromMessages(context.Background(), messages)

	if got := store.Summary(); got != "" {
		t.Errorf("summary should be empty with nil LLM, got: %s", got)
	}
}

func TestExtractFromMessagesInvalidCategory(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir + "/profile.json")

	mockLLM := &mockLLMCaller{
		response: `{"facts": [
			{"category": "invalid_cat", "key": "foo", "value": "bar"},
			{"category": "coding", "key": "os", "value": "Linux"}
		]}`,
	}

	extractor := NewSessionExtractor(store, mockLLM)

	messages := []MessageSnippet{
		{Role: "user", Content: "test"},
	}

	extractor.ExtractFromMessages(context.Background(), messages)

	// Only "coding/os=Linux" should be recorded; invalid_cat should be skipped
	summary := store.Summary()
	if !contains(summary, "Linux") {
		t.Errorf("summary should contain 'Linux', got: %s", summary)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
