package musictools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/music"
	"github.com/earendil-works/pi-go/internal/music/netease"
)

// PlaylistTool fetches and displays a playlist's songs.
type PlaylistTool struct {
	client       *netease.Client
	cache        *music.Cache
	audioBaseURL string
}

func NewPlaylistTool(client *netease.Client, cache *music.Cache, audioBaseURL string) *PlaylistTool {
	return &PlaylistTool{client: client, cache: cache, audioBaseURL: audioBaseURL}
}

func (t *PlaylistTool) Name() string { return "music_playlist" }
func (t *PlaylistTool) Description() string {
	return "Fetch a NetEase Cloud Music playlist by ID. Returns the playlist name, description, and a list of songs with their IDs. Use the song IDs with music_play to play them."
}

func (t *PlaylistTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"playlist_id": map[string]any{
				"type":        "integer",
				"description": "The playlist ID (from music_recommend or search results)",
			},
		},
		"required": []string{"playlist_id"},
	}
}

func (t *PlaylistTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		PlaylistID int64 `json:"playlist_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if p.PlaylistID == 0 {
		return nil, fmt.Errorf("playlist_id is required")
	}
	return params, nil
}

func (t *PlaylistTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		PlaylistID int64 `json:"playlist_id"`
	}
	_ = json.Unmarshal(params, &p)

	detail, err := t.client.GetPlaylistDetail(p.PlaylistID)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("Failed to fetch playlist: %v", err), IsError: true}, nil
	}

	pl := detail.Playlist
	output := fmt.Sprintf(
		"📋 %s\nBy %s | %d tracks | %s plays\n",
		pl.Name, pl.Creator, pl.TrackCount, formatPlayCount(pl.PlayCount),
	)
	if pl.Description != "" {
		desc := pl.Description
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		output += desc + "\n"
	}
	output += "\n"

	if len(detail.Songs) > 0 {
		output += "Songs:\n"
		for i, s := range detail.Songs {
			duration := fmt.Sprintf("%d:%02d", s.Duration/60000, (s.Duration%60000)/1000)
			output += fmt.Sprintf(
				"%d. %s — %s (%s) [ID: %d]\n",
				i+1, s.Name, s.Artist, duration, s.ID,
			)
		}
		if pl.TrackCount > len(detail.Songs) {
			output += fmt.Sprintf("\n... and %d more tracks (showing first %d)\n", pl.TrackCount-len(detail.Songs), len(detail.Songs))
		}
	}

	return agent.ToolResult{Content: output}, nil
}

func (t *PlaylistTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func formatPlayCount(count int64) string {
	if count >= 100000000 {
		return fmt.Sprintf("%.1f亿", float64(count)/100000000)
	}
	if count >= 10000 {
		return fmt.Sprintf("%.1f万", float64(count)/10000)
	}
	return fmt.Sprintf("%d", count)
}
