package music

// ────────────────────────────────────────────────────────────────────────────
//  Common types shared across all music sources (netease, bilibili, …).
//  Source-specific packages (internal/music/netease, bilibili) define their
//  own internal response types and adapt them into these common types.
// ────────────────────────────────────────────────────────────────────────────

// Source identifies a music backend.
type Source string

const (
	SourceNetease  Source = "netease"
	SourceBilibili Source = "bilibili"
)

// SourceID composes a source + raw ID into a single string key.
// Examples: "netease:12345", "bilibili:BV1qD4y1U7fs"
func SourceID(source Source, rawID string) string {
	return string(source) + ":" + rawID
}

// ParseSourceID splits a composite ID back into source + raw ID.
// If no ":" prefix is found, defaults to SourceNetease (backward compat).
func ParseSourceID(id string) (Source, string) {
	for i, c := range id {
		if c == ':' {
			return Source(id[:i]), id[i+1:]
		}
	}
	return SourceNetease, id
}

// Song represents a track from any music source.
type Song struct {
	ID         string   `json:"id"`          // Composite ID: "netease:123" or "bilibili:BV1xx"
	Name       string   `json:"name"`        // Track title
	Artist     string   `json:"artist"`      // Primary artist display name (may include multiple with " / ")
	Artists    []string `json:"artists"`     // Individual artist names
	AlbumName  string   `json:"album_name"`  // Album or collection name
	AlbumCover string   `json:"album_cover"` // Cover art URL
	Duration   int      `json:"duration"`    // Duration in milliseconds
	Source     Source   `json:"source"`      // Which backend this came from
}

// SearchResult represents a search response.
type SearchResult struct {
	Query string `json:"query"`
	Songs []Song `json:"songs"`
	HasMore bool  `json:"has_more"`
	Source  Source `json:"source"`
}

// Lyrics holds LRC-format lyrics.
type Lyrics struct {
	LRC      string `json:"lrc"`      // Main LRC text
	TransLRC string `json:"tlyric"`   // Translated LRC text (may be empty)
}

// Playlist represents a song collection.
type Playlist struct {
	ID         string `json:"id"`          // Composite ID
	Name       string `json:"name"`
	Description string `json:"description"`
	CoverURL   string `json:"cover_url"`
	TrackCount int    `json:"track_count"`
	PlayCount  int64  `json:"play_count"`
	Creator    string `json:"creator"`
	Source     Source `json:"source"`
}

// PlaylistDetail contains a playlist and its songs.
type PlaylistDetail struct {
	Playlist Playlist `json:"playlist"`
	Songs    []Song   `json:"songs"`
	Source   Source   `json:"source"`
}

// RankingEntry is a lightweight representation of a ranking list.
type RankingEntry struct {
	ID              string `json:"id"`              // Composite ID
	Name            string `json:"name"`
	Description     string `json:"description"`
	CoverURL        string `json:"cover_url"`
	TrackCount      int    `json:"track_count"`
	PlayCount       int64  `json:"play_count"`
	UpdateFrequency string `json:"update_frequency"`
	Source          Source `json:"source"`
}
