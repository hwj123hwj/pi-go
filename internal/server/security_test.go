package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hwj123hwj/easyagent/internal/app"
	"github.com/hwj123hwj/easyagent/sdk/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestAppWithWorkspace 构造指定 workspace 的测试 App（Config() 返回值拷贝，
// 必须在 New 前设置）。
func newTestAppWithWorkspace(t *testing.T, ws string) *app.App {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.Provider = "openai"
	cfg.OpenAIAPIKey = "test-key"
	cfg.OpenAIBaseURL = "http://localhost:4001"
	cfg.Workspace = ws
	application, err := app.New(app.AppOptions{Config: cfg})
	require.NoError(t, err)
	t.Cleanup(func() { application.Close() })
	return application
}

// ─── 路径收窄 ────────────────────────────────────────────────────────────────

// realPath 解析 macOS /var→/private/var 之类的符号链接，供断言对齐。
func realPath(t *testing.T, p string) string {
	t.Helper()
	rp, err := filepath.EvalSymlinks(p)
	require.NoError(t, err)
	return rp
}

func TestSecurePath_InsideWorkspace(t *testing.T) {
	ws := realPath(t, t.TempDir())
	p, err := securePath(ws, "a/b.txt")
	require.NoError(t, err)
	assert.True(t, within(p, ws), "resolved path should stay inside workspace")

	p, err = securePath(ws, filepath.Join(ws, "sub", "file.go"))
	require.NoError(t, err)
	assert.True(t, within(p, ws))
}

func TestSecurePath_RejectsEscape(t *testing.T) {
	ws := t.TempDir()
	for _, target := range []string{
		"/etc/passwd",
		"../../etc/passwd",
		filepath.Join(ws, "..", "outside.txt"),
		"/",
	} {
		_, err := securePath(ws, target)
		assert.Error(t, err, "target %q should be rejected", target)
	}
}

func TestSecurePath_SymlinkEscape(t *testing.T) {
	ws := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(ws, "evil")
	require.NoError(t, os.Symlink(outside, link))

	// 通过 workspace 内的符号链接访问外部文件 → 拒绝
	_, err := securePath(ws, filepath.Join("evil", "secret.txt"))
	assert.Error(t, err, "symlink escape should be rejected")
}

func TestSecurePath_NonExistentInsideWorkspace(t *testing.T) {
	ws := realPath(t, t.TempDir())
	p, err := securePath(ws, "new/dir/file.txt")
	require.NoError(t, err)
	assert.True(t, within(p, ws))
}

func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ─── 访问控制 ────────────────────────────────────────────────────────────────

func TestAuth_WithAPIKey(t *testing.T) {
	application := newTestApp(t)
	srv := New(application, nil)
	srv.SetAPIKey("secret-key")
	handler := srv.Handler()

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"wrong key", "Bearer wrong", http.StatusUnauthorized},
		{"correct key", "Bearer secret-key", http.StatusOK},
		{"correct key no prefix", "secret-key", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, tc.want, w.Code)
		})
	}

	// /health 始终开放
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_DefaultLoopbackOnly(t *testing.T) {
	application := newTestApp(t)
	srv := New(application, nil) // 无 API key
	handler := srv.Handler()

	// httptest.NewRequest 默认 RemoteAddr 为 192.0.2.1（非 loopback）；
	// 显式设为 loopback 模拟本机请求 → 放行
	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	req.RemoteAddr = "127.0.0.1:55555"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 伪造非 loopback 来源 → 401
	req = httptest.NewRequest(http.MethodGet, "/sessions", nil)
	req.RemoteAddr = "203.0.113.5:44444"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp.Error, "EA_API_KEY")
}

func TestWSAuth(t *testing.T) {
	application := newTestApp(t)
	srv := New(application, nil)
	srv.SetAPIKey("secret-key")

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	assert.False(t, srv.wsAuthorized(req))

	req = httptest.NewRequest(http.MethodGet, "/ws?token=secret-key", nil)
	assert.True(t, srv.wsAuthorized(req))

	req = httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Bearer secret-key")
	assert.True(t, srv.wsAuthorized(req))
}

// ─── HTTP 端到端：文件端点被限制在 workspace 内 ─────────────────────────────

// localReq 构造 loopback 来源的测试请求（httptest 默认 RemoteAddr 非回环）。
func localReq(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.RemoteAddr = "127.0.0.1:55555"
	return req
}

func TestWorkspaceEndpoints_PathContainment(t *testing.T) {
	ws := t.TempDir()
	application := newTestAppWithWorkspace(t, ws)
	require.NoError(t, os.WriteFile(filepath.Join(ws, "hello.txt"), []byte("hi"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "sub"), 0o755))

	srv := New(application, nil)
	handler := srv.Handler()

	// 读 workspace 内文件 → 200
	req := localReq(http.MethodGet, "/workspace/read-file?path=hello.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 读外部绝对路径 → 400
	req = localReq(http.MethodGet, "/workspace/read-file?path=/etc/hosts", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// ../ 穿越 → 400
	req = localReq(http.MethodGet, "/workspace/read-file?path="+filepath.Join("..", "..", "etc", "passwd"), nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// list-dir 外部目录 → 400
	req = localReq(http.MethodGet, "/workspace/list-dir?path=/etc", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 写外部 → 400
	body := bytes.NewReader([]byte(`{"content":"pwned"}`))
	req = localReq(http.MethodPut, "/workspace/write-file?path=/tmp/easyagent-pwned.txt", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	_, err := os.Stat("/tmp/easyagent-pwned.txt")
	assert.True(t, os.IsNotExist(err), "external write must not land on disk")
}

func TestSessionFile_PathContainment(t *testing.T) {
	ws := t.TempDir()
	application := newTestAppWithWorkspace(t, ws)

	srv := New(application, nil)
	handler := srv.Handler()

	// 创建会话（workspace = ws）
	body := bytes.NewReader([]byte(`{"cwd":"` + ws + `"}`))
	req := localReq(http.MethodPost, "/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var sess struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&sess))
	require.NotEmpty(t, sess.ID)

	// 会话内读 → 200
	req = localReq(http.MethodGet, "/sessions/"+sess.ID+"/file?path=.", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusBadRequest, w.Code)

	// 会话外读 → 400
	req = localReq(http.MethodGet, "/sessions/"+sess.ID+"/file?path=/etc/passwd", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
