package lsp

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ─── File path ↔ URI conversion ───────────────────────────────────────────────
//
// LSP uses file:// URIs. On Unix this is straightforward:
//
//	/home/user/project/main.go → file:///home/user/project/main.go
//
// On Windows, drive letters are encoded in the authority component:
//
//	C:\Users\project\main.go → file:///c%3A/Users/project/main.go
//
// Some servers (notably gopls) are picky about the exact format. We normalise
// to the lowercase-drive-letter %3A format which is the most widely accepted.

// PathToURI converts a local file path to a file:// URI.
func PathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	if runtime.GOOS == "windows" {
		// Normalise backslashes to forward slashes
		abs = filepath.ToSlash(abs)
		// Handle drive letter: C:/... → /C:/... then encode colon
		if len(abs) >= 2 && abs[1] == ':' {
			drive := strings.ToLower(abs[:1])
			rest := abs[2:] // skip "C:"
			return "file:///" + drive + "%3A" + rest
		}
		return "file:///" + abs
	}

	// Unix: prefix with file:// and URL-encode special characters
	u := url.URL{Scheme: "file", Path: abs}
	return u.String()
}

// URIToPath converts a file:// URI back to a local file path.
func URIToPath(uri string) string {
	if !strings.HasPrefix(uri, "file:") {
		return uri
	}

	u, err := url.Parse(uri)
	if err != nil {
		// Fallback: strip prefix manually
		return strings.TrimPrefix(strings.TrimPrefix(uri, "file:///"), "file://")
	}

	path := u.Path

	if runtime.GOOS == "windows" {
		// Windows path reconstruction from URI authority + path
		// file:///c%3A/Users/... → C:\Users\...
		if u.Host != "" {
			path = "/" + u.Host + path
		}
		// URL-decode %3A → :
		path = strings.ReplaceAll(path, "%3A", ":")
		path = strings.ReplaceAll(path, "%3a", ":")
		// Extract drive letter: /C:/... → C:\...
		if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
			drive := strings.ToUpper(path[1:2])
			rest := path[3:]
			path = drive + ":" + string(os.PathSeparator) + rest
		}
		return filepath.FromSlash(path)
	}

	// Unix: the path is already correct after url.Parse
	// url.Parse decodes %XX sequences in u.Path
	if path == "" {
		path = u.Path
	}
	return path
}

// languageID returns the LSP language identifier for a file extension.
func languageID(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "cpp"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".html":
		return "html"
	case ".css":
		return "css"
	default:
		return "plaintext"
	}
}
