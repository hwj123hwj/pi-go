package util

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// HomeDir returns the current user's home directory.
func HomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if runtime.GOOS == "windows" {
		if home := os.Getenv("USERPROFILE"); home != "" {
			return home
		}
	}
	return ""
}

// RunShell runs a shell command and returns combined output.
// dir sets the working directory; if empty, uses current directory.
func RunShell(dir string, cmd string, timeoutSeconds int) (string, error) {
	args := []string{"-c", cmd}
	shell := "/bin/sh"
	if runtime.GOOS == "windows" {
		shell = "cmd"
		args = []string{"/C", cmd}
	}

	c := exec.Command(shell, args...)
	if dir != "" {
		c.Dir = dir
	}
	out, err := c.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("shell command failed: %w", err)
	}
	return string(out), nil
}

// CWD returns the current working directory, or "." on error.
func CWD() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
