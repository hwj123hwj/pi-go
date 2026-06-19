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
// Supports two modes: by song_id, or by query (searches first, then plays the top result).
// When query mode fails (e.g. VIP-only), it automatically retries with the next search result.
type PlayTool struct {
	client       *netease.Client
	cache        *music.Cache
	audioBaseURL string // e.g. "http://localhost:8080/music/audio"
}

func NewPlayTool(client *netease.Client, cache *music.Cache, audioBaseURL string) *PlayTool {
	return &PlayTool{client: client, cache: cache, audioBaseURL: audioBaseURL}
}

func (t *PlayTool) Name() string { return "music_play" }
func (t *PlayTool) Description() string {
	return "Play a song. Provide either song_id (from search/recommend results) or query to search and play the top result directly. If the top result requires VIP, automatically tries the next one. Returns the streaming URL, song name, and artist."
}

func (t *PlayTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"song_id": map[string]any{
				"type":        "integer",
				"description": "The song ID (from music_search/music_recommend results). Use this OR query.",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Search query to find and play a song directly (e.g. '周杰伦 晴天'). Use this OR song_id. Will auto-retry if top result requires VIP.",
			},
		},
	}
}

func (t *PlayTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		SongID int64  `json:"song_id"`
		Query  string `json:"query"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if p.SongID == 0 && p.Query == "" {
		return nil, fmt.Errorf("provide either song_id or query")
	}
	return params, nil
}

func (t *PlayTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		SongID int64  `json:"song_id"`
		Query  string `json:"query"`
	}
	_ = json.Unmarshal(params, &p)

	// Mode 1: Direct song_id — no retry possible
	if p.SongID != 0 && p.Query == "" {
		return t.playByID(p.SongID)
	}

	// Mode 2: Query — search and auto-retry on VIP failure
	songs, err := t.client.SearchSongs(p.Query, 10)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("Search failed: %v", err), IsError: true}, nil
	}
	if len(songs.Songs) == 0 {
		return agent.ToolResult{Content: "No songs found for: " + p.Query, IsError: true}, nil
	}

	var lastErr string
	for i, song := range songs.Songs {
		if i >= 5 {
			break // max 5 attempts
		}
		audioURL, err := t.getAudioURL(song.ID)
		if err != nil {
			lastErr = err.Error()
			continue // VIP or unavailable, try next
		}

		proxyURL := audioURL
		if t.audioBaseURL != "" {
			proxyURL = fmt.Sprintf("%s/%d", t.audioBaseURL, song.ID)
		}

		output := fmt.Sprintf(
			"🎵 Now playing: %s — %s\n\n"+
				"Direct URL: %s\n"+
				"Proxy URL: %s\n\n"+
				"Use the proxy URL for playback in browsers (avoids CORS issues).",
			song.Name, song.Artist, audioURL, proxyURL,
		)
		if i > 0 {
			output = fmt.Sprintf("Top %d results require VIP, playing #%d instead.\n\n", i, i+1) + output
		}
		return agent.ToolResult{Content: output}, nil
	}

	return agent.ToolResult{Content: fmt.Sprintf("None of the top 5 results can be played (last error: %s). Try a different query.", lastErr), IsError: true}, nil
}

func (t *PlayTool) playByID(songID int64) (agent.ToolResult, error) {
	songName := fmt.Sprintf("Song #%d", songID)
	artist := ""

	detail, _ := t.client.GetSongDetail([]int64{songID})
	if len(detail) > 0 {
		songName = detail[0].Name
		artist = detail[0].Artist
	}

	audioURL, err := t.getAudioURL(songID)
	if err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("Cannot play \"%s — %s\": %v\nThis song may require VIP. Try a different song_id.", songName, artist, err),
			IsError: true,
		}, nil
	}

	proxyURL := audioURL
	if t.audioBaseURL != "" {
		proxyURL = fmt.Sprintf("%s/%d", t.audioBaseURL, songID)
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
