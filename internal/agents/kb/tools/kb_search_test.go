package kbtools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createTestRepo creates a temporary knowledge base repo with sample entries
// in multiple formats found in the real agent-lessons repo.
func createTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// ── Format 1: YAML frontmatter (issues/) ──
	issuesDir := filepath.Join(tmpDir, "issues")
	os.MkdirAll(issuesDir, 0755)

	os.WriteFile(filepath.Join(issuesDir, "2026-05-05-cron-env.md"), []byte(`---
project: OpenClaw
date: 2026-05-05
tags: [Linux, cron, 踩坑]
status: solved
---

# Cron 环境变量不加载

## 遇到什么问题

crontab 里设置的定时任务一直报 command not found，
手动跑却没问题。

## 怎么解决的

cron 默认不加载 .bashrc / .profile，需要在脚本里
显式 source 或写绝对路径。
`), 0644)

	// ── Format 2: doubao-knowledge card ──
	techDir := filepath.Join(tmpDir, "doubao-knowledge", "tech")
	os.MkdirAll(techDir, 0755)

	os.WriteFile(filepath.Join(techDir, "go-context.md"), []byte(`# Go Context 详解

> 来源：ChatGPT 对话 | 2026-04-10 | 11条消息
> 分类：💻 技术/编程

## 关键要点
- context.WithCancel 用于主动取消
- WithTimeout 用于超时控制

## 摘要
Go 的 context 包用于控制 goroutine 生命周期、传递请求范围的值。

## 标签
`+"`Go` `并发` `Context`"+`

`), 0644)

	// ── Format 3: chatgpt-export conversation ──
	exportDir := filepath.Join(tmpDir, "chatgpt-export")
	os.MkdirAll(exportDir, 0755)

	os.WriteFile(filepath.Join(exportDir, "React_Hooks.md"), []byte(`# React Hooks 使用指南

> URL: https://chatgpt.com/c/xxx
> Created: 2026-03-15

## 👤 User

What are React Hooks?

## 🤖 ChatGPT

React Hooks 是 React 16.8 引入的新特性，允许在函数组件中使用 state。
`), 0644)

	// ── Format 4: doubao-knowledge/life (tags as comma-separated) ──
	lifeDir := filepath.Join(tmpDir, "doubao-knowledge", "life")
	os.MkdirAll(lifeDir, 0755)

	os.WriteFile(filepath.Join(lifeDir, "cooking-tips.md"), []byte(`# 烹饪技巧

> 来源：豆包对话 | 2026-05-01 | 2条消息
> 分类：🏠 生活/常识

## 关键要点
- 热锅冷油，先把锅烧热再倒油

## 摘要
炒菜不粘锅的秘诀是热锅冷油。

## 标签
烹饪, 厨房技巧, 生活常识
`), 0644)

	// ── Files that should be skipped ──
	dkRoot := filepath.Join(tmpDir, "doubao-knowledge")
	os.WriteFile(filepath.Join(dkRoot, "INDEX.md"), []byte("# Index\n"), 0644)
	os.WriteFile(filepath.Join(dkRoot, "tags-index.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Repo\n"), 0644)

	// ── project-journals ──
	pjDir := filepath.Join(tmpDir, "project-journals", "pi-go")
	os.MkdirAll(pjDir, 0755)
	os.WriteFile(filepath.Join(pjDir, "journal.md"), []byte(`# pi-go 开发日志

> 自动生成于 2026-06-24

## 2026-06-07

### 🎯 目标
- 深度对比 pi-go 架构
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

func TestSearchTool_SearchDoubaoKnowledge(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewSearchTool(repoPath)

	params := json.RawMessage(`{"query": "Context"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}
	if !contains(result.Content, "Go Context") {
		t.Errorf("Expected to find 'Go Context' (doubao-knowledge format), got:\n%s", result.Content)
	}
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
	if contains(result.Content, "烹饪") {
		t.Errorf("Should not find cooking entries in tech category")
	}
}

func TestSearchTool_SearchByTag(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewSearchTool(repoPath)

	params := json.RawMessage(`{"tag": "Go"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !contains(result.Content, "Go Context") {
		t.Errorf("Expected to find 'Go Context' entry with Go tag, got:\n%s", result.Content)
	}
}

func TestSearchTool_SearchByTag_CommaSeparated(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewSearchTool(repoPath)

	// The cooking entry uses comma-separated tags: 烹饪, 厨房技巧, 生活常识
	params := json.RawMessage(`{"tag": "烹饪"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !contains(result.Content, "烹饪技巧") {
		t.Errorf("Expected to find cooking entry with 烹饪 tag, got:\n%s", result.Content)
	}
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
	if !contains(result.Content, "Cron") {
		t.Errorf("Expected 'Cron' entry in list")
	}
	if !contains(result.Content, "Go Context") {
		t.Errorf("Expected 'Go Context' entry in list")
	}
	if !contains(result.Content, "烹饪") {
		t.Errorf("Expected cooking entry in list")
	}
	// Should skip INDEX.md and README.md
	if contains(result.Content, "Index") && strings.Contains(result.Content, "Index\n") {
		t.Errorf("Should not include INDEX.md")
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
		t.Errorf("Expected cooking entry in life category, got:\n%s", result.Content)
	}
	if contains(result.Content, "Go Context") {
		t.Errorf("Should not find tech entries in life category")
	}
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

	entries, _ := os.ReadDir(filepath.Join(tmpDir, "tech"))
	if len(entries) != 1 {
		t.Fatalf("Expected 1 file in tech/, got %d", len(entries))
	}

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

	entries, _ := os.ReadDir(filepath.Join(tmpDir, "issues"))
	if len(entries) != 1 {
		t.Fatalf("Expected 1 file in issues/, got %d", len(entries))
	}

	data, _ := os.ReadFile(filepath.Join(tmpDir, "issues", entries[0].Name()))
	if !contains(string(data), "Go 并发编程踩坑") {
		t.Errorf("File content doesn't contain Chinese title")
	}
	t.Logf("Save result:\n%s", result.Content)
}

func TestIndex_ParsesAllFormats(t *testing.T) {
	repoPath := createTestRepo(t)
	ClearCache()
	idx, err := GetIndex(repoPath)
	if err != nil {
		t.Fatalf("GetIndex failed: %v", err)
	}
	// Should have 5 knowledge entries (skipping INDEX.md, tags-index.json, README.md)
	if len(idx.Entries) != 5 {
		t.Fatalf("Expected 5 entries (got %d):", len(idx.Entries))
		for _, e := range idx.Entries {
			t.Logf("  - %s [%s]", e.Title, e.RelPath)
		}
	}

	// Check cron entry (frontmatter format)
	cronEntry := findEntryByTitle(idx, "Cron")
	if cronEntry == nil {
		t.Fatal("Cron entry not found")
	}
	if cronEntry.Category != "issues" {
		t.Errorf("Expected category 'issues', got '%s'", cronEntry.Category)
	}
	if !sliceContains(cronEntry.Tags, "Linux") {
		t.Errorf("Expected Linux tag, got: %v", cronEntry.Tags)
	}

	// Check Go Context entry (doubao-knowledge format)
	goEntry := findEntryByTitle(idx, "Go Context")
	if goEntry == nil {
		t.Fatal("Go Context entry not found")
	}
	if goEntry.Category != "tech" {
		t.Errorf("Expected category 'tech', got '%s'", goEntry.Category)
	}
	if !sliceContains(goEntry.Tags, "Go") || !sliceContains(goEntry.Tags, "并发") {
		t.Errorf("Expected Go and 并发 tags, got: %v", goEntry.Tags)
	}
	if goEntry.Source == "" {
		t.Errorf("Expected source line from > 来源 metadata")
	}

	// Check cooking entry (comma-separated tags format)
	cookEntry := findEntryByTitle(idx, "烹饪")
	if cookEntry == nil {
		t.Fatal("Cooking entry not found")
	}
	if !sliceContains(cookEntry.Tags, "烹饪") || !sliceContains(cookEntry.Tags, "生活常识") {
		t.Errorf("Expected 烹饪 and 生活常识 tags, got: %v", cookEntry.Tags)
	}
}

func TestIndex_SkipsIndexFiles(t *testing.T) {
	repoPath := createTestRepo(t)
	ClearCache()
	idx, _ := GetIndex(repoPath)
	for _, e := range idx.Entries {
		if e.RelPath == "INDEX.md" || e.RelPath == "doubao-knowledge/INDEX.md" {
			t.Errorf("INDEX.md should be skipped, but found at %s", e.RelPath)
		}
		if e.RelPath == "README.md" {
			t.Errorf("README.md should be skipped")
		}
		if strings.HasSuffix(e.RelPath, "tags-index.json") {
			t.Errorf("tags-index.json should be skipped")
		}
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

// ── Helpers ──

func findEntryByTitle(idx *Index, titlePart string) *Entry {
	for i := range idx.Entries {
		if contains(idx.Entries[i].Title, titlePart) {
			return &idx.Entries[i]
		}
	}
	return nil
}

func sliceContains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
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
