package musictools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/music/netease"
)

// SearchTool searches for songs on NetEase Cloud Music.
type SearchTool struct {
	client *netease.Client
}

func NewSearchTool(client *netease.Client) *SearchTool {
	return &SearchTool{client: client}
}

func (t *SearchTool) Name() string { return "music_search" }
func (t *SearchTool) Description() string {
	return "Search for songs on NetEase Cloud Music. Returns song ID, name, artist, album, and duration."
}

func (t *SearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query (song name, artist name, or keywords)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max number of results (default 10, max 30)",
			},
		},
		"required": []string{"query"},
	}
}

func (t *SearchTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if p.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	return params, nil
}

func (t *SearchTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	_ = json.Unmarshal(params, &p)
	if p.Limit <= 0 {
		p.Limit = 10
	}
	if p.Limit > 30 {
		p.Limit = 30
	}

	result, err := t.client.SearchSongs(p.Query, p.Limit)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("Search failed: %v", err), IsError: true}, nil
	}

	if len(result.Songs) == 0 {
		return agent.ToolResult{Content: "No songs found for: " + p.Query}, nil
	}

	output := formatSearchResults(result.Songs)
	return agent.ToolResult{Content: output}, nil
}

func formatSearchResults(songs []netease.Song) string {
	var b []byte
	b = append(b, []byte(fmt.Sprintf("Found %d song(s):\n\n", len(songs)))...)
	for i, s := range songs {
		duration := fmt.Sprintf("%d:%02d", s.Duration/60000, (s.Duration%60000)/1000)
		b = append(b, []byte(fmt.Sprintf(
			"%d. %s — %s\n   Album: %s | Duration: %s | ID: %d\n",
			i+1, s.Name, s.Artist, s.AlbumName, duration, s.ID,
		))...)
	}
	return string(b)
}

// IsConcurrencySafe declares this tool is safe to run concurrently.
func (t *SearchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
