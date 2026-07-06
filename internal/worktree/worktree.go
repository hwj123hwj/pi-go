package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var ErrNotGitRepository = errors.New("not a git repository")
var ErrCleanWorktree = errors.New("worktree has no changes to commit")

// Manager creates git worktrees for isolated agent workspaces.
type Manager struct {
	BaseDir      string
	BranchPrefix string
	now          func() time.Time
}

// CreateOptions describes the project worktree to create.
type CreateOptions struct {
	ProjectPath string
	Name        string
	Task        string
}

// Info describes a created worktree.
type Info struct {
	SourceRoot        string
	SourceProjectPath string
	WorktreeRoot      string
	ProjectPath       string
	Branch            string
	TaskPath          string
}

// Status reports the current state of a worktree.
type Status struct {
	WorktreeRoot string
	Branch       string
	Short        string
	Dirty        bool
}

// CommitOptions describes a worktree commit and cleanup operation.
type CommitOptions struct {
	SourceRoot   string
	WorktreeRoot string
	Message      string
}

// CommitResult describes a committed worktree branch.
type CommitResult struct {
	Branch     string
	CommitHash string
	Status     string
	Removed    bool
}

// DiscardOptions describes a worktree discard operation.
type DiscardOptions struct {
	SourceRoot   string
	WorktreeRoot string
	Branch       string
}

// DiscardResult describes a discarded worktree.
type DiscardResult struct {
	Removed       bool
	BranchDeleted bool
}

// NewManager returns a manager with pi-go defaults.
func NewManager() *Manager {
	return &Manager{
		BranchPrefix: "pi-go/",
		now:          time.Now,
	}
}

// Create creates a git worktree and writes a TASK.md into its root.
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (*Info, error) {
	if opts.ProjectPath == "" {
		return nil, fmt.Errorf("project path is required")
	}
	absProject, err := filepath.Abs(opts.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("resolve project path: %w", err)
	}
	if realProject, err := filepath.EvalSymlinks(absProject); err == nil {
		absProject = realProject
	}
	info, err := os.Stat(absProject)
	if err != nil {
		return nil, fmt.Errorf("stat project path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project path is not a directory: %s", absProject)
	}

	sourceRoot, err := gitRoot(ctx, absProject)
	if err != nil {
		return nil, err
	}
	relProject, err := filepath.Rel(sourceRoot, absProject)
	if err != nil {
		return nil, fmt.Errorf("resolve project path inside repo: %w", err)
	}
	if relProject == ".." || strings.HasPrefix(relProject, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("project path %s is outside git root %s", absProject, sourceRoot)
	}

	now := m.nowFunc().UTC()
	stamp := fmt.Sprintf("%s-%06d", now.Format("20060102-150405"), now.Nanosecond()/1000)
	slug := slugify(opts.Name)
	if slug == "" {
		slug = slugify(filepath.Base(absProject))
	}
	if slug == "" {
		slug = "task"
	}

	baseDir := m.BaseDir
	if baseDir == "" {
		baseDir = filepath.Join(sourceRoot, ".pi-go", "worktrees")
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create worktree base dir: %w", err)
	}
	if err := ensureInfoExclude(ctx, sourceRoot); err != nil {
		return nil, err
	}

	worktreeRoot := uniquePath(baseDir, slug+"-"+stamp)
	branch := m.branchPrefix() + slug + "-" + stamp
	if out, err := runGit(ctx, sourceRoot, "worktree", "add", "-b", branch, worktreeRoot, "HEAD"); err != nil {
		return nil, fmt.Errorf("create worktree: %w: %s", err, strings.TrimSpace(string(out)))
	}

	projectPath := worktreeRoot
	if relProject != "." {
		projectPath = filepath.Join(worktreeRoot, relProject)
	}
	taskPath := filepath.Join(worktreeRoot, "TASK.md")
	result := &Info{
		SourceRoot:        sourceRoot,
		SourceProjectPath: absProject,
		WorktreeRoot:      worktreeRoot,
		ProjectPath:       projectPath,
		Branch:            branch,
		TaskPath:          taskPath,
	}
	if err := os.WriteFile(taskPath, []byte(renderTask(opts, result, now)), 0o644); err != nil {
		return nil, fmt.Errorf("write task file: %w", err)
	}
	return result, nil
}

// Status returns a short git status for a worktree.
func (m *Manager) Status(ctx context.Context, worktreeRoot string) (*Status, error) {
	root, err := canonicalDir(worktreeRoot)
	if err != nil {
		return nil, err
	}
	branch, err := currentBranch(ctx, root)
	if err != nil {
		return nil, err
	}
	status, err := shortStatus(ctx, root)
	if err != nil {
		return nil, err
	}
	return &Status{
		WorktreeRoot: root,
		Branch:       branch,
		Short:        status,
		Dirty:        strings.TrimSpace(status) != "",
	}, nil
}

// CommitAndRemove commits all worktree changes, then removes the worktree.
func (m *Manager) CommitAndRemove(ctx context.Context, opts CommitOptions) (*CommitResult, error) {
	if strings.TrimSpace(opts.Message) == "" {
		return nil, fmt.Errorf("commit message is required")
	}
	worktreeRoot, err := canonicalDir(opts.WorktreeRoot)
	if err != nil {
		return nil, err
	}
	sourceRoot, err := resolveSourceRoot(ctx, opts.SourceRoot, worktreeRoot)
	if err != nil {
		return nil, err
	}
	branch, err := currentBranch(ctx, worktreeRoot)
	if err != nil {
		return nil, err
	}
	before, err := shortStatus(ctx, worktreeRoot)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(before) == "" {
		return nil, ErrCleanWorktree
	}
	if out, err := runGit(ctx, worktreeRoot, "add", "-A"); err != nil {
		return nil, fmt.Errorf("stage worktree changes: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := runGit(ctx, worktreeRoot, "diff", "--cached", "--quiet"); err == nil {
		return nil, ErrCleanWorktree
	} else if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
		return nil, fmt.Errorf("check staged changes: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := runGit(ctx, worktreeRoot, "commit", "-m", strings.TrimSpace(opts.Message)); err != nil {
		return nil, fmt.Errorf("commit worktree changes: %w: %s", err, strings.TrimSpace(string(out)))
	}
	hashOut, err := runGit(ctx, worktreeRoot, "rev-parse", "--short", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("read commit hash: %w: %s", err, strings.TrimSpace(string(hashOut)))
	}
	result := &CommitResult{
		Branch:     branch,
		CommitHash: strings.TrimSpace(string(hashOut)),
		Status:     before,
	}
	if out, err := runGit(ctx, sourceRoot, "worktree", "remove", worktreeRoot); err != nil {
		return result, fmt.Errorf("remove worktree: %w: %s", err, strings.TrimSpace(string(out)))
	}
	result.Removed = true
	return result, nil
}

// Discard removes a worktree and deletes its branch when provided.
func (m *Manager) Discard(ctx context.Context, opts DiscardOptions) (*DiscardResult, error) {
	worktreeRoot, err := canonicalDir(opts.WorktreeRoot)
	if err != nil {
		return nil, err
	}
	sourceRoot, err := resolveSourceRoot(ctx, opts.SourceRoot, worktreeRoot)
	if err != nil {
		return nil, err
	}
	result := &DiscardResult{}
	if out, err := runGit(ctx, sourceRoot, "worktree", "remove", "--force", worktreeRoot); err != nil {
		return result, fmt.Errorf("remove worktree: %w: %s", err, strings.TrimSpace(string(out)))
	}
	result.Removed = true
	branch := strings.TrimSpace(opts.Branch)
	if branch == "" {
		return result, nil
	}
	if out, err := runGit(ctx, sourceRoot, "branch", "-D", branch); err != nil {
		return result, fmt.Errorf("delete branch: %w: %s", err, strings.TrimSpace(string(out)))
	}
	result.BranchDeleted = true
	return result, nil
}

func (m *Manager) nowFunc() time.Time {
	if m != nil && m.now != nil {
		return m.now()
	}
	return time.Now()
}

func (m *Manager) branchPrefix() string {
	if m == nil || m.BranchPrefix == "" {
		return "pi-go/"
	}
	return m.BranchPrefix
}

func canonicalDir(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("directory is required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve directory: %w", err)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	return abs, nil
}

func resolveSourceRoot(ctx context.Context, sourceRoot, worktreeRoot string) (string, error) {
	if sourceRoot != "" {
		return canonicalDir(sourceRoot)
	}
	out, err := runGit(ctx, worktreeRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w: %s", err, strings.TrimSpace(string(out)))
	}
	for _, block := range strings.Split(strings.TrimSpace(string(out)), "\n\n") {
		lines := strings.Split(block, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "worktree ") {
				return canonicalDir(strings.TrimPrefix(line, "worktree "))
			}
		}
	}
	return gitRoot(ctx, worktreeRoot)
}

func currentBranch(ctx context.Context, dir string) (string, error) {
	out, err := runGit(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("read branch: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func shortStatus(ctx context.Context, dir string) (string, error) {
	out, err := runGit(ctx, dir, "status", "--short")
	if err != nil {
		return "", fmt.Errorf("read status: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func gitRoot(ctx context.Context, dir string) (string, error) {
	out, err := runGit(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNotGitRepository, strings.TrimSpace(string(out)))
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", ErrNotGitRepository
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve git root: %w", err)
	}
	if realRoot, err := filepath.EvalSymlinks(abs); err == nil {
		abs = realRoot
	}
	return abs, nil
}

func ensureInfoExclude(ctx context.Context, sourceRoot string) error {
	out, err := runGit(ctx, sourceRoot, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return fmt.Errorf("resolve git exclude path: %w: %s", err, strings.TrimSpace(string(out)))
	}
	excludePath := strings.TrimSpace(string(out))
	if excludePath == "" {
		return nil
	}
	if !filepath.IsAbs(excludePath) {
		excludePath = filepath.Join(sourceRoot, excludePath)
	}
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return fmt.Errorf("create git exclude dir: %w", err)
	}
	existing, _ := os.ReadFile(excludePath)
	line := ".pi-go/worktrees/"
	if strings.Contains(string(existing), line) {
		return nil
	}
	f, err := os.OpenFile(excludePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open git exclude: %w", err)
	}
	defer f.Close()
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("append git exclude newline: %w", err)
		}
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("append git exclude: %w", err)
	}
	return nil
}

func runGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func uniquePath(baseDir, name string) string {
	path := filepath.Join(baseDir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	for i := 2; ; i++ {
		candidate := filepath.Join(baseDir, fmt.Sprintf("%s-%d", name, i))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

var slugInvalid = regexp.MustCompile(`[^a-z0-9._-]+`)

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = slugInvalid.ReplaceAllString(value, "-")
	value = strings.Trim(value, ".-_")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return value
}

func renderTask(opts CreateOptions, info *Info, createdAt time.Time) string {
	task := strings.TrimSpace(opts.Task)
	if task == "" {
		task = "No task description was provided yet. Use the Feishu group to clarify the goal and update this file."
	}

	var b strings.Builder
	b.WriteString("# Task\n\n")
	b.WriteString(fmt.Sprintf("- Name: %s\n", opts.Name))
	b.WriteString(fmt.Sprintf("- Source project: %s\n", info.SourceProjectPath))
	b.WriteString(fmt.Sprintf("- Worktree root: %s\n", info.WorktreeRoot))
	b.WriteString(fmt.Sprintf("- Branch: %s\n", info.Branch))
	b.WriteString(fmt.Sprintf("- Created at: %s\n\n", createdAt.Format(time.RFC3339)))
	b.WriteString("## Request\n\n")
	b.WriteString(task)
	b.WriteString("\n\n## Notes\n\n")
	b.WriteString("- This worktree is isolated from the source checkout.\n")
	b.WriteString("- Commit, copy, or discard changes before deleting the worktree.\n")
	return b.String()
}
