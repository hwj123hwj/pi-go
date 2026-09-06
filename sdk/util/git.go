package util

import (
	"os"
	"path/filepath"
	"strings"
)

// CurrentGitBranch reads .git/HEAD in the given directory and returns the
// current branch name. Returns empty string on failure.
func CurrentGitBranch(dir string) string {
	if dir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/")
	}
	if len(content) >= 8 {
		return content[:8]
	}
	return content
}

// IsGitRepo returns true if dir contains a .git directory or file.
func IsGitRepo(dir string) bool {
	if dir == "" {
		return false
	}
	fi, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && fi != nil
}
