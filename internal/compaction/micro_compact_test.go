package compaction

import (
	"strings"
	"testing"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/stretchr/testify/assert"
)

// 构造一个工具调用 turn：assistant 发起 tool_call，紧接 tool result。
func turn(toolName, id, resultContent string) []ai.Message {
	return []ai.Message{
		ai.AssistantMessage{
			ToolCalls:  []ai.ToolCall{{ID: id, Name: toolName, Args: "{}"}},
			StopReason: ai.StopReasonToolUse,
		},
		ai.ToolResultMessage{ToolCallID: id, Content: resultContent},
	}
}

// 清理旧 tool result：除最近 keepRecent 个外替换为占位符。
func TestMicroCompact_ClearsOldToolResults(t *testing.T) {
	var hist []ai.Message
	hist = append(hist, ai.NewTextUserMessage("start"))
	// 7 个 read result，每个内容 100 字符
	for i := 0; i < 7; i++ {
		hist = append(hist, turn("read", "r"+strings.Repeat("0", i), strings.Repeat("x", 100))...)
	}

	out, cleared := MicroCompact(hist, 5)
	// 7 个可压缩 result，保留最近 5 个 → 清理 2 个
	assert.Equal(t, 2, cleared)

	// 统计：被清理的（占位符）数量 + 保留的（原内容）数量
	clearedCount, keptCount := 0, 0
	for _, m := range out {
		if tr, ok := m.(ai.ToolResultMessage); ok {
			if tr.Content == clearedPlaceholder {
				clearedCount++
			} else {
				keptCount++
			}
		}
	}
	assert.Equal(t, 2, clearedCount, "应清理 2 个")
	assert.Equal(t, 5, keptCount, "应保留 5 个完整")
}

// 不清理写操作的 result（edit/write 不在 compactableTools）。
func TestMicroCompact_KeepsWriteResults(t *testing.T) {
	hist := []ai.Message{ai.NewTextUserMessage("start")}
	// 7 个 edit result
	for i := 0; i < 7; i++ {
		hist = append(hist, turn("edit", "e"+strings.Repeat("0", i), strings.Repeat("y", 100))...)
	}

	out, cleared := MicroCompact(hist, 5)
	assert.Equal(t, 0, cleared, "edit 是写操作，不应被清理")

	// 所有 edit result 内容应保持完整
	for _, m := range out {
		if tr, ok := m.(ai.ToolResultMessage); ok {
			assert.NotEqual(t, clearedPlaceholder, tr.Content)
		}
	}
}

// 不动 user/assistant 文本消息。
func TestMicroCompact_KeepsTextMessages(t *testing.T) {
	hist := []ai.Message{
		ai.NewTextUserMessage("important user message"),
		ai.AssistantMessage{Text: "important assistant text", StopReason: ai.StopReasonStop},
	}
	hist = append(hist, turn("read", "r1", strings.Repeat("x", 100))...)

	out, cleared := MicroCompact(hist, 5)
	assert.Equal(t, 0, cleared) // 只有 1 个 read，不清理

	assert.Equal(t, "important user message", out[0].(ai.UserMessage).Content[0].Text)
	assert.Equal(t, "important assistant text", out[1].(ai.AssistantMessage).Text)
}

// 不足 keepRecent 个时不清理。
func TestMicroCompact_NoClearWhenFewerThanKeep(t *testing.T) {
	hist := []ai.Message{ai.NewTextUserMessage("start")}
	hist = append(hist, turn("read", "r1", strings.Repeat("x", 100))...)
	hist = append(hist, turn("bash", "b1", strings.Repeat("y", 100))...)

	_, cleared := MicroCompact(hist, 5)
	assert.Equal(t, 0, cleared, "只有 2 个可压缩 result，不足 5，不应清理")
}

// ToolCallID 关联：tool result 通过 ID 找到工具名，只清理可压缩工具的。
func TestMicroCompact_ToolNameResolution(t *testing.T) {
	// 混合 read（可压缩）和 edit（不可压缩），各 6 个
	hist := []ai.Message{ai.NewTextUserMessage("start")}
	for i := 0; i < 6; i++ {
		hist = append(hist, turn("read", "rd"+strings.Repeat("0", i), strings.Repeat("a", 100))...)
		hist = append(hist, turn("edit", "ed"+strings.Repeat("0", i), strings.Repeat("b", 100))...)
	}

	out, cleared := MicroCompact(hist, 5)
	// 6 个 read 可压缩，保留 5 → 清理 1；edit 不动
	assert.Equal(t, 1, cleared)

	// 验证：被清理的那个是 read（通过 ID 关联），edit 全保留
	for _, m := range out {
		if tr, ok := m.(ai.ToolResultMessage); ok {
			if tr.Content == clearedPlaceholder {
				// 被清理的 ID 应以 rd 开头（read）
				assert.True(t, strings.HasPrefix(tr.ToolCallID, "rd"))
			}
		}
	}
}

// 占位符比原内容长时不清理（避免反向膨胀）。
func TestMicroCompact_SkipsWhenPlaceholderLonger(t *testing.T) {
	hist := []ai.Message{ai.NewTextUserMessage("start")}
	// 6 个 read，但内容极短（比 clearedPlaceholder 短）
	for i := 0; i < 6; i++ {
		hist = append(hist, turn("read", "r"+strings.Repeat("0", i), "x")...)
	}

	_, cleared := MicroCompact(hist, 5)
	// 占位符比 "x" 长，清理反而膨胀 → 不清理
	assert.Equal(t, 0, cleared)
}

// keepRecent=0 时清理所有可压缩 result。
func TestMicroCompact_KeepZero(t *testing.T) {
	hist := []ai.Message{ai.NewTextUserMessage("start")}
	for i := 0; i < 5; i++ {
		hist = append(hist, turn("read", "r"+strings.Repeat("0", i), strings.Repeat("x", 100))...)
	}

	out, cleared := MicroCompact(hist, 0)
	assert.Equal(t, 5, cleared)
	for _, m := range out {
		if tr, ok := m.(ai.ToolResultMessage); ok {
			assert.Equal(t, clearedPlaceholder, tr.Content)
		}
	}
}
