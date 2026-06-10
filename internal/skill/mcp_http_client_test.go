package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPMCPSkillClient_LoadsJSONAndSSEResources(t *testing.T) {
	var sawSession bool
	var sawBearer bool
	client := NewHTTPMCPSkillClient("https://mcp.example.test/mcp", "token")
	client.Client = &http.Client{Transport: mcpHTTPRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		if r.Header.Get("Authorization") == "Bearer token" {
			sawBearer = true
		}
		var req struct {
			ID     json.RawMessage `json:"id,omitempty"`
			Method string          `json:"method"`
			Params struct {
				URI string `json:"uri"`
			} `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.Method {
		case "initialize":
			return httpMCPResult(t, req.ID, map[string]any{
				"protocolVersion": defaultMCPProtocolVersion,
				"capabilities":    map[string]any{"resources": map[string]any{}},
				"serverInfo":      map[string]string{"name": "test-mcp", "version": "0"},
			}, http.Header{mcpSessionIDHeader: []string{"session-1"}}), nil
		case "notifications/initialized":
			if r.Header.Get(mcpSessionIDHeader) == "session-1" {
				sawSession = true
			}
			return httpMCPStatus(http.StatusAccepted), nil
		case "resources/list":
			assert.Equal(t, "session-1", r.Header.Get(mcpSessionIDHeader))
			sawSession = true
			body := fmt.Sprintf("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%s,\"result\":{\"resources\":[{\"uri\":\"mcp://remote/skills/deck/SKILL.md\",\"name\":\"deck\"}]}}\n\n", string(req.ID))
			return httpMCPResponse(http.StatusOK, "text/event-stream", body, nil), nil
		case "resources/read":
			assert.Equal(t, "mcp://remote/skills/deck/SKILL.md", req.Params.URI)
			return httpMCPResult(t, req.ID, map[string]any{
				"contents": []map[string]string{{
					"uri":  req.Params.URI,
					"text": "---\nname: deck\ndescription: Build remote decks\n---\nRemote deck workflow.",
				}},
			}, nil), nil
		default:
			return httpMCPError(t, req.ID, -32601, "method not found"), nil
		}
	})}

	resources, err := client.ListSkillResources(context.Background())
	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "mcp://remote/skills/deck/SKILL.md", resources[0].URI)

	data, err := client.ReadSkillResource(context.Background(), resources[0].URI)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Build remote decks")
	assert.True(t, sawSession)
	assert.True(t, sawBearer)
}

func TestHTTPMCPSkillSourceProvider_LoadsSkills(t *testing.T) {
	client := NewHTTPMCPSkillClient("https://mcp.example.test/mcp", "")
	client.Client = &http.Client{Transport: mcpHTTPRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		var req struct {
			ID     json.RawMessage `json:"id,omitempty"`
			Method string          `json:"method"`
			Params struct {
				URI string `json:"uri"`
			} `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.Method {
		case "initialize":
			return httpMCPResult(t, req.ID, map[string]any{"protocolVersion": defaultMCPProtocolVersion}, nil), nil
		case "notifications/initialized":
			return httpMCPStatus(http.StatusAccepted), nil
		case "resources/list":
			return httpMCPResult(t, req.ID, map[string]any{
				"resources": []map[string]string{{"uri": "mcp://remote/skills/deck/SKILL.md", "name": "deck"}},
			}, nil), nil
		case "resources/read":
			return httpMCPResult(t, req.ID, map[string]any{
				"contents": []map[string]string{{
					"uri":  req.Params.URI,
					"text": "---\nname: deck\ndescription: Build remote decks\n---\nRemote deck workflow.",
				}},
			}, nil), nil
		default:
			return httpMCPError(t, req.ID, -32601, "method not found"), nil
		}
	})}

	result := NewMCPSkillSourceProvider("remote", client).LoadSkills(context.Background())
	require.Empty(t, result.Diagnostics)
	require.Len(t, result.Skills, 1)
	assert.Equal(t, "deck", result.Skills[0].Name)
	assert.Equal(t, "Remote deck workflow.", result.Skills[0].Content)
	assert.Equal(t, "mcp://remote/skills/deck", result.Skills[0].BaseDir)
}

func TestHTTPMCPSkillClient_ReportsRPCError(t *testing.T) {
	client := NewHTTPMCPSkillClient("https://mcp.example.test/mcp", "")
	client.Client = &http.Client{Transport: mcpHTTPRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		var req struct {
			ID     json.RawMessage `json:"id,omitempty"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.Method {
		case "initialize":
			return httpMCPResult(t, req.ID, map[string]any{"protocolVersion": defaultMCPProtocolVersion}, nil), nil
		case "notifications/initialized":
			return httpMCPStatus(http.StatusAccepted), nil
		default:
			return httpMCPError(t, req.ID, -32004, "not found"), nil
		}
	})}

	_, err := client.ReadSkillResource(context.Background(), "mcp://remote/missing/SKILL.md")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

type mcpHTTPRoundTripFunc func(*http.Request) (*http.Response, error)

func (f mcpHTTPRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func httpMCPResult(t *testing.T, id json.RawMessage, result any, header http.Header) *http.Response {
	t.Helper()
	var b strings.Builder
	require.NoError(t, json.NewEncoder(&b).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"result":  result,
	}))
	return httpMCPResponse(http.StatusOK, "application/json", b.String(), header)
}

func httpMCPError(t *testing.T, id json.RawMessage, code int, message string) *http.Response {
	t.Helper()
	var b strings.Builder
	require.NoError(t, json.NewEncoder(&b).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}))
	return httpMCPResponse(http.StatusOK, "application/json", b.String(), nil)
}

func httpMCPStatus(status int) *http.Response {
	return httpMCPResponse(status, "", "", nil)
}

func httpMCPResponse(status int, contentType, body string, header http.Header) *http.Response {
	if header == nil {
		header = http.Header{}
	}
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
