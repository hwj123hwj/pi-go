package music

import "context"

// ────────────────────────────────────────────────────────────────────────────
//  MusicSource is the unified interface that every music backend implements.
//  Tools and the HTTP handler interact with music exclusively through this
//  interface, making the system source-agnostic.
// ────────────────────────────────────────────────────────────────────────────

// MusicSource defines the contract for a music backend.
type MusicSource interface {
	// Source returns the backend identifier.
	Source() Source

	// Search finds tracks matching the query.
	Search(ctx context.Context, query string, limit int) (*SearchResult, error)

	// GetSongByID fetches a single song's metadata by its raw ID.
	// The rawID is the source-specific identifier (e.g. "12345" for netease,
	// "BV1xx" for bilibili), NOT the composite "source:id" form.
	GetSongByID(ctx context.Context, rawID string) (*Song, error)

	// GetAudioURL returns a playable audio URL for the given raw song ID.
	// The URL may be time-limited; callers should cache with appropriate TTL.
	GetAudioURL(ctx context.Context, rawID string) (string, error)

	// GetLyrics returns LRC lyrics for the given raw song ID.
	// If the source has no lyrics, returns an empty Lyrics (no error).
	GetLyrics(ctx context.Context, rawID string) (*Lyrics, error)

	// GetPlaylistDetail fetches a playlist and its songs.
	GetPlaylistDetail(ctx context.Context, rawID string) (*PlaylistDetail, error)

	// GetRankings returns available ranking charts.
	GetRankings(ctx context.Context) ([]RankingEntry, error)

	// GetTopList fetches a specific ranking list.
	GetTopList(ctx context.Context, rawID string) (*PlaylistDetail, error)

	// GetNewSongs returns personalized new song recommendations.
	GetNewSongs(ctx context.Context, limit int) ([]Song, error)

	// GetDailyRecommend returns daily personalized recommendations.
	// Sources that don't support this return nil with no error.
	GetDailyRecommend(ctx context.Context) (*PlaylistDetail, error)
}
