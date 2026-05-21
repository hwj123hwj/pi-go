package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath resolves a possibly-relative path against a workspace root.
// If workspace is non-empty, relative paths are joined with workspace.
// Absolute paths are cleaned but returned as-is.
func ResolvePath(workspace, path string) string {
	path = filepath.Clean(path)
	if filepath.IsAbs(path) {
		return path
	}
	if workspace != "" {
		return filepath.Clean(filepath.Join(workspace, path))
	}
	// No workspace: resolve relative to cwd
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// IsPathSafe checks whether the resolved path stays within the workspace.
// If workspace is empty, it always returns true (no restriction).
func IsPathSafe(workspace, path string) bool {
	if workspace == "" {
		return true
	}
	resolved := ResolvePath(workspace, path)
	absWorkspace := filepath.Clean(workspace)
	if !strings.HasSuffix(absWorkspace, string(filepath.Separator)) {
		absWorkspace += string(filepath.Separator)
	}
	return strings.HasPrefix(resolved, absWorkspace) || resolved == filepath.Clean(workspace)
}

// EnsureDir creates the parent directory of the given path if it doesn't exist.
func EnsureDir(path string) error {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}
	return nil
}

// FormatByteCount returns a human-readable byte count string.
func FormatByteCount(n int) string {
	const (
		KB = 1024
		MB = 1024 * KB
	)
	switch {
	case n >= MB:
		return fmt.Sprintf("%.1fM", float64(n)/float64(MB))
	case n >= KB:
		return fmt.Sprintf("%.1fK", float64(n)/float64(KB))
	default:
		return fmt.Sprintf("%dB", n)
	}
}
