package compaction

import (
	"context"
	"fmt"

	"github.com/earendil-works/pi-go/internal/ai"
)

// Settings 控制上下文压缩行为。
type Settings struct {
	Enabled          bool
	ReserveTokens    int // 默认 16384，给总结 prompt 和输出留的 token
	KeepRecentTokens int // 默认 20000，保留的最近上下文 token 数
}

func DefaultSettings() Settings {
	return Settings{
		Enabled:          true,
		ReserveTokens:    16384,
		KeepRecentTokens: 20000,
	}
}

// ShouldCompact 判断是否需要压缩。
func ShouldCompact(contextTokens int, contextWindow int, settings Settings) bool {
	if !settings.Enabled {
		return false
	}
	return contextTokens > contextWindow-settings.ReserveTokens
}

// SummarizeFunc 是调用 LLM 生成摘要的函数，由调用方注入（依赖倒置）。
type SummarizeFunc func(ctx context.Context, history []ai.Message, recent []ai.Message) (string, error)

// Compact 执行上下文压缩。
// history: 被压缩的历史消息
// recent: 保留的最近消息
// 返回摘要文本。
func Compact(ctx context.Context, history []ai.Message, recent []ai.Message, summarize SummarizeFunc) (string, error) {
	if len(history) == 0 {
		return "", nil
	}
	summary, err := summarize(ctx, history, recent)
	if err != nil {
		return "", fmt.Errorf("compaction summarize failed: %w", err)
	}
	return summary, nil
}
