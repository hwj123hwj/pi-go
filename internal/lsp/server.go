package lsp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// ─── Language Server Definitions ──────────────────────────────────────────────
//
// ServerSpec describes how to find and launch a language server.
// Mirrors hwjcode's LSPServer.Info, adapted for Go.

// ServerSpec defines a language server.
type ServerSpec struct {
	ID          string
	DisplayName string
	Extensions  []string // lowercase, with leading dot (e.g. ".go")

	// RootDir determines the workspace root for a given source file.
	RootDir func(file string, projectRoot string) string

	// Spawn launches the server process.
	Spawn func(ctx context.Context, root string) (*exec.Cmd, error)
}

// ─── Spawn helper: find cached binary or fall back to PATH ────────────────────

// findOrEnsureBinary resolves the server binary path:
//  1. Check if already in our cache (fast path)
//  2. Check PATH (user's own install)
//  3. Run EnsureBinary to download/install
//
// Returns the absolute path to the binary.
func findOrEnsureBinary(serverID string, installer InstallerFunc, binName string) (string, error) {
	// 1. Check cache directly first (fast path, no installer invocation)
	cacheDir, _ := serverCacheDir(serverID)
	cachedBin := filepath.Join(cacheDir, binName)
	if runtime.GOOS == "windows" && !strings.HasSuffix(binName, ".exe") {
		cachedBin += ".exe"
	}
	if _, err := os.Stat(cachedBin); err == nil {
		return cachedBin, nil
	}

	// Also check node_modules/.bin for npm-installed binaries
	npmBin := filepath.Join(cacheDir, "node_modules", ".bin", binName)
	if runtime.GOOS == "windows" {
		npmBin += ".cmd"
	}
	if _, err := os.Stat(npmBin); err == nil {
		return npmBin, nil
	}

	// 2. Check PATH
	if p := findOnPath(binName); p != "" {
		return p, nil
	}

	// 3. Download/install
	return EnsureBinary(serverID, installer)
}

// ─── gopls (Go language server) ───────────────────────────────────────────────

func GoplsSpec() ServerSpec {
	return ServerSpec{
		ID:          "gopls",
		DisplayName: "Go Language Server",
		Extensions:  []string{".go"},
		RootDir:     nearestRoot([]string{"go.mod", "go.sum"}),
		Spawn:       spawnGopls,
	}
}

func spawnGopls(ctx context.Context, root string) (*exec.Cmd, error) {
	binName := "gopls"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	// 1. Explicit env override
	if p := os.Getenv("PI_GO_GPLS_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return exec.CommandContext(ctx, p), nil
		}
	}

	installer := GoInstaller("golang.org/x/tools/gopls", "gopls")
	binPath, err := findOrEnsureBinary("gopls", installer, binName)
	if err != nil {
		return nil, &ServerNotFoundError{ServerID: "gopls", Cause: err,
			Hint: "install with: go install golang.org/x/tools/gopls@latest"}
	}
	return exec.CommandContext(ctx, binPath), nil
}

// ─── TypeScript / JavaScript ──────────────────────────────────────────────────

func TypeScriptLSPSpec() ServerSpec {
	return ServerSpec{
		ID:          "typescript-language-server",
		DisplayName: "TypeScript/JavaScript Language Server",
		Extensions:  []string{".ts", ".tsx", ".js", ".jsx"},
		RootDir:     nearestRoot([]string{"package.json", "tsconfig.json", "jsconfig.json"}),
		Spawn:       spawnTypeScriptLSP,
	}
}

func spawnTypeScriptLSP(ctx context.Context, root string) (*exec.Cmd, error) {
	binName := "typescript-language-server"

	installer := NpmInstaller(
		[]string{"typescript-language-server", "typescript"},
		binName,
	)
	binPath, err := findOrEnsureBinary("typescript-language-server", installer, binName)
	if err != nil {
		return nil, &ServerNotFoundError{ServerID: "typescript-language-server", Cause: err,
			Hint: "install with: npm install -g typescript-language-server typescript"}
	}
	return exec.CommandContext(ctx, binPath, "--stdio"), nil
}

// ─── Pyright (Python) ─────────────────────────────────────────────────────────

func PyrightSpec() ServerSpec {
	return ServerSpec{
		ID:          "pyright",
		DisplayName: "Python Language Server",
		Extensions:  []string{".py"},
		RootDir:     nearestRoot([]string{"pyproject.toml", "setup.py", "requirements.txt"}),
		Spawn:       spawnPyright,
	}
}

func spawnPyright(ctx context.Context, root string) (*exec.Cmd, error) {
	binName := "pyright-langserver"

	installer := NpmInstaller([]string{"pyright"}, "pyright-langserver")
	binPath, err := findOrEnsureBinary("pyright", installer, binName)
	if err != nil {
		return nil, &ServerNotFoundError{ServerID: "pyright", Cause: err,
			Hint: "install with: npm install -g pyright"}
	}
	return exec.CommandContext(ctx, binPath, "--stdio"), nil
}

// ─── rust-analyzer (Rust) ─────────────────────────────────────────────────────

func RustAnalyzerSpec() ServerSpec {
	return ServerSpec{
		ID:          "rust-analyzer",
		DisplayName: "Rust Language Server",
		Extensions:  []string{".rs"},
		RootDir:     nearestRoot([]string{"Cargo.toml"}),
		Spawn:       spawnRustAnalyzer,
	}
}

func spawnRustAnalyzer(ctx context.Context, root string) (*exec.Cmd, error) {
	binName := "rust-analyzer"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	matcher := func(goos, goarch string) *regexp.Regexp {
		var pattern string
		switch goos {
		case "windows":
			pattern = `rust-analyzer-.*x86_64.*windows.*\.zip`
			if goarch == "arm64" {
				pattern = `rust-analyzer-.*aarch64.*windows.*\.zip`
			}
		case "darwin":
			if goarch == "arm64" {
				pattern = `rust-analyzer-.*aarch64.*apple-darwin.*\.gz`
			} else {
				pattern = `rust-analyzer-.*x86_64.*apple-darwin.*\.gz`
			}
		default: // linux
			if goarch == "arm64" {
				pattern = `rust-analyzer-.*aarch64.*linux.*\.gz`
			} else {
				pattern = `rust-analyzer-.*x86_64.*linux.*\.gz`
			}
		}
		return regexp.MustCompile(pattern)
	}

	installer := GitHubInstaller("rust-lang", "rust-analyzer", matcher)
	binPath, err := findOrEnsureBinary("rust-analyzer", installer, binName)
	if err != nil {
		return nil, &ServerNotFoundError{ServerID: "rust-analyzer", Cause: err,
			Hint: "install from: https://github.com/rust-lang/rust-analyzer/releases"}
	}
	return exec.CommandContext(ctx, binPath), nil
}

// ─── clangd (C/C++) ───────────────────────────────────────────────────────────

func ClangdSpec() ServerSpec {
	return ServerSpec{
		ID:          "clangd",
		DisplayName: "C/C++ Language Server",
		Extensions:  []string{".c", ".cpp", ".cc", ".cxx", ".h", ".hpp"},
		RootDir:     nearestRoot([]string{"compile_commands.json", "CMakeLists.txt"}),
		Spawn:       spawnClangd,
	}
}

func spawnClangd(ctx context.Context, root string) (*exec.Cmd, error) {
	binName := "clangd"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	matcher := func(goos, _ string) *regexp.Regexp {
		var pattern string
		switch goos {
		case "windows":
			pattern = `clangd-.*windows.*\.zip`
		case "darwin":
			pattern = `clangd-.*mac.*\.zip`
		default:
			pattern = `clangd-.*linux.*\.zip`
		}
		return regexp.MustCompile(pattern)
	}

	installer := GitHubInstaller("clangd", "clangd", matcher)
	binPath, err := findOrEnsureBinary("clangd", installer, binName)
	if err != nil {
		return nil, &ServerNotFoundError{ServerID: "clangd", Cause: err,
			Hint: "install via your system package manager (e.g. apt install clangd)"}
	}
	return exec.CommandContext(ctx, binPath), nil
}

// ─── vscode-langservers-extracted (HTML/CSS/JSON/ESLint) ──────────────────────

func WebLSPSpec() ServerSpec {
	return ServerSpec{
		ID:          "vscode-langservers-extracted",
		DisplayName: "HTML/CSS/JSON/ESLint Language Server",
		Extensions:  []string{".html", ".css", ".json", ".jsonc"},
		RootDir: func(_, projectRoot string) string {
			return projectRoot
		},
		Spawn: spawnWebLSP,
	}
}

func spawnWebLSP(ctx context.Context, root string) (*exec.Cmd, error) {
	binName := "vscode-html-language-server"

	installer := NpmInstaller([]string{"vscode-langservers-extracted"}, binName)
	binPath, err := findOrEnsureBinary("vscode-langservers-extracted", installer, binName)
	if err != nil {
		return nil, &ServerNotFoundError{ServerID: "vscode-langservers-extracted", Cause: err,
			Hint: "install with: npm install -g vscode-langservers-extracted"}
	}
	return exec.CommandContext(ctx, binPath, "--stdio"), nil
}

// ─── yaml-language-server ─────────────────────────────────────────────────────

func YamlLSPSpec() ServerSpec {
	return ServerSpec{
		ID:          "yaml-language-server",
		DisplayName: "YAML Language Server",
		Extensions:  []string{".yaml", ".yml"},
		RootDir: func(_, projectRoot string) string {
			return projectRoot
		},
		Spawn: spawnYamlLSP,
	}
}

func spawnYamlLSP(ctx context.Context, root string) (*exec.Cmd, error) {
	binName := "yaml-language-server"

	installer := NpmInstaller([]string{"yaml-language-server"}, binName)
	binPath, err := findOrEnsureBinary("yaml-language-server", installer, binName)
	if err != nil {
		return nil, &ServerNotFoundError{ServerID: "yaml-language-server", Cause: err,
			Hint: "install with: npm install -g yaml-language-server"}
	}
	return exec.CommandContext(ctx, binPath, "--stdio"), nil
}

// ─── dockerfile-language-server ───────────────────────────────────────────────

func DockerLSPSpec() ServerSpec {
	return ServerSpec{
		ID:          "dockerfile-language-server-nodejs",
		DisplayName: "Dockerfile Language Server",
		Extensions:  []string{"Dockerfile", ".dockerfile"},
		RootDir: func(_, projectRoot string) string {
			return projectRoot
		},
		Spawn: spawnDockerLSP,
	}
}

func spawnDockerLSP(ctx context.Context, root string) (*exec.Cmd, error) {
	binName := "docker-langserver"

	installer := NpmInstaller([]string{"dockerfile-language-server-nodejs"}, binName)
	binPath, err := findOrEnsureBinary("dockerfile-language-server-nodejs", installer, binName)
	if err != nil {
		return nil, &ServerNotFoundError{ServerID: "dockerfile-language-server-nodejs", Cause: err,
			Hint: "install with: npm install -g dockerfile-language-server-nodejs"}
	}
	return exec.CommandContext(ctx, binPath, "--stdio"), nil
}

// ─── sql-language-server ──────────────────────────────────────────────────────

func SqlLSPSpec() ServerSpec {
	return ServerSpec{
		ID:          "sql-language-server",
		DisplayName: "SQL Language Server",
		Extensions:  []string{".sql"},
		RootDir: func(_, projectRoot string) string {
			return projectRoot
		},
		Spawn: spawnSqlLSP,
	}
}

func spawnSqlLSP(ctx context.Context, root string) (*exec.Cmd, error) {
	binName := "sql-language-server"

	installer := NpmInstaller([]string{"sql-language-server"}, binName)
	binPath, err := findOrEnsureBinary("sql-language-server", installer, binName)
	if err != nil {
		return nil, &ServerNotFoundError{ServerID: "sql-language-server", Cause: err,
			Hint: "install with: npm install -g sql-language-server"}
	}
	return exec.CommandContext(ctx, binPath, "up", "--method", "stdio"), nil
}

// ─── Server registry ─────────────────────────────────────────────────────────

// DefaultServers returns all supported language servers.
// Mirrors hwjcode's DefaultServers — 9 servers covering Go, TS/JS, Python,
// Rust, C/C++, Web (HTML/CSS/JSON), YAML, Dockerfile, SQL.
func DefaultServers() []ServerSpec {
	return []ServerSpec{
		GoplsSpec(),
		TypeScriptLSPSpec(),
		PyrightSpec(),
		RustAnalyzerSpec(),
		ClangdSpec(),
		WebLSPSpec(),
		YamlLSPSpec(),
		DockerLSPSpec(),
		SqlLSPSpec(),
	}
}

// ─── Error type ───────────────────────────────────────────────────────────────

// ServerNotFoundError indicates a language server binary could not be located.
type ServerNotFoundError struct {
	ServerID string
	Cause    error // underlying error (e.g. install failure)
	Hint     string
}

func (e *ServerNotFoundError) Error() string {
	msg := "LSP server not found: " + e.ServerID
	if e.Cause != nil {
		msg += fmt.Sprintf(" (%v)", e.Cause)
	}
	if e.Hint != "" {
		msg += " — " + e.Hint
	}
	return msg
}

func (e *ServerNotFoundError) Unwrap() error {
	return e.Cause
}

// findServersForFile returns all server specs that handle the file's extension.
func findServersForFile(servers []ServerSpec, file string) []ServerSpec {
	ext := strings.ToLower(filepath.Ext(file))
	// Also match bare filename for Dockerfile (no extension)
	base := strings.ToLower(filepath.Base(file))

	var result []ServerSpec
	for _, s := range servers {
		for _, e := range s.Extensions {
			if e == ext || e == base {
				result = append(result, s)
				break
			}
		}
	}
	return result
}

// ─── Root directory detection ─────────────────────────────────────────────────

// nearestRoot returns a function that walks up from the file's directory to
// find the nearest ancestor containing any of the marker files, bounded by
// projectRoot. If no marker is found, projectRoot is returned.
func nearestRoot(markers []string) func(file string, projectRoot string) string {
	return func(file string, projectRoot string) string {
		current := filepath.Dir(filepath.Clean(file))
		stop, err := filepath.Abs(projectRoot)
		if err != nil {
			stop = projectRoot
		}

		for {
			for _, marker := range markers {
				if _, err := os.Stat(filepath.Join(current, marker)); err == nil {
					return current
				}
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			// Don't go beyond projectRoot
			if !isSubPath(current, stop) && current != stop {
				break
			}
			current = parent
		}
		return stop
	}
}

// isSubPath reports whether child is the same as or under parent.
func isSubPath(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
