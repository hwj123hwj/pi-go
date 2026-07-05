package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// types.go 测试
// ---------------------------------------------------------------------------

func TestJSONRPCRequestMarshal(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      mustMarshalJSON(1),
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}
	data, err := json.Marshal(req)
	require.NoError(t, err)

	// 验证 JSON-RPC 字段
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &m))
	assert.JSONEq(t, `"2.0"`, string(m["jsonrpc"]))
	assert.JSONEq(t, `"initialize"`, string(m["method"]))
	assert.JSONEq(t, `1`, string(m["id"]))
}

func TestJSONRPCErrorUnmarshal(t *testing.T) {
	raw := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`)
	var resp JSONRPCErrorResponse
	require.NoError(t, json.Unmarshal(raw, &resp))
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Equal(t, "Method not found", resp.Error.Message)
}

func TestInitializeParamsMarshal(t *testing.T) {
	params := InitializeParams{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities:    ClientCapabilities{},
		ClientInfo:      ClientInfo{Name: "test-client", Version: "0.1"},
	}
	data, err := json.Marshal(params)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, MCPProtocolVersion, m["protocolVersion"])
}

func TestCallToolParamsMarshal(t *testing.T) {
	params := CallToolParams{
		Name:      "search",
		Arguments: json.RawMessage(`{"query":"hello"}`),
	}
	data, err := json.Marshal(params)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "search", m["name"])
}

func TestMCPToolJSONRoundTrip(t *testing.T) {
	tool := MCPTool{
		Name:        "read_file",
		Description: "Read a file",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
	}
	data, err := json.Marshal(tool)
	require.NoError(t, err)

	var decoded MCPTool
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "read_file", decoded.Name)
	assert.Equal(t, "Read a file", decoded.Description)
	assert.Contains(t, string(decoded.InputSchema), "object")
}

// ---------------------------------------------------------------------------
// config.go 测试
// ---------------------------------------------------------------------------

func TestParseConfig(t *testing.T) {
	raw := []byte(`{
		"mcpServers": {
			"filesystem": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
				"env": {"DEBUG": "true"}
			},
			"weather": {
				"url": "http://localhost:3000/sse"
			},
			"api": {
				"httpUrl": "http://localhost:8080/mcp"
			}
		}
	}`)

	cfg, err := ParseConfig(raw)
	require.NoError(t, err)
	require.Len(t, cfg.MCPServers, 3)

	// stdio 配置
	fs := cfg.MCPServers["filesystem"]
	assert.Equal(t, "stdio", fs.Transport())
	assert.Equal(t, "npx", fs.Command)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"}, fs.Args)
	assert.Equal(t, "true", fs.Env["DEBUG"])

	// SSE 配置
	weather := cfg.MCPServers["weather"]
	assert.Equal(t, "sse", weather.Transport())
	assert.Equal(t, "http://localhost:3000/sse", weather.URL)

	// HTTP 配置
	api := cfg.MCPServers["api"]
	assert.Equal(t, "http", api.Transport())
	assert.Equal(t, "http://localhost:8080/mcp", api.HTTPURL)
}

func TestParseConfig_Invalid(t *testing.T) {
	// 缺少所有传输配置
	raw := []byte(`{"mcpServers":{"bad":{}}}`)
	_, err := ParseConfig(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must specify")
}

func TestParseConfig_IncludeExclude(t *testing.T) {
	raw := []byte(`{
		"mcpServers": {
			"filtered": {
				"command": "echo",
				"includeTools": ["tool_a", "tool_b"],
				"excludeTools": ["tool_c"]
			}
		}
	}`)
	cfg, err := ParseConfig(raw)
	require.NoError(t, err)
	s := cfg.MCPServers["filtered"]
	assert.Equal(t, []string{"tool_a", "tool_b"}, s.IncludeTools)
	assert.Equal(t, []string{"tool_c"}, s.ExcludeTools)
}

func TestMCPServerConfigTransport(t *testing.T) {
	tests := []struct {
		name     string
		cfg      MCPServerConfig
		expected string
	}{
		{"stdio", MCPServerConfig{Command: "node"}, "stdio"},
		{"sse", MCPServerConfig{URL: "http://x/sse"}, "sse"},
		{"http", MCPServerConfig{HTTPURL: "http://x/mcp"}, "http"},
		{"empty", MCPServerConfig{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cfg.Transport())
		})
	}
}

// ---------------------------------------------------------------------------
// tool.go 测试
// ---------------------------------------------------------------------------

func TestGenerateValidName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"read_file", "read_file"},
		{"read-file", "read-file"},
		{"read file", "read_file"},
		{"read.file", "read_file"},
		{"123start", "_123start"},
		{"", "_"},
		{"already_valid_name-123", "already_valid_name-123"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, generateValidName(tt.input))
		})
	}
}

func TestGenerateValidToolName(t *testing.T) {
	name := generateValidToolName("my-server", "read.file")
	assert.Equal(t, "my-server__read_file", name)
}

func TestMCPToolAdapterName(t *testing.T) {
	adapter := &MCPToolAdapter{
		tool:       MCPTool{Name: "search"},
		serverName: "brave",
	}
	assert.Equal(t, "brave__search", adapter.Name())
}

func TestMCPToolAdapterParameters(t *testing.T) {
	t.Run("with schema", func(t *testing.T) {
		adapter := &MCPToolAdapter{
			tool: MCPTool{
				Name:        "test",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`),
			},
			serverName: "srv",
		}
		params := adapter.Parameters()
		assert.Equal(t, "object", params["type"])
	})

	t.Run("without schema returns default", func(t *testing.T) {
		adapter := &MCPToolAdapter{
			tool:       MCPTool{Name: "test"},
			serverName: "srv",
		}
		params := adapter.Parameters()
		assert.Equal(t, "object", params["type"])
		props, ok := params["properties"].(map[string]any)
		require.True(t, ok)
		assert.Empty(t, props)
	})

	t.Run("invalid schema returns default", func(t *testing.T) {
		adapter := &MCPToolAdapter{
			tool:       MCPTool{Name: "test", InputSchema: json.RawMessage(`invalid`)},
			serverName: "srv",
		}
		params := adapter.Parameters()
		assert.Equal(t, "object", params["type"])
	})
}

func TestMCPToolAdapterValidate(t *testing.T) {
	adapter := &MCPToolAdapter{
		tool:       MCPTool{Name: "test"},
		serverName: "srv",
	}
	raw := json.RawMessage(`{"key":"value"}`)
	validated, err := adapter.Validate(raw)
	require.NoError(t, err)
	assert.Equal(t, raw, validated)
}

// ---------------------------------------------------------------------------
// manager.go 测试
// ---------------------------------------------------------------------------

func TestManagerRegisterAndList(t *testing.T) {
	m := NewManager()

	err := m.RegisterServer("fs", MCPServerConfig{Command: "echo"})
	require.NoError(t, err)

	err = m.RegisterServer("api", MCPServerConfig{URL: "http://localhost/sse"})
	require.NoError(t, err)

	servers := m.ListServers()
	assert.Len(t, servers, 2)
	assert.Contains(t, servers, "fs")
	assert.Contains(t, servers, "api")
}

func TestManagerRegisterInvalidConfig(t *testing.T) {
	m := NewManager()
	err := m.RegisterServer("bad", MCPServerConfig{}) // 无传输配置
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must specify")
}

func TestManagerRegisterDuplicate(t *testing.T) {
	m := NewManager()
	require.NoError(t, m.RegisterServer("srv", MCPServerConfig{Command: "echo"}))
	// 覆盖同名
	require.NoError(t, m.RegisterServer("srv", MCPServerConfig{Command: "node"}))

	m.mu.RLock()
	cfg := m.servers["srv"].config
	m.mu.RUnlock()
	assert.Equal(t, "node", cfg.Command)
}

func TestManagerStatus(t *testing.T) {
	m := NewManager()
	require.NoError(t, m.RegisterServer("srv", MCPServerConfig{Command: "echo"}))

	status, ok := m.GetStatus("srv")
	require.True(t, ok)
	assert.Equal(t, StatusDisconnected, status)

	_, ok = m.GetStatus("nonexistent")
	assert.False(t, ok)
}

func TestManagerAllStatuses(t *testing.T) {
	m := NewManager()
	require.NoError(t, m.RegisterServer("a", MCPServerConfig{Command: "echo"}))
	require.NoError(t, m.RegisterServer("b", MCPServerConfig{Command: "echo"}))

	statuses := m.AllStatuses()
	assert.Len(t, statuses, 2)
	assert.Equal(t, StatusDisconnected, statuses["a"])
	assert.Equal(t, StatusDisconnected, statuses["b"])
}

func TestManagerUnregister(t *testing.T) {
	m := NewManager()
	require.NoError(t, m.RegisterServer("srv", MCPServerConfig{Command: "echo"}))

	err := m.UnregisterServer(context.Background(), "srv")
	require.NoError(t, err)

	assert.NotContains(t, m.ListServers(), "srv")
}

func TestManagerUnregisterNotFound(t *testing.T) {
	m := NewManager()
	err := m.UnregisterServer(context.Background(), "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestIsToolEnabled(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		cfg      MCPServerConfig
		expected bool
	}{
		{"no filter", "any", MCPServerConfig{}, true},
		{"in include", "good", MCPServerConfig{IncludeTools: []string{"good", "ok"}}, true},
		{"not in include", "bad", MCPServerConfig{IncludeTools: []string{"good"}}, false},
		{"in exclude", "bad", MCPServerConfig{ExcludeTools: []string{"bad"}}, false},
		{"exclude overrides include", "bad", MCPServerConfig{IncludeTools: []string{"bad"}, ExcludeTools: []string{"bad"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isToolEnabled(tt.toolName, tt.cfg))
		})
	}
}

func TestManagerAllToolsEmpty(t *testing.T) {
	m := NewManager()
	require.NoError(t, m.RegisterServer("srv", MCPServerConfig{Command: "echo"}))
	// 未连接，不应有工具
	tools := m.AllTools()
	assert.Empty(t, tools)
}

// ---------------------------------------------------------------------------
// parseID 测试
// ---------------------------------------------------------------------------

func TestParseID(t *testing.T) {
	t.Run("numeric", func(t *testing.T) {
		id, err := parseID(json.RawMessage(`42`))
		require.NoError(t, err)
		assert.Equal(t, int64(42), id)
	})

	t.Run("string returns error", func(t *testing.T) {
		_, err := parseID(json.RawMessage(`"abc"`))
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// 辅助函数测试
// ---------------------------------------------------------------------------

func TestMustMarshalJSON(t *testing.T) {
	raw := mustMarshalJSON(42)
	assert.JSONEq(t, `42`, string(raw))
}

// 确保 ConnectTimeout / DefaultTimeout 常量被使用（避免 unused 警告）
func TestTimeoutConstants(t *testing.T) {
	assert.Equal(t, 10*time.Minute, DefaultTimeout)
	assert.Equal(t, 30*time.Second, ConnectTimeout)
}
