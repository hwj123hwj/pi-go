package compaction

import (
	"testing"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/stretchr/testify/assert"
)

func TestEstimateTokens(t *testing.T) {
	msgs := []ai.Message{
		ai.NewTextUserMessage("Hello, this is a test message that has some length to it."),
		ai.AssistantMessage{Text: "And here is a response that also has some content in it."},
	}
	tokens := EstimateTokens(msgs)
	assert.Greater(t, tokens, 0)
	// 大约 (72 + 60) / 4 ≈ 33 tokens
	assert.Less(t, tokens, 100)
}

func TestShouldCompact(t *testing.T) {
	settings := DefaultSettings()

	// 未启用
	settings.Enabled = false
	assert.False(t, ShouldCompact(100000, 128000, settings))

	// 启用但未超限
	settings.Enabled = true
	assert.False(t, ShouldCompact(100000, 128000, settings))

	// 超过限制
	assert.True(t, ShouldCompact(120000, 128000, settings))
}

func TestSplitMessages(t *testing.T) {
	msgs := []ai.Message{
		ai.NewTextUserMessage("msg 1"),
		ai.AssistantMessage{Text: "response 1"},
		ai.NewTextUserMessage("msg 2"),
		ai.AssistantMessage{Text: "response 2"},
		ai.NewTextUserMessage("msg 3"),
		ai.AssistantMessage{Text: "response 3"},
	}

	history, recent := SplitMessages(msgs, 5) // 极小的 budget
	// 应该至少保留一些 recent 消息
	assert.NotNil(t, recent)
	// history 应该比 recent 少或相等
	assert.LessOrEqual(t, len(history), len(msgs))
}

func TestSplitMessages_Empty(t *testing.T) {
	history, recent := SplitMessages(nil, 1000)
	assert.Nil(t, history)
	assert.Nil(t, recent)
}

func TestSplitMessages_AllRecent(t *testing.T) {
	msgs := []ai.Message{
		ai.NewTextUserMessage("short"),
	}
	history, recent := SplitMessages(msgs, 10000)
	assert.Nil(t, history)
	assert.Equal(t, msgs, recent)
}

func TestSummarizePrompt(t *testing.T) {
	msgs := []ai.Message{
		ai.NewTextUserMessage("Build a web server"),
		ai.AssistantMessage{Text: "I'll create a web server using Go."},
	}
	prompt := SummarizePrompt(msgs)
	assert.Contains(t, prompt, "Build a web server")
	assert.Contains(t, prompt, "I'll create a web server")
	assert.Contains(t, prompt, "Goal")
}
