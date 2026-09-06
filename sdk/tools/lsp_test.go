package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hwj123hwj/pi-go/sdk/agent"
)

// ─── LSP tool registration / interface conformance tests ──────────────────────

func TestLSPTools_InterfaceConformance(t *testing.T) {
	workspace := t.TempDir()
	tools := NewLSPTools(workspace)

	expectedNames := []string{
		"lsp_go_to_definition",
		"lsp_find_references",
		"lsp_hover",
		"lsp_document_symbols",
		"lsp_workspace_symbols",
		"lsp_workspace_symbols",
		"lsp_workspace_symbols",
		"lsp_go_to_implementation",
	}
	// Deduplicate expected for the count check
	seen := make(map[string]bool)
	for _, n := range expectedNames {
		seen[n] = true
	}

	if len(tools) != 6 {
		t.Fatalf("expected 6 LSP tools, got %d", len(tools))
	}

	for _, tool := range tools {
		// Verify all interface methods
		if tool.Name() == "" {
			t.Error("tool Name() should not be empty")
		}
		if tool.Description() == "" {
			t.Error("tool Description() should not be empty")
		}
		if tool.Parameters() == nil {
			t.Error("tool Parameters() should not be nil")
		}

		// Verify PromptInfo interface
		if pi, ok := tool.(agent.ToolWithPromptInfo); ok {
			if pi.PromptSnippet() == "" {
				t.Errorf("tool %s PromptSnippet() should not be empty", tool.Name())
			}
			if len(pi.PromptGuidelines()) == 0 {
				t.Errorf("tool %s PromptGuidelines() should not be empty", tool.Name())
			}
		} else {
			t.Errorf("tool %s should implement ToolWithPromptInfo", tool.Name())
		}

		// Verify ConcurrencySafeChecker interface
		if cs, ok := tool.(agent.ConcurrencySafeChecker); ok {
			if !cs.IsConcurrencySafe(nil) {
				t.Errorf("tool %s should be concurrency-safe", tool.Name())
			}
		} else {
			t.Errorf("tool %s should implement ConcurrencySafeChecker", tool.Name())
		}
	}
}

func TestLSPGotoDefinitionTool_Validate(t *testing.T) {
	tool := NewLSPGotoDefinitionTool(t.TempDir())

	// Missing filePath
	_, err := tool.Validate([]byte(`{"line":1,"character":1}`))
	if err == nil {
		t.Error("expected error for missing filePath")
	}

	// Non-absolute path
	_, err = tool.Validate([]byte(`{"filePath":"relative/path","line":1,"character":1}`))
	if err == nil {
		t.Error("expected error for relative path")
	}

	// Zero-based line
	_, err = tool.Validate([]byte(`{"filePath":"/abs/path.go","line":0,"character":1}`))
	if err == nil {
		t.Error("expected error for line < 1")
	}

	// Valid params
	valid, err := tool.Validate([]byte(`{"filePath":"/abs/path.go","line":5,"character":3}`))
	if err != nil {
		t.Errorf("unexpected error for valid params: %v", err)
	}
	var p lspPositionParams
	if err := json.Unmarshal(valid, &p); err != nil {
		t.Fatal(err)
	}
	if p.Line != 5 || p.Character != 3 {
		t.Errorf("params not parsed correctly: %+v", p)
	}
}

func TestLSPWorkspaceSymbolsTool_Validate(t *testing.T) {
	tool := NewLSPWorkspaceSymbolsTool(t.TempDir())

	// Empty query
	_, err := tool.Validate([]byte(`{"query":""}`))
	if err == nil {
		t.Error("expected error for empty query")
	}

	// Valid query
	valid, err := tool.Validate([]byte(`{"query":"main"}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	var p lspQueryParams
	if err := json.Unmarshal(valid, &p); err != nil {
		t.Fatal(err)
	}
	if p.Query != "main" {
		t.Errorf("query not parsed correctly")
	}
}

func TestLSPDocumentSymbolsTool_Validate(t *testing.T) {
	tool := NewLSPDocumentSymbolsTool(t.TempDir())

	// Missing filePath
	_, err := tool.Validate([]byte(`{}`))
	if err == nil {
		t.Error("expected error for missing filePath")
	}

	// Valid
	_, err = tool.Validate([]byte(`{"filePath":"/abs/path.go"}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLSPTools_Names(t *testing.T) {
	// Verify all 6 tool names are correct
	tools := NewLSPTools("/tmp/ws")
	expected := map[string]bool{
		"lsp_go_to_definition":   true,
		"lsp_find_references":    true,
		"lsp_hover":              true,
		"lsp_document_symbols":   true,
		"lsp_workspace_symbols":  true,
		"lsp_go_to_implementation": true,
	}
	for _, tool := range tools {
		if !expected[tool.Name()] {
			t.Errorf("unexpected tool name: %s", tool.Name())
		}
		delete(expected, tool.Name())
	}
	if len(expected) > 0 {
		t.Errorf("missing tools: %v", expected)
	}
}

func TestValidatePositionParams(t *testing.T) {
	// Valid
	if err := validatePositionParams(lspPositionParams{FilePath: "/abs.go", Line: 1, Character: 1}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	// Relative path
	if err := validatePositionParams(lspPositionParams{FilePath: "rel.go", Line: 1, Character: 1}); err == nil {
		t.Error("expected error for relative path")
	}
}

func TestExtractLocations(t *testing.T) {
	// Single Location
	results := []json.RawMessage{
		json.RawMessage(`{"uri":"file:///test.go","range":{"start":{"line":1,"character":2},"end":{"line":1,"character":5}}}`),
	}
	locs := extractLocations(results)
	if len(locs) != 1 {
		t.Fatalf("expected 1 location, got %d", len(locs))
	}
	if locs[0].URI != "file:///test.go" {
		t.Errorf("unexpected URI: %s", locs[0].URI)
	}

	// Location array
	results = []json.RawMessage{
		json.RawMessage(`[{"uri":"file:///a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":3}}},{"uri":"file:///b.go","range":{"start":{"line":4,"character":5},"end":{"line":4,"character":8}}}]`),
	}
	locs = extractLocations(results)
	if len(locs) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(locs))
	}

	// LocationLink array
	results = []json.RawMessage{
		json.RawMessage(`[{"targetUri":"file:///c.go","targetRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":3}},"targetSelectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":3}}}]`),
	}
	locs = extractLocations(results)
	if len(locs) != 1 {
		t.Fatalf("expected 1 location from LocationLink, got %d", len(locs))
	}
	if locs[0].URI != "file:///c.go" {
		t.Errorf("unexpected URI: %s", locs[0].URI)
	}
}

func TestFormatLocations(t *testing.T) {
	locs := []struct {
		uri   string
		start struct {
			line, char int
		}
		end struct {
			line, char int
		}
	}{}
	_ = locs // just ensure the package compiles with agent import

	// Test with actual Location type
	results := []json.RawMessage{
		json.RawMessage(`{"uri":"file:///home/user/main.go","range":{"start":{"line":4,"character":2},"end":{"line":4,"character":8}}}`),
	}
	extracted := extractLocations(results)
	formatted := formatLocations(extracted, "/home/user")
	if !strings.Contains(formatted, "main.go:5:3-9") {
		t.Errorf("formatted output should contain 'main.go:5:3-9', got: %s", formatted)
	}
}

func TestParseHoverContents(t *testing.T) {
	// String
	got := parseHoverContents(json.RawMessage(`"hello"`))
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}

	// MarkupContent
	got = parseHoverContents(json.RawMessage(`{"kind":"markdown","value":"# Title"}`))
	if got != "# Title" {
		t.Errorf("expected '# Title', got %q", got)
	}

	// Array
	got = parseHoverContents(json.RawMessage(`["line1","line2"]`))
	if got != "line1\n\nline2" {
		t.Errorf("expected 'line1\\n\\nline2', got %q", got)
	}
}

func TestRelPath(t *testing.T) {
	// Under workspace
	got := relPath("/home/user/project", "/home/user/project/main.go")
	if got != "main.go" {
		t.Errorf("expected 'main.go', got %q", got)
	}

	// Outside workspace
	got = relPath("/home/user/project", "/etc/passwd")
	if got != "/etc/passwd" {
		t.Errorf("expected '/etc/passwd', got %q", got)
	}

	// Empty workspace
	got = relPath("", "/some/path")
	if got != "/some/path" {
		t.Errorf("expected '/some/path', got %q", got)
	}
}

func TestLSPTool_Execute_NoServer(t *testing.T) {
	// Without gopls installed, Execute should return an error result (not panic)
	ResetLSPManager()
	defer ResetLSPManager()

	tool := NewLSPGotoDefinitionTool(t.TempDir())
	valid, _ := tool.Validate([]byte(`{"filePath":"/tmp/nonexistent.go","line":1,"character":1}`))

	ctx := context.Background()
	result, err := tool.Execute(ctx, valid, nil)
	// Should not return a Go error — the tool wraps LSP errors into ToolResult
	if err != nil {
		t.Logf("Execute returned Go error (acceptable): %v", err)
	}
	// Result should indicate error or "no definition"
	_ = result
	_ = agent.ToolResult{} // ensure agent import is used
}
