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

func TestValidateCallbackURL_RejectsLocalhost(t *testing.T) {
	for _, url := range []string{
		"http://localhost/tool",
		"http://127.0.0.1/tool",
		"http://[::1]/tool",
		"http://0.0.0.0/tool",
	} {
		err := validateCallbackURL(url)
		assert.Error(t, err, "expected error for URL: %s", url)
	}
}

func TestValidateCallbackURL_RejectsPrivateIPs(t *testing.T) {
	for _, url := range []string{
		"http://10.0.0.1/tool",
		"http://192.168.1.1/tool",
		"http://172.16.0.1/tool",
		"http://169.254.169.254/latest/meta-data/",
		"http://169.254.1.1/tool",
	} {
		err := validateCallbackURL(url)
		assert.Error(t, err, "expected error for private IP URL: %s", url)
	}
}

func TestValidateCallbackURL_RejectsUnresolvableHost(t *testing.T) {
	err := validateCallbackURL("http://this-host-definitely-does-not-exist-xyz123/tool")
	assert.Error(t, err)
}

func TestNewExternalTool_RejectsBadURL(t *testing.T) {
	for _, url := range []string{
		"file:///etc/passwd",
		"http://localhost/tool",
		"http://192.168.1.1/tool",
	} {
		_, err := NewExternalTool(ExternalToolDef{
			Name:        "bad-tool",
			Description: "bad",
			CallbackURL: url,
		})
		assert.Error(t, err, "expected error for URL: %s", url)
	}
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
