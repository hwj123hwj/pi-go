package coding

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/hwj123hwj/easyagent/sdk/config"
	"github.com/hwj123hwj/easyagent/sdk/slashcmd"
	"github.com/stretchr/testify/require"
)

func TestGatewayCatalogIsAuthoritative(t *testing.T) {
	t.Setenv("EA_MODELS_FILE", filepath.Join(t.TempDir(), "missing.json"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"coding"},{"id":"claude-sonnet-4-6"}]}`))
	}))
	defer server.Close()
	cfg := config.Default()
	cfg.Provider, cfg.OpenAIBaseURL, cfg.OpenAIAPIKey = "openai", server.URL+"/v1", "test-key"
	app := NewCodingApplication(cfg)
	require.ElementsMatch(t, []slashcmd.ModelInfo{
		{Provider: "openai", ModelID: "coding"},
		{Provider: "openai", ModelID: "claude-sonnet-4-6"},
	}, app.AvailableModels())
	def, ok := app.modelReg.Get("claude-sonnet-4-6")
	require.True(t, ok)
	require.Equal(t, 200000, def.ContextWindow)
}

func TestGatewayEmptyCatalogDoesNotInventModels(t *testing.T) {
	t.Setenv("EA_MODELS_FILE", filepath.Join(t.TempDir(), "missing.json"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()
	cfg := config.Default()
	cfg.Provider, cfg.OpenAIBaseURL = "openai", server.URL
	require.Empty(t, NewCodingApplication(cfg).AvailableModels())
}
