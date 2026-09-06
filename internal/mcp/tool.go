package mcp

// tool.go — MCP 工具适配器：将 MCP 服务器提供的工具适配为 pi-go agent.Tool 接口。
//
// pi-go 的 agent.Tool 接口（见 internal/agent/tool.go）要求实现：
//   - Name() string
//   - Description() string
//   - Parameters() map[string]any
//   - Validate(json.RawMessage) (json.RawMessage, error)
//   - Execute(ctx, params, onUpdate) (ToolResult, error)
//
// MCP 工具通过 tools/list 获取元数据（name/description/inputSchema），
// 通过 tools/call 执行。本文件中的 MCPToolAdapter 将这些操作桥接到 agent.Tool。
//
// 参考 hwjcode: packages/core/src/tools/mcp-tool.ts (DiscoveredMCPTool)

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/hwj123hwj/pi-go/sdk/agent"
)

// ---------------------------------------------------------------------------
// MCPToolAdapter
// ---------------------------------------------------------------------------

// MCPToolAdapter 将单个 MCP 工具适配为 pi-go agent.Tool 接口。
type MCPToolAdapter struct {
	client      *Client
	tool        MCPTool
	serverName  string
	description string
	timeout     time.Duration

	// 缓存解析后的 inputSchema（避免每次 Parameters() 都解析）
	paramsMu  sync.RWMutex
	params    map[string]any
	paramsErr error
	paramsOnce bool
}

// NewMCPToolAdapter 创建一个 MCP 工具适配器。
// serverName 用于工具名命名空间和日志。timeout 为 0 时使用默认值。
func NewMCPToolAdapter(client *Client, serverName string, tool MCPTool, timeout time.Duration) *MCPToolAdapter {
	t := &MCPToolAdapter{
		client:      client,
		tool:        tool,
		serverName:  serverName,
		description: tool.Description,
		timeout:     timeout,
	}
	if t.timeout == 0 {
		t.timeout = DefaultTimeout
	}
	if t.description == "" {
		t.description = fmt.Sprintf("MCP tool %s from server %s", tool.Name, serverName)
	}
	return t
}

// ToolName 返回 MCP 原始工具名（不含服务器前缀）。
func (t *MCPToolAdapter) ToolName() string { return t.tool.Name }

// ServerName 返回提供此工具的 MCP 服务器名。
func (t *MCPToolAdapter) ServerName() string { return t.serverName }

// Name 实现它不包含点号等无效字符，符合 Gemini / Claude 工具名规范。
func (t *MCPToolAdapter) Name() string {
	return generateValidToolName(t.serverName, t.tool.Name)
}

// Description 实现它不包含点号等无效字符，符合 Gemini / Claude 工具名规范。
func (t *MCPToolAdapter) Description() string {
	return t.description
}

// Parameters 返回工具参数的 JSON Schema（解析为 map[string]any）。
// 如果 MCP 工具没有提供 inputSchema，返回默认空 object schema。
func (t *MCPToolAdapter) Parameters() map[string]any {
	t.paramsMu.RLock()
	if t.paramsOnce {
		p, err := t.params, t.paramsErr
		t.paramsMu.RUnlock()
		if err != nil {
			return defaultObjectSchema()
		}
		return p
	}
	t.paramsMu.RUnlock()

	t.paramsMu.Lock()
	defer t.paramsMu.Unlock()
	if t.paramsOnce {
		return t.params
	}

	t.paramsOnce = true
	if len(t.tool.InputSchema) == 0 {
		t.params = defaultObjectSchema()
		return t.params
	}
	if err := json.Unmarshal(t.tool.InputSchema, &t.params); err != nil {
		t.paramsErr = err
		slog.Warn("MCP tool has invalid inputSchema, using default",
			"server", t.serverName, "tool", t.tool.Name, "err", err)
		t.params = defaultObjectSchema()
	}
	return t.params
}

// Validate 验证参数。当前实现直接透传——MCP 服务器会自行校验。
func (t *MCPToolAdapter) Validate(params json.RawMessage) (json.RawMessage, error) {
	return params, nil
}

// Execute 调用 MCP 服务器的 tools/call 方法执行工具。
func (t *MCPToolAdapter) Execute(ctx context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	slog.Debug("MCP tool execute", "server", t.serverName, "tool", t.tool.Name)

	result, err := t.client.CallTool(ctx, t.tool.Name, params)
	if err != nil {
		return agent.ToolResult{
			IsError: true,
			Content: fmt.Sprintf("MCP tool %q call failed: %v", t.tool.Name, err),
		}, nil
	}

	// 将 MCP CallToolResult.Content 拼接为字符串
	var b strings.Builder
	for _, c := range result.Content {
		if c.Text != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(c.Text)
		}
	}
	content := b.String()
	if content == "" {
		// 如果没有 text content，序列化整个 content 数组
		raw, _ := json.Marshal(result.Content)
		content = string(raw)
	}

	return agent.ToolResult{
		Content: content,
		IsError: result.IsError,
	}, nil
}

// IsConcurrencySafe 实现 agent.ConcurrencySafeChecker。
// MCP 工具调用是到外部服务器的 RPC，可以安全并发。
func (t *MCPToolAdapter) IsConcurrencySafe(_ json.RawMessage) bool {
	return true
}

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

// defaultObjectSchema 返回一个空的 object JSON Schema。
func defaultObjectSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

// generateValidName 生成合法的工具名（兼容 Gemini / Claude）。
// 参考 hwjcode mcp-tool.ts 中的 generateValidName。
func generateValidName(name string) string {
	var b strings.Builder
	for _, ch := range name {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9', ch == '_', ch == '-':
			b.WriteRune(ch)
		default:
			b.WriteByte('_')
		}
	}
	valid := b.String()
	// 必须以字母或下划线开头
	if len(valid) == 0 || !((valid[0] >= 'a' && valid[0] <= 'z') ||
		(valid[0] >= 'A' && valid[0] <= 'Z') || valid[0] == '_') {
		valid = "_" + valid
	}
	// 最大 128 字符
	if len(valid) > 128 {
		valid = valid[:128]
	}
	return valid
}

// generateValidToolName 生成带服务器前缀的工具名：server__tool。
func generateValidToolName(serverName, toolName string) string {
	return generateValidName(serverName) + "__" + generateValidName(toolName)
}
