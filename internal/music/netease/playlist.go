package netease

import (
	"encoding/json"
	"fmt"
)

// ── Playlist Detail ─────────────────────────────────────────────────────────

type playlistDetailResponse struct {
	Playlist struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		CoverImgURL string `json:"coverImgUrl"`
		TrackCount  int    `json:"trackCount"`
		PlayCount   int64  `json:"playCount"`
		Creator     struct {
			Nickname string `json:"nickname"`
		} `json:"creator"`
		Tracks []struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"ar"`
			Album struct {
				Name   string `json:"name"`
				PicURL string `json:"picUrl"`
			} `json:"al"`
			Duration int `json:"dt"`
		} `json:"tracks"`
		TrackIds []struct {
			ID int64 `json:"id"`
		} `json:"trackIds"`
	} `json:"playlist"`
	Code int `json:"code"`
}

// GetPlaylistDetail fetches a playlist's metadata and songs.
// For large playlists (1000+ tracks), it fetches track details separately via trackIds.
func (c *Client) GetPlaylistDetail(playlistID int64) (*PlaylistDetail, error) {
	apiURL := fmt.Sprintf(
		"%s/api/v6/playlist/detail?id=%d",
		c.apiBase(), playlistID,
	)

	body, err := c.doRequest(apiURL)
	if err != nil {
		return nil, err
	}

	var resp playlistDetailResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode playlist detail response: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("playlist detail API returned code %d", resp.Code)
	}

	p := resp.Playlist
	playlist := Playlist{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		CoverURL:    p.CoverImgURL + "?param=300y300",
		TrackCount:  p.TrackCount,
		PlayCount:   p.PlayCount,
		Creator:     p.Creator.Nickname,
	}

	// If tracks are included directly (small playlists), use them.
	var songs []Song
	if len(p.Tracks) > 0 {
		songs = make([]Song, 0, len(p.Tracks))
		for _, t := range p.Tracks {
			artists := make([]string, 0, len(t.Artists))
			for _, a := range t.Artists {
				if a.Name != "" {
					artists = append(artists, a.Name)
				}
			}
			cover := ""
			if t.Album.PicURL != "" {
				cover = t.Album.PicURL + "?param=300y300"
			}
			songs = append(songs, Song{
				ID:         t.ID,
				Name:       t.Name,
				Artist:     JoinArtists(artists),
				Artists:    artists,
				AlbumName:  t.Album.Name,
				AlbumCover: cover,
				Duration:   t.Duration,
			})
		}
	} else if len(p.TrackIds) > 0 {
		// Large playlist: fetch first 50 songs by ID.
		ids := make([]int64, 0, 50)
		for i, tid := range p.TrackIds {
			if i >= 50 {
				break
			}
			ids = append(ids, tid.ID)
		}
		songs, err = c.GetSongDetail(ids)
		if err != nil {
			return nil, fmt.Errorf("fetch playlist tracks: %w", err)
		}
	}

	return &PlaylistDetail{
		Playlist: playlist,
		Songs:    songs,
	}, nil
}

// ── Rankings (Top List) ─────────────────────────────────────────────────────

type toplistResponse struct {
	List []struct {
		ID              int64  `json:"id"`
		Name            string `json:"name"`
		Description     string `json:"description"`
		CoverImgURL     string `json:"coverImgUrl"`
		TrackCount      int    `json:"trackCount"`
		PlayCount       int64  `json:"playCount"`
		UpdateFrequency string `json:"updateFrequency"`
	} `json:"list"`
	Code int `json:"code"`
}

// RankingEntry is a lightweight representation of a ranking list.
type RankingEntry struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	CoverURL        string `json:"cover_url"`
	TrackCount      int    `json:"track_count"`
	PlayCount       int64  `json:"play_count"`
	UpdateFrequency string `json:"update_frequency"`
}

// GetRankings fetches the list of all available ranking charts.
func (c *Client) GetRankings() ([]RankingEntry, error) {
	body, err := c.doRequest(c.apiBase() + "/api/toplist")
	if err != nil {
		return nil, err
	}

	var resp toplistResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode toplist response: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("toplist API returned code %d", resp.Code)
	}

	rankings := make([]RankingEntry, 0, len(resp.List))
	for _, r := range resp.List {
		rankings = append(rankings, RankingEntry{
			ID:              r.ID,
			Name:            r.Name,
			Description:     r.Description,
			CoverURL:        r.CoverImgURL + "?param=300y300",
			TrackCount:      r.TrackCount,
			PlayCount:       r.PlayCount,
			UpdateFrequency: r.UpdateFrequency,
		})
	}
	return rankings, nil
}

// Well-known ranking playlist IDs on NetEase Cloud Music.
const (
	RankSoaring  int64 = 19723756 // 飙升榜
	RankHot      int64 = 3778678  // 热歌榜
	RankNew      int64 = 3779629  // 新歌榜
	RankOriginal int64 = 2884035  // 原创榜
)

// GetTopList fetches a ranking list by its playlist ID.
// Returns the ranking metadata and top songs (max 50).
func (c *Client) GetTopList(listID int64) (*PlaylistDetail, error) {
	return c.GetPlaylistDetail(listID)
}

// ── New Songs (Personalized) ────────────────────────────────────────────────

type newSongsResponse struct {
	Result []struct {
		Song struct {
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
		} `json:"song"`
	} `json:"result"`
	Code int `json:"code"`
}

// GetNewSongs returns recommended new songs (no auth required).
func (c *Client) GetNewSongs(limit int) ([]Song, error) {
	if limit <= 0 {
		limit = 10
	}

	apiURL := fmt.Sprintf(
		"%s/api/personalized/newsong?limit=%d",
		c.apiBase(), limit,
	)

	body, err := c.doRequest(apiURL)
	if err != nil {
		return nil, err
	}

	var resp newSongsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode new songs response: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("new songs API returned code %d", resp.Code)
	}

	songs := make([]Song, 0, len(resp.Result))
	for _, item := range resp.Result {
		s := item.Song
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
