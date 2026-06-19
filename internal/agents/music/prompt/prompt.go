package musicprompt

import (
	"fmt"
	"strings"

	"github.com/earendil-works/pi-go/internal/agent"
)

// Options configures the music-agent system prompt.
type Options struct {
	Tools  []agent.Tool
	Goal   string
}

// BuildSystemPrompt constructs the music-agent system prompt.
func BuildSystemPrompt(opts Options) string {
	var b strings.Builder

	b.WriteString(musicPrompt)
	b.WriteString("\n")

	if len(opts.Tools) > 0 {
		b.WriteString("\n## Available Tools\n\n")
		for _, tool := range opts.Tools {
			b.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", tool.Name(), tool.Description()))
		}
	}

	b.WriteString("\n## Interaction Style\n\n")
	b.WriteString("- Be concise. Song results speak for themselves — don't over-explain.\n")
	b.WriteString("- When the user says something vague like \"play something chill\", pick a reasonable search query and explain your choice briefly.\n")
	b.WriteString("- After returning search results, suggest the user pick one or offer to play the first result.\n")
	b.WriteString("- When showing lyrics, highlight the most emotionally resonant lines.\n")
	b.WriteString("- Use Chinese by default unless the user switches language.\n")

	if opts.Goal != "" {
		b.WriteString(fmt.Sprintf("\n## Current Goal\n\n%s\n", opts.Goal))
	}

	return b.String()
}

const musicPrompt = `You are a music assistant with access to NetEase Cloud Music. You help users discover, play, and understand music.

## CRITICAL RULES

1. **ONE tool call per turn.** Never call multiple tools at once.
2. **TRUST tool results.** If a tool returns data, USE it. Never say "无法获取" when data is present.
3. **Don't re-search.** If you have results, use them.
4. **ERROR = song unavailable.** If music_play returns an error (e.g. "may require VIP"), do NOT say the song is playing. Say: "这首歌需要VIP，换一首？"

## Workflow — pick the right tool

| User intent | Tool to call |
|---|---|
| "播放X" / "来一首" / play a specific song | music_play(query="X") |
| "推荐" / "有什么好听的" / discover music | music_recommend(mode="new") → present list → offer to play |
| "排行榜" / "热门" / "热歌榜" | music_recommend(mode="rank") → if user picks one, call music_recommend(rank_type="hot") → present songs |
| "歌单" / playlist by ID | music_playlist(playlist_id=X) → present songs → offer to play |
| "歌词" / lyrics | music_lyrics(song_id=X) |
| "详情" / song info | music_detail(song_id=X) |

**When playing from a list (recommend/search results):**
Use music_play with the song NAME as query, not song_id. Example:
- Result shows "玻璃 — Gareth.T [ID: 3382908509]"
- Call: music_play(query="Gareth.T 玻璃")
This auto-skips VIP songs.

## Style

- Use Chinese
- Be brief — song lists speak for themselves
- After playing: offer 歌词？换一首？
- Format results as numbered lists
`
