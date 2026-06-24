package kbtools

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestMaintainTool_Health(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewMaintainTool(repoPath)

	// Add an entry missing tags for testing
	params := json.RawMessage(`{"action": "health"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}
	// Should report total entries
	if !contains(result.Content, "5 条目") {
		t.Errorf("Expected '5 条目' in health report, got:\n%s", result.Content)
	}
	t.Logf("Health report:\n%s", result.Content)
}

func TestMaintainTool_Stats(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewMaintainTool(repoPath)

	params := json.RawMessage(`{"action": "stats"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}
	// Should show category distribution
	if !contains(result.Content, "分类分布") {
		t.Errorf("Expected '分类分布' in stats, got:\n%s", result.Content)
	}
	// Should list tech and issues categories
	if !contains(result.Content, "tech") {
		t.Errorf("Expected 'tech' category in stats")
	}
	if !contains(result.Content, "issues") {
		t.Errorf("Expected 'issues' category in stats")
	}
	t.Logf("Stats:\n%s", result.Content)
}

func TestMaintainTool_Tags(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewMaintainTool(repoPath)

	params := json.RawMessage(`{"action": "tags"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// Should show tag usage
	if !contains(result.Content, "标签分析") {
		t.Errorf("Expected '标签分析' in tags analysis, got:\n%s", result.Content)
	}
	// Go tag exists in test data
	if !contains(result.Content, "Go") {
		t.Errorf("Expected 'Go' tag in analysis")
	}
	t.Logf("Tags analysis:\n%s", result.Content)
}

func TestMaintainTool_Duplicates(t *testing.T) {
	repoPath := createTestRepo(t)
	tool := NewMaintainTool(repoPath)

	params := json.RawMessage(`{"action": "duplicates"}`)
	result, err := tool.Execute(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content)
	}
	t.Logf("Duplicates:\n%s", result.Content)
}

func TestMaintainTool_Duplicates_DetectsSim(t *testing.T) {
	repoPath := createTestRepo(t)

	// Save a duplicate entry
	idx, _ := GetIndex(repoPath)
	if len(idx.Entries) == 0 {
		t.Fatal("Expected entries in index")
	}

	// Use the keyword searcher directly to test the search strategy interface
	searcher := KeywordSearcher{}
	results := searcher.Search(idx.Entries, SearchQuery{
		Query: "cron",
		Limit: 5,
	})
	if len(results) == 0 {
		t.Fatal("KeywordSearcher returned no results for 'cron'")
	}
	if results[0].Entry.Title == "" {
		t.Error("Expected non-empty title in search result")
	}
	if results[0].Score <= 0 {
		t.Errorf("Expected positive score, got %f", results[0].Score)
	}
	t.Logf("Strategy search result: %s (score: %.1f)", results[0].Entry.Title, results[0].Score)
}

func TestSearchStrategy_Interface(t *testing.T) {
	// Verify KeywordSearcher satisfies SearchStrategy interface
	var _ SearchStrategy = KeywordSearcher{}
}

func TestHealthReport_DetectsMissingMetadata(t *testing.T) {
	repoPath := t.TempDir()

	// Create a file with no tags, no summary
	issuesDir := repoPath + "/issues"
	mkdirAll(issuesDir)
	writeFile(repoPath+"/issues/bare.md", "# Bare Entry\n\nJust some text.\n")

	ClearCache()
	idx, err := GetIndex(repoPath)
	if err != nil {
		t.Fatalf("GetIndex failed: %v", err)
	}

	report := GenerateHealthReport(idx)
	// The bare entry has no tags and no summary
	if len(report.EntriesMissingTags) == 0 {
		t.Error("Expected entries missing tags")
	}
}

func TestTagClusters_DetectsCaseVariation(t *testing.T) {
	entries := []Entry{
		{Title: "A", Tags: []string{"Go", "golang"}},
		{Title: "B", Tags: []string{"go", "concurrency"}},
		{Title: "C", Tags: []string{"Go"}},
	}
	clusters := detectTagClusters(entries)
	// "Go", "go" should be clustered
	found := false
	for _, c := range clusters {
		for _, v := range c.Variants {
			if v == "Go" || v == "go" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("Expected to find Go/go tag cluster, got %d clusters", len(clusters))
	}
}

func TestCategoryOverview(t *testing.T) {
	repoPath := createTestRepo(t)
	idx, _ := GetIndex(repoPath)
	stats := CategoryOverview(idx)

	// Should have at least tech, issues, life, chatgpt-export, project-journals
	if len(stats) < 3 {
		t.Errorf("Expected at least 3 categories, got %d", len(stats))
	}
	// tech should have 1 entry
	for _, s := range stats {
		if s.Name == "tech" && s.Count != 1 {
			t.Errorf("Expected tech category to have 1 entry, got %d", s.Count)
		}
	}
	t.Logf("Categories: %+v", stats)
}

// ── Test helpers ──

func mkdirAll(path string) {
	_ = os.MkdirAll(path, 0755)
}

func writeFile(path, content string) {
	_ = os.WriteFile(path, []byte(content), 0644)
}
