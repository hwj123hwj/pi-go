package worktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreate(t *testing.T) {
	requireGit(t)

	repo := initRepo(t)
	manager := NewManager()
	manager.now = func() time.Time {
		return time.Date(2026, 7, 6, 1, 2, 3, 456789000, time.UTC)
	}

	info, err := manager.Create(context.Background(), CreateOptions{
		ProjectPath: repo,
		Name:        "我的 Project!",
		Task:        "Build the worktree feature.",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if info.SourceRoot != repo {
		t.Fatalf("SourceRoot = %q, want %q", info.SourceRoot, repo)
	}
	if info.ProjectPath != info.WorktreeRoot {
		t.Fatalf("ProjectPath = %q, want worktree root %q", info.ProjectPath, info.WorktreeRoot)
	}
	if !strings.HasPrefix(info.Branch, "pi-go/project-20260706-010203-456789") {
		t.Fatalf("Branch = %q", info.Branch)
	}
	if _, err := os.Stat(filepath.Join(info.WorktreeRoot, ".git")); err != nil {
		t.Fatalf("worktree .git missing: %v", err)
	}

	task, err := os.ReadFile(info.TaskPath)
	if err != nil {
		t.Fatalf("read TASK.md: %v", err)
	}
	taskText := string(task)
	for _, want := range []string{"Build the worktree feature.", info.Branch, info.SourceProjectPath} {
		if !strings.Contains(taskText, want) {
			t.Fatalf("TASK.md missing %q:\n%s", want, taskText)
		}
	}

	exclude, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	if !strings.Contains(string(exclude), ".pi-go/worktrees/") {
		t.Fatalf("exclude missing worktree ignore:\n%s", string(exclude))
	}
}

func TestCreateWithProjectSubdir(t *testing.T) {
	requireGit(t)

	repo := initRepo(t)
	subdir := filepath.Join(repo, "internal", "app")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "app.go"), []byte("package app\n"), 0o644); err != nil {
		t.Fatalf("write subdir file: %v", err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "add subdir")

	info, err := NewManager().Create(context.Background(), CreateOptions{
		ProjectPath: subdir,
		Name:        "subdir task",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	want := filepath.Join(info.WorktreeRoot, "internal", "app")
	if info.ProjectPath != want {
		t.Fatalf("ProjectPath = %q, want %q", info.ProjectPath, want)
	}
	if _, err := os.Stat(filepath.Join(info.ProjectPath, "app.go")); err != nil {
		t.Fatalf("subdir file missing in worktree: %v", err)
	}
}

func TestCreateNonGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := NewManager().Create(context.Background(), CreateOptions{
		ProjectPath: dir,
		Name:        "plain",
	})
	if !errors.Is(err, ErrNotGitRepository) {
		t.Fatalf("error = %v, want ErrNotGitRepository", err)
	}
}

func TestStatus(t *testing.T) {
	requireGit(t)

	info, err := NewManager().Create(context.Background(), CreateOptions{
		ProjectPath: initRepo(t),
		Name:        "status task",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	status, err := NewManager().Status(context.Background(), info.WorktreeRoot)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.Branch != info.Branch {
		t.Fatalf("Branch = %q, want %q", status.Branch, info.Branch)
	}
	if !status.Dirty {
		t.Fatal("expected dirty worktree because TASK.md is untracked")
	}
	if !strings.Contains(status.Short, "TASK.md") {
		t.Fatalf("Short status missing TASK.md:\n%s", status.Short)
	}
}

func TestCommitAndRemove(t *testing.T) {
	requireGit(t)

	repo := initRepo(t)
	manager := NewManager()
	info, err := manager.Create(context.Background(), CreateOptions{
		ProjectPath: repo,
		Name:        "commit task",
		Task:        "commit me",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(info.WorktreeRoot, "README.md"), []byte("# changed\n"), 0o644); err != nil {
		t.Fatalf("write worktree file: %v", err)
	}

	result, err := manager.CommitAndRemove(context.Background(), CommitOptions{
		SourceRoot:   info.SourceRoot,
		WorktreeRoot: info.WorktreeRoot,
		Message:      "complete task",
	})
	if err != nil {
		t.Fatalf("CommitAndRemove failed: %v", err)
	}
	if result.CommitHash == "" {
		t.Fatal("expected commit hash")
	}
	if result.Branch != info.Branch {
		t.Fatalf("Branch = %q, want %q", result.Branch, info.Branch)
	}
	if !result.Removed {
		t.Fatal("expected worktree to be removed")
	}
	if _, err := os.Stat(info.WorktreeRoot); !os.IsNotExist(err) {
		t.Fatalf("worktree still exists or stat failed: %v", err)
	}
	if !branchExists(t, repo, info.Branch) {
		t.Fatalf("expected branch %q to remain", info.Branch)
	}
	readme := gitOutput(t, repo, "show", info.Branch+":README.md")
	if !strings.Contains(readme, "# changed") {
		t.Fatalf("branch README not committed:\n%s", readme)
	}
}

func TestCommitAndRemoveCleanWorktree(t *testing.T) {
	requireGit(t)

	info, err := NewManager().Create(context.Background(), CreateOptions{
		ProjectPath: initRepo(t),
		Name:        "clean task",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	git(t, info.WorktreeRoot, "add", "-A")
	git(t, info.WorktreeRoot, "commit", "-m", "record task")

	_, err = NewManager().CommitAndRemove(context.Background(), CommitOptions{
		SourceRoot:   info.SourceRoot,
		WorktreeRoot: info.WorktreeRoot,
		Message:      "nothing else",
	})
	if !errors.Is(err, ErrCleanWorktree) {
		t.Fatalf("error = %v, want ErrCleanWorktree", err)
	}
}

func TestDiscard(t *testing.T) {
	requireGit(t)

	repo := initRepo(t)
	manager := NewManager()
	info, err := manager.Create(context.Background(), CreateOptions{
		ProjectPath: repo,
		Name:        "discard task",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(info.WorktreeRoot, "scratch.txt"), []byte("discard me\n"), 0o644); err != nil {
		t.Fatalf("write scratch file: %v", err)
	}

	result, err := manager.Discard(context.Background(), DiscardOptions{
		SourceRoot:   info.SourceRoot,
		WorktreeRoot: info.WorktreeRoot,
		Branch:       info.Branch,
	})
	if err != nil {
		t.Fatalf("Discard failed: %v", err)
	}
	if !result.Removed {
		t.Fatal("expected worktree to be removed")
	}
	if !result.BranchDeleted {
		t.Fatal("expected branch to be deleted")
	}
	if _, err := os.Stat(info.WorktreeRoot); !os.IsNotExist(err) {
		t.Fatalf("worktree still exists or stat failed: %v", err)
	}
	if branchExists(t, repo, info.Branch) {
		t.Fatalf("expected branch %q to be deleted", info.Branch)
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func initRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	git(t, repo, "init")
	git(t, repo, "config", "user.email", "pi-go@example.com")
	git(t, repo, "config", "user.name", "pi-go")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# repo\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")
	realRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("eval repo symlink: %v", err)
	}
	return realRepo
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func branchExists(t *testing.T, dir, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = dir
	return cmd.Run() == nil
}
