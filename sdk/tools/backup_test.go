package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupSnapshotAndRestore(t *testing.T) {
	bm := NewBackupManager()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	// Write initial content
	os.WriteFile(path, []byte("original"), 0o644)

	// Snapshot
	if _, err := bm.Snapshot(path); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// Modify the file
	os.WriteFile(path, []byte("modified"), 0o644)

	// Verify modification
	data, _ := os.ReadFile(path)
	if string(data) != "modified" {
		t.Fatalf("Expected modified, got %s", string(data))
	}

	// Restore
	if err := bm.Restore(path); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Verify restoration
	data, _ = os.ReadFile(path)
	if string(data) != "original" {
		t.Errorf("Expected original, got %s", string(data))
	}
}

func TestBackupRestoreNonExistentFile(t *testing.T) {
	bm := NewBackupManager()
	dir := t.TempDir()
	path := filepath.Join(dir, "newfile.txt")

	// Snapshot of non-existent file (should record "empty")
	if _, err := bm.Snapshot(path); err != nil {
		t.Fatalf("Snapshot non-existent: %v", err)
	}

	// Create the file
	os.WriteFile(path, []byte("newly created"), 0o644)

	// Restore should delete it
	if err := bm.Restore(path); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Expected file to be deleted after restore")
	}
}

func TestBackupStack(t *testing.T) {
	bm := NewBackupManager()
	dir := t.TempDir()
	path := filepath.Join(dir, "stack.txt")

	// Write + snapshot 3 times
	for i, content := range []string{"v1", "v2", "v3"} {
		os.WriteFile(path, []byte(content), 0o644)
		if _, err := bm.Snapshot(path); err != nil {
			t.Fatalf("Snapshot %d: %v", i+1, err)
		}
	}

	// Modify to "v4"
	os.WriteFile(path, []byte("v4"), 0o644)

	if bm.BackupCount(path) != 3 {
		t.Errorf("Expected 3 backups, got %d", bm.BackupCount(path))
	}

	// First restore: v4 → v3
	bm.Restore(path)
	data, _ := os.ReadFile(path)
	if string(data) != "v3" {
		t.Errorf("Expected v3, got %s", string(data))
	}

	// Second restore: v3 → v2
	bm.Restore(path)
	data, _ = os.ReadFile(path)
	if string(data) != "v2" {
		t.Errorf("Expected v2, got %s", string(data))
	}
}

func TestBackupRestoreAll(t *testing.T) {
	bm := NewBackupManager()
	dir := t.TempDir()

	// Two files
	path1 := filepath.Join(dir, "a.txt")
	path2 := filepath.Join(dir, "b.txt")

	os.WriteFile(path1, []byte("a-orig"), 0o644)
	os.WriteFile(path2, []byte("b-orig"), 0o644)

	bm.Snapshot(path1)
	bm.Snapshot(path2)

	os.WriteFile(path1, []byte("a-mod"), 0o644)
	os.WriteFile(path2, []byte("b-mod"), 0o644)

	// Restore all
	errs := bm.RestoreAll()
	if len(errs) > 0 {
		t.Fatalf("RestoreAll errors: %v", errs)
	}

	data1, _ := os.ReadFile(path1)
	data2, _ := os.ReadFile(path2)
	if string(data1) != "a-orig" {
		t.Errorf("a.txt = %s, want a-orig", string(data1))
	}
	if string(data2) != "b-orig" {
		t.Errorf("b.txt = %s, want b-orig", string(data2))
	}
}

func TestBackupHasBackup(t *testing.T) {
	bm := NewBackupManager()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if bm.HasBackup(path) {
		t.Error("Should not have backup before snapshot")
	}

	os.WriteFile(path, []byte("test"), 0o644)
	bm.Snapshot(path)

	if !bm.HasBackup(path) {
		t.Error("Should have backup after snapshot")
	}
}

func TestBackupClear(t *testing.T) {
	bm := NewBackupManager()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	os.WriteFile(path, []byte("test"), 0o644)
	bm.Snapshot(path)

	bm.Clear()

	if bm.HasBackup(path) {
		t.Error("Should not have backup after Clear")
	}
}

func TestBackupMaxDepth(t *testing.T) {
	bm := NewBackupManager()
	bm.maxDepth = 3 // small limit for testing

	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	// Take 5 snapshots
	for i := 0; i < 5; i++ {
		os.WriteFile(path, []byte("v"+string(rune('0'+i))), 0o644)
		bm.Snapshot(path)
	}

	// Should only have 3 (oldest evicted)
	if count := bm.BackupCount(path); count != 3 {
		t.Errorf("Expected 3 backups after depth limit, got %d", count)
	}
}

func TestBackupListBackups(t *testing.T) {
	bm := NewBackupManager()
	dir := t.TempDir()

	path1 := filepath.Join(dir, "a.txt")
	path2 := filepath.Join(dir, "b.txt")

	os.WriteFile(path1, []byte("a"), 0o644)
	os.WriteFile(path2, []byte("b"), 0o644)

	bm.Snapshot(path1)
	bm.Snapshot(path1) // 2 snapshots
	bm.Snapshot(path2)  // 1 snapshot

	infos := bm.ListBackups()
	if len(infos) != 2 {
		t.Fatalf("Expected 2 files with backups, got %d", len(infos))
	}

	summary := bm.FormatListBackups()
	if summary == "" {
		t.Error("FormatListBackups should not be empty")
	}
}
