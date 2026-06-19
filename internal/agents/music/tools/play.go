package musictools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/music"
	"github.com/earendil-works/pi-go/internal/music/netease"
)

// PlayTool gets a playable audio URL for a song.
type PlayTool struct {
	client       *netease.Client
	cache        *music.Cache
	audioBaseURL string // e.g. "http://localhost:8081/music/audio"
}

func NewPlayTool(client *netease.Client, cache *music.Cache, audioBaseURL string) *PlayTool {
	return &PlayTool{client: client, cache: cache, audioBaseURL: audioBaseURL}
}

func (t *PlayTool) Name() string { return "music_play" }
func (t *PlayTool) Description() string {
	return "Get a playable audio URL for a song by its ID. Returns the streaming URL, song name, and artist."
}

func (t *PlayTool) Parameters() map[string]any {
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

func (t *PlayTool) Validate(params json.RawMessage) (json.RawMessage, error) {
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

func (t *PlayTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		SongID int64 `json:"song_id"`
	}
	_ = json.Unmarshal(params, &p)

	// Get audio URL (cached)
	audioURL, err := t.getAudioURL(p.SongID)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("Failed to get audio URL: %v", err), IsError: true}, nil
	}

	// Get song detail for display name
	detail, _ := t.client.GetSongDetail([]int64{p.SongID})
	songName := fmt.Sprintf("Song #%d", p.SongID)
	artist := ""
	if len(detail) > 0 {
		songName = detail[0].Name
		artist = detail[0].Artist
	}

	// Build proxy URL (avoids CORS/Referer issues for the frontend)
	proxyURL := audioURL
	if t.audioBaseURL != "" {
		proxyURL = fmt.Sprintf("%s/%d", t.audioBaseURL, p.SongID)
	}

	output := fmt.Sprintf(
		"🎵 Now playing: %s — %s\n\n"+
			"Direct URL: %s\n"+
			"Proxy URL: %s\n\n"+
			"Use the proxy URL for playback in browsers (avoids CORS issues).",
		songName, artist, audioURL, proxyURL,
	)
	return agent.ToolResult{Content: output}, nil
}

func (t *PlayTool) getAudioURL(songID int64) (string, error) {
	cached := t.cache.Get(music.AudioKey(songID))
	if cached != nil {
		return cached.(string), nil
	}

	url, err := t.client.GetAudioURL(songID)
	if err != nil {
		return "", err
	}

	t.cache.Set(music.AudioKey(songID), url, music.TTLAudio)
	return url, nil
}

// IsConcurrencySafe declares this tool is safe to run concurrently.
func (t *PlayTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
