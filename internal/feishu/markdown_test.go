package feishu

import (
	"encoding/json"
	"testing"
)

func TestMdToPostContent_PlainText(t *testing.T) {
	result := mdToPostContent("hello world")
	var post map[string]any
	if err := json.Unmarshal([]byte(result), &post); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Should be valid JSON with zh_cn key
	zhCn, ok := post["zh_cn"].(map[string]any)
	if !ok {
		t.Fatal("missing zh_cn key")
	}
	content, ok := zhCn["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatal("missing or empty content")
	}
}

func TestMdToPostContent_Bold(t *testing.T) {
	result := mdToPostContent("this is **bold** text")
	var post map[string]any
	json.Unmarshal([]byte(result), &post)

	// Should contain a styled element
	encoded, _ := json.Marshal(post)
	if !contains(encoded, "bold") {
		t.Errorf("expected bold style in output, got: %s", string(encoded))
	}
}

func TestMdToPostContent_InlineCode(t *testing.T) {
	result := mdToPostContent("use `code` here")
	var post map[string]any
	json.Unmarshal([]byte(result), &post)

	encoded, _ := json.Marshal(post)
	if !contains(encoded, "background") {
		t.Errorf("expected background style for inline code, got: %s", string(encoded))
	}
}

func TestMdToPostContent_Heading(t *testing.T) {
	result := mdToPostContent("# Title\n## Subtitle")
	var post map[string]any
	json.Unmarshal([]byte(result), &post)

	encoded, _ := json.Marshal(post)
	if !contains(encoded, "#") || !contains(encoded, "##") {
		t.Errorf("expected heading markers in output, got: %s", string(encoded))
	}
}

func TestMdToPostContent_Link(t *testing.T) {
	result := mdToPostContent("[click here](https://example.com)")
	var post map[string]any
	json.Unmarshal([]byte(result), &post)

	encoded, _ := json.Marshal(post)
	if !contains(encoded, "https://example.com") {
		t.Errorf("expected link href in output, got: %s", string(encoded))
	}
}

func TestMdToPostContent_CodeBlock(t *testing.T) {
	result := mdToPostContent("before\n```go\nfmt.Println(\"hi\")\n```\nafter")
	var post map[string]any
	json.Unmarshal([]byte(result), &post)

	encoded, _ := json.Marshal(post)
	if !contains(encoded, "fmt.Println") {
		t.Errorf("expected code block content in output, got: %s", string(encoded))
	}
}

func TestMdToPostContent_Table(t *testing.T) {
	result := mdToPostContent("| Name | Value |\n|---|---|\n| foo | bar |")
	var post map[string]any
	json.Unmarshal([]byte(result), &post)

	encoded, _ := json.Marshal(post)
	if !contains(encoded, "foo") || !contains(encoded, "bar") {
		t.Errorf("expected table content in output, got: %s", string(encoded))
	}
}

func TestMdToPostContent_List(t *testing.T) {
	result := mdToPostContent("- item 1\n- item 2")
	var post map[string]any
	json.Unmarshal([]byte(result), &post)

	encoded, _ := json.Marshal(post)
	if !contains(encoded, "item 1") || !contains(encoded, "item 2") {
		t.Errorf("expected list items in output, got: %s", string(encoded))
	}
}

func TestParseInlineMarkdown_PlainText(t *testing.T) {
	blocks := parseInlineMarkdown("plain text")
	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}
}

func contains(data []byte, substr string) bool {
	return string(data) != "" && len(data) > 0 &&
		(len(data) >= len(substr) && stringContains(string(data), substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
