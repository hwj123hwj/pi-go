package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/hwj123hwj/pi-go/internal/ai"
)

// OpenAIProvider 通过 HTTP 调用 OpenAI Chat Completions API。
// 兼容所有 OpenAI 格式的中转服务（如 https://api.longcat.chat/openai）。
type OpenAIProvider struct {
	apiKey  string
	baseURL string // 例如 https://api.openai.com/v1 或 https://api.longcat.chat/openai/v1
	client  *http.Client
}

func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) StreamSimple(ctx context.Context, req ai.SimpleStreamRequest) (*ai.EventStream, error) {
	return p.Stream(ctx, ai.StreamRequest{
		Model:     req.Model,
		Messages:  req.Messages,
		System:    req.System,
		Tools:     req.Tools,
		MaxTokens: req.MaxTokens,
	})
}

// ─── OpenAI 请求/响应结构 ──────────────────────────────────────────────────────────

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIToolParam struct {
	Type     string          `json:"type"`
	Function json.RawMessage `json:"function"`
}

type openAIRequest struct {
	Model       string            `json:"model"`
	Messages    []openAIMessage   `json:"messages"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	Stream      bool              `json:"stream"`
	Tools       []openAIToolParam `json:"tools,omitempty"`
}

type openAIStreamChoice struct {
	Index        int               `json:"index"`
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason *string           `json:"finish_reason"`
}

type openAIStreamDelta struct {
	Role      string                 `json:"role,omitempty"`
	Content   string                 `json:"content,omitempty"`
	ToolCalls []openAIStreamToolCall `json:"tool_calls,omitempty"`
}

type openAIStreamToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

type openAIStreamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Model   string               `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
}

// ─── Stream ───────────────────────────────────────────────────────────────────────

func (p *OpenAIProvider) Stream(ctx context.Context, req ai.StreamRequest) (*ai.EventStream, error) {
	stream := ai.NewEventStream(8)

	oaiReq := p.buildOpenAIRequest(req, true)
	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

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
			stream.SetResult(partial, errors.New(errMsg))
			return
		}

		p.handleSSE(ctx, stream, resp.Body, &partial)
	}()

	return stream, nil
}

// ─── SSE 解析 ────────────────────────────────────────────────────────────────────

func (p *OpenAIProvider) handleSSE(ctx context.Context, stream *ai.EventStream, body io.Reader, partial *ai.StreamAssistantMessage) {
	_ = stream.Push(ctx, ai.EventStart{Partial: *partial})

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	// 用于增量拼装 tool_calls（key = index）
	type toolCallAccum struct {
		id        string
		name      string
		arguments strings.Builder
	}
	toolCalls := map[int]*toolCallAccum{}

	textAccum := strings.Builder{}
	textBlockStarted := false

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		// --- 文本内容 ---
		if delta.Content != "" {
			textAccum.WriteString(delta.Content)
			if !textBlockStarted {
				textBlockStarted = true
				_ = stream.Push(ctx, ai.EventTextStart{ContentIndex: 0, Partial: *partial})
			}
			_ = stream.Push(ctx, ai.EventTextDelta{ContentIndex: 0, Delta: delta.Content, Partial: *partial})
		}

		// --- 工具调用（增量拼装）---
		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			if _, ok := toolCalls[idx]; !ok {
				toolCalls[idx] = &toolCallAccum{}
			}
			acc := toolCalls[idx]
			if tc.ID != "" {
				acc.id = tc.ID
			}
			if tc.Function.Name != "" {
				acc.name = tc.Function.Name
			}
			acc.arguments.WriteString(tc.Function.Arguments)
		}

		// --- finish_reason ---
		if choice.FinishReason != nil {
			switch *choice.FinishReason {
			case "stop":
				partial.StopReason = ai.StopReasonStop
			case "tool_calls":
				partial.StopReason = ai.StopReasonToolUse
			case "length":
				partial.StopReason = ai.StopReasonLength
			}
		}
	}

	// 关闭文本 block
	if textBlockStarted {
		partial.Text = textAccum.String()
		_ = stream.Push(ctx, ai.EventTextEnd{ContentIndex: 0, Text: partial.Text, Partial: *partial})
	}

	// 输出拼装完整的 tool_calls（按 index 顺序）
	// 收集并排序 index，兼容 0-based 和 1-based indexing
	indices := make([]int, 0, len(toolCalls))
	for idx := range toolCalls {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	for _, i := range indices {
		acc := toolCalls[i]
		tc := ai.ToolCall{
			ID:   acc.id,
			Name: acc.name,
			Args: acc.arguments.String(),
		}
		partial.ToolCalls = append(partial.ToolCalls, tc)
		// 工具 index 在文本之后
		toolIdx := i
		if textBlockStarted {
			toolIdx = i + 1
		}
		_ = stream.Push(ctx, ai.EventToolCallStart{ContentIndex: toolIdx, Partial: *partial})
		_ = stream.Push(ctx, ai.EventToolCallDelta{ContentIndex: toolIdx, Delta: tc.Args, Partial: *partial})
		_ = stream.Push(ctx, ai.EventToolCallEnd{ContentIndex: toolIdx, ToolCall: tc, Partial: *partial})
	}

	if partial.StopReason == "" {
		partial.StopReason = ai.StopReasonStop
	}
	_ = stream.Push(ctx, ai.EventDone{Reason: partial.StopReason, Message: *partial})
	stream.SetResult(*partial, scanner.Err())
}

// ─── 消息转换 ────────────────────────────────────────────────────────────────────

func (p *OpenAIProvider) buildOpenAIRequest(req ai.StreamRequest, stream bool) openAIRequest {
	maxTokens := 4096
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	oaiReq := openAIRequest{
		Model:       req.Model.ID,
		MaxTokens:   maxTokens,
		Stream:      stream,
		Temperature: 0.7,
	}

	// system prompt 作为第一条 system 消息
	if req.System != "" {
		oaiReq.Messages = append(oaiReq.Messages, openAIMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	// 转换 messages
	for _, msg := range req.Messages {
		switch m := msg.(type) {
		case ai.UserMessage:
			hasOnlyText := true
			for _, block := range m.Content {
				if block.Type != "text" {
					hasOnlyText = false
					break
				}
			}
			if hasOnlyText {
				var text string
				for _, block := range m.Content {
					text += block.Text
				}
				if text == "" {
					text = "..."
				}
				oaiReq.Messages = append(oaiReq.Messages, openAIMessage{
					Role:    "user",
					Content: text,
				})
			} else {
				// Multi-modal: convert to OpenAI content parts
				parts := make([]map[string]any, 0, len(m.Content))
				for _, block := range m.Content {
					switch block.Type {
					case "text":
						parts = append(parts, map[string]any{
							"type": "text",
							"text": block.Text,
						})
					case "image":
						imgPart := map[string]any{"type": "image_url"}
						if block.Image != nil {
							if block.Image.URL != "" {
								imgPart["image_url"] = map[string]any{"url": block.Image.URL}
							} else if len(block.Image.Data) > 0 {
								mediaType := block.Image.MediaType
								if mediaType == "" {
									mediaType = "image/png"
								}
								b64 := base64.StdEncoding.EncodeToString(block.Image.Data)
								imgPart["image_url"] = map[string]any{"url": "data:" + mediaType + ";base64," + b64}
							}
						}
						parts = append(parts, imgPart)
					}
				}
				oaiReq.Messages = append(oaiReq.Messages, openAIMessage{
					Role:    "user",
					Content: parts,
				})
			}

		case ai.AssistantMessage:
			msg := openAIMessage{Role: "assistant", Content: m.Text}
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, openAIToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tc.Name,
						Arguments: tc.Args,
					},
				})
			}
			oaiReq.Messages = append(oaiReq.Messages, msg)

		case ai.ToolResultMessage:
			oaiReq.Messages = append(oaiReq.Messages, openAIMessage{
				Role:       "tool",
				Content:    m.Content,
				ToolCallID: m.ToolCallID,
			})
		}
	}

	// 转换 tools
	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			fn := map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			}
			fnBytes, _ := json.Marshal(fn)
			oaiReq.Tools = append(oaiReq.Tools, openAIToolParam{
				Type:     "function",
				Function: fnBytes,
			})
		}
	}

	return oaiReq
}
