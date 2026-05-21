package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/earendil-works/pi-go/internal/app"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestApp(t *testing.T) *app.App {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	application, err := app.New(app.AppOptions{Config: cfg})
	require.NoError(t, err)
	t.Cleanup(func() { application.Close() })
	return application
}

func TestServer_Health(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "ok", resp["status"])
}

func TestServer_Chat(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

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
	application := newTestApp(t)
	srv := New(application)

	body := bytes.NewReader([]byte(`{"prompt":""}`))
	req := httptest.NewRequest(http.MethodPost, "/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServer_Chat_InvalidJSON(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	body := bytes.NewReader([]byte(`invalid json`))
	req := httptest.NewRequest(http.MethodPost, "/chat", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServer_Tools(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest(http.MethodGet, "/tools", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_Sessions(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_CreateSession(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest(http.MethodPost, "/sessions", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp SessionResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp.ID)
}

func TestServer_DeleteSession(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	// Create first
	req := httptest.NewRequest(http.MethodPost, "/sessions", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp SessionResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	// Delete
	req2 := httptest.NewRequest(http.MethodDelete, "/sessions/"+resp.ID, nil)
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestServer_SessionMessages_NotFound(t *testing.T) {
	application := newTestApp(t)
	srv := New(application)

	req := httptest.NewRequest(http.MethodGet, "/sessions/nonexistent/messages", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
