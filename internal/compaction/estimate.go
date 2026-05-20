package compaction

import (
	"strings"

	"github.com/earendil-works/pi-go/internal/ai"
)

// EstimateTokens 估算消息列表的 token 数。
// 不要求精确（不需要完整 tokenizer），按 4 字符 ≈ 1 token 估算。
// 用于触发 ShouldCompact 前的快速判断。
func EstimateTokens(msgs []ai.Message) int {
	total := 0
	for _, msg := range msgs {
		total += estimateMessageTokens(msg)
	}
	return total
}

func estimateMessageTokens(msg ai.Message) int {
	var text string
	switch m := msg.(type) {
	case ai.UserMessage:
		for _, block := range m.Content {
			if block.Type == "text" {
				text += block.Text
			}
		}
	case ai.AssistantMessage:
		text = m.Text
		if m.Thinking != "" {
			text += m.Thinking
		}
		for _, tc := range m.ToolCalls {
			text += tc.Name + tc.Args
		}
	case ai.ToolResultMessage:
		text = m.Content
	}
	return max(1, len(text)/4)
}

// EstimateTextTokens 估算纯文本的 token 数。
func EstimateTextTokens(text string) int {
	return max(1, len(text)/4)
}

// SplitMessages 将消息列表按 token 预算分割为历史部分和保留部分。
// 从后往前累积 token，直到达到 keepRecentTokens。
// 找到最近的有效切割点（user/assistant 分界）。
func SplitMessages(msgs []ai.Message, keepRecentTokens int) (history []ai.Message, recent []ai.Message) {
	if len(msgs) == 0 {
		return nil, nil
	}

	// 从后往前累积 token
	recentTokens := 0
	cutIndex := len(msgs)

	for i := len(msgs) - 1; i >= 0; i-- {
		msgTokens := estimateMessageTokens(msgs[i])
		if recentTokens+msgTokens > keepRecentTokens {
			break
		}
		recentTokens += msgTokens
		cutIndex = i
	}

	// 找到有效的切割点（在 user 消息之前切割）
	// 确保历史部分以完整的 turn 结束
	for cutIndex > 0 {
		if msgs[cutIndex-1].Role() == ai.RoleUser && msgs[cutIndex].Role() == ai.RoleAssistant {
			// 在 user→assistant 之间切割
			break
		}
		cutIndex--
	}

	if cutIndex == 0 {
		return nil, msgs
	}

	return msgs[:cutIndex], msgs[cutIndex:]
}

// SummarizePrompt 生成摘要的 prompt。
func SummarizePrompt(history []ai.Message) string {
	var b strings.Builder
	b.WriteString("Summarize the following conversation history. ")
	b.WriteString("Focus on:\n")
	b.WriteString("1. Goal: What was the user trying to accomplish?\n")
	b.WriteString("2. Progress: What has been done so far?\n")
	b.WriteString("3. Key Decisions: Any important choices made?\n")
	b.WriteString("4. Next Steps: What remains to be done?\n")
	b.WriteString("5. Critical Context: Any important details that must be preserved?\n\n")
	b.WriteString("Conversation history:\n\n")

	for _, msg := range history {
		switch m := msg.(type) {
		case ai.UserMessage:
			for _, block := range m.Content {
				if block.Type == "text" {
					b.WriteString("User: ")
					b.WriteString(block.Text)
					b.WriteString("\n")
				}
			}
		case ai.AssistantMessage:
			b.WriteString("Assistant: ")
			b.WriteString(m.Text)
			b.WriteString("\n")
			for _, tc := range m.ToolCalls {
				b.WriteString("  Tool call: ")
				b.WriteString(tc.Name)
				b.WriteString("(")
				b.WriteString(tc.Args)
				b.WriteString(")\n")
			}
		case ai.ToolResultMessage:
			b.WriteString("Tool result: ")
			// 截断过长的 tool result
			content := m.Content
			if len(content) > 500 {
				content = content[:500] + "...(truncated)"
			}
			b.WriteString(content)
			b.WriteString("\n")
		}
	}

	return b.String()
}
