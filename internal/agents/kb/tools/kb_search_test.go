package kbtools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchTool_SearchByQuery(t *testing.T) {
	// Create a temporary test repo structure
	tmpDir := t.TempDir()
	kbDir := filepath.Join(tmpDir, "doubao-knowledge")
	techDir := filepath.Join(kbDir, "tech")
	os.MkdirAll(techDir, 0755)

	// Create a test tags-index.json
	index := `{
		"total": 2,
		"cards": [
			{
				"file": "tech/React-Hooks.md",
				"title": "React Hooks 使用指南",
				"tags": ["React", "Hooks", "JavaScript"],
				"summary": "React Hooks 是 React 16.8 引入的新特性",
				"category": "tech"
			},
			{
				"file": "tech/Go-Context.md",
				"title": "Go Context 详解",
				"tags": ["Go", "Context", "并发"],
				"summary": "Go 的 context 包用于控制 goroutine 的生命周期",
				"category": "tech"
			}
		]
	}`
	os.WriteFile(filepath.Join(kbDir, "tags-index.json"), []byte(index), 0644)

	// Create tool
	tool := NewSearchTool(tmpDir)

	// Test search by query
	params := json.RawMessage(`{"query": "React"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}
	if result.Content == "" {
		t.Fatal("Expected non-empty content")
	}
	t.Logf("Search result:\n%s", result.Content)
}

func TestSearchTool_SearchByCategory(t *testing.T) {
	tmpDir := t.TempDir()
	kbDir := filepath.Join(tmpDir, "doubao-knowledge")
	os.MkdirAll(kbDir, 0755)

	index := `{
		"total": 2,
		"cards": [
			{
				"file": "tech/React.md",
				"title": "React",
				"tags": ["React"],
				"summary": "React 框架",
				"category": "tech"
			},
			{
				"file": "life/Cooking.md",
				"title": "烹饪技巧",
				"tags": ["生活"],
				"summary": "烹饪小技巧",
				"category": "life"
			}
		]
	}`
	os.WriteFile(filepath.Join(kbDir, "tags-index.json"), []byte(index), 0644)

	tool := NewSearchTool(tmpDir)
	params := json.RawMessage(`{"category": "life"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}
	if result.Content == "" {
		t.Fatal("Expected non-empty content")
	}
	t.Logf("Category search result:\n%s", result.Content)
}
