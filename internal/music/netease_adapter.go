package music

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hwj123hwj/pi-go/internal/music/netease"
)

// ────────────────────────────────────────────────────────────────────────────
//  NetEase adapter: wraps *netease.Client to implement MusicSource.
//  Lives in the music package to avoid circular imports.
// ────────────────────────────────────────────────────────────────────────────

// Compile-time check.
var _ MusicSource = (*NetEaseAdapter)(nil)

// NetEaseAdapter wraps a netease.Client and implements MusicSource.
type NetEaseAdapter struct {
	client *netease.Client
}

// NewNetEaseAdapter creates a MusicSource backed by NetEase Cloud Music.
func NewNetEaseAdapter(client *netease.Client) *NetEaseAdapter {
	return &NetEaseAdapter{client: client}
}

func (a *NetEaseAdapter) Source() Source { return SourceNetease }

func (a *NetEaseAdapter) Search(ctx context.Context, query string, limit int) (*SearchResult, error) {
	result, err := a.client.SearchSongs(query, limit)
	if err != nil {
		return nil, err
	}
	songs := make([]Song, 0, len(result.Songs))
	for _, s := range result.Songs {
		songs = append(songs, neteaseToSong(s))
	}
	return &SearchResult{
		Query:   query,
		Songs:   songs,
		HasMore: result.Total > len(result.Songs),
		Source:  SourceNetease,
	}, nil
}

func (a *NetEaseAdapter) GetSongByID(ctx context.Context, rawID string) (*Song, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid netease song id %q: %w", rawID, err)
	}
	songs, err := a.client.GetSongDetail([]int64{id})
	if err != nil {
		return nil, err
	}
	if len(songs) == 0 {
		return nil, fmt.Errorf("song not found: %s", rawID)
	}
	s := neteaseToSong(songs[0])
	return &s, nil
}

func (a *NetEaseAdapter) GetAudioURL(ctx context.Context, rawID string) (string, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid netease song id %q: %w", rawID, err)
	}
	return a.client.GetAudioURL(id)
}

func (a *NetEaseAdapter) GetLyrics(ctx context.Context, rawID string) (*Lyrics, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid netease song id %q: %w", rawID, err)
	}
	lyrics, err := a.client.GetLyrics(id)
	if err != nil {
		return nil, err
	}
	return &Lyrics{LRC: lyrics.LRC, TransLRC: lyrics.TransLRC}, nil
}

func (a *NetEaseAdapter) GetPlaylistDetail(ctx context.Context, rawID string) (*PlaylistDetail, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid netease playlist id %q: %w", rawID, err)
	}
	detail, err := a.client.GetPlaylistDetail(id)
	if err != nil {
		return nil, err
	}
	return neteaseToPlaylistDetail(detail), nil
}

func (a *NetEaseAdapter) GetRankings(ctx context.Context) ([]RankingEntry, error) {
	rankings, err := a.client.GetRankings()
	if err != nil {
		return nil, err
	}
	entries := make([]RankingEntry, 0, len(rankings))
	for _, r := range rankings {
		entries = append(entries, RankingEntry{
			ID:              SourceID(SourceNetease, strconv.FormatInt(r.ID, 10)),
			Name:            r.Name,
			Description:     r.Description,
			CoverURL:        r.CoverURL,
			TrackCount:      r.TrackCount,
			PlayCount:       r.PlayCount,
			UpdateFrequency: r.UpdateFrequency,
			Source:          SourceNetease,
		})
	}
	return entries, nil
}

func (a *NetEaseAdapter) GetTopList(ctx context.Context, rawID string) (*PlaylistDetail, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid netease ranking id %q: %w", rawID, err)
	}
	detail, err := a.client.GetTopList(id)
	if err != nil {
		return nil, err
	}
	return neteaseToPlaylistDetail(detail), nil
}

func (a *NetEaseAdapter) GetNewSongs(ctx context.Context, limit int) ([]Song, error) {
	songs, err := a.client.GetNewSongs(limit)
	if err != nil {
		return nil, err
	}
	out := make([]Song, 0, len(songs))
	for _, s := range songs {
		out = append(out, neteaseToSong(s))
	}
	return out, nil
}

func (a *NetEaseAdapter) GetDailyRecommend(ctx context.Context) (*PlaylistDetail, error) {
	return nil, nil
}

// ── Conversion helpers ──

func neteaseToSong(s netease.Song) Song {
	return Song{
		ID:         SourceID(SourceNetease, strconv.FormatInt(s.ID, 10)),
		Name:       s.Name,
		Artist:     s.Artist,
		Artists:    s.Artists,
		AlbumName:  s.AlbumName,
		AlbumCover: s.AlbumCover,
		Duration:   s.Duration,
		Source:     SourceNetease,
	}
}

func neteaseToPlaylist(p netease.Playlist) Playlist {
	return Playlist{
		ID:          SourceID(SourceNetease, strconv.FormatInt(p.ID, 10)),
		Name:        p.Name,
		Description: p.Description,
		CoverURL:    p.CoverURL,
		TrackCount:  p.TrackCount,
		PlayCount:   p.PlayCount,
		Creator:     p.Creator,
		Source:      SourceNetease,
	}
}

func neteaseToPlaylistDetail(d *netease.PlaylistDetail) *PlaylistDetail {
	if d == nil {
		return nil
	}
	songs := make([]Song, 0, len(d.Songs))
	for _, s := range d.Songs {
		songs = append(songs, neteaseToSong(s))
	}
	return &PlaylistDetail{
		Playlist: neteaseToPlaylist(d.Playlist),
		Songs:    songs,
		Source:   SourceNetease,
	}
}
