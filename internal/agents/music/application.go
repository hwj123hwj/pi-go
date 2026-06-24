package music

import (
	"fmt"

	"github.com/hwj123hwj/pi-go/internal/agent"
	musicprompt "github.com/hwj123hwj/pi-go/internal/agents/music/prompt"
	musictools "github.com/hwj123hwj/pi-go/internal/agents/music/tools"
	"github.com/hwj123hwj/pi-go/internal/config"
	"github.com/hwj123hwj/pi-go/internal/music"
	"github.com/hwj123hwj/pi-go/internal/runtime"
)

// MusicApplication implements runtime.Application for the music-agent.
type MusicApplication struct {
	Cfg    config.Config
	Router *music.SourceRouter
	Cache  *music.Cache
}

// NewMusicApplication creates a new MusicApplication with multi-source support.
func NewMusicApplication(cfg config.Config, router *music.SourceRouter, cache *music.Cache) MusicApplication {
	return MusicApplication{
		Cfg:    cfg,
		Router: router,
		Cache:  cache,
	}
}

// BuildTools assembles the music-agent toolset.
func (a MusicApplication) BuildTools(opts runtime.ToolBuildOptions) []agent.Tool {
	audioBaseURL := fmt.Sprintf("http://%s:%d/music/audio", a.Cfg.Host, a.Cfg.Port)
	return musictools.BuildList(musictools.ListOptions{
		Router:        a.Router,
		Cache:         a.Cache,
		AudioBaseURL:  audioBaseURL,
		AllowedTools:  opts.AllowedTools,
		BlockedTools:  opts.BlockedTools,
	})
}

// BuildPrompt constructs the music-agent system prompt.
func (a MusicApplication) BuildPrompt(opts runtime.PromptBuildOptions, profile, goal string) string {
	return musicprompt.BuildSystemPrompt(musicprompt.Options{
		Tools: opts.Tools,
		Goal:  goal,
	})
}

// NewSessionExt creates a per-session MusicSessionExt.
func (a MusicApplication) NewSessionExt() runtime.SessionExt {
	return NewMusicSessionExt()
}

// ToolNames returns the canonical music-agent tool names.
func (MusicApplication) ToolNames(_ bool) []string {
	return musictools.BaseToolNames()
}

// Verify interface compliance at compile time.
var _ runtime.Application = MusicApplication{}
