package musictools

import (
	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/earendil-works/pi-go/internal/music"
	"github.com/earendil-works/pi-go/internal/music/netease"
)

// ListOptions controls how the music-agent toolset is assembled.
type ListOptions struct {
	NetEaseClient *netease.Client
	Cache         *music.Cache
	AudioBaseURL  string // e.g. "http://localhost:8081/music/audio"
	AllowedTools  []string
	BlockedTools  []string
}

// BaseToolNames returns the canonical music-agent tool names.
func BaseToolNames() []string {
	return []string{"music_search", "music_play", "music_lyrics", "music_detail", "music_playlist", "music_recommend"}
}

// BuildList assembles the concrete music-agent toolset.
func BuildList(opts ListOptions) []agent.Tool {
	toolList := []agent.Tool{
		NewSearchTool(opts.NetEaseClient),
		NewPlayTool(opts.NetEaseClient, opts.Cache, opts.AudioBaseURL),
		NewLyricsTool(opts.NetEaseClient, opts.Cache),
		NewDetailTool(opts.NetEaseClient),
		NewPlaylistTool(opts.NetEaseClient, opts.Cache, opts.AudioBaseURL),
		NewRecommendTool(opts.NetEaseClient),
	}
	return filterTools(toolList, opts.AllowedTools, opts.BlockedTools)
}

func filterTools(tools []agent.Tool, allowed []string, blocked []string) []agent.Tool {
	if len(allowed) == 0 && len(blocked) == 0 {
		return tools
	}

	allowedSet := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		allowedSet[name] = true
	}
	blockedSet := make(map[string]bool, len(blocked))
	for _, name := range blocked {
		blockedSet[name] = true
	}

	var filtered []agent.Tool
	for _, tool := range tools {
		name := tool.Name()
		if len(allowed) > 0 && !allowedSet[name] {
			continue
		}
		if blockedSet[name] {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}
