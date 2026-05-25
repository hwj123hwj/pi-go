package deepv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderProvider_HeadersCreatesRepoMetadata(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Setenv("DEEPV_GIT_REMOTE", "https://example.com/org/repo.git"))
	t.Cleanup(func() {
		_ = os.Unsetenv("DEEPV_GIT_REMOTE")
	})

	headers := HeaderProvider{WorkDir: dir}.Headers()
	require.NotNil(t, headers)
	assert.Contains(t, headers, "X-Git-Remotes")

	var remotes map[string]string
	require.NoError(t, json.Unmarshal([]byte(headers["X-Git-Remotes"]), &remotes))
	assert.Equal(t, "https://example.com/org/repo.git", remotes["origin"])
	_, err := os.Stat(filepath.Join(dir, ".git"))
	assert.NoError(t, err)
}
