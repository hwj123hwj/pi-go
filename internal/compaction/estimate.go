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
// SplitMessages 把消息切成 (history, recent) 两段：recent 保留最近 keepRecentTokens
// token 的消息，history 是更早的可被压缩部分。
//
// 切割点必须在"完整 turn 边界"上——即 history 的最后一条是一个交互单元的自然收尾，
// 不会把 assistant(tool_use) 和它后续的 tool_result 拆开。安全边界包括：
//   - tool 结果之后（一个工具调用 turn 完整结束）
//   - 纯文本 assistant 回复之后（一段对话完整结束）
//
// 注意：assistant 带 tool_call 时不算安全边界（它的 tool_result 还在后面）。
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

	// 往前找到最近的安全切割点（msgs[cutIndex-1] 是完整 turn 收尾）。
	for cutIndex > 0 && !isSafeCutBoundary(msgs[cutIndex-1]) {
		cutIndex--
	}

	if cutIndex == 0 {
		return nil, msgs
	}

	return msgs[:cutIndex], msgs[cutIndex:]
}

// isSafeCutBoundary 判断一条消息是否可作为 history 的安全收尾。
// 安全 = 该消息后面可以开始新的 turn，不会割裂正在进行的工具调用。
func isSafeCutBoundary(msg ai.Message) bool {
	switch m := msg.(type) {
	case ai.ToolResultMessage:
		// tool 结果后是新 turn，安全
		return true
	case ai.AssistantMessage:
		// 纯文本 assistant（不带未完成 tool_call）是安全边界；
		// 带 tool_call 的 assistant 后面还有 tool_result，不能在此切。
		return len(m.ToolCalls) == 0
	default:
		// user 消息后是它触发的 assistant，不能切
		return false
	}
}

// SummarizePrompt 生成摘要的 prompt。
// customInstructions: 可选的用户自定义摘要指令，会追加到基础 prompt 中。
func SummarizePrompt(history []ai.Message, customInstructions string) string {
	var b strings.Builder
	b.WriteString("Summarize the following conversation history. ")
	b.WriteString("Focus on:\n")
	b.WriteString("1. Goal: What was the user trying to accomplish?\n")
	b.WriteString("2. Progress: What has been done so far?\n")
	b.WriteString("3. Key Decisions: Any important choices made?\n")
	b.WriteString("4. Next Steps: What remains to be done?\n")
	b.WriteString("5. Critical Context: Any important details that must be preserved?\n")
	if customInstructions != "" {
		b.WriteString("\nAdditional instructions from user:\n")
		b.WriteString(customInstructions)
		b.WriteString("\n")
	}
	b.WriteString("\nConversation history:\n\n")

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
