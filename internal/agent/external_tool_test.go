package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCallbackURL_AllowsHTTP(t *testing.T) {
	assert.NoError(t, validateCallbackURL("http://example.com/tool"))
	assert.NoError(t, validateCallbackURL("https://example.com/tool"))
}

func TestValidateCallbackURL_AllowsLocalhost(t *testing.T) {
	// Localhost and private IPs are now allowed (needed for Feishu bridge etc.)
	for _, url := range []string{
		"http://localhost/tool",
		"http://127.0.0.1/tool",
		"http://[::1]/tool",
		"http://0.0.0.0/tool",
		"http://10.0.0.1/tool",
		"http://192.168.1.1/tool",
		"http://172.16.0.1/tool",
	} {
		assert.NoError(t, validateCallbackURL(url), "expected no error for URL: %s", url)
	}
}

func TestValidateCallbackURL_RejectsInvalidScheme(t *testing.T) {
	for _, url := range []string{
		"file:///etc/passwd",
		"ftp://example.com",
		"gopher://evil.com",
		"javascript:alert(1)",
	} {
		err := validateCallbackURL(url)
		assert.Error(t, err, "expected error for URL: %s", url)
	}
}

func TestNewExternalTool_AcceptsLocalhost(t *testing.T) {
	tool, err := NewExternalTool(ExternalToolDef{
		Name:        "test",
		Description: "test tool",
		CallbackURL: "http://127.0.0.1:9090/tool-callback",
	})
	require.NoError(t, err)
	assert.Equal(t, "test", tool.Name())
}

func TestNewExternalTool_AcceptsValidURL(t *testing.T) {
	tool, err := NewExternalTool(ExternalToolDef{
		Name:        "test",
		Description: "test tool",
		CallbackURL: "http://example.com/test",
	})
	require.NoError(t, err)
	assert.Equal(t, "test", tool.Name())
}

func TestNewExternalTool_RejectsInvalidScheme(t *testing.T) {
	_, err := NewExternalTool(ExternalToolDef{
		Name:        "bad-tool",
		Description: "bad",
		CallbackURL: "file:///etc/passwd",
	})
	assert.Error(t, err)
}
