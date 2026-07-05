package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebSearchValidate(t *testing.T) {
	tool := NewWebSearchTool()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid query",
			input:   `{"query": "golang context tutorial"}`,
			wantErr: false,
		},
		{
			name:    "empty query",
			input:   `{"query": ""}`,
			wantErr: true,
		},
		{
 name:    "missing query",
			input:   `{"max_results": 5}`,
			wantErr: true,
		},
		{
			name:    "with max_results",
			input:   `{"query": "test", "max_results": 5}`,
			wantErr: false,
		},
		{
			name:    "max_results too large gets clamped",
			input:   `{"query": "test", "max_results": 100}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validated, err := tool.Validate(json.RawMessage(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				// Verify the validated params are sensible
				var params WebSearchParams
				json.Unmarshal(validated, &params)
				if params.MaxResults > 20 {
					t.Errorf("MaxResults should be clamped to 20, got %d", params.MaxResults)
				}
			}
		})
	}
}

func TestWebSearchSearXNG(t *testing.T) {
	// Mock SearXNG server
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, "missing query", http.StatusBadRequest)
			return
		}

		response := map[string]any{
			"results": []map[string]any{
				{"title": "Go Context Tutorial", "url": "https://golang.org/doc/context", "content": "Learn how to use context in Go."},
				{"title": "Advanced Go Patterns", "url": "https://example.com/advanced", "content": "Deep dive into Go concurrency."},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	engine := newSearXNGEngine(server.URL, 0)
	// Override client to skip real network checks
	engine.client = server.Client()

	results, err := engine.search(context.Background(), "golang context", 5)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	if results[0].Title != "Go Context Tutorial" {
		t.Errorf("First result title = %q, want %q", results[0].Title, "Go Context Tutorial")
	}
	if results[0].URL != "https://golang.org/doc/context" {
		t.Errorf("First result URL = %q", results[0].URL)
	}
}

func TestWebSearchFormatOutput(t *testing.T) {
	tool := NewWebSearchTool()

	// Test with mock SearXNG
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"results": []map[string]any{
				{"title": "Test Result", "url": "https://example.com", "content": "This is a snippet."},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("SEARXNG_URL", server.URL)
	tool = NewWebSearchTool()

	validated, _ := tool.Validate(json.RawMessage(`{"query": "test"}`))
	result, err := tool.Execute(context.Background(), validated, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.IsError {
		t.Error("Expected non-error result")
	}

	if result.Content == "" {
		t.Error("Expected non-empty content")
	}
}

func TestParseDuckDuckGoHTML(t *testing.T) {
	html := `<html><body>
	<div class="result">
		<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org%2F&amp;rut=abc">Go Programming Language</a>
		<a class="result__snippet" href="#">A statically typed, compiled programming language</a>
	</div>
	<div class="result">
		<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fgo&amp;rut=def">Learn Go</a>
		<a class="result__snippet" href="#">Go tutorial for beginners</a>
	</div>
	</body></html>`

	results := parseDuckDuckGoHTML(html, 5)

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	if results[0].Title != "Go Programming Language" {
		t.Errorf("Title[0] = %q, want %q", results[0].Title, "Go Programming Language")
	}
	if results[0].URL != "https://golang.org/" {
		t.Errorf("URL[0] = %q, want %q", results[0].URL, "https://golang.org/")
	}
	if results[1].Title != "Learn Go" {
		t.Errorf("Title[1] = %q, want %q", results[1].Title, "Learn Go")
	}
}

func TestExtractDDGRedirectURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "//duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org%2F&rut=abc",
			expected: "https://golang.org/",
		},
		{
			input:    "https://example.com/direct",
			expected: "https://example.com/direct",
		},
		{
			input:    "duckduckgo.com/l/?uddg=https%3A%2F%2Fpkg.go.dev",
			expected: "https://pkg.go.dev",
		},
		{
			input:    "invalid",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(len(tt.input), 20)], func(t *testing.T) {
			got := extractDDGRedirectURL(tt.input)
			if got != tt.expected {
				t.Errorf("extractDDGRedirectURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCleanHTMLText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<b>hello</b>", "hello"},
		{"plain text", "plain text"},
		{"a &amp; b", "a & b"},
		{"&lt;tag&gt;", "<tag>"},
		{"", ""},
	}

	for _, tt := range tests {
		got := cleanHTMLText(tt.input)
		if got != tt.expected {
			t.Errorf("cleanHTMLText(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestTruncateSnippet(t *testing.T) {
	short := "hello"
	if got := truncateSnippet(short, 10); got != "hello" {
		t.Errorf("Expected %q, got %q", "hello", got)
	}

	long := "This is a very long snippet that should be truncated to fit within the specified limit."
	got := truncateSnippet(long, 80)
	// We check it was truncated (shorter than original + ends with ...)
	if len(got) >= len(long) {
		t.Errorf("Expected truncation, got full length: %q", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("Expected truncation suffix '...', got %q", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
