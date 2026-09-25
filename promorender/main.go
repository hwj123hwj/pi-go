// Command promorender renders pi-go TUI states with the real internal/tui
// package and exports them as ANSI text frames for the video project.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/hwj123hwj/pi-go/internal/tui"
)

const (
	width  = 96
	height = 30
)

// Frame is one exported scene state. Pre is the scene without its overlay
// (or the base state), Pop is the state with the overlay / progression applied.
type Frame struct {
	ID  string `json:"id"`
	Pre string `json:"pre"`
	Pop string `json:"pop"`
}

func main() {
	// Force TrueColor + dark background so AdaptiveColor picks the dark palette
	// regardless of the non-TTY environment.
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)

	outDir := os.Args[1]
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}
	frames := []Frame{
		modelSelectorScene(),
		diffReviewScene(),
		workingScene(),
	}
	data, err := json.MarshalIndent(frames, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "frames.json"), data, 0o644); err != nil {
		panic(err)
	}
}

// fullFrame assembles viewport + separator + [popup] + input + help + status,
// mirroring TuiModel.View layout (statusBarHeight=4: sep + help + status + blank).
func msg(role, content string, hh, mm int) tui.ChatMessage {
	return tui.ChatMessage{Role: role, Content: content, Timestamp: time.Date(2026, 9, 10, hh, mm, 0, 0, time.Local)}
}

func fullFrame(vp tui.MessageViewport, popup string, inTok, outTok int,
	status string, spinnerIdx int, provider, modelID, workspace string) string {

	const inputH = 1  // single-line input
	const chromeH = 4 // separator + help hint + status bar + blank

	viewportHeight := height - inputH - chromeH
	if popup != "" {
		viewportHeight -= lipgloss.Height(popup)
	}
	vp.Resize(width, viewportHeight)

	sb := tui.NewStatusBar()
	input := tui.NewInputModel()
	var b strings.Builder
	b.WriteString(vp.View())
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", width))
	b.WriteByte('\n')
	if popup != "" {
		b.WriteString(popup)
		b.WriteByte('\n')
	}
	b.WriteString(input.View())
	b.WriteByte('\n')
	b.WriteString(sb.HelpHint(false))
	b.WriteByte('\n')
	b.WriteString(sb.Render(width, status, spinnerIdx, provider, modelID, workspace, false, inTok, outTok))
	return b.String()
}

// modelSelectorScene renders the Ctrl+P model selector over a finished-coding
// conversation. Pre: popup closed. Pop: popup open with the last model selected.
func modelSelectorScene() Frame {
	msgs := []tui.ChatMessage{
		msg("user", "把 internal/server 的模型列表接口改成从网关 /v1/models 同步", 14, 31),
		msg("assistant", "已读取 `internal/server/server.go`，`listModels` 目前返回硬编码列表。\n\n**方案**\n1. 请求网关 `GET /v1/models`\n2. 把 `display_name` 映射为模型名\n3. 网关不可达时回退到内置列表", 14, 31),
		msg("user", "补充测试，然后跑一遍 TUI 相关用例", 14, 33),
		msg("assistant", "新增 `TestListModelsFallsBackWithoutGateway`，覆盖网关不可达时的回退路径。\n\n```\nok  github.com/hwj123hwj/pi-go/internal/server  0.8s\nok  github.com/hwj123hwj/pi-go/internal/tui    1.2s\n```", 14, 33),
	}

	models := []tui.ModelOption{
		{Provider: "openai", ModelID: "glm-5", Description: "当前使用"},
		{Provider: "openai", ModelID: "deepseek-v4-flash", Description: "快速响应"},
		{Provider: "openai", ModelID: "claude-sonnet-4-6", Description: "复杂推理"},
		{Provider: "openai", ModelID: "kimi-k3", Description: "长上下文"},
		{Provider: "openai", ModelID: "qwen4-max", Description: "中文优化"},
	}

	vp := tui.NewMessageViewport(width, height-1-4)
	vp.SetMessages(msgs)
	pre := fullFrame(vp, "", 18300, 1240, "ready", 0, "openai", "glm-5", "/Users/weijian/Desktop/hwj/pi-go")

	pop := ""
	for sel := 0; sel < 5; sel++ {
		cm := tui.NewCompletionState()
		cm.TriggerModel(models)
		for i := 0; i < sel; i++ {
			cm.Next()
		}
		popup := tui.NewCompletionPopup().RenderModelPopup(&cm, width)
		pop = fullFrame(vp, popup, 18300, 1240, "ready", 0, "openai", "glm-5", "/Users/weijian/Desktop/hwj/pi-go")
	}

	return Frame{ID: "model-selector", Pre: pre, Pop: pop}
}

// diffReviewScene renders a tool panel containing a colored diff.
func diffReviewScene() Frame {
	diff := strings.Join([]string{
		"diff --git a/internal/server/server.go b/internal/server/server.go",
		"index a1b2c3d..e4f5a6b 100644",
		"--- a/internal/server/server.go",
		"+++ b/internal/server/server.go",
		"@@ -516,9 +516,17 @@ func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {",
		" \t// Determine current model from config",
		"+\t// Try to fetch models dynamically from the gateway",
		"+\tvar models []ModelInfo",
		"+\tif cfg.OpenAIBaseURL != \"\" && cfg.OpenAIAPIKey != \"\" {",
		"+\t\tmodels = s.fetchGatewayModels(cfg.OpenAIBaseURL, cfg.OpenAIAPIKey)",
		"+\t}",
		"+\n+\t// Fallback to hardcoded list if gateway is unreachable",
		"+\tif len(models) == 0 {",
		"+\t\tmodels = builtinModels()",
		"+\t}",
		" \tw.Header().Set(\"Content-Type\", \"application/json\")",
		"-\tmodels := hardcodedModelList()",
	}, "\n")

	msgs := []tui.ChatMessage{
		msg("user", "继续，把 fallback 逻辑也写上", 14, 36),
		{Role: "assistant", Tools: []tui.ToolCallInfo{{
			Name:      "edit",
			Args:      "internal/server/server.go",
			Result:    diff,
			StartTime: time.Now().Add(-2 * time.Second),
		}}, Timestamp: time.Date(2026, 9, 10, 14, 36, 0, 0, time.Local)},
		// msg("assistant", "diff applied", 14, 36),
		msg("assistant", "diff 已应用：新增 gateway 同步 + 不可达回退，共 **+8/-1** 行。", 14, 36),
	}

	vp := tui.NewMessageViewport(width, height-1-4)
	vp.SetMessages(msgs)
	frame := fullFrame(vp, "", 21400, 2680, "ready", 0, "openai", "glm-5", "/Users/weijian/Desktop/hwj/pi-go")
	return Frame{ID: "diff-review", Pre: frame, Pop: frame}
}

// workingScene renders the streaming/thinking state with spinner.
func workingScene() Frame {
	userMsgs := []tui.ChatMessage{
		msg("user", "把模型列表改成从网关同步", 14, 30),
	}
	vp := tui.NewMessageViewport(width, height-1-4)
	vp.SetMessages(userMsgs)
	pre := fullFrame(vp, "", 0, 0, "ready", 0, "openai", "glm-5", "/Users/weijian/Desktop/hwj/pi-go")

	stream := tui.NewMessageViewport(width, height-1-4)
	stream.SetMessages(userMsgs)
	stream.SetStreaming("先读配置里的 baseURL 与 apiKey，再请求网关 /v1/models。\n将 display_name 映射为模型名，写入模型注册表。\n失败时回退内置列表，保证接口始终可用。")
	pop := fullFrame(stream, "", 9044, 661, "thinking", 3, "openai", "glm-5", "/Users/weijian/Desktop/hwj/pi-go")

	return Frame{ID: "working", Pre: pre, Pop: pop}
}
