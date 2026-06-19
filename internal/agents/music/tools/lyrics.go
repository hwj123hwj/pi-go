package musictools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/music"
	"github.com/earendil-works/pi-go/internal/music/netease"
)

// LyricsTool fetches LRC-format lyrics for a song.
type LyricsTool struct {
	client *netease.Client
	cache  *music.Cache
}

func NewLyricsTool(client *netease.Client, cache *music.Cache) *LyricsTool {
	return &LyricsTool{client: client, cache: cache}
}

func (t *LyricsTool) Name() string { return "music_lyrics" }
func (t *LyricsTool) Description() string {
	return "Get LRC-format lyrics (with timestamps) for a song by its ID. Optionally includes translated lyrics."
}

func (t *LyricsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"song_id": map[string]any{
				"type":        "integer",
				"description": "The song ID (from music_search results)",
			},
		},
		"required": []string{"song_id"},
	}
}

func (t *LyricsTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		SongID int64 `json:"song_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if p.SongID == 0 {
		return nil, fmt.Errorf("song_id is required")
	}
	return params, nil
}

func (t *LyricsTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		SongID int64 `json:"song_id"`
	}
	_ = json.Unmarshal(params, &p)

	lyrics, err := t.getLyrics(p.SongID)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("Failed to get lyrics: %v", err), IsError: true}, nil
	}

	if lyrics.LRC == "" {
		return agent.ToolResult{Content: "No lyrics available for this song."}, nil
	}

	output := lyrics.LRC
	if lyrics.TransLRC != "" {
		output += "\n\n--- Translation ---\n" + lyrics.TransLRC
	}

	return agent.ToolResult{Content: output}, nil
}

func (t *LyricsTool) getLyrics(songID int64) (*netease.Lyrics, error) {
	cached := t.cache.Get(music.LyricsKey(songID))
	if cached != nil {
		return cached.(*netease.Lyrics), nil
	}

	lyrics, err := t.client.GetLyrics(songID)
	if err != nil {
		return nil, err
	}

	t.cache.Set(music.LyricsKey(songID), lyrics, music.TTLLyrics)
	return lyrics, nil
}

// IsConcurrencySafe declares this tool is safe to run concurrently.
func (t *LyricsTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
