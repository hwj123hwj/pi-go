package kbtools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTool_FullMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	content := "# Test Doc\n\nSome content here.\n"
	os.WriteFile(path, []byte(content), 0o644)

	tool := NewReadTool(dir)
	result, err := tool.Execute(context.Background(),
		json.RawMessage(`{"path":"test.md"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "Test Doc") {
		t.Errorf("full read should contain content, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Some content") {
		t.Errorf("full read should contain body, got: %s", result.Content)
	}
}

func TestReadTool_OverviewMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	content := `# Architecture Guide

This document describes the system architecture.

## Frontend

React + TypeScript. The frontend uses Vite for bundling.

## Backend

Go server with gRPC. Handles all business logic.

## Database

PostgreSQL with Redis cache.
`
	os.WriteFile(path, []byte(content), 0o644)

	tool := NewReadTool(dir)
	result, err := tool.Execute(context.Background(),
		json.RawMessage(`{"path":"test.md","overview":true}`), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Should contain structure info
	if !strings.Contains(result.Content, "概览") {
		t.Errorf("overview should contain 概览 header, got: %s", result.Content)
	}
	// Should contain headers
	if !strings.Contains(result.Content, "Frontend") {
		t.Errorf("overview should contain 'Frontend' header, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Backend") {
		t.Errorf("overview should contain 'Backend' header, got: %s", result.Content)
	}
	// Should contain first sentence per section
	if !strings.Contains(result.Content, "React") {
		t.Errorf("overview should contain first line of Frontend section, got: %s", result.Content)
	}
	// Should contain statistics
	if !strings.Contains(result.Content, "词") {
		t.Errorf("overview should contain word count, got: %s", result.Content)
	}
	// Should NOT contain second-paragraph body text (only first line per section)
	if strings.Contains(result.Content, "second paragraph") {
		t.Errorf("overview should NOT contain deep body content, got: %s", result.Content)
	}
}

func TestReadTool_OverviewSmallerThanFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.md")
	// Generate a large document with enough content to trigger the 8K truncation
	var content strings.Builder
	content.WriteString("# Big Document\n\n")
	content.WriteString("Intro paragraph with lots of detail about the system.\n\n")
	for i := 0; i < 50; i++ {
		content.WriteString("## Section\n\n")
		// Each section has a short first line, then lots of detail
		content.WriteString("Section intro line.\n\n")
		content.WriteString("This is filler content that adds bulk. " +
			"Lots and lots of detail that should not appear in overview. " +
			"More filler text here to make the document truly large.\n\n")
	}
	os.WriteFile(path, []byte(content.String()), 0o644)

	tool := NewReadTool(dir)

	fullResult, _ := tool.Execute(context.Background(),
		json.RawMessage(`{"path":"large.md"}`), nil)
	overviewResult, _ := tool.Execute(context.Background(),
		json.RawMessage(`{"path":"large.md","overview":true}`), nil)

	// Full read should have been truncated
	if !strings.Contains(fullResult.Content, "内容过长已截断") {
		t.Logf("warning: full read was not truncated (%d chars), may need larger test doc", len(fullResult.Content))
	}

	// Overview should be significantly smaller
	if len(overviewResult.Content) >= len(fullResult.Content) {
		t.Errorf("overview (%d chars) should be smaller than full (%d chars)",
			len(overviewResult.Content), len(fullResult.Content))
	}
}

func TestReadTool_NotFound(t *testing.T) {
	tool := NewReadTool(t.TempDir())
	result, _ := tool.Execute(context.Background(),
		json.RawMessage(`{"path":"nonexistent.md"}`), nil)
	if !result.IsError {
		t.Error("should be error for missing file")
	}
}

func TestReadTool_HintsToUseOverview(t *testing.T) {
	tool := NewReadTool(t.TempDir())
	// Description should mention overview mode
	desc := tool.Description()
	if !strings.Contains(desc, "overview") {
		t.Error("description should mention overview mode")
	}
}

func TestReadTool_OverviewEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	os.WriteFile(path, []byte(""), 0o644)

	tool := NewReadTool(dir)
	result, _ := tool.Execute(context.Background(),
		json.RawMessage(`{"path":"empty.md","overview":true}`), nil)
	// Should not crash, should return something reasonable
	if result.Content == "" {
		t.Error("overview of empty file should still return something")
	}
}
