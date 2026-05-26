package models

import "testing"

func TestContextWindow_KnownModels(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{"claude-sonnet-4-6", 200000},
		{"claude-sonnet-4-5", 200000},
		{"claude-3-5-sonnet", 200000},
		{"gpt-4o", 128000},
		{"gpt-4", 8192},
		{"o1", 200000},
		{"glm-5", 128000},
	}
	for _, tt := range tests {
		got := ContextWindow(tt.model)
		if got != tt.want {
			t.Errorf("ContextWindow(%q) = %d, want %d", tt.model, got, tt.want)
		}
	}
}

func TestContextWindow_UnknownModel(t *testing.T) {
	got := ContextWindow("nonexistent-model-v1")
	if got != 128000 {
		t.Errorf("ContextWindow(unknown) = %d, want 128000", got)
	}
}
