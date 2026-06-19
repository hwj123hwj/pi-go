package netease

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ── Song Detail ──────────────────────────────────────────────────────────────

type detailResponse struct {
	Songs []struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
		Album struct {
			Name   string `json:"name"`
			PicURL string `json:"picUrl"`
		} `json:"album"`
		Duration int `json:"duration"`
	} `json:"songs"`
	Code int `json:"code"`
}

// GetSongDetail fetches detailed information for one or more songs.
// ids should contain at most 50 song IDs.
func (c *Client) GetSongDetail(ids []int64) ([]Song, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > 50 {
		ids = ids[:50]
	}

	// Build ids parameter: [123,456,789]
	var b strings.Builder
	b.WriteString("[")
	for i, id := range ids {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "%d", id)
	}
	b.WriteString("]")

	apiURL := fmt.Sprintf(
		"http://music.163.com/api/song/detail?ids=%s",
		b.String(),
	)

	body, err := c.doRequest(apiURL)
	if err != nil {
		return nil, err
	}

	var resp detailResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode detail response: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("detail API returned code %d", resp.Code)
	}

	songs := make([]Song, 0, len(resp.Songs))
	for _, s := range resp.Songs {
		artist := ""
		if len(s.Artists) > 0 {
			artist = s.Artists[0].Name
		}
		cover := ""
		if s.Album.PicURL != "" {
			cover = s.Album.PicURL + "?param=300y300"
		}
		songs = append(songs, Song{
			ID:         s.ID,
			Name:       s.Name,
			Artist:     artist,
			AlbumName:  s.Album.Name,
			AlbumCover: cover,
			Duration:   s.Duration,
		})
	}
	return songs, nil
}

// ── Lyrics ───────────────────────────────────────────────────────────────────

type lyricsResponse struct {
	LRC struct {
		Lyric string `json:"lyric"`
	} `json:"lrc"`
	TLyric struct {
		Lyric string `json:"lyric"`
	} `json:"tlyric"`
	Code int `json:"code"`
}

// GetLyrics fetches LRC-format lyrics for a song.
func (c *Client) GetLyrics(songID int64) (*Lyrics, error) {
	apiURL := fmt.Sprintf(
		"http://music.163.com/api/song/lyric?id=%d&lv=1&kv=1&tv=-1",
		songID,
	)

	body, err := c.doRequest(apiURL)
	if err != nil {
		return nil, err
	}

	var resp lyricsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode lyrics response: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("lyrics API returned code %d", resp.Code)
	}

	return &Lyrics{
		LRC:      resp.LRC.Lyric,
		TransLRC: resp.TLyric.Lyric,
	}, nil
}

// ── Audio URL ────────────────────────────────────────────────────────────────

type enhancePlayerResponse struct {
	Data []struct {
		URL string `json:"url"`
	} `json:"data"`
	Code int `json:"code"`
}

// GetAudioURL returns a playable audio URL for the given song.
// It uses the outer URL as primary method, falling back to the enhance/player API.
func (c *Client) GetAudioURL(songID int64) (string, error) {
	// Strategy 1: outer URL (works for free songs)
	outerURL := fmt.Sprintf("https://music.163.com/song/media/outer/url?id=%d.mp3", songID)
	if c.checkURL(outerURL) {
		return outerURL, nil
	}

	// Strategy 2: enhance/player/url API (CDN direct link)
	apiURL := fmt.Sprintf(
		"http://music.163.com/api/song/enhance/player/url?id=%d&ids=[%d]&br=320000",
		songID, songID,
	)

	body, err := c.doRequest(apiURL)
	if err != nil {
		return "", fmt.Errorf("enhance player API: %w", err)
	}

	var resp enhancePlayerResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("decode enhance player response: %w", err)
	}

	if len(resp.Data) == 0 || resp.Data[0].URL == "" {
		return "", fmt.Errorf("no audio URL available for song %d (may require VIP)", songID)
	}

	return resp.Data[0].URL, nil
}

// checkURL does a HEAD request to verify the URL returns a valid audio response.
func (c *Client) checkURL(url string) bool {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Referer", "https://music.163.com/")
	req.Header.Set("User-Agent", c.randomUA())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Accept 200 with audio content type, or 302 redirect (outer URL redirects to CDN)
	if resp.StatusCode == http.StatusOK {
		ct := resp.Header.Get("Content-Type")
		return strings.Contains(ct, "audio") || strings.Contains(ct, "mpeg")
	}
	return resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently
}
