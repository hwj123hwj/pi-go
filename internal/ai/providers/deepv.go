package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/earendil-works/pi-go/internal/ai"
)

type headerProvider interface {
	Headers() map[string]string
}

// DeepVProvider 通过 DeepVcode Server API 提供 LLM 服务。
// 使用 GenAI 格式通信，从 ~/.deepv/jwt-token.json 读取认证 token。
type DeepVProvider struct {
	serverURL      string // 例如 https://api-code.deepvlab.ai
	client         *http.Client
	tokenCache     *jwtToken
	headerProvider headerProvider
}

// jwtToken 本地存储的 JWT token 结构。
type jwtToken struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresAt    int64  `json:"expiresAt"`
}

// ─── GenAI 请求/响应结构 ──────────────────────────────────────────────────────

type deepVRequest struct {
	Model             string         `json:"model"`
	Contents          []deepVContent `json:"contents"`
	SystemInstruction *deepVContent  `json:"systemInstruction,omitempty"`
	Config            *deepVConfig   `json:"config,omitempty"`
}

type deepVContent struct {
	Role  string      `json:"role"`
	Parts []deepVPart `json:"parts"`
}

type deepVPart struct {
	Text             string                 `json:"text,omitempty"`
	FunctionCall     *deepVFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *deepVFunctionResponse `json:"functionResponse,omitempty"`
}

type deepVFunctionCall struct {
	ID   string                 `json:"id,omitempty"`
	Name string                 `json:"name,omitempty"`
	Args map[string]interface{} `json:"args,omitempty"`
}

type deepVFunctionResponse struct {
	ID       string                 `json:"id,omitempty"`
	Name     string                 `json:"name,omitempty"`
	Response map[string]interface{} `json:"response,omitempty"`
}

type deepVConfig struct {
	MaxOutputTokens int         `json:"maxOutputTokens,omitempty"`
	Temperature     float64     `json:"temperature,omitempty"`
	Tools           []deepVTool `json:"tools,omitempty"`
}

type deepVTool struct {
	FunctionDeclarations []deepVFunctionDecl `json:"functionDeclarations,omitempty"`
}

type deepVFunctionDecl struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type deepVResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text         string `json:"text,omitempty"`
				FunctionCall *struct {
					ID   string                 `json:"id,omitempty"`
					Name string                 `json:"name,omitempty"`
					Args map[string]interface{} `json:"args,omitempty"`
				} `json:"functionCall,omitempty"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// ─── 构造函数 ──────────────────────────────────────────────────────────────────

func NewDeepVProvider(serverURL string, hp headerProvider) *DeepVProvider {
	return &DeepVProvider{
		serverURL:      strings.TrimRight(serverURL, "/"),
		client:         &http.Client{Timeout: 300 * time.Second},
		headerProvider: hp,
	}
}

func (p *DeepVProvider) Name() string { return "deepv" }

func (p *DeepVProvider) StreamSimple(ctx context.Context, req ai.SimpleStreamRequest) (*ai.EventStream, error) {
	return p.Stream(ctx, ai.StreamRequest{
		Model:     req.Model,
		Messages:  req.Messages,
		System:    req.System,
		Tools:     req.Tools,
		MaxTokens: req.MaxTokens,
	})
}

// ─── Stream（核心方法）────────────────────────────────────────────────────────

func (p *DeepVProvider) Stream(ctx context.Context, req ai.StreamRequest) (*ai.EventStream, error) {
	stream := ai.NewEventStream(8)

	deepVReq, err := p.convertRequest(req)
	if err != nil {
		return nil, fmt.Errorf("convert request: %w", err)
	}

	body, err := json.Marshal(deepVReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 使用流式端点
	streamURL := p.serverURL + "/v1/chat/stream"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, streamURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if err := p.setHeaders(httpReq); err != nil {
		return nil, fmt.Errorf("set headers: %w", err)
	}

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
			errMsg := fmt.Sprintf("DeepV API error %d: %s", resp.StatusCode, string(respBody))
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

// ─── SSE 解析（GenAI 格式 → pi-go 事件）───────────────────────────────────────

func (p *DeepVProvider) handleSSE(ctx context.Context, stream *ai.EventStream, body io.Reader, partial *ai.StreamAssistantMessage) {
	_ = stream.Push(ctx, ai.EventStart{Partial: *partial})

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	var toolCalls []ai.ToolCall
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

		var chunk deepVResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}

		for _, candidate := range chunk.Candidates {
			for _, part := range candidate.Content.Parts {
				// 文本内容
				if part.Text != "" {
					if !textBlockStarted {
						textBlockStarted = true
						_ = stream.Push(ctx, ai.EventTextStart{ContentIndex: 0, Partial: *partial})
					}
					textAccum.WriteString(part.Text)
					_ = stream.Push(ctx, ai.EventTextDelta{ContentIndex: 0, Delta: part.Text, Partial: *partial})
				}

				// 工具调用
				if part.FunctionCall != nil {
					argsJSON, _ := json.Marshal(part.FunctionCall.Args)
					tc := ai.ToolCall{
						ID:   part.FunctionCall.ID,
						Name: part.FunctionCall.Name,
						Args: string(argsJSON),
					}
					// 如果没有 ID，生成一个
					if tc.ID == "" {
						tc.ID = fmt.Sprintf("deepv_%d", time.Now().UnixNano())
					}
					toolCalls = append(toolCalls, tc)
				}
			}
		}
	}

	// Deduplicate tool call IDs: DeepV server may return duplicate IDs for the same function call.
	seen := make(map[string]bool, len(toolCalls))
	deduped := make([]ai.ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		if seen[tc.ID] {
			continue
		}
		seen[tc.ID] = true
		deduped = append(deduped, tc)
	}
	toolCalls = deduped

	// 关闭文本 block
	if textBlockStarted {
		partial.Text = textAccum.String()
		_ = stream.Push(ctx, ai.EventTextEnd{ContentIndex: 0, Text: partial.Text, Partial: *partial})
	}

	// 输出 tool_calls 事件
	for i, tc := range toolCalls {
		partial.ToolCalls = append(partial.ToolCalls, tc)
		toolIdx := i
		if textBlockStarted {
			toolIdx = i + 1
		}
		_ = stream.Push(ctx, ai.EventToolCallStart{ContentIndex: toolIdx, Partial: *partial})
		_ = stream.Push(ctx, ai.EventToolCallDelta{ContentIndex: toolIdx, Delta: tc.Args, Partial: *partial})
		_ = stream.Push(ctx, ai.EventToolCallEnd{ContentIndex: toolIdx, ToolCall: tc, Partial: *partial})
	}

	// 设置 stop reason
	if len(toolCalls) > 0 {
		partial.StopReason = ai.StopReasonToolUse
	} else {
		partial.StopReason = ai.StopReasonStop
	}

	_ = stream.Push(ctx, ai.EventDone{Reason: partial.StopReason, Message: *partial})
	stream.SetResult(*partial, scanner.Err())
}

// ─── 请求格式转换（pi-go → GenAI）─────────────────────────────────────────────

func (p *DeepVProvider) convertRequest(req ai.StreamRequest) (*deepVRequest, error) {
	result := &deepVRequest{
		Model: req.Model.ID,
	}

	// 记录 tool_use ID → name 映射，用于 tool_result
	toolUseIDToName := make(map[string]string)

	// 转换 messages
	for _, msg := range req.Messages {
		switch m := msg.(type) {
		case ai.UserMessage:
			content := deepVContent{Role: "user"}
			for _, block := range m.Content {
				if block.Type == "text" && block.Text != "" {
					content.Parts = append(content.Parts, deepVPart{Text: block.Text})
				}
			}
			if len(content.Parts) > 0 {
				result.Contents = append(result.Contents, content)
			}

		case ai.AssistantMessage:
			content := deepVContent{Role: "model"} // GenAI 用 "model" 而非 "assistant"
			if m.Text != "" {
				content.Parts = append(content.Parts, deepVPart{Text: m.Text})
			}
			for _, tc := range m.ToolCalls {
				toolUseIDToName[tc.ID] = tc.Name
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Args), &args); err != nil {
					args = make(map[string]interface{})
				}
				content.Parts = append(content.Parts, deepVPart{
					FunctionCall: &deepVFunctionCall{
						ID:   tc.ID,
						Name: tc.Name,
						Args: args,
					},
				})
			}
			if len(content.Parts) > 0 {
				result.Contents = append(result.Contents, content)
			}

		case ai.ToolResultMessage:
			// GenAI 的 functionResponse 放在 "user" role 下
			content := deepVContent{Role: "user"}
			toolName := toolUseIDToName[m.ToolCallID]
			if toolName == "" {
				toolName = m.ToolCallID
			}
			responseContent := m.Content
			if responseContent == "" {
				responseContent = "(empty)"
			}
			content.Parts = append(content.Parts, deepVPart{
				FunctionResponse: &deepVFunctionResponse{
					ID:   m.ToolCallID,
					Name: toolName,
					Response: map[string]interface{}{
						"result": responseContent,
					},
				},
			})
			result.Contents = append(result.Contents, content)
		}
	}

	// system prompt → systemInstruction
	if req.System != "" {
		result.SystemInstruction = &deepVContent{
			Parts: []deepVPart{{Text: req.System}},
		}
	}

	// config
	maxTokens := 4096
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	result.Config = &deepVConfig{
		MaxOutputTokens: maxTokens,
		Temperature:     0.7,
	}

	// tools
	if len(req.Tools) > 0 {
		tool := deepVTool{
			FunctionDeclarations: make([]deepVFunctionDecl, len(req.Tools)),
		}
		for i, t := range req.Tools {
			params, _ := t.Parameters.(map[string]any)
			if params == nil {
				params = make(map[string]interface{})
			}
			tool.FunctionDeclarations[i] = deepVFunctionDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			}
		}
		result.Config.Tools = []deepVTool{tool}
	}

	return result, nil
}

// ─── HTTP Headers ─────────────────────────────────────────────────────────────

func (p *DeepVProvider) setHeaders(req *http.Request) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DeepVCode/CLI/1.0.338 (darwin; arm64)")
	req.Header.Set("X-Client-Version", "1.0.338")

	// 认证 token
	token, err := p.getAccessToken()
	if err != nil {
		slog.Warn("failed to get DeepV access token", "error", err)
	} else if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if p.headerProvider != nil {
		for key, value := range p.headerProvider.Headers() {
			if value != "" {
				req.Header.Set(key, value)
			}
		}
	}

	return nil
}

// ─── Token 读取 ───────────────────────────────────────────────────────────────

func (p *DeepVProvider) getAccessToken() (string, error) {
	// 检查缓存
	if p.tokenCache != nil && time.Now().Unix() < p.tokenCache.ExpiresAt {
		return p.tokenCache.AccessToken, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	tokenPath := filepath.Join(homeDir, ".deepv", "jwt-token.json")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("read ~/.deepv/jwt-token.json: %w", err)
	}

	var token jwtToken
	if err := json.Unmarshal(data, &token); err != nil {
		return "", fmt.Errorf("parse jwt-token.json: %w", err)
	}

	p.tokenCache = &token
	return token.AccessToken, nil
}
