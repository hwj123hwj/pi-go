package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BackupManager provides per-file backup snapshots for undo/rollback.
//
// Before any destructive file operation (edit/write), call Snapshot() to
// capture the current content. If the operation needs to be undone, call
// Restore() to revert to the snapshot.
//
// Backups are kept in-memory as a stack per file path (normalized).
// The stack depth is capped (default: 50 per file) to prevent unbounded memory.
type BackupManager struct {
	mu       sync.Mutex
	backups  map[string]*fileBackupStack // normalized path → stack
	maxDepth int                        // max snapshots per file
}

type fileBackupStack struct {
	snapshots []fileSnapshot
}

type fileSnapshot struct {
	content  []byte
	modTime  time.Time
	isEmpty  bool // true if the file didn't exist before
}

// NewBackupManager creates a new BackupManager.
func NewBackupManager() *BackupManager {
	return &BackupManager{
		backups:  make(map[string]*fileBackupStack),
		maxDepth: 50,
	}
}

// Snapshot captures the current state of a file before modification.
// If the file doesn't exist, records an "empty" snapshot (for undo → delete).
// Returns a token that can be used with RestoreByToken for targeted restore.
func (bm *BackupManager) Snapshot(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	absPath = filepath.Clean(absPath)

	var snapshot fileSnapshot
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			snapshot = fileSnapshot{isEmpty: true}
		} else {
			return "", fmt.Errorf("stat file: %w", err)
		}
	} else {
		data, err := os.ReadFile(absPath)
		if err != nil {
			return "", fmt.Errorf("read file for backup: %w", err)
		}
		snapshot = fileSnapshot{
			content: data,
			modTime: info.ModTime(),
		}
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	stack, ok := bm.backups[absPath]
	if !ok {
		stack = &fileBackupStack{}
		bm.backups[absPath] = stack
	}

	// Enforce max depth (keep most recent)
	if len(stack.snapshots) >= bm.maxDepth {
		stack.snapshots = stack.snapshots[1:]
	}
	stack.snapshots = append(stack.snapshots, snapshot)

	// Token = path + index (1-based)
	token := fmt.Sprintf("%s#%d", absPath, len(stack.snapshots))
	return token, nil
}

// Restore reverts a file to its last snapshot.
// If the snapshot was "empty", the file is deleted.
func (bm *BackupManager) Restore(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	absPath = filepath.Clean(absPath)

	bm.mu.Lock()
	stack, ok := bm.backups[absPath]
	if !ok || len(stack.snapshots) == 0 {
		bm.mu.Unlock()
		return fmt.Errorf("no backup available for %s", path)
	}

	// Pop the last snapshot
	lastIdx := len(stack.snapshots) - 1
	snapshot := stack.snapshots[lastIdx]
	stack.snapshots = stack.snapshots[:lastIdx]

	// Clean up if stack is empty
	if len(stack.snapshots) == 0 {
		delete(bm.backups, absPath)
	}
	bm.mu.Unlock()

	return bm.applySnapshot(absPath, snapshot)
}

// RestoreAll reverts all files that have backups, in reverse order of snapshot time.
func (bm *BackupManager) RestoreAll() []error {
	bm.mu.Lock()
	paths := make([]string, 0, len(bm.backups))
	for p := range bm.backups {
		paths = append(paths, p)
	}
	bm.mu.Unlock()

	var errs []error
	for _, p := range paths {
		if err := bm.Restore(p); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p, err))
		}
	}
	return errs
}

// applySnapshot writes the snapshot content back to disk.
func (bm *BackupManager) applySnapshot(path string, snapshot fileSnapshot) error {
	if snapshot.isEmpty {
		// File didn't exist before — delete it
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete file on restore: %w", err)
		}
		return nil
	}

	// Ensure parent dir exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	if err := os.WriteFile(path, snapshot.content, 0o644); err != nil {
		return fmt.Errorf("write restored file: %w", err)
	}

	// Restore modTime
	if !snapshot.modTime.IsZero() {
		_ = os.Chtimes(path, snapshot.modTime, snapshot.modTime)
	}

	return nil
}

// HasBackup returns true if a backup exists for the given path.
func (bm *BackupManager) HasBackup(path string) bool {
	absPath, _ := filepath.Abs(path)
	absPath = filepath.Clean(absPath)

	bm.mu.Lock()
	defer bm.mu.Unlock()

	stack, ok := bm.backups[absPath]
	return ok && len(stack.snapshots) > 0
}

// BackupCount returns the number of snapshots for a given path.
func (bm *BackupManager) BackupCount(path string) int {
	absPath, _ := filepath.Abs(path)
	absPath = filepath.Clean(absPath)

	bm.mu.Lock()
	defer bm.mu.Unlock()

	if stack, ok := bm.backups[absPath]; ok {
		return len(stack.snapshots)
	}
	return 0
}

// ListBackups returns a summary of all files with backups.
func (bm *BackupManager) ListBackups() []BackupInfo {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	var infos []BackupInfo
	for path, stack := range bm.backups {
		infos = append(infos, BackupInfo{
			Path:       path,
			Snapshots:  len(stack.snapshots),
		})
	}
	return infos
}

// BackupInfo describes the backup state for a file.
type BackupInfo struct {
	Path      string `json:"path"`
	Snapshots int    `json:"snapshots"`
}

// Clear removes all backups (called on session end or /undo clear).
func (bm *BackupManager) Clear() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.backups = make(map[string]*fileBackupStack)
}

// FormatListBackups returns a human-readable summary of all backups.
func (bm *BackupManager) FormatListBackups() string {
	infos := bm.ListBackups()
	if len(infos) == 0 {
		return "No backups available. Use /undo to restore after file operations."
	}

	var b strings.Builder
	b.WriteString("📂 Available backups:\n\n")
	for _, info := range infos {
		rel := info.Path
		b.WriteString(fmt.Sprintf("  %s (%d snapshot%s)\n", rel, info.Snapshots, pluralS(info.Snapshots)))
	}
	b.WriteString("\nUse /undo to restore the last backup, or /undo all to restore everything.")
	return b.String()
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
