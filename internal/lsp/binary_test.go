package lsp

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestLSPCacheDir(t *testing.T) {
	dir := LSPCacheDir()
	if dir == "" {
		t.Fatal("LSPCacheDir should not return empty")
	}
	// Should contain .pi-go/lsp
	if !contains(dir, ".pi-go") {
		t.Errorf("expected cache dir to contain '.pi-go', got %s", dir)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestServerCacheDir(t *testing.T) {
	dir, err := serverCacheDir("test-server")
	if err != nil {
		t.Fatalf("serverCacheDir failed: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("cache dir should exist: %v", err)
	}
	// Cleanup
	os.RemoveAll(filepath.Join(LSPCacheDir(), "test-server"))
}

func TestEnsureBinary_AlreadyCached(t *testing.T) {
	// Create a fake cached binary in the cache dir
	dir, _ := serverCacheDir("fake-server")
	fakeBin := filepath.Join(dir, "fake-bin")
	os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755)
	defer os.RemoveAll(dir)

	// The installer checks for cached binary internally and returns immediately
	installer := func(destDir string) (string, error) {
		binPath := filepath.Join(destDir, "fake-bin")
		if _, err := os.Stat(binPath); err == nil {
			return binPath, nil // cached
		}
		t.Error("installer should find cached binary")
		return binPath, nil
	}

	result, err := EnsureBinary("fake-server", installer)
	if err != nil {
		t.Fatalf("EnsureBinary failed: %v", err)
	}
	if result != fakeBin {
		t.Errorf("expected %s, got %s", fakeBin, result)
	}
}

func TestEnsureBinary_InstallerCalled(t *testing.T) {
	// Use a unique server ID with no cache
	serverID := "test-ensure-uncached"
	defer cleanBinaryCache(serverID)

	called := false
	installer := func(destDir string) (string, error) {
		called = true
		binPath := filepath.Join(destDir, "my-bin")
		os.WriteFile(binPath, []byte("fake"), 0o755)
		return binPath, nil
	}

	result, err := EnsureBinary(serverID, installer)
	if err != nil {
		t.Fatalf("EnsureBinary failed: %v", err)
	}
	if !called {
		t.Error("installer should have been called")
	}
	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestNpmInstaller_CreatesPackageJSON(t *testing.T) {
	dir := t.TempDir()
	installer := NpmInstaller([]string{"some-package"}, "some-bin")

	// npm is not available in test env, so this will fail,
	// but we can verify package.json was created first
	_, err := installer(dir)
	if err == nil {
		// If npm is available (unlikely in CI), that's fine
		return
	}
	// Check package.json was created
	pkgJSON := filepath.Join(dir, "package.json")
	if _, err := os.Stat(pkgJSON); err != nil {
		t.Errorf("package.json should have been created: %v", err)
	}
}

func TestGoInstaller_ReturnsFunction(t *testing.T) {
	installer := GoInstaller("golang.org/x/tools/gopls", "gopls")
	if installer == nil {
		t.Fatal("GoInstaller should return a non-nil function")
	}
}

func TestGitHubInstaller_ReturnsFunction(t *testing.T) {
	matcher := func(goos, goarch string) *regexp.Regexp {
		return regexp.MustCompile(".*")
	}
	installer := GitHubInstaller("test", "repo", matcher)
	if installer == nil {
		t.Fatal("GitHubInstaller should return a non-nil function")
	}
}

func TestCleanBinaryCache(t *testing.T) {
	// Create a fake cache dir
	dir, _ := serverCacheDir("clean-test")
	fakeFile := filepath.Join(dir, "marker")
	os.WriteFile(fakeFile, []byte("test"), 0o644)

	cleanBinaryCache("clean-test")

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("cache dir should have been removed")
	}
}

func TestDefaultServersCount(t *testing.T) {
	servers := DefaultServers()
	// Should have all 9 servers to match hwjcode
	if len(servers) != 9 {
		t.Errorf("expected 9 servers, got %d", len(servers))
	}
}

func TestDefaultServers_AllIDs(t *testing.T) {
	servers := DefaultServers()
	expected := map[string]bool{
		"gopls":                             true,
		"typescript-language-server":        true,
		"pyright":                           true,
		"rust-analyzer":                     true,
		"clangd":                            true,
		"vscode-langservers-extracted":      true,
		"yaml-language-server":              true,
		"dockerfile-language-server-nodejs": true,
		"sql-language-server":               true,
	}

	for _, s := range servers {
		delete(expected, s.ID)
	}
	if len(expected) > 0 {
		t.Errorf("missing servers: %v", expected)
	}
}
