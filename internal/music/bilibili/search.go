package bilibili

import (
	"encoding/json"
	"fmt"
	"strings"
)

// VideoResult is a processed search result from Bilibili.
type VideoResult struct {
	Bvid     string
	Title    string
	Author   string
	Duration int // milliseconds
	Pic      string
	Play     int64 // view count
}

// Search searches for videos on Bilibili.
func (c *Client) Search(query string, limit int) ([]VideoResult, error) {
	if limit <= 0 {
		limit = 20
	}

	params := map[string]string{
		"search_type": "video",
		"keyword":     query,
		"order":       "totalrank",
		"page":        "1",
	}

	body, err := c.signedRequest("/x/web-interface/wbi/search/type", params)
	if err != nil {
		return nil, err
	}

	var resp SearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode bilibili search: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("bilibili search API error: code=%d msg=%s", resp.Code, resp.Message)
	}

	results := make([]VideoResult, 0, len(resp.Data.Result))
	for i, r := range resp.Data.Result {
		if i >= limit {
			break
		}
		pic := r.Pic
		if pic != "" && !strings.HasPrefix(pic, "http") {
			pic = "https:" + pic
		}
		results = append(results, VideoResult{
			Bvid:     r.Bvid,
			Title:    cleanHTMLTags(r.Title),
			Author:   r.Author,
			Duration: parseDuration(r.Duration),
			Pic:      pic,
			Play:     r.Play,
		})
	}
	// 质量过滤：剔除鼓谱/reaction/教学/混剪等"不是在放这首歌"的视频（见 filter.go）。
	// 在 client 内部过滤，让 adapter/playByQuery/跨源兜底等所有调用方自动受益。
	return filterQualityResults(query, results), nil
}

// GetView fetches video metadata by BV ID.
func (c *Client) GetView(bvid string) (*ViewData, error) {
	apiURL := fmt.Sprintf("%s/x/web-interface/view?bvid=%s", apiBase, bvid)
	body, err := c.doRequest(apiURL)
	if err != nil {
		return nil, err
	}

	var resp ViewResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode view response: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("bilibili view API error: code=%d msg=%s", resp.Code, resp.Message)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("bilibili view: no data for %s", bvid)
	}
	return resp.Data, nil
}

// GetAudioURL fetches the best audio stream URL for a video.
func (c *Client) GetAudioURL(bvid string) (string, error) {
	view, err := c.GetView(bvid)
	if err != nil {
		return "", fmt.Errorf("get video info: %w", err)
	}

	apiURL := fmt.Sprintf(
		"%s/x/player/playurl?bvid=%s&cid=%d&fnval=16&qn=64",
		apiBase, bvid, view.CID,
	)
	body, err := c.doRequest(apiURL)
	if err != nil {
		return "", err
	}

	var resp PlayURLResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("decode playurl response: %w", err)
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("bilibili playurl API error: code=%d msg=%s", resp.Code, resp.Message)
	}
	if resp.Data == nil {
		return "", fmt.Errorf("bilibili playurl: no data for %s", bvid)
	}

	if resp.Data.Dash == nil || len(resp.Data.Dash.Audio) == 0 {
		if len(resp.Data.DURL) > 0 {
			return resp.Data.DURL[0].URL, nil
		}
		return "", fmt.Errorf("bilibili: no audio stream for %s", bvid)
	}

	best := resp.Data.Dash.Audio[0]
	for _, a := range resp.Data.Dash.Audio[1:] {
		if a.Bandwidth > best.Bandwidth {
			best = a
		}
	}
	if best.BaseURL == "" {
		return "", fmt.Errorf("bilibili: empty audio URL for %s", bvid)
	}
	return best.BaseURL, nil
}

// GetRanking fetches the Bilibili ranking for a given category.
// rid=0 for all, rid=3 for music, rid=1 for anime, etc.
func (c *Client) GetRanking(rid int) ([]RankingItem, error) {
	apiURL := fmt.Sprintf("%s/x/web-interface/ranking/v2?rid=%d&type=all", apiBase, rid)
	body, err := c.doRequest(apiURL)
	if err != nil {
		return nil, err
	}

	var resp RankingResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode ranking response: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("bilibili ranking API error: code=%d msg=%s", resp.Code, resp.Message)
	}
	return resp.Data.List, nil
}

// ── Helpers ──

func cleanHTMLTags(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for _, c := range s {
		switch {
		case c == '<':
			inTag = true
		case c == '>':
			inTag = false
		case !inTag:
			b.WriteRune(c)
		}
	}
	return b.String()
}

func parseDuration(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		n := 0
		for _, c := range p {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}
		total = total*60 + n
	}
	return total * 1000
}
