package kbtools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// createTestRepo creates a temporary knowledge base repo with sample entries.
func createTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create issues directory with a legacy-format entry
	issuesDir := filepath.Join(tmpDir, "issues")
	os.MkdirAll(issuesDir, 0755)

	os.WriteFile(filepath.Join(issuesDir, "2026-05-05-cron-env.md"), []byte(`# Cron 环境变量不加载

**项目**: OpenClaw
**日期**: 2026-05-05
**Tags**: Linux, cron, 踩坑

## 遇到什么问题

crontab 里设置的定时任务一直报 command not found，
手动跑却没问题。

## 怎么解决的

cron 默认不加载 .bashrc / .profile，需要在脚本里
显式 source 或写绝对路径。
`), 0644)

	// Create tech directory with frontmatter format
	techDir := filepath.Join(tmpDir, "tech")
	os.MkdirAll(techDir, 0755)

	os.WriteFile(filepath.Join(techDir, "go-context.md"), []byte(`---
title: "Go Context 详解"
date: 2026-04-10
category: tech
tags: ["Go", "并发", "Context"]
---

# Go Context 详解

Go 的 context 包用于控制 goroutine 生命周期、传递请求范围的值。

## 核心模式

context.WithCancel 用于主动取消，WithTimeout 用于超时控制。
`), 0644)

	os.WriteFile(filepath.Join(techDir, "react-hooks.md"), []byte(`---
title: "React Hooks 使用指南"
date: 2026-03-15
category: tech
tags: ["React", "Hooks", "JavaScript"]
---

# React Hooks 使用指南

React Hooks 是 React 16.8 引入的新特性。
`), 0644)

	// Create a life category entry
	lifeDir := filepath.Join(tmpDir, "life")
	os.MkdirAll(lifeDir, 0755)

	os.WriteFile(filepath.Join(lifeDir, "cooking-tips.md"), []byte(`# 烹饪技巧

## 炒菜不粘锅

热锅冷油，先把锅烧热再倒油。
`), 0644)

	return tmpDir
}

func TestSearchTool_SearchByQuery(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewSearchTool(repoPath)

	params := json.RawMessage(`{"query": "cron"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}
	if !contains(result.Content, "Cron") {
		t.Errorf("Expected to find 'Cron' in results, got:\n%s", result.Content)
	}
	t.Logf("Search result:\n%s", result.Content)
}

func TestSearchTool_SearchByCategory(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewSearchTool(repoPath)

	params := json.RawMessage(`{"category": "tech"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}
	if !contains(result.Content, "Go Context") {
		t.Errorf("Expected to find 'Go Context' in results")
	}
	if !contains(result.Content, "React Hooks") {
		t.Errorf("Expected to find 'React Hooks' in results")
	}
	if contains(result.Content, "烹饪") {
		t.Errorf("Should not find cooking entries in tech category")
	}
	t.Logf("Category search result:\n%s", result.Content)
}

func TestSearchTool_SearchByTag(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewSearchTool(repoPath)

	params := json.RawMessage(`{"tag": "Go"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}
	if !contains(result.Content, "Go Context") {
		t.Errorf("Expected to find 'Go Context' entry with Go tag")
	}
	t.Logf("Tag search result:\n%s", result.Content)
}

func TestSearchTool_NoResults(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewSearchTool(repoPath)

	params := json.RawMessage(`{"query": "nonexistent"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !contains(result.Content, "没有找到") {
		t.Errorf("Expected 'no results' message, got: %s", result.Content)
	}
}

func TestListTool_All(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewListTool(repoPath)

	params := json.RawMessage(`{}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}
	// Should list all entries
	if !contains(result.Content, "Cron") {
		t.Errorf("Expected 'Cron' entry in list")
	}
	if !contains(result.Content, "Go Context") {
		t.Errorf("Expected 'Go Context' entry in list")
	}
	if !contains(result.Content, "烹饪") {
		t.Errorf("Expected cooking entry in list")
	}
	t.Logf("List result:\n%s", result.Content)
}

func TestListTool_ByCategory(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewListTool(repoPath)

	params := json.RawMessage(`{"category": "life"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !contains(result.Content, "烹饪") {
		t.Errorf("Expected cooking entry in life category")
	}
	if contains(result.Content, "Go Context") {
		t.Errorf("Should not find tech entries in life category")
	}
	t.Logf("List by category result:\n%s", result.Content)
}

func TestSaveTool_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewSaveTool(tmpDir)

	params := json.RawMessage(`{
		"title": "Test Knowledge Entry",
		"content": "This is a test entry about testing.",
		"tags": ["test", "unit-test"],
		"category": "tech"
	}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}

	// Verify the file was created
	entries, _ := os.ReadDir(filepath.Join(tmpDir, "tech"))
	if len(entries) != 1 {
		t.Fatalf("Expected 1 file in tech/, got %d", len(entries))
	}

	// Read back the file
	data, _ := os.ReadFile(filepath.Join(tmpDir, "tech", entries[0].Name()))
	if !contains(string(data), "Test Knowledge Entry") {
		t.Errorf("File content doesn't contain title")
	}
	if !contains(string(data), "This is a test entry") {
		t.Errorf("File content doesn't contain body")
	}
	t.Logf("Save result:\n%s", result.Content)
}

func TestSaveTool_ChineseTitle(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewSaveTool(tmpDir)

	params := json.RawMessage(`{
		"title": "Go 并发编程踩坑",
		"content": "goroutine 泄漏的常见原因。",
		"tags": ["Go", "并发"]
	}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}

	// Verify file was created (should use hash slug)
	entries, _ := os.ReadDir(filepath.Join(tmpDir, "issues"))
	if len(entries) != 1 {
		t.Fatalf("Expected 1 file in issues/, got %d", len(entries))
	}

	// Read back and verify
	data, _ := os.ReadFile(filepath.Join(tmpDir, "issues", entries[0].Name()))
	if !contains(string(data), "Go 并发编程踩坑") {
		t.Errorf("File content doesn't contain Chinese title")
	}
	t.Logf("Save result:\n%s", result.Content)
}

func TestIndex_ParsesBothFormats(t *testing.T) {
	repoPath := createTestRepo(t)
	idx, err := GetIndex(repoPath)
	if err != nil {
		t.Fatalf("GetIndex failed: %v", err)
	}
	if len(idx.Entries) != 4 {
		t.Fatalf("Expected 4 entries, got %d", len(idx.Entries))
	}

	// Find the cron entry (legacy format)
	var cronEntry *Entry
	for i := range idx.Entries {
		if contains(idx.Entries[i].Title, "Cron") {
			cronEntry = &idx.Entries[i]
			break
		}
	}
	if cronEntry == nil {
		t.Fatal("Cron entry not found")
	}
	if cronEntry.Category != "issues" {
		t.Errorf("Expected category 'issues', got '%s'", cronEntry.Category)
	}
	// Should have extracted tags from **Tags**: line
	foundLinuxTag := false
	for _, tag := range cronEntry.Tags {
		if tag == "Linux" || tag == "cron" {
			foundLinuxTag = true
		}
	}
	if !foundLinuxTag {
		t.Errorf("Expected to find Linux/cron tag, got tags: %v", cronEntry.Tags)
	}

	// Find the Go Context entry (frontmatter format)
	var goEntry *Entry
	for i := range idx.Entries {
		if contains(idx.Entries[i].Title, "Go Context") {
			goEntry = &idx.Entries[i]
			break
		}
	}
	if goEntry == nil {
		t.Fatal("Go Context entry not found")
	}
	if len(goEntry.Tags) != 3 {
		t.Errorf("Expected 3 tags from frontmatter, got %d: %v", len(goEntry.Tags), goEntry.Tags)
	}
}

func TestReadTool_RelativePath(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewReadTool(repoPath)

	params := json.RawMessage(`{"path": "issues/2026-05-05-cron-env.md"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}
	if !contains(result.Content, "Cron") {
		t.Errorf("Expected content to contain 'Cron'")
	}
}

// contains is a helper for substring check.
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
