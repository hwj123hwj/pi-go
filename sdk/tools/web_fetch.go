package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/hwj123hwj/pi-go/sdk/agent"
)

// WebFetchTool 抓取给定 URL 的内容并转为 markdown。
// 参考 cc-haha（=Claude Code 官方）WebFetchTool，第一版只做核心：
// URL 校验 + SSRF 防护 + HTTP 抓取 + HTML→markdown + 截断。
type WebFetchTool struct {
	timeout      time.Duration // 单次请求超时
	maxOutputLen int           // 最终 markdown 截断长度（复用 TruncateOutput）
	client       *http.Client  // 自带 CheckRedirect（每跳校验 isPrivateHost）
}

// WebFetchParams 工具参数（结构化，学 cc-haha 非 DeepV）。
type WebFetchParams struct {
	URL    string `json:"url"`    // 必填，http/https
	Prompt string `json:"prompt"` // 可选，对内容的处理意图（第一版仅记录，不二次处理）
}

// WebFetchToolOption configures a WebFetchTool during construction.
type WebFetchToolOption func(*WebFetchTool)

// WithWebFetchTimeout sets the per-request timeout (seconds).
func WithWebFetchTimeout(seconds int) WebFetchToolOption {
	return func(t *WebFetchTool) {
		if seconds > 0 {
			t.timeout = time.Duration(seconds) * time.Second
		}
	}
}

// WithWebFetchMaxOutputLen sets the max markdown truncation length.
func WithWebFetchMaxOutputLen(n int) WebFetchToolOption {
	return func(t *WebFetchTool) { t.maxOutputLen = n }
}

// NewWebFetchTool creates WebFetchTool with optional configuration.
func NewWebFetchTool(opts ...WebFetchToolOption) *WebFetchTool {
	t := &WebFetchTool{
		timeout:      30 * time.Second,
		maxOutputLen: DefaultMaxOutputLen,
		client: &http.Client{
			Timeout: 30 * time.Second,
			// 每跳重定向都校验目标 host，防 302 链绕过入口校验跳到内网
			CheckRedirect: func(req *http.Request, _ []*http.Request) error {
				if isPrivateHost(req.URL.Hostname()) {
					return fmt.Errorf("重定向到内网地址被拒绝: %s", req.URL.Hostname())
				}
				return nil
			},
		},
	}
	for _, opt := range opts {
		opt(t)
	}
	// timeout 改了同步给 client（client.Timeout 管整体含重定向）
	t.client.Timeout = t.timeout
	return t
}

func (t *WebFetchTool) Name() string { return "web_fetch" }
func (t *WebFetchTool) Description() string {
	return "Fetch content from a URL and return it as markdown. " +
		"Useful for reading documentation, API references, or articles. " +
		"IMPORTANT: fails for authenticated/private URLs (e.g. Google Docs, Confluence, private GitHub) — " +
		"only public pages are accessible."
}
func (t *WebFetchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url":    map[string]any{"type": "string", "description": "The URL to fetch (http or https)."},
			"prompt": map[string]any{"type": "string", "description": "Optional: what to extract or focus on from the content."},
		},
		"required": []string{"url"},
	}
}

func (t *WebFetchTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params WebFetchParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if params.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	if err := validateURL(params.URL); err != nil {
		return nil, err
	}
	return json.Marshal(params)
}

func (t *WebFetchTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	var params WebFetchParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	// SSRF：入口校验目标 host（重定向的每跳由 client.CheckRedirect 校验）
	parsed, _ := url.Parse(params.URL) // validateURL 已保证合法
	if isPrivateHost(parsed.Hostname()) {
		return agent.ToolResult{
			Content: fmt.Sprintf("refused: %s resolves to a private/internal address (SSRF protection)", params.URL),
			IsError: true,
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, params.URL, nil)
	if err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	req.Header.Set("User-Agent", "pi-go-agent/1.0 (+https://github.com/hwj123hwj/pi-go)")

	resp, err := t.client.Do(req)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("fetch failed: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return agent.ToolResult{
			Content: fmt.Sprintf("HTTP %d %s for %s", resp.StatusCode, resp.Status, params.URL),
			IsError: true,
		}, nil
	}

	// 只处理 HTML（非 HTML 如图片/JSON/PDF 不转 markdown）
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return agent.ToolResult{
			Content: fmt.Sprintf("unsupported content type %q (only text/html is supported)", ct),
			IsError: true,
		}, nil
	}

	// 限流读取，防超大响应撑爆内存
	body := io.LimitReader(resp.Body, maxHTTPContentLength)
	htmlBytes, err := io.ReadAll(body)
	if err != nil {
		return agent.ToolResult{IsError: true}, err
	}

	// HTML → markdown
	markdown, err := convertToMarkdown(string(htmlBytes))
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("html-to-markdown failed: %v", err), IsError: true}, nil
	}

	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return agent.ToolResult{Content: fmt.Sprintf("no readable content extracted from %s", params.URL), IsError: true}, nil
	}

	// 截断到上下文友好长度
	markdown = TruncateOutput(markdown, t.maxOutputLen)
	return agent.ToolResult{Content: markdown}, nil
}

// convertToMarkdown 把 HTML 转为 markdown。抽成独立函数便于测试（绕过网络）。
func convertToMarkdown(html string) (string, error) {
	converter := md.NewConverter("", true, nil)
	return converter.ConvertString(html)
}
