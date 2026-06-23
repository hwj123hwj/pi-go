package netease

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ── Song Detail ──────────────────────────────────────────────────────────────

type detailResponse struct {
	Songs []struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
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
		"%s/api/song/detail?ids=%s",
		c.apiBase(), b.String(),
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
		artists := make([]string, 0, len(s.Artists))
		for _, a := range s.Artists {
			if a.Name != "" {
				artists = append(artists, a.Name)
			}
		}
		cover := ""
		if s.Album.PicURL != "" {
			cover = s.Album.PicURL + "?param=300y300"
		}
		songs = append(songs, Song{
			ID:         s.ID,
			Name:       s.Name,
			Artist:     JoinArtists(artists),
			Artists:    artists,
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
		"%s/api/song/lyric?id=%d&lv=1&kv=1&tv=-1",
		c.apiBase(), songID,
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
// It tries multiple strategies: outer URL (GET range check), then the
// enhance/player API.  As a last resort it returns the outer URL unchecked
// so the proxy handler can try it directly.
func (c *Client) GetAudioURL(songID int64) (string, error) {
	// Strategy 1: outer URL (works for free songs)
	outerURL := fmt.Sprintf("%s/song/media/outer/url?id=%d.mp3", c.apiBase(), songID)
	if c.checkURL(outerURL) {
		return outerURL, nil
	}

	// Strategy 2: enhance/player/url API with multiple bitrate attempts
	for _, br := range []int{320000, 192000, 128000} {
		apiURL := fmt.Sprintf(
			"%s/api/song/enhance/player/url?id=%d&ids=[%d]&br=%d",
			c.apiBase(), songID, songID, br,
		)

		body, err := c.doRequest(apiURL)
		if err != nil {
			continue
		}

		var resp enhancePlayerResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		if len(resp.Data) > 0 && resp.Data[0].URL != "" {
			return resp.Data[0].URL, nil
		}
	}

	// Strategy 3: return the outer URL anyway — the proxy will try to fetch it
	// and if it truly fails the user gets a proper error in the player.  This
	// avoids failing outright when the HEAD check was over-strict (NetEase
	// sometimes blocks HEAD but allows GET).
	return outerURL, nil
}

// checkURL does a GET request with a small range to verify the URL returns
// playable audio. NetEase often blocks HEAD requests but allows GET with Range.
func (c *Client) checkURL(url string) bool {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}
	// Use a small range — we only want to check the URL works, not download.
	req.Header.Set("Range", "bytes=0-1023")
	req.Header.Set("Referer", "https://music.163.com/")
	req.Header.Set("User-Agent", c.randomUA())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	// Drain and discard to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusPartialContent:
		// 206 = range supported, definitely real audio
		return true
	case http.StatusFound, http.StatusMovedPermanently:
		// 302/301 = redirect to CDN
		return true
	case http.StatusOK:
		// 200: must verify content type (NetEase sometimes returns HTML error pages)
		ct := resp.Header.Get("Content-Type")
		return strings.Contains(ct, "audio") || strings.Contains(ct, "mpeg")
	default:
		return false
	}
}
