package bilibili

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
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

	// ensures cookies (buvid3) are fetched once
	initOnce sync.Once
	initErr  error
}

// NewClient creates a new Bilibili API client.
func NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
		},
	}
}

// ensureCookies fetches buvid3 and other required cookies from Bilibili.
// This is necessary since the search API started rejecting requests without
// valid fingerprint cookies (returns v_voucher instead of results).
func (c *Client) ensureCookies() {
	c.initOnce.Do(func() {
		// 1. Visit the main site to get basic cookies
		req, err := http.NewRequest("GET", siteURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", userAgent)
			resp, err := c.httpClient.Do(req)
			if err == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}

		// 2. Fetch buvid3/buvid4 from the SPI endpoint
		spiURL := apiBase + "/x/frontend/finger/spi"
		req, err = http.NewRequest("GET", spiURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", userAgent)
			req.Header.Set("Referer", siteURL)
			resp, err := c.httpClient.Do(req)
			if err == nil {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				var spi struct {
					Data struct {
						B3 string `json:"b_3"`
						B4 string `json:"b_4"`
					} `json:"data"`
				}
				if json.Unmarshal(body, &spi) == nil && spi.Data.B3 != "" {
					// Manually set buvid cookies on the API domain
					apiURL, _ := url.Parse(apiBase)
					c.httpClient.Jar.SetCookies(apiURL, []*http.Cookie{
						{Name: "buvid3", Value: spi.Data.B3, Path: "/", Domain: ".bilibili.com"},
						{Name: "buvid4", Value: spi.Data.B4, Path: "/", Domain: ".bilibili.com"},
						{Name: "b_nut", Value: strconv.FormatInt(time.Now().Unix(), 10), Path: "/", Domain: ".bilibili.com"},
					})
					slog.Debug("bilibili: set buvid cookies", "b3", spi.Data.B3[:8]+"...")
				}
			}
		}

		// 3. Visit the search page — this triggers additional cookie
		// generation (b_lsid, _uuid, etc.) that the search API now requires.
		searchPageURL := "https://search.bilibili.com/all?keyword=1"
		req, err = http.NewRequest("GET", searchPageURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", userAgent)
			req.Header.Set("Referer", siteURL)
			resp, err := c.httpClient.Do(req)
			if err == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}
	})
}

// doRequest performs a signed GET request to the Bilibili API.
func (c *Client) doRequest(apiURL string) ([]byte, error) {
	c.ensureCookies()

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
