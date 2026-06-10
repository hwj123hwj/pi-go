package providers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/earendil-works/pi-go/internal/ai"
)

func TestHandleSSE_ToolCallIndexFromOne(t *testing.T) {
	sseData := strings.Join([]string{
		`data: {"id":"msg_1","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"msg_1","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_abc","type":"function","function":{"name":"read"}}]},"finish_reason":null}]}`,
		`data: {"id":"msg_1","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"name":"","arguments":"{\"path\":\"test.txt\"}"}}]},"finish_reason":null}]}`,
		`data: {"id":"msg_1","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	}, "\n")

	p := &OpenAIProvider{}
	stream := ai.NewEventStream(16)
	partial := &ai.StreamAssistantMessage{}

	p.handleSSE(context.Background(), stream, strings.NewReader(sseData), partial)
	stream.Close()

	// Find the EventDone in the stream
	var msg ai.StreamAssistantMessage
	for event := range stream.Events() {
		if done, ok := event.(ai.EventDone); ok {
			msg = done.Message
		}
	}

	if msg.StopReason != ai.StopReasonToolUse {
		t.Fatalf("expected StopReasonToolUse, got %s", msg.StopReason)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	tc := msg.ToolCalls[0]
	if tc.Name != "read" {
		t.Errorf("expected tool name 'read', got %q", tc.Name)
	}
	if tc.Args != `{"path":"test.txt"}` {
		t.Errorf("expected args '{\"path\":\"test.txt\"}', got %q", tc.Args)
	}
	if tc.ID != "call_abc" {
		t.Errorf("expected ID 'call_abc', got %q", tc.ID)
	}
}

func TestHandleSSE_ToolCallIndexFromZero(t *testing.T) {
	sseData := strings.Join([]string{
		`data: {"id":"msg_2","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"msg_2","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_xyz","type":"function","function":{"name":"write"}}]},"finish_reason":null}]}`,
		`data: {"id":"msg_2","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"","arguments":"{\"content\":\"hello\"}"}}]},"finish_reason":null}]}`,
		`data: {"id":"msg_2","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	}, "\n")

	p := &OpenAIProvider{}
	stream := ai.NewEventStream(16)
	partial := &ai.StreamAssistantMessage{}

	p.handleSSE(context.Background(), stream, strings.NewReader(sseData), partial)
	stream.Close()

	var msg ai.StreamAssistantMessage
	for event := range stream.Events() {
		if done, ok := event.(ai.EventDone); ok {
			msg = done.Message
		}
	}

	if msg.StopReason != ai.StopReasonToolUse {
		t.Fatalf("expected StopReasonToolUse, got %s", msg.StopReason)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Name != "write" {
		t.Errorf("expected tool name 'write', got %q", msg.ToolCalls[0].Name)
	}
}

func TestBuildOpenAIRequest_AssistantToolCallsContentNull(t *testing.T) {
	p := &OpenAIProvider{}

	req := ai.StreamRequest{
		Model: ai.Model{ID: "test-model"},
		Messages: []ai.Message{
			ai.NewTextUserMessage("do something"),
			ai.AssistantMessage{
				ToolCalls: []ai.ToolCall{
					{ID: "call_1", Name: "bash", Args: `{"command":"ls"}`},
				},
			},
			ai.ToolResultMessage{ToolCallID: "call_1", Content: "file1.txt\nfile2.txt"},
		},
	}

	oaiReq := p.buildOpenAIRequest(req, false)
	b, err := json.Marshal(oaiReq)
	if err != nil {
		t.Fatal(err)
	}

	// The assistant message with tool_calls should have content: null, not ""
	// Parse just the messages array to check
	var parsed struct {
		Messages []json.RawMessage `json:"messages"`
	}
	json.Unmarshal(b, &parsed)

	// The assistant message is at index 1 (after system prompt is absent, user is 0)
	// Actually: no system prompt, so index 0=user, index 1=assistant
	if len(parsed.Messages) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(parsed.Messages))
	}

	var assistantMsg struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		ToolCalls  json.RawMessage `json:"tool_calls"`
	}
	json.Unmarshal(parsed.Messages[1], &assistantMsg)

	if assistantMsg.Role != "assistant" {
		t.Fatalf("expected assistant role, got %q", assistantMsg.Role)
	}
	// content should be null (not "" or omitted)
	if string(assistantMsg.Content) != "null" {
		t.Errorf("expected content=null for assistant with tool_calls, got %s", string(assistantMsg.Content))
	}

	// Tool result should have correct format
	var toolResult struct {
		Role       string `json:"role"`
		Content    string `json:"content"`
		ToolCallID string `json:"tool_call_id"`
	}
	json.Unmarshal(parsed.Messages[2], &toolResult)
	if toolResult.Role != "tool" {
		t.Errorf("expected tool role, got %q", toolResult.Role)
	}
	if toolResult.ToolCallID != "call_1" {
		t.Errorf("expected tool_call_id 'call_1', got %q", toolResult.ToolCallID)
	}
	if toolResult.Content == "" {
		t.Error("tool result content should not be empty")
	}
}
