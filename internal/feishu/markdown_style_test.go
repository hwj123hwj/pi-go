package feishu

import (
	"strings"
	"testing"
)

func TestOptimizeMarkdownStyle_PreservesCodeBlocks(t *testing.T) {
	input := "**Result**\n````text\nbefore\n```\n# inside heading\n````\ntail"
	result := OptimizeMarkdownStyle(input, 1)
	if result != input {
		t.Errorf("code blocks with longer fences should be preserved.\ngot:  %q\nwant: %q", result, input)
	}
}

func TestOptimizeMarkdownStyle_HeadingDowngrade(t *testing.T) {
	input := "# Heading 1\nSome intro text.\n## Heading 2\nDetail description.\n### Heading 3\nMore details."
	result := OptimizeMarkdownStyle(input, 2)

	if !strings.Contains(result, "#### Heading 1") {
		t.Errorf("H1 should be downgraded to H4, got:\n%s", result)
	}
	if !strings.Contains(result, "##### Heading 2") {
		t.Errorf("H2 should be downgraded to H5, got:\n%s", result)
	}
	if !strings.Contains(result, "##### Heading 3") {
		t.Errorf("H3 should be downgraded to H5, got:\n%s", result)
	}
}

func TestOptimizeMarkdownStyle_NoDowngradeWithoutH1H3(t *testing.T) {
	input := "#### Existing H4\n##### Existing H5"
	result := OptimizeMarkdownStyle(input, 2)
	// Should NOT downgrade since there's no H1~H3 in original
	if !strings.Contains(result, "#### Existing H4") {
		t.Error("H4 should be preserved when no H1~H3 exists")
	}
	if !strings.Contains(result, "##### Existing H5") {
		t.Error("H5 should be preserved when no H1~H3 exists")
	}
}

func TestOptimizeMarkdownStyle_StripsInvalidImages(t *testing.T) {
	input := "Here is an image: ![logo](https://example.com/logo.png)\nAnd a valid one: ![img](img_v3_02vb_12345)"
	result := OptimizeMarkdownStyle(input, 2)

	if strings.Contains(result, "https://example.com/logo.png") {
		t.Error("should strip non-img_ image references")
	}
	if !strings.Contains(result, "![img](img_v3_02vb_12345)") {
		t.Error("should preserve valid img_ image references")
	}
}

func TestOptimizeMarkdownStyle_EmptyText(t *testing.T) {
	result := OptimizeMarkdownStyle("", 2)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestOptimizeMarkdownStyle_CompressesNewlines(t *testing.T) {
	input := "line1\n\n\n\n\nline2"
	result := OptimizeMarkdownStyle(input, 2)
	if strings.Count(result, "\n") > 2 {
		t.Errorf("should compress 3+ newlines to 2, got:\n%q", result)
	}
}

func TestOptimizeMarkdownStyle_TableBrSpacing(t *testing.T) {
	input := "Some text\n| a | b |\n|---|---|\n| 1 | 2 |\nMore text"
	result := OptimizeMarkdownStyle(input, 2)

	// Should have <br> around the table
	if !strings.Contains(result, "<br>") {
		t.Errorf("expected <br> spacing around table, got:\n%s", result)
	}
}

func TestOptimizeMarkdownStyle_CodeBlockNotModified(t *testing.T) {
	input := "text\n```go\n# this is not a heading\n## neither is this\n```\ntail"
	result := OptimizeMarkdownStyle(input, 2)

	// Code block content should be unchanged
	if !strings.Contains(result, "# this is not a heading") {
		t.Error("headings inside code blocks should not be modified")
	}
	if !strings.Contains(result, "## neither is this") {
		t.Error("headings inside code blocks should not be modified")
	}
}

func TestStripInvalidImageKeys_InvalidURL(t *testing.T) {
	input := "![logo](https://example.com/logo.png)"
	got := StripInvalidImageKeys(input)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestStripInvalidImageKeys_ValidKey(t *testing.T) {
	input := "![img](img_v3_02vb_12345)"
	got := StripInvalidImageKeys(input)
	if got != input {
		t.Errorf("expected preserved, got %q", got)
	}
}

func TestStripInvalidImageKeys_Mixed(t *testing.T) {
	input := "text ![a](http://x.com/a.png) ![b](img_v3_xyz) end"
	got := StripInvalidImageKeys(input)
	if strings.Contains(got, "http://x.com") {
		t.Error("should strip URL images")
	}
	if !strings.Contains(got, "img_v3_xyz") {
		t.Error("should preserve img_ keys")
	}
}

func TestStripInvalidImageKeys_NoImages(t *testing.T) {
	input := "just plain text, no images"
	got := StripInvalidImageKeys(input)
	if got != input {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestStripInvalidImageKeys_LocalPath(t *testing.T) {
	input := "![file](/tmp/feishu-image-abc.png)"
	got := StripInvalidImageKeys(input)
	if got != "" {
		t.Errorf("should strip local path images, got %q", got)
	}
}

func TestOptimizeMarkdownStyle_ConsecutiveHeadings(t *testing.T) {
	input := "#### Heading A\n#### Heading B"
	result := OptimizeMarkdownStyle(input, 2)

	// Consecutive headings should have <br> between them
	if !strings.Contains(result, "#### Heading A\n<br>\n#### Heading B") {
		t.Errorf("expected <br> between consecutive headings, got:\n%s", result)
	}
}
