package deepv

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultFakeRemoteURL = "https://gitlab.liebaopay.com/fake/pi-go-workspace.git"

// HeaderProvider supplies coding-agent repository metadata expected by DeepV.
type HeaderProvider struct {
	WorkDir string
}

// Headers returns DeepV-specific repository headers, or nil when no repo metadata is available.
func (p HeaderProvider) Headers() map[string]string {
	info := p.gitInfo()
	if info == nil {
		return nil
	}

	headers := map[string]string{}
	if remotesJSON, err := json.Marshal(info.Remotes); err == nil {
		headers["X-Git-Remotes"] = string(remotesJSON)
	}
	if info.Branch != "" {
		headers["X-Git-Branch"] = info.Branch
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

type gitInfo struct {
	Remotes map[string]string
	Branch  string
}

func (p HeaderProvider) gitInfo() *gitInfo {
	workDir := p.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	remotes := p.gitRemotes(workDir)
	if len(remotes) == 0 {
		remotes = p.ensureFakeRemote(workDir)
	}

	info := &gitInfo{
		Remotes: remotes,
		Branch:  p.gitBranch(workDir),
	}
	if len(info.Remotes) == 0 {
		return nil
	}
	return info
}

func (p HeaderProvider) ensureFakeRemote(dir string) map[string]string {
	remotes := make(map[string]string)
	fakeURL := os.Getenv("DEEPV_GIT_REMOTE")
	if fakeURL == "" {
		fakeURL = defaultFakeRemoteURL
	}

	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		initCmd := exec.Command("git", "init")
		initCmd.Dir = dir
		if err := initCmd.Run(); err != nil {
			slog.Debug("failed to git init for fake remote", "dir", dir, "error", err)
			return remotes
		}
		slog.Info("initialized git repo for DeepV remote", "dir", dir)
	}

	addCmd := exec.Command("git", "remote", "add", "origin", fakeURL)
	addCmd.Dir = dir
	if err := addCmd.Run(); err != nil {
		setCmd := exec.Command("git", "remote", "set-url", "origin", fakeURL)
		setCmd.Dir = dir
		if err := setCmd.Run(); err != nil {
			slog.Debug("failed to add/set fake remote", "dir", dir, "error", err)
			return remotes
		}
	}
	slog.Info("set fake git remote for DeepV", "dir", dir, "url", fakeURL)

	remotes["origin"] = fakeURL
	return remotes
}

func (p HeaderProvider) gitRemotes(dir string) map[string]string {
	remotes := make(map[string]string)
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return remotes
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "(fetch)") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				remotes[parts[0]] = parts[1]
			}
		}
	}
	return remotes
}

func (p HeaderProvider) gitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
