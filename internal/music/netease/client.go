package netease

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

// Client is an HTTP client for the NetEase Cloud Music API.
type Client struct {
	httpClient *http.Client
	userAgents []string
	baseURL    string // 可空：空则用默认 apiBase()；测试可注入 httptest server 地址来 mock
}

// apiBase 返回 API 根地址。baseURL 非空时用它（测试注入），否则默认网易云线上。
func (c *Client) apiBase() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return "https://music.163.com"
}

// SetBaseURL 注入 API 根地址（测试用，指向 httptest server）。
func (c *Client) SetBaseURL(url string) { c.baseURL = url }

// SetHTTPClient 注入 HTTP 客户端（测试用，指向 httptest.Client）。
func (c *Client) SetHTTPClient(hc *http.Client) { c.httpClient = hc }

// NewClient creates a new NetEase API client with default settings.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		userAgents: defaultUserAgents,
	}
}

// doRequest executes an HTTP GET with the required NetEase headers.
func (c *Client) doRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Referer", "https://music.163.com/")
	req.Header.Set("User-Agent", c.randomUA())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}

func (c *Client) randomUA() string {
	return c.userAgents[rand.Intn(len(c.userAgents))]
}

var defaultUserAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}
