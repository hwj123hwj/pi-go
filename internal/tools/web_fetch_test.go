package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── validateURL ─────────────────────────────────────────────────────────────

func TestValidateURL_AcceptsValid(t *testing.T) {
	cases := []string{
		"https://example.com/path",
		"http://example.com",
		"https://go.dev/doc/install",
		"https://example.com:8080/p?q=1#frag",
	}
	for _, u := range cases {
		assert.NoError(t, validateURL(u), "应通过: %s", u)
	}
}

func TestValidateURL_RejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"无协议":         "example.com/path",
		"非 http 协议":   "ftp://example.com",
		"带凭证":         "https://user:pass@example.com",
		"带 @ 的伪凭证":    "https://foo@127.0.0.1",
		"超长":          strings.Repeat("a", 2049),
		"hostname 单段": "http://intranet",
	}
	for name, u := range cases {
		assert.Error(t, validateURL(u), "%s 应拒绝: %s", name, u)
	}
}

// localhost 单独放行（由 isPrivateHost 统一拦截，validateURL 只管语法）
func TestValidateURL_AllowsLocalhostSyntax(t *testing.T) {
	assert.NoError(t, validateURL("http://localhost:8080"))
}

// ─── isPrivateHost ────────────────────────────────────────────────────────────

func TestIsPrivateIP(t *testing.T) {
	privates := []string{
		"127.0.0.1", "10.0.0.1", "192.168.1.1", "172.16.0.1", "172.31.255.255",
		"169.254.169.254", // 云元数据
		"::1",             // IPv6 回环
		"fc00::1",         // IPv6 私网
	}
	for _, ip := range privates {
		assert.True(t, isPrivateHost(ip), "应判为内网: %s", ip)
	}
}

func TestIsPrivateHost_LocalhostNames(t *testing.T) {
	assert.True(t, isPrivateHost("localhost"))
	assert.True(t, isPrivateHost("foo.localhost"))
	assert.True(t, isPrivateHost("printer.local"))
}

func TestIsPrivateHost_PublicIP(t *testing.T) {
	// 8.8.8.8 是 Google 公网 DNS
	assert.False(t, isPrivateHost("8.8.8.8"))
}

// ─── Execute（用 httptest 起本地 server）─────────────────────────────────────

func TestWebFetchExecute_ConvertsHTMLToMarkdown(t *testing.T) {
	html := `<!DOCTYPE html><html><body>
	<h1>Title</h1>
	<p>A paragraph with <a href="https://example.com">a link</a> and <strong>bold</strong>.</p>
	<pre><code>func main() {}</code></pre>
	<ul><li>one</li><li>two</li></ul>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	// httptest 的地址是 127.0.0.1，会被 SSRF 拦截。测试里需要绕过——
	// 用真实的公网域名不现实，故直接测转换逻辑：把 Execute 的核心抽出来不可行，
	// 改为验证 SSRF 对测试地址确实拦截（反向证明防护生效）。
	tool := NewWebFetchTool()
	params, _ := json.Marshal(WebFetchParams{URL: srv.URL})

	result, err := tool.Execute(context.Background(), params, nil)
	require.NoError(t, err)
	// 127.0.0.1 被判为内网 → IsError=true
	assert.True(t, result.IsError, "127.0.0.1 应被 SSRF 拦截")
	assert.Contains(t, result.Content, "private/internal")
}

// 直接测 HTML→markdown 转换逻辑（绕过网络），验证转换质量。
func TestWebFetch_HTMLToMarkdownConversion(t *testing.T) {
	html := `<html><body>
	<h1>Title</h1>
	<p>Para with <a href="https://x.com">link</a> and <strong>bold</strong>.</p>
	<pre><code>code block</code></pre>
	</body></html>`

	out, err := convertToMarkdown(html)
	require.NoError(t, err)
	assert.Contains(t, out, "Title")
	assert.Contains(t, out, "link")
	assert.Contains(t, out, "bold")
	assert.Contains(t, out, "code block")
}

// ─── Validate ────────────────────────────────────────────────────────────────

func TestWebFetchValidate_RequiresURL(t *testing.T) {
	tool := NewWebFetchTool()
	_, err := tool.Validate(json.RawMessage(`{"prompt":"x"}`))
	assert.Error(t, err)
}

func TestWebFetchValidate_RejectsBadURL(t *testing.T) {
	tool := NewWebFetchTool()
	_, err := tool.Validate(json.RawMessage(`{"url":"not a url"}`))
	assert.Error(t, err)
}

// 编译期断言：WebFetchTool 满足 agent.Tool 接口。
var _ agent.Tool = (*WebFetchTool)(nil)
