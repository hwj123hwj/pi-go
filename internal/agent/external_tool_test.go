package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCallbackURL_AllowsHTTP(t *testing.T) {
	assert.NoError(t, validateCallbackURL("http://example.com/tool", false))
	assert.NoError(t, validateCallbackURL("https://example.com/tool", false))
}

func TestValidateCallbackURL_AllowsLocalhostWithAllowPrivate(t *testing.T) {
	// With allowPrivate=true, localhost and private IPs are allowed
	// (needed for Feishu bridge etc.)
	for _, url := range []string{
		"http://localhost/tool",
		"http://127.0.0.1/tool",
		"http://[::1]/tool",
		"http://0.0.0.0/tool",
		"http://10.0.0.1/tool",
		"http://192.168.1.1/tool",
		"http://172.16.0.1/tool",
	} {
		assert.NoError(t, validateCallbackURL(url, true), "expected no error for URL: %s", url)
	}
}

func TestValidateCallbackURL_BlocksPrivateByDefault(t *testing.T) {
	// By default (allowPrivate=false), private IPs are blocked (SSRF protection)
	for _, url := range []string{
		"http://127.0.0.1/tool",
		"http://10.0.0.1/tool",
		"http://192.168.1.1/tool",
		"http://169.254.169.254/latest/meta-data/",
	} {
		err := validateCallbackURL(url, false)
		assert.Error(t, err, "expected error for private URL: %s", url)
	}
}

func TestValidateCallbackURL_RejectsInvalidScheme(t *testing.T) {
	for _, url := range []string{
		"file:///etc/passwd",
		"ftp://example.com",
		"gopher://evil.com",
		"javascript:alert(1)",
	} {
		err := validateCallbackURL(url, false)
		assert.Error(t, err, "expected error for URL: %s", url)
	}
}

func TestNewExternalTool_AcceptsLocalhostWithAllowPrivate(t *testing.T) {
	tool, err := NewExternalTool(ExternalToolDef{
		Name:         "test",
		Description:  "test tool",
		CallbackURL:  "http://127.0.0.1:9090/tool-callback",
		AllowPrivate: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "test", tool.Name())
}

func TestNewExternalTool_RejectsLocalhostByDefault(t *testing.T) {
	// Without AllowPrivate, localhost should be rejected
	_, err := NewExternalTool(ExternalToolDef{
		Name:        "test",
		Description: "test tool",
		CallbackURL: "http://127.0.0.1:9090/tool-callback",
	})
	assert.Error(t, err, "expected error for localhost without AllowPrivate")
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
