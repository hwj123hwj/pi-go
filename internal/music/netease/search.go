package netease

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// searchResponse is the JSON structure returned by the NetEase search API.
type searchResponse struct {
	Result struct {
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
		SongCount int `json:"songCount"`
	} `json:"result"`
	Code int `json:"code"`
}

// SearchSongs searches for songs on NetEase Cloud Music.
// query is the search term (song name, artist, etc.), limit is max results (default 10).
func (c *Client) SearchSongs(query string, limit int) (*SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	apiURL := fmt.Sprintf(
		"http://music.163.com/api/search/get?s=%s&type=1&limit=%d",
		url.QueryEscape(query), limit,
	)

	body, err := c.doRequest(apiURL)
	if err != nil {
		return nil, err
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("search API returned code %d", resp.Code)
	}

	songs := make([]Song, 0, len(resp.Result.Songs))
	for _, s := range resp.Result.Songs {
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

	return &SearchResult{
		Songs: songs,
		Total: resp.Result.SongCount,
	}, nil
}
