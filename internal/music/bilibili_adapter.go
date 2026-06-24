package music

import (
	"context"
	"fmt"

	"github.com/hwj123hwj/pi-go/internal/music/bilibili"
)

// ────────────────────────────────────────────────────────────────────────────
//  Bilibili adapter: wraps *bilibili.Client to implement MusicSource.
//  Lives in the music package to avoid circular imports.
// ────────────────────────────────────────────────────────────────────────────

// Compile-time check.
var _ MusicSource = (*BilibiliAdapter)(nil)

// BilibiliAdapter wraps a bilibili.Client and implements MusicSource.
type BilibiliAdapter struct {
	client *bilibili.Client
}

// NewBilibiliAdapter creates a MusicSource backed by Bilibili.
func NewBilibiliAdapter(client *bilibili.Client) *BilibiliAdapter {
	return &BilibiliAdapter{client: client}
}

func (a *BilibiliAdapter) Source() Source { return SourceBilibili }

func (a *BilibiliAdapter) Search(ctx context.Context, query string, limit int) (*SearchResult, error) {
	results, err := a.client.Search(query, limit)
	if err != nil {
		return nil, err
	}
	songs := make([]Song, 0, len(results))
	for _, r := range results {
		songs = append(songs, Song{
			ID:         SourceID(SourceBilibili, r.Bvid),
			Name:       r.Title,
			Artist:     r.Author,
			Artists:    []string{r.Author},
			AlbumCover: r.Pic,
			Duration:   r.Duration,
			Source:     SourceBilibili,
		})
	}
	return &SearchResult{
		Query:   query,
		Songs:   songs,
		HasMore: len(results) >= limit,
		Source:  SourceBilibili,
	}, nil
}

func (a *BilibiliAdapter) GetSongByID(ctx context.Context, rawID string) (*Song, error) {
	view, err := a.client.GetView(rawID)
	if err != nil {
		return nil, err
	}
	return bilibiliViewToSong(view), nil
}

func (a *BilibiliAdapter) GetAudioURL(ctx context.Context, rawID string) (string, error) {
	return a.client.GetAudioURL(rawID)
}

func (a *BilibiliAdapter) GetLyrics(ctx context.Context, rawID string) (*Lyrics, error) {
	return &Lyrics{}, nil
}

func (a *BilibiliAdapter) GetPlaylistDetail(ctx context.Context, rawID string) (*PlaylistDetail, error) {
	return nil, fmt.Errorf("bilibili favorites not yet implemented")
}

func (a *BilibiliAdapter) GetRankings(ctx context.Context) ([]RankingEntry, error) {
	categories := []struct {
		rid  int
		name string
	}{
		{0, "全站"},
		{3, "音乐"},
		{1, "动画"},
	}
	entries := make([]RankingEntry, 0, len(categories))
	for _, cat := range categories {
		entries = append(entries, RankingEntry{
			ID:          SourceID(SourceBilibili, fmt.Sprintf("ranking:%d", cat.rid)),
			Name:        cat.name + "排行榜",
			Description: "B站" + cat.name + "热门视频排行",
			Source:      SourceBilibili,
		})
	}
	return entries, nil
}

func (a *BilibiliAdapter) GetTopList(ctx context.Context, rawID string) (*PlaylistDetail, error) {
	rid := 0
	fmt.Sscanf(rawID, "ranking:%d", &rid)

	items, err := a.client.GetRanking(rid)
	if err != nil {
		return nil, err
	}

	songs := make([]Song, 0, len(items))
	for _, item := range items {
		songs = append(songs, Song{
			ID:         SourceID(SourceBilibili, item.Bvid),
			Name:       item.Title,
			Artist:     item.Owner.Name,
			Artists:    []string{item.Owner.Name},
			AlbumCover: item.Pic,
			Duration:   item.Duration * 1000,
			Source:     SourceBilibili,
		})
	}

	name := "B站排行榜"
	switch rid {
	case 3:
		name = "B站音乐排行榜"
	case 1:
		name = "B站动画排行榜"
	}

	return &PlaylistDetail{
		Playlist: Playlist{
			ID:     SourceID(SourceBilibili, rawID),
			Name:   name,
			Source: SourceBilibili,
		},
		Songs:  songs,
		Source: SourceBilibili,
	}, nil
}

func (a *BilibiliAdapter) GetNewSongs(ctx context.Context, limit int) ([]Song, error) {
	detail, err := a.GetTopList(ctx, "ranking:3")
	if err != nil {
		return nil, err
	}
	songs := detail.Songs
	if limit > 0 && len(songs) > limit {
		songs = songs[:limit]
	}
	return songs, nil
}

func (a *BilibiliAdapter) GetDailyRecommend(ctx context.Context) (*PlaylistDetail, error) {
	return nil, nil
}

func bilibiliViewToSong(v *bilibili.ViewData) *Song {
	return &Song{
		ID:         SourceID(SourceBilibili, v.Bvid),
		Name:       v.Title,
		Artist:     v.Owner.Name,
		Artists:    []string{v.Owner.Name},
		AlbumCover: v.Pic,
		Duration:   v.Duration * 1000,
		Source:     SourceBilibili,
	}
}
