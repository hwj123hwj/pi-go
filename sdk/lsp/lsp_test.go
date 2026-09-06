package lsp

import (
	"bufio"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager("/tmp/test-project")
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.ProjectRoot() == "" {
		t.Error("ProjectRoot should not be empty")
	}
	// Should not panic
	mgr.Shutdown(t.Context())
}

func TestDefaultServersContainsGopls(t *testing.T) {
	servers := DefaultServers()
	found := false
	for _, s := range servers {
		if s.ID == "gopls" {
			found = true
			if len(s.Extensions) == 0 {
				t.Error("gopls should have extensions configured")
			}
		}
	}
	if !found {
		t.Error("DefaultServers should include gopls")
	}
	// Should also include other servers (matching hwjcode)
	if len(servers) < 9 {
		t.Errorf("expected at least 9 servers, got %d", len(servers))
	}
}

func TestGoplsSpec(t *testing.T) {
	spec := GoplsSpec()
	if spec.ID != "gopls" {
		t.Errorf("expected ID 'gopls', got %s", spec.ID)
	}
	// Check extensions include .go
	hasGo := false
	for _, ext := range spec.Extensions {
		if ext == ".go" {
			hasGo = true
		}
	}
	if !hasGo {
		t.Error("gopls should handle .go files")
	}
	// RootDir should return projectRoot when no marker is found
	root := spec.RootDir("/tmp/nonexistent-file.go", "/tmp/project")
	if root == "" {
		t.Error("RootDir should not return empty")
	}
}

func TestPathToURI_URIToPath_RoundTrip(t *testing.T) {
	tests := []string{
		"/home/user/project/main.go",
		"/tmp/test.go",
		"/var/folders/abc/file.go",
	}
	for _, path := range tests {
		uri := PathToURI(path)
		if uri == "" {
			t.Errorf("PathToURI(%q) returned empty", path)
		}
		// URIToPath should return the original path (after abs/clean)
		recovered := URIToPath(uri)
		if recovered != path {
			t.Errorf("round-trip failed: %q → %q → %q", path, uri, recovered)
		}
	}
}

func TestURIToPath_NonFileURI(t *testing.T) {
	// Non-file: URIs should be returned as-is
	uri := "http://example.com/path"
	if got := URIToPath(uri); got != uri {
		t.Errorf("URIToPath(%q) = %q, want %q", uri, got, uri)
	}
}

func TestLanguageID(t *testing.T) {
	tests := map[string]string{
		"main.go":      "go",
		"app.ts":       "typescript",
		"app.tsx":      "typescriptreact",
		"script.js":    "javascript",
		"script.jsx":   "javascriptreact",
		"module.py":    "python",
		"main.rs":      "rust",
		"main.c":       "c",
		"main.cpp":     "cpp",
		"config.json":  "json",
		"config.yaml":  "yaml",
		"config.yml":   "yaml",
		"page.html":    "html",
		"style.css":    "css",
		"unknown.xyz":  "plaintext",
	}
	for filename, expected := range tests {
		got := languageID(filename)
		if got != expected {
			t.Errorf("languageID(%q) = %q, want %q", filename, got, expected)
		}
	}
}

func TestSymbolKindName(t *testing.T) {
	tests := map[int]string{
		SymbolKindFile:       "File",
		SymbolKindFunction:   "Function",
		SymbolKindInterface:  "Interface",
		SymbolKindStruct:     "Struct",
		SymbolKindMethod:     "Method",
		SymbolKindVariable:   "Variable",
		SymbolKindConstant:   "Constant",
		999:                  "Unknown",
	}
	for kind, expected := range tests {
		got := SymbolKindName(kind)
		if got != expected {
			t.Errorf("SymbolKindName(%d) = %q, want %q", kind, got, expected)
		}
	}
}

func TestFindServersForFile(t *testing.T) {
	servers := DefaultServers()

	// .go file should match gopls
	goServers := findServersForFile(servers, "main.go")
	if len(goServers) == 0 {
		t.Error("expected at least one server for .go files")
	}

	// .xyz file should match nothing
	none := findServersForFile(servers, "file.xyz")
	if len(none) != 0 {
		t.Error("expected no servers for .xyz files")
	}
}

func TestNearestRoot(t *testing.T) {
	// Create a temp dir structure and test marker detection
	dir := t.TempDir()

	// nearestRoot with go.mod marker
	detector := nearestRoot([]string{"go.mod"})
	// File in a subdirectory — should walk up to dir (where no go.mod exists,
	// so it returns the projectRoot)
	root := detector(filepath.Join(dir, "subdir", "main.go"), dir)
	if root == "" {
		t.Error("nearestRoot should not return empty")
	}
}

func TestManager_UnknownFileType(t *testing.T) {
	mgr := NewManager(t.TempDir())
	// Getting clients for a file type with no server should error
	_, err := mgr.getClientsForFile(t.Context(), "/tmp/test.unknownext")
	if err == nil {
		t.Error("expected error for unknown file type")
	}
	mgr.Shutdown(t.Context())
}

// ─── JSON-RPC framing tests ──────────────────────────────────────────────────

func TestReadMessage_Empty(t *testing.T) {
	// readMessage should handle a minimal valid message
	msg := "Content-Length: 2\r\n\r\n{}"
	br := bufio.NewReader(strings.NewReader(msg))
	data, err := readMessage(br)
	if err != nil {
		t.Fatalf("readMessage failed: %v", err)
	}
	if string(data) != "{}" {
		t.Errorf("expected '{}', got %s", string(data))
	}
}

func TestConnRegisterHandler(t *testing.T) {
	conn := NewConn(nil, nil) // no actual I/O needed for this test
	conn.RegisterHandler("test/method", func(params json.RawMessage) (any, error) {
		return map[string]any{"ok": true}, nil
	})
	// Should not panic
	conn.RegisterHandler("test/method", nil) // remove
}

func TestTruncateForLog(t *testing.T) {
	short := "hello"
	long := strings.Repeat("a", 300)

	if got := truncateForLog(short, 200); got != short {
		t.Errorf("short string should not be truncated")
	}
	got := truncateForLog(long, 200)
	if len(got) > 210 { // 200 + "..."
		t.Errorf("long string should be truncated, got len=%d", len(got))
	}
}

func TestIsEmptyArray(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"null", true},
		{"[]", true},
		{"", true},
		{"[{\"uri\":\"file:///test\"}]", false},
		{"{\"contents\":\"test\"}", false},
	}
	for _, tt := range tests {
		got := isEmptyArray(json.RawMessage(tt.input))
		if got != tt.want {
			t.Errorf("isEmptyArray(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
