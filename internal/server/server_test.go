package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/earendil-works/pi-go/internal/ai/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer() *Server {
	registry := providers.NewRegistry()
	registry.Register(providers.NewMockProvider())

	ag := agent.New(agent.Options{
		Model: ai.Model{
			ID:       "mock",
			Name:     "mock",
			Provider: "mock",
		},
		Registry: registry,
		System:   "test",
		MaxTurns: 5,
	})
	return New(ag)
}

func TestServer_Health(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "ok", resp["status"])
}

func TestServer_Chat(t *testing.T) {
	srv := newTestServer()

	body := bytes.NewReader([]byte(`{"prompt":"hello"}`))
	req := httptest.NewRequest(http.MethodPost, "/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ChatResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp.Text, "MockProvider")
}

func TestServer_Chat_EmptyPrompt(t *testing.T) {
	srv := newTestServer()

	body := bytes.NewReader([]byte(`{"prompt":""}`))
	req := httptest.NewRequest(http.MethodPost, "/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServer_Chat_InvalidJSON(t *testing.T) {
	srv := newTestServer()

	body := bytes.NewReader([]byte(`invalid json`))
	req := httptest.NewRequest(http.MethodPost, "/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServer_Tools(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/tools", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_Sessions(t *testing.T) {
	srv := newTestServer()
	srv.dataDir = t.TempDir()

	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_CreateSession(t *testing.T) {
	srv := newTestServer()
	srv.dataDir = t.TempDir()

	req := httptest.NewRequest(http.MethodPost, "/sessions", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp SessionResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp.ID)
}

func TestServer_DeleteSession(t *testing.T) {
	srv := newTestServer()
	srv.dataDir = t.TempDir()

	// 先创建
	req := httptest.NewRequest(http.MethodPost, "/sessions", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp SessionResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	// 再删除
	req2 := httptest.NewRequest(http.MethodDelete, "/sessions/"+resp.ID, nil)
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestServer_SessionMessages_NotFound(t *testing.T) {
	srv := newTestServer()
	srv.dataDir = t.TempDir()

	req := httptest.NewRequest(http.MethodGet, "/sessions/nonexistent/messages", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
