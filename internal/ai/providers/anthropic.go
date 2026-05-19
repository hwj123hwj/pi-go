package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/earendil-works/pi-go/internal/ai"
)

// AnthropicProvider 通过 HTTP 直接调用 Anthropic Messages API。
// 不依赖 anthropic-sdk-go，减少外部依赖。
type AnthropicProvider struct {
	apiKey  string
	baseURL string // 例如 https://api.anthropic.com 或 https://api.longcat.chat/anthropic
	client  *http.Client
}

func NewAnthropicProvider(apiKey, baseURL string) *AnthropicProvider {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &AnthropicProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) StreamSimple(ctx context.Context, req ai.SimpleStreamRequest) (*ai.EventStream, error) {
	return p.Stream(ctx, ai.StreamRequest{
		Model:     req.Model,
		Messages:  req.Messages,
		System:    req.System,
		Tools:     req.Tools,
		MaxTokens: req.MaxTokens,
	})
}

// ─── Anthropic API 请求/响应结构 ──────────────────────────────────────────────────

type anthropicContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicRequest struct {
	Model     string                `json:"model"`
	Messages  []anthropicMessage    `json:"messages"`
	System    string                `json:"system,omitempty"`
	MaxTokens int                   `json:"max_tokens"`
	Stream    bool                  `json:"stream"`
	Tools     []anthropicToolSchema `json:"tools,omitempty"`
}

// ─── Stream ───────────────────────────────────────────────────────────────────────

func (p *AnthropicProvider) Stream(ctx context.Context, req ai.StreamRequest) (*ai.EventStream, error) {
	stream := ai.NewEventStream(8)

	anthropicReq := p.buildRequest(req, true)

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	go func() {
		defer stream.Close()

		partial := ai.StreamAssistantMessage{}

		resp, err := p.client.Do(httpReq)
		if err != nil {
			partial.StopReason = ai.StopReasonError
			partial.ErrorMsg = err.Error()
			_ = stream.Push(ctx, ai.EventError{Reason: "error", Error: err.Error()})
			stream.SetResult(partial, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			errMsg := fmt.Sprintf("API error %d: %s", resp.StatusCode, string(respBody))
			partial.StopReason = ai.StopReasonError
			partial.ErrorMsg = errMsg
			_ = stream.Push(ctx, ai.EventError{Reason: "error", Error: errMsg})
			stream.SetResult(partial, fmt.Errorf(errMsg))
			return
		}

		p.handleSSE(ctx, stream, resp.Body, &partial)
	}()

	return stream, nil
}

// ─── SSE 解析 ────────────────────────────────────────────────────────────────────

func (p *AnthropicProvider) handleSSE(ctx context.Context, stream *ai.EventStream, body io.Reader, partial *ai.StreamAssistantMessage) {
	_ = stream.Push(ctx, ai.EventStart{Partial: *partial})

	scanner := bufio.NewScanner(body)
	// 增大 buffer 防止长行截断
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	var currentEvent string
	contentIndex := 0

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			p.handleSSEEvent(ctx, stream, partial, currentEvent, data, &contentIndex)
			currentEvent = ""
		}
	}

	// 保险：如果流正常结束但没收到 message_stop
	if partial.StopReason == "" {
		partial.StopReason = ai.StopReasonStop
		_ = stream.Push(ctx, ai.EventDone{Reason: partial.StopReason, Message: *partial})
	}
	stream.SetResult(*partial, scanner.Err())
}

func (p *AnthropicProvider) handleSSEEvent(ctx context.Context, stream *ai.EventStream, partial *ai.StreamAssistantMessage, eventType, data string, contentIndex *int) {
	switch eventType {
	case "ping":
		return

	case "message_start":
		var msg struct {
			Message struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		_ = json.Unmarshal([]byte(data), &msg)
		if msg.Message.Usage.InputTokens > 0 {
			partial.Usage = ai.Usage{InputTokens: msg.Message.Usage.InputTokens}
		}

		// 简化：直接用 map 判断类型
		var raw map[string]any
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			return
		}
		cb, _ := raw["content_block"].(map[string]any)
		if cb == nil {
			return
		}
		cbType, _ := cb["type"].(string)
		switch cbType {
		case "text":
			_ = stream.Push(ctx, ai.EventTextStart{ContentIndex: *contentIndex, Partial: *partial})
		case "tool_use":
			id, _ := cb["id"].(string)
			name, _ := cb["name"].(string)
			tc := ai.ToolCall{ID: id, Name: name}
			partial.ToolCalls = append(partial.ToolCalls, tc)
			_ = stream.Push(ctx, ai.EventToolCallStart{ContentIndex: *contentIndex, Partial: *partial})
		}

	case "content_block_delta":
		var raw map[string]any
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			return
		}
		delta, _ := raw["delta"].(map[string]any)
		if delta == nil {
			return
		}
		deltaType, _ := delta["type"].(string)
		switch deltaType {
		case "text_delta":
			text, _ := delta["text"].(string)
			partial.Text += text
			_ = stream.Push(ctx, ai.EventTextDelta{ContentIndex: *contentIndex, Delta: text, Partial: *partial})
		case "input_json_delta":
			partialJSON, _ := delta["partial_json"].(string)
			if len(partial.ToolCalls) > 0 {
				idx := len(partial.ToolCalls) - 1
				partial.ToolCalls[idx].Args += partialJSON
			}
			_ = stream.Push(ctx, ai.EventToolCallDelta{ContentIndex: *contentIndex, Delta: partialJSON, Partial: *partial})
		}

	case "content_block_stop":
		if *contentIndex < len(partial.ToolCalls) {
			_ = stream.Push(ctx, ai.EventToolCallEnd{ContentIndex: *contentIndex, ToolCall: partial.ToolCalls[*contentIndex], Partial: *partial})
		} else {
			_ = stream.Push(ctx, ai.EventTextEnd{ContentIndex: *contentIndex, Text: partial.Text, Partial: *partial})
		}
		*contentIndex++

	case "message_delta":
		var raw map[string]any
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			return
		}
		delta, _ := raw["delta"].(map[string]any)
		if delta != nil {
			stopReason, _ := delta["stop_reason"].(string)
			switch stopReason {
			case "end_turn", "stop":
				partial.StopReason = ai.StopReasonStop
			case "tool_use":
				partial.StopReason = ai.StopReasonToolUse
			case "max_tokens":
				partial.StopReason = ai.StopReasonLength
			}
		}
		usage, _ := raw["usage"].(map[string]any)
		if usage != nil {
			if out, ok := usage["output_tokens"].(float64); ok {
				partial.Usage.OutputTokens = int(out)
			}
		}

	case "message_stop":
		_ = stream.Push(ctx, ai.EventDone{Reason: partial.StopReason, Message: *partial})
		stream.SetResult(*partial, nil)

	case "error":
		var errResp struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal([]byte(data), &errResp)
		errMsg := errResp.Error.Message
		if errMsg == "" {
			errMsg = data
		}
		partial.StopReason = ai.StopReasonError
		partial.ErrorMsg = errMsg
		_ = stream.Push(ctx, ai.EventError{Reason: "error", Error: errMsg})
		stream.SetResult(*partial, fmt.Errorf("API error: %s", errMsg))
	}
}

// ─── 消息转换 ────────────────────────────────────────────────────────────────────

func (p *AnthropicProvider) buildRequest(req ai.StreamRequest, stream bool) anthropicRequest {
	maxTokens := 4096
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	apiReq := anthropicRequest{
		Model:     req.Model.ID,
		MaxTokens: maxTokens,
		Stream:    stream,
		System:    req.System,
		Messages:  convertToAnthropicMessages(req.Messages),
	}

	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			params, _ := json.Marshal(t.Parameters)
			apiReq.Tools = append(apiReq.Tools, anthropicToolSchema{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: params,
			})
		}
	}

	return apiReq
}

func convertToAnthropicMessages(msgs []ai.Message) []anthropicMessage {
	var result []anthropicMessage

	for _, msg := range msgs {
		switch m := msg.(type) {
		case ai.UserMessage:
			var blocks []anthropicContentBlock
			for _, block := range m.Content {
				switch block.Type {
				case "text":
					blocks = append(blocks, anthropicContentBlock{Type: "text", Text: block.Text})
				case "image":
					blocks = append(blocks, anthropicContentBlock{Type: "text", Text: "[Image]"})
				}
			}
			if len(blocks) == 0 {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: "..."})
			}
			result = append(result, anthropicMessage{Role: "user", Content: blocks})

		case ai.AssistantMessage:
			var blocks []anthropicContentBlock
			if m.Text != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: m.Text})
			}
			for _, tc := range m.ToolCalls {
				var input json.RawMessage
				if err := json.Unmarshal([]byte(tc.Args), &input); err != nil {
					input = json.RawMessage(tc.Args)
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: "..."})
			}
			result = append(result, anthropicMessage{Role: "assistant", Content: blocks})

		case ai.ToolResultMessage:
			result = append(result, anthropicMessage{
				Role: "user",
				Content: []anthropicContentBlock{{
					Type: "tool_result",
					ID:   m.ToolCallID,
					Text: m.Content,
				}},
			})
		}
	}

	return result
}
