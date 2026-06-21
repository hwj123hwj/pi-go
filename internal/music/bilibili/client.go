package bilibili

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
//  Bilibili API Client
//  Handles wbi-signed requests, search, video info, and audio stream URLs.
// ────────────────────────────────────────────────────────────────────────────

const (
	apiBase = "https://api.bilibili.com"
	siteURL = "https://www.bilibili.com"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// wbiKeyMixinEncTab is the standard Bilibili wbi signature shuffle table.
var wbiKeyMixinEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}

// Client is the Bilibili API client.
type Client struct {
	httpClient *http.Client

	// wbi key cache (refreshed daily)
	mu        sync.RWMutex
	mixinKey  string
	keyExpiry time.Time
}

// NewClient creates a new Bilibili API client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// doRequest performs a signed GET request to the Bilibili API.
func (c *Client) doRequest(apiURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("bilibili: create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", siteURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bilibili: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bilibili: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bilibili: HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 500)]))
	}
	return body, nil
}

// signedRequest builds a wbi-signed URL and performs the request.
func (c *Client) signedRequest(endpoint string, params map[string]string) ([]byte, error) {
	mixinKey, err := c.getMixinKey()
	if err != nil {
		return nil, fmt.Errorf("bilibili: get mixin key: %w", err)
	}

	// Add timestamp
	params["wts"] = strconv.FormatInt(time.Now().Unix(), 10)

	// Sort params by key
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build query string
	var queryParts []string
	for _, k := range keys {
		// Filter invalid characters from values
		v := filterParamValue(params[k])
		queryParts = append(queryParts, url.QueryEscape(k)+"="+url.QueryEscape(v))
	}
	query := strings.Join(queryParts, "&")

	// Compute w_rid
	wRid := fmt.Sprintf("%x", md5.Sum([]byte(query+mixinKey)))

	// Build final URL
	fullURL := apiBase + endpoint + "?" + query + "&w_rid=" + wRid
	return c.doRequest(fullURL)
}

// getMixinKey returns the cached wbi mixin key, refreshing if expired.
func (c *Client) getMixinKey() (string, error) {
	c.mu.RLock()
	if c.mixinKey != "" && time.Now().Before(c.keyExpiry) {
		key := c.mixinKey
		c.mu.RUnlock()
		return key, nil
	}
	c.mu.RUnlock()

	return c.refreshMixinKey()
}

// refreshMixinKey fetches fresh wbi keys from the nav API.
func (c *Client) refreshMixinKey() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.mixinKey != "" && time.Now().Before(c.keyExpiry) {
		return c.mixinKey, nil
	}

	body, err := c.doRequest(apiBase + "/x/web-interface/nav")
	if err != nil {
		return "", fmt.Errorf("fetch nav: %w", err)
	}

	var resp struct {
		Data struct {
			WbiImg struct {
				ImgURL string `json:"img_url"`
				SubURL string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("decode nav response: %w", err)
	}

	imgKey := extractFilename(resp.Data.WbiImg.ImgURL)
	subKey := extractFilename(resp.Data.WbiImg.SubURL)
	if imgKey == "" || subKey == "" {
		return "", fmt.Errorf("bilibili: empty wbi keys")
	}

	// Apply mixin shuffle
	raw := imgKey + subKey
	var b strings.Builder
	b.Grow(32)
	for i := 0; i < 32 && i < len(wbiKeyMixinEncTab); i++ {
		idx := wbiKeyMixinEncTab[i]
		if idx < len(raw) {
			b.WriteByte(raw[idx])
		}
	}
	c.mixinKey = b.String()
	// Expire at next midnight (UTC+8)
	now := time.Now()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	c.keyExpiry = nextMidnight

	slog.Debug("bilibili: refreshed wbi mixin key")
	return c.mixinKey, nil
}

// extractFilename extracts the filename (without extension) from a URL path.
// e.g. "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png"
// → "7cd084941338484aae1ad9425b84077c"
func extractFilename(rawURL string) string {
	parts := strings.Split(rawURL, "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	if dot := strings.LastIndex(last, "."); dot >= 0 {
		return last[:dot]
	}
	return last
}

// filterParamValue removes characters that Bilibili's wbi signing rejects.
func filterParamValue(s string) string {
	replacer := strings.NewReplacer(
		"'", "",
		"!", "",
		"(", "",
		")", "",
		"*", "",
	)
	return replacer.Replace(s)
}
