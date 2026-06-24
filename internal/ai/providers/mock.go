package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/hwj123hwj/pi-go/internal/ai"
)

type MockProvider struct{}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (p *MockProvider) Name() string { return "mock" }

func (p *MockProvider) StreamSimple(ctx context.Context, req ai.SimpleStreamRequest) (*ai.EventStream, error) {
	return p.Stream(ctx, ai.StreamRequest{
		Model:     req.Model,
		Messages:  req.Messages,
		System:    req.System,
		Tools:     req.Tools,
		MaxTokens: req.MaxTokens,
	})
}

func (p *MockProvider) Stream(ctx context.Context, req ai.StreamRequest) (*ai.EventStream, error) {
	stream := ai.NewEventStream(8)
	go func() {
		defer stream.Close()
		partial := ai.StreamAssistantMessage{}
		_ = stream.Push(ctx, ai.EventStart{Partial: partial})

		last := lastUserText(req.Messages)
		if call := mockToolCall(last, req.Tools); call != nil {
			partial.ToolCalls = []ai.ToolCall{*call}
			partial.StopReason = ai.StopReasonToolUse
			_ = stream.Push(ctx, ai.EventToolCallStart{ContentIndex: 0, Partial: partial})
			_ = stream.Push(ctx, ai.EventToolCallDelta{ContentIndex: 0, Delta: call.Args, Partial: partial})
			_ = stream.Push(ctx, ai.EventToolCallEnd{ContentIndex: 0, ToolCall: *call, Partial: partial})
			_ = stream.Push(ctx, ai.EventDone{Reason: ai.StopReasonToolUse, Message: partial})
			stream.SetResult(partial, nil)
			return
		}

		text := fmt.Sprintf("MockProvider response: %s", last)
		if hasToolResult(req.Messages) {
			text = "MockProvider observed tool results and finished the turn."
		}
		partial.Text = text
		partial.StopReason = ai.StopReasonStop
		_ = stream.Push(ctx, ai.EventTextStart{ContentIndex: 0, Partial: partial})
		_ = stream.Push(ctx, ai.EventTextDelta{ContentIndex: 0, Delta: text, Partial: partial})
		_ = stream.Push(ctx, ai.EventTextEnd{ContentIndex: 0, Text: text, Partial: partial})
		_ = stream.Push(ctx, ai.EventDone{Reason: ai.StopReasonStop, Message: partial})
		stream.SetResult(partial, nil)
	}()
	return stream, nil
}

func lastUserText(messages []ai.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if msg, ok := messages[i].(ai.UserMessage); ok {
			for _, block := range msg.Content {
				if block.Type == "text" {
					return block.Text
				}
			}
			return "(empty user message)"
		}
	}
	return "hello"
}

func hasToolResult(messages []ai.Message) bool {
	for _, msg := range messages {
		if _, ok := msg.(ai.ToolResultMessage); ok {
			return true
		}
	}
	return false
}

func mockToolCall(text string, tools []ai.ToolDefinition) *ai.ToolCall {
	if hasToolResultInText(text) {
		return nil
	}
	lower := strings.ToLower(text)
	for _, tool := range tools {
		switch tool.Name {
		case "bash":
			if strings.Contains(lower, "tool:bash") {
				return &ai.ToolCall{ID: "mock_bash_1", Name: "bash", Args: `{"command":"pwd"}`}
			}
		case "read":
			if strings.Contains(lower, "tool:read") {
				return &ai.ToolCall{ID: "mock_read_1", Name: "read", Args: `{"path":"go.mod"}`}
			}
		case "write":
			if strings.Contains(lower, "tool:write") {
				return &ai.ToolCall{ID: "mock_write_1", Name: "write", Args: `{"path":"tmp/mock.txt","content":"hello from pi-go\n"}`}
			}
		case "edit":
			if strings.Contains(lower, "tool:edit") {
				return &ai.ToolCall{ID: "mock_edit_1", Name: "edit", Args: `{"path":"tmp/mock.txt","old_string":"hello","new_string":"hello world"}`}
			}
		case "grep":
			if strings.Contains(lower, "tool:grep") {
				return &ai.ToolCall{ID: "mock_grep_1", Name: "grep", Args: `{"pattern":"TODO","path":"."}`}
			}
		case "find":
			if strings.Contains(lower, "tool:find") {
				return &ai.ToolCall{ID: "mock_find_1", Name: "find", Args: `{"path":".","pattern":"*.go"}`}
			}
		case "ls":
			if strings.Contains(lower, "tool:ls") {
				return &ai.ToolCall{ID: "mock_ls_1", Name: "ls", Args: `{"path":"."}`}
			}
		}
	}
	return nil
}

func hasToolResultInText(text string) bool {
	return strings.Contains(strings.ToLower(text), "tool result")
}
