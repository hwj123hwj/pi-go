package musictools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/music/netease"
)

// RecommendTool fetches recommended playlists, rankings, and new songs.
type RecommendTool struct {
	client *netease.Client
}

func NewRecommendTool(client *netease.Client) *RecommendTool {
	return &RecommendTool{client: client}
}

func (t *RecommendTool) Name() string { return "music_recommend" }
func (t *RecommendTool) Description() string {
	return "Get music recommendations. Modes: 'rank' lists all ranking charts or fetches a specific ranking by rank_type; 'new' returns recommended new songs. Returns playlist IDs that can be used with music_playlist to see full song lists."
}

func (t *RecommendTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"rank", "new"},
				"description": "Recommendation mode: 'rank' for ranking charts, 'new' for new song recommendations",
			},
			"rank_type": map[string]any{
				"type":        "string",
				"enum":        []string{"soaring", "hot", "new", "original", "all"},
				"description": "Ranking type (only for rank mode): soaring=飙升榜, hot=热歌榜, new=新歌榜, original=原创榜, all=list all available rankings",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max number of new songs to return (default 10, max 30). Only for new mode.",
			},
		},
		"required": []string{"mode"},
	}
}

func (t *RecommendTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	switch p.Mode {
	case "rank", "new":
		// valid
	default:
		return nil, fmt.Errorf("mode must be one of: rank, new")
	}
	return params, nil
}

func (t *RecommendTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	var p struct {
		Mode     string `json:"mode"`
		RankType string `json:"rank_type"`
		Limit    int    `json:"limit"`
	}
	_ = json.Unmarshal(params, &p)

	switch p.Mode {
	case "rank":
		return t.executeRank(p.RankType)
	case "new":
		return t.executeNew(p.Limit)
	default:
		return agent.ToolResult{Content: "Unknown mode: " + p.Mode, IsError: true}, nil
	}
}

func (t *RecommendTool) executeRank(rankType string) (agent.ToolResult, error) {
	rankMap := map[string]struct {
		id   int64
		name string
	}{
		"soaring": {netease.RankSoaring, "飙升榜"},
		"hot":     {netease.RankHot, "热歌榜"},
		"new":     {netease.RankNew, "新歌榜"},
		"original": {netease.RankOriginal, "原创榜"},
	}

	if rankType == "" || rankType == "all" {
		// Show all available rankings
		rankings, err := t.client.GetRankings()
		if err != nil {
			return agent.ToolResult{Content: fmt.Sprintf("Failed to fetch rankings: %v", err), IsError: true}, nil
		}

		var b strings.Builder
		b.WriteString("📊 排行榜\n\n")
		// Show well-known ones first
		b.WriteString("常用榜单:\n")
		for key, r := range rankMap {
			b.WriteString(fmt.Sprintf("  - %s (rank_type: \"%s\", playlist_id: %d)\n", r.name, key, r.id))
		}
		b.WriteString(fmt.Sprintf("\n全部榜单 (共 %d 个):\n", len(rankings)))
		for i, r := range rankings {
			freq := r.UpdateFrequency
			if freq == "" {
				freq = "-"
			}
			b.WriteString(fmt.Sprintf(
				"%d. %s | %d tracks | %s | ID: %d\n",
				i+1, r.Name, r.TrackCount, freq, r.ID,
			))
		}
		b.WriteString("\nCall with rank_type \"soaring\"/\"hot\"/\"new\"/\"original\" to see songs, or use music_playlist with a playlist_id.")
		return agent.ToolResult{Content: b.String()}, nil
	}

	r, ok := rankMap[rankType]
	if !ok {
		return agent.ToolResult{Content: fmt.Sprintf("Unknown rank_type: %s. Use: soaring, hot, new, original, all", rankType), IsError: true}, nil
	}

	detail, err := t.client.GetTopList(r.id)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("Failed to fetch %s: %v", r.name, err), IsError: true}, nil
	}

	output := fmt.Sprintf("📊 %s\n", r.name)
	if detail.Playlist.Description != "" {
		output += detail.Playlist.Description + "\n"
	}
	output += fmt.Sprintf("共 %d 首，显示前 %d 首\n\n", detail.Playlist.TrackCount, len(detail.Songs))

	if len(detail.Songs) > 0 {
		for i, s := range detail.Songs {
			duration := fmt.Sprintf("%d:%02d", s.Duration/60000, (s.Duration%60000)/1000)
			output += fmt.Sprintf(
				"%d. %s — %s (%s) [ID: %d]\n",
				i+1, s.Name, s.Artist, duration, s.ID,
			)
		}
		output += fmt.Sprintf("\nUse music_play to play any song, or music_playlist with id %d to see all tracks.", r.id)
	}

	return agent.ToolResult{Content: output}, nil
}

func (t *RecommendTool) executeNew(limit int) (agent.ToolResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 30 {
		limit = 30
	}

	songs, err := t.client.GetNewSongs(limit)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("Failed to fetch new songs: %v", err), IsError: true}, nil
	}
	if len(songs) == 0 {
		return agent.ToolResult{Content: "No new songs available."}, nil
	}

	output := "🆕 推荐新歌\n\n"
	for i, s := range songs {
		duration := fmt.Sprintf("%d:%02d", s.Duration/60000, (s.Duration%60000)/1000)
		output += fmt.Sprintf(
			"%d. %s — %s (%s) [ID: %d]\n",
			i+1, s.Name, s.Artist, duration, s.ID,
		)
	}
	output += "\nUse music_play to play any song."

	return agent.ToolResult{Content: output}, nil
}

func (t *RecommendTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
