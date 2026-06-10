package skill

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultMCPHTTPTimeout     = 20 * time.Second
	defaultMCPHTTPMaxBytes    = 8 << 20
	mcpSessionIDHeader        = "Mcp-Session-Id"
	mcpProtocolVersionHeader  = "MCP-Protocol-Version"
	mcpDefaultJSONContentType = "application/json"
)

// HTTPMCPSkillClient talks to an MCP Streamable HTTP endpoint. It accepts both
// JSON responses and text/event-stream responses carrying JSON-RPC payloads.
type HTTPMCPSkillClient struct {
	Endpoint        string
	Token           string
	Headers         map[string]string
	ProtocolVersion string
	Timeout         time.Duration
	MaxBytes        int64
	Client          *http.Client
	requestID       atomic.Int64
	mu              sync.Mutex
	initialized     bool
	sessionID       string
}

func NewHTTPMCPSkillClient(endpoint, token string) *HTTPMCPSkillClient {
	return &HTTPMCPSkillClient{
		Endpoint:        strings.TrimSpace(endpoint),
		Token:           strings.TrimSpace(token),
		ProtocolVersion: defaultMCPProtocolVersion,
		Timeout:         defaultMCPHTTPTimeout,
		MaxBytes:        defaultMCPHTTPMaxBytes,
	}
}

func (c *HTTPMCPSkillClient) ListSkillResources(ctx context.Context) ([]MCPSkillResource, error) {
	var result mcpResourcesListResult
	if err := c.call(ctx, "resources/list", map[string]any{}, &result); err != nil {
		return nil, err
	}
	out := make([]MCPSkillResource, 0, len(result.Resources))
	for _, res := range result.Resources {
		uri := strings.TrimSpace(res.URI)
		if uri == "" {
			continue
		}
		out = append(out, MCPSkillResource{URI: uri, Name: strings.TrimSpace(res.Name)})
	}
	return out, nil
}

func (c *HTTPMCPSkillClient) ReadSkillResource(ctx context.Context, uri string) ([]byte, error) {
	var result mcpResourcesReadResult
	if err := c.call(ctx, "resources/read", map[string]any{"uri": uri}, &result); err != nil {
		return nil, err
	}
	return mcpResourceContents(uri, result)
}

func (c *HTTPMCPSkillClient) call(ctx context.Context, method string, params any, result any) error {
	if err := c.ensureInitialized(ctx); err != nil {
		return err
	}
	return c.rpc(ctx, method, params, result)
}

func (c *HTTPMCPSkillClient) ensureInitialized(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.initialized {
		return nil
	}
	protocolVersion := strings.TrimSpace(c.ProtocolVersion)
	if protocolVersion == "" {
		protocolVersion = defaultMCPProtocolVersion
	}
	var ignored json.RawMessage
	if err := c.rpcLocked(ctx, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]string{
			"name":    "pi-go",
			"version": "0",
		},
	}, &ignored); err != nil {
		return err
	}
	if err := c.notificationLocked(ctx, "notifications/initialized", map[string]any{}); err != nil {
		return err
	}
	c.initialized = true
	return nil
}

func (c *HTTPMCPSkillClient) rpc(ctx context.Context, method string, params any, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rpcLocked(ctx, method, params, result)
}

func (c *HTTPMCPSkillClient) rpcLocked(ctx context.Context, method string, params any, result any) error {
	id := c.requestID.Add(1)
	data, resp, err := c.postJSONRPC(ctx, mcpRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return err
	}
	c.captureSession(resp)
	payload, err := mcpResponsePayload(data, resp.Header.Get("Content-Type"), id)
	if err != nil {
		return fmt.Errorf("mcp request %q failed: %w", method, err)
	}
	var rpcResp mcpResponse
	if err := json.Unmarshal(payload, &rpcResp); err != nil {
		return fmt.Errorf("parse mcp response %q: %w", method, err)
	}
	if rpcResp.ID == nil || !mcpIDEqual(rpcResp.ID, id) {
		return fmt.Errorf("mcp response id did not match request %d", id)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("mcp request %q failed: %s", method, rpcResp.Error.Message)
	}
	if result == nil || len(rpcResp.Result) == 0 {
		return nil
	}
	if raw, ok := result.(*json.RawMessage); ok {
		*raw = append((*raw)[:0], rpcResp.Result...)
		return nil
	}
	if err := json.Unmarshal(rpcResp.Result, result); err != nil {
		return fmt.Errorf("parse mcp result %q: %w", method, err)
	}
	return nil
}

func (c *HTTPMCPSkillClient) notificationLocked(ctx context.Context, method string, params any) error {
	_, resp, err := c.postJSONRPC(ctx, mcpRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return err
	}
	c.captureSession(resp)
	return nil
}

func (c *HTTPMCPSkillClient) postJSONRPC(ctx context.Context, value any) ([]byte, *http.Response, error) {
	endpoint := strings.TrimSpace(c.Endpoint)
	if endpoint == "" {
		return nil, nil, fmt.Errorf("mcp http endpoint is empty")
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultMCPHTTPTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(value)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create mcp http request: %w", err)
	}
	req.Header.Set("Content-Type", mcpDefaultJSONContentType)
	req.Header.Set("Accept", "application/json, text/event-stream")
	if protocolVersion := strings.TrimSpace(c.ProtocolVersion); protocolVersion != "" {
		req.Header.Set(mcpProtocolVersionHeader, protocolVersion)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.sessionID != "" {
		req.Header.Set(mcpSessionIDHeader, c.sessionID)
	}
	for key, value := range c.Headers {
		if strings.TrimSpace(key) != "" {
			req.Header.Set(key, value)
		}
	}

	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("send mcp http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent {
		return nil, resp, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp, fmt.Errorf("mcp http endpoint returned HTTP %d", resp.StatusCode)
	}
	maxBytes := c.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMCPHTTPMaxBytes
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, resp, fmt.Errorf("read mcp http response: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, resp, fmt.Errorf("mcp http response exceeds %d bytes", maxBytes)
	}
	return data, resp, nil
}

func (c *HTTPMCPSkillClient) captureSession(resp *http.Response) {
	if resp == nil {
		return
	}
	if sessionID := strings.TrimSpace(resp.Header.Get(mcpSessionIDHeader)); sessionID != "" {
		c.sessionID = sessionID
	}
}

func mcpResponsePayload(data []byte, contentType string, id int64) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty mcp response")
	}
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return firstMatchingMCPEvent(data, id)
	}
	return data, nil
}

func firstMatchingMCPEvent(data []byte, id int64) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), int(defaultMCPHTTPMaxBytes))
	var eventData []string
	flush := func() ([]byte, bool) {
		if len(eventData) == 0 {
			return nil, false
		}
		payload := []byte(strings.Join(eventData, "\n"))
		eventData = nil
		var resp mcpResponse
		if err := json.Unmarshal(payload, &resp); err != nil {
			return nil, false
		}
		if resp.ID == nil || !mcpIDEqual(resp.ID, id) {
			return nil, false
		}
		return payload, true
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if payload, ok := flush(); ok {
				return payload, nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			eventData = append(eventData, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if payload, ok := flush(); ok {
		return payload, nil
	}
	return nil, fmt.Errorf("no matching mcp SSE response event")
}
