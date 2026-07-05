package mcp

// types.go — MCP (Model Context Protocol) 协议类型定义。
//
// MCP 使用 JSON-RPC 2.0 over stdio / SSE 进行通信。本文件定义了
// JSON-RPC 消息信封、MCP 核心 capability / tool / resource / prompt 类型，
// 与 hwjcode TypeScript 版本保持一致。
//
// 参考:
//   - https://spec.modelcontextprotocol.io/
//   - hwjcode: packages/core/src/tools/mcp-client.ts

import "encoding/json"

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 消息信封
// ---------------------------------------------------------------------------

// JSONRPCRequest 是 JSON-RPC 2.0 请求 / 通知信封。
// 通知没有 id 字段（见 JSONRPCNotification）。
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`          // 固定 "2.0"
	ID      json.RawMessage `json:"id"`               // string | int | null（通知时省略）
	Method  string          `json:"method"`           // 方法名，如 "initialize"
	Params  json.RawMessage `json:"params,omitempty"` // 参数（可选）
}

// JSONRPCNotification 是不带 id 的 JSON-RPC 消息（单向通知）。
type JSONRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse 是 JSON-RPC 2.0 成功响应。
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
}

// JSONRPCError 是 JSON-RPC 2.0 错误对象。
type JSONRPCError struct {
	Code    int             `json:"code"`           // 错误码（如 -32601 Method not found）
	Message string          `json:"message"`        // 简短错误描述
	Data    json.RawMessage `json:"data,omitempty"` // 附加信息（可选）
}

// JSONRPCErrorResponse 是 JSON-RPC 2.0 错误响应。
type JSONRPCErrorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   JSONRPCError    `json:"error"`
}

// ---------------------------------------------------------------------------
// MCP initialize 握手
// ---------------------------------------------------------------------------

// ClientInfo 描述客户端 / 服务器的名称和版本（用于 initialize 握手）。
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities 声明客户端支持的能力（initialize 请求中发送）。
type ClientCapabilities struct {
	Roots        *RootsCapability        `json:"roots,omitempty"`
	Sampling     *struct{}               `json:"sampling,omitempty"`
	Elicitation  *struct{}               `json:"elicitation,omitempty"`
	Experimental map[string]any          `json:"experimental,omitempty"`
}

// RootsCapability 声明客户端的 roots（文件系统根）能力。
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerCapabilities 声明服务器支持的能力（initialize 响应中返回）。
type ServerCapabilities struct {
	Tools        *ToolsCapability        `json:"tools,omitempty"`
	Resources    *ResourcesCapability    `json:"resources,omitempty"`
	Prompts      *PromptsCapability      `json:"prompts,omitempty"`
	Logging      *struct{}               `json:"logging,omitempty"`
	Experimental map[string]any          `json:"experimental,omitempty"`
}

// ToolsCapability 声明服务器的工具能力。
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability 声明服务器的资源能力。
type ResourcesCapability struct {
	ListChanged   bool `json:"listChanged,omitempty"`
	Subscribe     bool `json:"subscribe,omitempty"`
}

// PromptsCapability 声明服务器的提示模板能力。
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializeParams 是 initialize 请求的参数。
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

// InitializeResult 是 initialize 请求的响应。
type InitializeResult struct {
	ProtocolVersion string              `json:"protocolVersion"`
	Capabilities    ServerCapabilities  `json:"capabilities"`
	ServerInfo      ClientInfo          `json:"serverInfo"`
	Instructions    string              `json:"instructions,omitempty"`
}

// ---------------------------------------------------------------------------
// MCP tools/list, tools/call
// ---------------------------------------------------------------------------

// MCPTool 是 tools/list 返回的单个工具描述。
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"` // JSON Schema
}

// ListToolsResult 是 tools/list 的响应。
type ListToolsResult struct {
	Tools      []MCPTool `json:"tools"`
	NextCursor string    `json:"nextCursor,omitempty"`
}

// CallToolParams 是 tools/call 请求的参数。
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResultContent 是工具调用返回的内容项（text / image / resource 等）。
// MCP 的 content 是一个异构数组，这里用 RawMessage 保留原始结构，
// 同时提供 Text 字段方便常见场景直接读取。
type CallToolResultContent struct {
	Type string          `json:"type"`
	Text string          `json:"text,omitempty"`
	Data json.RawMessage `json:"data,omitempty"` // 用于非 text 类型（image 等）
}

// CallToolResult 是 tools/call 的响应。
type CallToolResult struct {
	Content []CallToolResultContent `json:"content"`
	IsError bool                    `json:"isError,omitempty"`
}

// ---------------------------------------------------------------------------
// MCP resources/list, resources/read
// ---------------------------------------------------------------------------

// MCPResource 是 resources/list 返回的单个资源描述。
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ListResourcesResult 是 resources/list 的响应。
type ListResourcesResult struct {
	Resources  []MCPResource `json:"resources"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

// ResourceContent 是 resources/read 返回的内容项。
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// ReadResourceResult 是 resources/read 的响应。
type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ---------------------------------------------------------------------------
// MCP prompts/list, prompts/get
// ---------------------------------------------------------------------------

// MCPPromptArgument 是 prompt 模板的参数定义。
type MCPPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// MCPPrompt 是 prompts/list 返回的单个提示模板描述。
type MCPPrompt struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Arguments   []MCPPromptArgument `json:"arguments,omitempty"`
}

// ListPromptsResult 是 prompts/list 的响应。
type ListPromptsResult struct {
	Prompts    []MCPPrompt `json:"prompts"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

// ---------------------------------------------------------------------------
// 常量
// ---------------------------------------------------------------------------

const (
	// JSONRPCVersion 是 JSON-RPC 协议版本。
	JSONRPCVersion = "2.0"

	// MCPProtocolVersion 是 MCP 协议版本（2025-03-26 草案）。
	MCPProtocolVersion = "2024-11-05"

	// MCPClientName 是本客户端在 initialize 握手中报告的名称。
	MCPClientName = "pi-go-mcp-client"

	// MCPClientVersion 是本客户端报告的版本。
	MCPClientVersion = "1.0.0"
)

// 标准 JSON-RPC 错误码。
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)
