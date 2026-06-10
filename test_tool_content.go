package main

import (
	"encoding/json"
)

type openAIMessage struct {
	Role       string          `json:"role"`
	Content    any             `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func main() {
	// Test case 1: Assistant message with tool_calls and empty text content
	msg1 := openAIMessage{
		Role:    "assistant",
		Content: "",  // Empty string
		ToolCalls: []openAIToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      "read",
					Arguments: "{\"path\":\"test.txt\"}",
				},
			},
		},
	}
	
	b1, _ := json.Marshal(msg1)
	println("Empty string content:", string(b1))
	
	// Test case 2: Assistant message with tool_calls and null content
	msg2 := openAIMessage{
		Role:    "assistant",
		Content: nil,  // null
		ToolCalls: []openAIToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      "read",
					Arguments: "{\"path\":\"test.txt\"}",
				},
			},
		},
	}
	
	b2, _ := json.Marshal(msg2)
	println("Null content:", string(b2))
	
	// Test case 3: Tool result message
	msg3 := openAIMessage{
		Role:       "tool",
		Content:    "File content here",
		ToolCallID: "call_123",
	}
	
	b3, _ := json.Marshal(msg3)
	println("Tool result:", string(b3))
}
