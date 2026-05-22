package operations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "'simple'"},
		{"with space", "'with space'"},
		{"it's", "'it'\\''s'"},
		{"", "''"},
		{"/path/to/file", "'/path/to/file'"},
	}

	for _, tt := range tests {
		got := shellQuote(tt.input)
		assert.Equal(t, tt.expected, got, "shellQuote(%q)", tt.input)
	}
}

func TestBuildSSHArgs(t *testing.T) {
	tests := []struct {
		name     string
		config   SSHConfig
		expected []string
	}{
		{
			name:     "default port",
			config:   SSHConfig{Host: "user@host", Port: 22},
			expected: []string{"user@host", "--"},
		},
		{
			name:     "custom port",
			config:   SSHConfig{Host: "user@host", Port: 2222},
			expected: []string{"-p", "2222", "user@host", "--"},
		},
		{
			name:     "no host",
			config:   SSHConfig{Port: 22},
			expected: []string{"--"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSSHArgs(tt.config)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseLsOutput(t *testing.T) {
	output := `total 24
drwxr-xr-x   5 user  staff   160 Jan 15 10:00 .
drwxr-xr-x   3 user  staff    96 Jan 15 10:00 ..
-rw-r--r--   1 user  staff   100 Jan 15 10:00 file.txt
drwxr-xr-x   2 user  staff    64 Jan 15 10:00 subdir
-rw-r--r--   1 user  staff   200 Jan 15 10:00 other.go`

	entries, err := parseLsOutput(output)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(entries))

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	assert.True(t, names["file.txt"])
	assert.True(t, names["subdir"])
	assert.True(t, names["other.go"])

	// Check directory detection
	for _, e := range entries {
		if e.Name == "subdir" {
			assert.True(t, e.IsDir)
		} else {
			assert.False(t, e.IsDir)
		}
	}
}

func TestParentDirSSH(t *testing.T) {
	assert.Equal(t, "/home/user", parentDirSSH("/home/user/file.txt"))
	assert.Equal(t, "/", parentDirSSH("/file.txt"))
	assert.Equal(t, "/a/b", parentDirSSH("/a/b/c"))
}

func TestParseStatOutput(t *testing.T) {
	info, err := parseStatOutput("1024|2024-01-15 10:00:00|regular file", "test.txt")
	assert.NoError(t, err)
	assert.Equal(t, "test.txt", info.Name)
	assert.Equal(t, int64(1024), info.Size)
	assert.False(t, info.IsDir)

	info, err = parseStatOutput("64|2024-01-15 10:00:00|directory", "mydir")
	assert.NoError(t, err)
	assert.True(t, info.IsDir)
}

func TestNewSSHOperations(t *testing.T) {
	ops := NewSSHOperations(SSHConfig{
		Host: "user@host",
		Port: 22,
	})
	assert.NotNil(t, ops.Bash)
	assert.NotNil(t, ops.Files)

	_, ok := ops.Bash.(SSHBashOperations)
	assert.True(t, ok)

	_, ok = ops.Files.(SSHFileOperations)
	assert.True(t, ok)
}
