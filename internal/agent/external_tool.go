package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ExternalToolDef holds the registration payload for an external tool.
type ExternalToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	CallbackURL string         `json:"callback_url"`
}

// ToolCallbackRequest is sent to the callback URL when the tool is executed.
type ToolCallbackRequest struct {
	ToolName string          `json:"tool_name"`
	Params   json.RawMessage `json:"params"`
}

// ToolCallbackResponse is the expected response from the callback URL.
type ToolCallbackResponse struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// ExternalTool wraps a remote tool registered via HTTP callback.
type ExternalTool struct {
	def        ExternalToolDef
	httpClient *http.Client
}

// NewExternalTool creates an ExternalTool from a registration definition.
func NewExternalTool(def ExternalToolDef) (*ExternalTool, error) {
	if def.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}
	if def.CallbackURL == "" {
		return nil, fmt.Errorf("callback URL is required")
	}
	return &ExternalTool{
		def:        def,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}, nil
}

func (t *ExternalTool) Name() string              { return t.def.Name }
func (t *ExternalTool) Description() string        { return t.def.Description }
func (t *ExternalTool) Parameters() map[string]any { return t.def.Parameters }

func (t *ExternalTool) Validate(params json.RawMessage) (json.RawMessage, error) {
	return params, nil
}

func (t *ExternalTool) Execute(ctx context.Context, params json.RawMessage, _ func(PartialResult)) (ToolResult, error) {
	reqBody, _ := json.Marshal(ToolCallbackRequest{
		ToolName: t.def.Name,
		Params:   params,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", t.def.CallbackURL, bytes.NewReader(reqBody))
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("create request: %v", err), IsError: true}, nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("callback request failed: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	var cbResp ToolCallbackResponse
	if err := json.Unmarshal(data, &cbResp); err != nil {
		if resp.StatusCode != http.StatusOK {
			return ToolResult{Content: fmt.Sprintf("callback failed (HTTP %d): %s", resp.StatusCode, string(data)), IsError: true}, nil
		}
		return ToolResult{Content: string(data)}, nil
	}

	return ToolResult{Content: cbResp.Content, IsError: cbResp.IsError}, nil
}
