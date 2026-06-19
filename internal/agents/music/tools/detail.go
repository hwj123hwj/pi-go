package musictools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/music/netease"
)

// DetailTool fetches detailed song information.
type DetailTool struct {
	client *netease.Client
}

func NewDetailTool(client *netease.Client) *DetailTool {
	return &DetailTool{client: client}
}

func (t *DetailTool) Name() string { return "music_detail" }
func (t *DetailTool) Description() string {
	return "Get detailed information about a song: name, artist, album, cover art URL, and duration."
}

func (t *DetailTool) Parameters() map[string]any {
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

func (t *DetailTool) Validate(params json.RawMessage) (json.RawMessage, error) {
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

func (t *DetailTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		SongID int64 `json:"song_id"`
	}
	_ = json.Unmarshal(params, &p)

	songs, err := t.client.GetSongDetail([]int64{p.SongID})
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("Failed to get song detail: %v", err), IsError: true}, nil
	}
	if len(songs) == 0 {
		return agent.ToolResult{Content: fmt.Sprintf("Song %d not found.", p.SongID)}, nil
	}

	s := songs[0]
	duration := fmt.Sprintf("%d:%02d", s.Duration/60000, (s.Duration%60000)/1000)
	output := fmt.Sprintf(
		"🎵 %s\n"+
			"🎤 %s\n"+
			"💿 %s\n"+
			"⏱  %s\n"+
			"🖼  %s\n"+
			"🆔 %d",
		s.Name, s.Artist, s.AlbumName, duration, s.AlbumCover, s.ID,
	)
	return agent.ToolResult{Content: output}, nil
}

// IsConcurrencySafe declares this tool is safe to run concurrently.
func (t *DetailTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
