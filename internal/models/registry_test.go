package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	m := ModelDef{ID: "test-model", Provider: "openai", Name: "Test Model", ContextWindow: 128000}
	r.Register(m)

	got, ok := r.Get("test-model")
	if !ok {
		t.Fatal("Expected to find registered model")
	}
	if got.Name != "Test Model" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Model")
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("Expected not to find unregistered model")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	r.Register(ModelDef{ID: "b-model", Provider: "anthropic", Name: "B Model"})
	r.Register(ModelDef{ID: "a-model", Provider: "anthropic", Name: "A Model"})
	r.Register(ModelDef{ID: "c-model", Provider: "openai", Name: "C Model"})
	r.Register(ModelDef{ID: "hidden-model", Provider: "openai", Name: "Hidden", Hidden: true})

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("Expected 3 visible models, got %d", len(list))
	}

	// Should be sorted by provider then name
	if list[0].ID != "a-model" {
		t.Errorf("First model should be a-model, got %s", list[0].ID)
	}
	if list[1].ID != "b-model" {
		t.Errorf("Second model should be b-model, got %s", list[1].ID)
	}
	if list[2].ID != "c-model" {
		t.Errorf("Third model should be c-model, got %s", list[2].ID)
	}
}

func TestRegistryListByProvider(t *testing.T) {
	r := NewRegistry()
	r.Register(ModelDef{ID: "a1", Provider: "anthropic", Name: "A1"})
	r.Register(ModelDef{ID: "a2", Provider: "anthropic", Name: "A2"})
	r.Register(ModelDef{ID: "o1", Provider: "openai", Name: "O1"})

	anthropicModels := r.ListByProvider("anthropic")
	if len(anthropicModels) != 2 {
		t.Fatalf("Expected 2 anthropic models, got %d", len(anthropicModels))
	}

	openaiModels := r.ListByProvider("openai")
	if len(openaiModels) != 1 {
		t.Fatalf("Expected 1 openai model, got %d", len(openaiModels))
	}

	emptyModels := r.ListByProvider("nonexistent")
	if len(emptyModels) != 0 {
		t.Errorf("Expected 0 models for unknown provider, got %d", len(emptyModels))
	}
}

func TestRegistryDefault(t *testing.T) {
	r := NewRegistry()
	r.Register(ModelDef{ID: "m1", Provider: "openai", Name: "M1"})
	r.Register(ModelDef{ID: "m2", Provider: "openai", Name: "M2"})

	def, ok := r.Default("openai")
	if !ok {
		t.Fatal("Expected to find a default model")
	}
	// Default should be one of the models
	if def.Provider != "openai" {
		t.Errorf("Default provider = %q, want openai", def.Provider)
	}

	_, ok = r.Default("nonexistent")
	if ok {
		t.Error("Expected false for unknown provider")
	}
}

func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "models.json")

	jsonContent := `[
		{"id": "custom-model-1", "provider": "openai", "name": "Custom 1", "context_window": 64000},
		{"id": "custom-model-2", "provider": "anthropic", "name": "Custom 2", "context_window": 200000}
	]`
	if err := os.WriteFile(path, []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("Write test file: %v", err)
	}

	r := NewRegistry()
	if err := r.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	m, ok := r.Get("custom-model-1")
	if !ok {
		t.Fatal("Expected to find custom-model-1")
	}
	if m.ContextWindow != 64000 {
		t.Errorf("ContextWindow = %d, want 64000", m.ContextWindow)
	}

	m2, ok := r.Get("custom-model-2")
	if !ok {
		t.Fatal("Expected to find custom-model-2")
	}
	if m2.Provider != "anthropic" {
		t.Errorf("Provider = %q, want anthropic", m2.Provider)
	}
}

func TestLoadFromFileSkipsInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "models.json")

	// Entries with empty ID or provider should be skipped
	jsonContent := `[
		{"id": "", "provider": "openai", "name": "Invalid 1"},
		{"id": "valid", "provider": "", "name": "Invalid 2"},
		{"id": "good", "provider": "openai", "name": "Good"}
	]`
	os.WriteFile(path, []byte(jsonContent), 0o644)

	r := NewRegistry()
	r.LoadFromFile(path)

	if _, ok := r.Get("good"); !ok {
		t.Error("Expected 'good' model to be registered")
	}
	if _, ok := r.Get("valid"); ok {
		t.Error("Expected 'valid' to be skipped (empty provider)")
	}
}

func TestDefaultModels(t *testing.T) {
	defs := DefaultModels()
	if len(defs) < 5 {
		t.Errorf("Expected at least 5 default models, got %d", len(defs))
	}

	// Verify Claude and GPT models are present
	hasClaude := false
	hasGPT := false
	for _, m := range defs {
		if m.ID == "claude-sonnet-4-6" {
			hasClaude = true
		}
		if m.ID == "gpt-4o" {
			hasGPT = true
		}
	}
	if !hasClaude {
		t.Error("Default models should include claude-sonnet-4-6")
	}
	if !hasGPT {
		t.Error("Default models should include gpt-4o")
	}
}

func TestNewDefaultRegistry(t *testing.T) {
	r := NewDefaultRegistry("")
	list := r.List()
	if len(list) < 5 {
		t.Errorf("Expected at least 5 models in default registry, got %d", len(list))
	}
}

func TestNewDefaultRegistryWithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "models.json")

	jsonContent := `[{"id": "my-custom", "provider": "openai", "name": "My Custom", "context_window": 32000}]`
	os.WriteFile(path, []byte(jsonContent), 0o644)

	r := NewDefaultRegistry(path)

	// Should have both defaults and the custom model
	_, ok := r.Get("my-custom")
	if !ok {
		t.Error("Expected custom model from config file")
	}
	_, ok = r.Get("claude-sonnet-4-6")
	if !ok {
		t.Error("Expected default model still present")
	}
}

func TestFormatForDisplay(t *testing.T) {
	m := ModelDef{ID: "test", Provider: "openai", Name: "Test", ContextWindow: 128000}
	s := m.FormatForDisplay()
	if s == "" {
		t.Error("FormatForDisplay should not return empty string")
	}
}

func TestProviderSummary(t *testing.T) {
	r := NewRegistry()
	r.Register(ModelDef{ID: "m1", Provider: "anthropic", Name: "M1"})
	r.Register(ModelDef{ID: "m2", Provider: "anthropic", Name: "M2"})
	r.Register(ModelDef{ID: "m3", Provider: "openai", Name: "M3"})

	summary := r.ProviderSummary()
	if summary == "" {
		t.Error("ProviderSummary should not be empty")
	}
}
