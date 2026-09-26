package appdir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrateLegacyHomeMovesDataWhenNewDirectoryIsMissing(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, legacyDirName)
	require.NoError(t, os.MkdirAll(filepath.Join(legacy, "sessions"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(legacy, "feishu-credentials.json"), []byte("credentials"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(legacy, "sessions", "history.jsonl"), []byte("history"), 0o600))

	require.NoError(t, migrateLegacyHome(home))
	require.NoFileExists(t, legacy)
	require.FileExists(t, filepath.Join(home, currentDirName, "feishu-credentials.json"))
	require.FileExists(t, filepath.Join(home, currentDirName, "sessions", "history.jsonl"))
}

func TestMigrateLegacyHomeLeavesBothDirectoriesUntouched(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, legacyDirName)
	current := filepath.Join(home, currentDirName)
	require.NoError(t, os.MkdirAll(legacy, 0o700))
	require.NoError(t, os.MkdirAll(current, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(legacy, "legacy.txt"), []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(current, "current.txt"), []byte("new"), 0o600))

	require.ErrorContains(t, migrateLegacyHome(home), "both legacy and current")
	require.FileExists(t, filepath.Join(legacy, "legacy.txt"))
	require.FileExists(t, filepath.Join(current, "current.txt"))
}
