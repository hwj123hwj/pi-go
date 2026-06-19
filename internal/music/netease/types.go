package netease

// Song represents a song from NetEase Cloud Music.
type Song struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Artist     string `json:"artist"`
	AlbumName  string `json:"album_name"`
	AlbumCover string `json:"album_cover"`
	Duration   int    `json:"duration"` // milliseconds
}

// Lyrics holds LRC-format lyrics for a song.
type Lyrics struct {
	LRC      string `json:"lrc"`      // Timestamped LRC lyrics
	TransLRC string `json:"tlyric"`   // Translated lyrics (may be empty)
}

// SearchResult holds songs returned from a search query.
type SearchResult struct {
	Songs []Song `json:"songs"`
	Total int    `json:"total"`
}
