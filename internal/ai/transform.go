package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TransformMessages 将消息列表转换为适合特定 provider 的格式。
// 处理：
//   - 图片降级（某些 provider 不支持图片）
//   - ToolCall ID 规范化（确保 ID 不为空）
//   - 连续同角色消息合并
//   - 空消息填充
//   - Thinking 内容处理
type TransformOptions struct {
	// 禁用图片支持，将图片降级为 [Image] 文本
	NoImageSupport bool

	// 需要确保 tool_call_id 在消息中正确关联
	NormalizeToolCallIDs bool

	// 合并连续的同角色消息
	MergeConsecutiveRoles bool

	// Provider 名称，用于日志
	ProviderName string
}

// DefaultTransformOptions 返回默认的转换选项。
func DefaultTransformOptions() TransformOptions {
	return TransformOptions{
		NoImageSupport:        false,
		NormalizeToolCallIDs:  true,
		MergeConsecutiveRoles: true,
		ProviderName:          "",
	}
}

// TransformMessages 执行消息转换。
func TransformMessages(messages []Message, opts TransformOptions) []Message {
	if len(messages) == 0 {
		return messages
	}

	result := make([]Message, 0, len(messages))

	for i, msg := range messages {
		transformed := transformMessage(msg, opts)
		if transformed == nil {
			continue
		}

		// 合并连续的同角色消息
		if opts.MergeConsecutiveRoles && len(result) > 0 {
			lastRole := result[len(result)-1].Role()
			currentRole := transformed.Role()
			if lastRole == currentRole && currentRole == RoleUser {
				// 合并两个 user message
				result = mergeUserMessages(result, transformed)
				continue
			}
		}

		result = append(result, transformed)

		// 确保下一个消息不是相同角色（assistant → assistant）
		_ = i // placeholder
	}

	return result
}

func transformMessage(msg Message, opts TransformOptions) Message {
	switch m := msg.(type) {
	case UserMessage:
		return transformUserMessage(m, opts)
	case AssistantMessage:
		return transformAssistantMessage(m, opts)
	case ToolResultMessage:
		return transformToolResultMessage(m, opts)
	default:
		return msg
	}
}

func transformUserMessage(m UserMessage, opts TransformOptions) Message {
	if !opts.NoImageSupport {
		// 不需要降级
		return m
	}

	// 降级图片为文本描述
	var newBlocks []ContentBlock
	for _, block := range m.Content {
		if block.Type == "image" && opts.NoImageSupport {
			newBlocks = append(newBlocks, ContentBlock{
				Type: "text",
				Text: "[Image]",
			})
		} else {
			newBlocks = append(newBlocks, block)
		}
	}
	if len(newBlocks) == 0 {
		newBlocks = append(newBlocks, ContentBlock{Type: "text", Text: "..."})
	}
	return UserMessage{Content: newBlocks}
}

func transformAssistantMessage(m AssistantMessage, opts TransformOptions) Message {
	if !opts.NormalizeToolCallIDs {
		return m
	}

	// 规范化 ToolCall ID
	for i := range m.ToolCalls {
		if m.ToolCalls[i].ID == "" {
			m.ToolCalls[i].ID = fmt.Sprintf("generated_tc_%d", i)
		}
	}

	// 清理 thinking 内容（某些 provider 不支持）
	// 这里只是标记，不做实际删除，因为 Thinking 可能包含有用信息
	// 如果 provider 不支持 thinking，发送方应该自行处理

	return m
}

func transformToolResultMessage(m ToolResultMessage, opts TransformOptions) Message {
	if !opts.NormalizeToolCallIDs {
		return m
	}

	// 确保 ToolCallID 不为空
	if m.ToolCallID == "" {
		m.ToolCallID = "unknown_tool_call"
	}
	return m
}

// mergeUserMessages 将两条 user message 合并为一条。
func mergeUserMessages(result []Message, newMsg Message) []Message {
	lastIdx := len(result) - 1
	lastUser, ok := result[lastIdx].(UserMessage)
	if !ok {
		result = append(result, newMsg)
		return result
	}

	newUser, ok := newMsg.(UserMessage)
	if !ok {
		result = append(result, newMsg)
		return result
	}

	// 合并 Content blocks
	merged := UserMessage{
		Content: append(lastUser.Content, newUser.Content...),
	}
	result[lastIdx] = merged
	return result
}

// ValidateMessageSequence 验证消息序列是否合法。
// 规则：
//   - 消息角色应交替出现（user → assistant → user 或 user → assistant → tool → assistant）
//   - ToolResultMessage 后必须跟 AssistantMessage
//   - 首条消息应为 UserMessage
func ValidateMessageSequence(messages []Message) error {
	if len(messages) == 0 {
		return nil
	}

	// 首条消息应该是 user（system 已在请求级别处理）
	if messages[0].Role() != RoleUser {
		// 允许，但可能某些 provider 会拒绝
	}

	for i := 1; i < len(messages); i++ {
		prev := messages[i-1].Role()
		curr := messages[i].Role()

		switch {
		case prev == RoleUser && curr == RoleUser:
			// 连续 user 消息 - 可以合并
			continue
		case prev == RoleAssistant && curr == RoleAssistant:
			return fmt.Errorf("consecutive assistant messages at index %d-%d", i-1, i)
		case prev == RoleTool && curr == RoleTool:
			// 多个 tool results 可以连续（并行执行结果）
			continue
		case prev == RoleTool && curr == RoleUser:
			return fmt.Errorf("tool result at index %d must be followed by assistant message, got user", i-1)
		}
	}

	return nil
}

// MessagesToJSON 将消息列表序列化为 JSON（用于调试/日志）。
func MessagesToJSON(messages []Message) (string, error) {
	type jsonMessage struct {
		Role    string `json:"role"`
		Content string `json:"content,omitempty"`
	}

	var result []jsonMessage
	for _, msg := range messages {
		jm := jsonMessage{Role: string(msg.Role())}
		switch m := msg.(type) {
		case UserMessage:
			var texts []string
			for _, block := range m.Content {
				if block.Type == "text" {
					texts = append(texts, block.Text)
				}
			}
			jm.Content = strings.Join(texts, "\n")
		case AssistantMessage:
			jm.Content = m.Text
			if len(m.ToolCalls) > 0 {
				var names []string
				for _, tc := range m.ToolCalls {
					names = append(names, tc.Name)
				}
				jm.Content += " [tools: " + strings.Join(names, ", ") + "]"
			}
		case ToolResultMessage:
			content := m.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			jm.Content = content
		}
		result = append(result, jm)
	}

	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
