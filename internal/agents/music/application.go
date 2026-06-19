package music

import (
	"fmt"

	"github.com/earendil-works/pi-go/internal/agent"
	musicprompt "github.com/earendil-works/pi-go/internal/agents/music/prompt"
	musictools "github.com/earendil-works/pi-go/internal/agents/music/tools"
	"github.com/earendil-works/pi-go/internal/config"
	"github.com/earendil-works/pi-go/internal/music"
	"github.com/earendil-works/pi-go/internal/music/netease"
	"github.com/earendil-works/pi-go/internal/runtime"
)

// MusicApplication implements runtime.Application for the music-agent.
// It is the concrete application that gets injected into the Platform layer,
// parallel to CodingApplication.
type MusicApplication struct {
	Cfg     config.Config
	Client  *netease.Client
	Cache   *music.Cache
}

// NewMusicApplication creates a new MusicApplication.
func NewMusicApplication(cfg config.Config, client *netease.Client, cache *music.Cache) MusicApplication {
	return MusicApplication{
		Cfg:    cfg,
		Client: client,
		Cache:  cache,
	}
}

// BuildTools assembles the music-agent toolset.
func (a MusicApplication) BuildTools(opts runtime.ToolBuildOptions) []agent.Tool {
	audioBaseURL := fmt.Sprintf("http://%s:%d/music/audio", a.Cfg.Host, a.Cfg.MusicPort)
	return musictools.BuildList(musictools.ListOptions{
		NetEaseClient: a.Client,
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
