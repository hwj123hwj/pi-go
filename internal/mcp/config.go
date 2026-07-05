package mcp

// config.go — MCP 服务器配置定义与加载。
//
// 配置格式与 hwjcode / Claude Desktop 兼容。每个 MCP 服务器用名字做 key，
// 配置内容包含 command+args+env（stdio 传输）或 url（SSE / HTTP 传输）。
//
// 配置文件示例（.pi-go/mcp.json）：
//
//	{
//	  "mcpServers": {
//	    "filesystem": {
//	      "command": "npx",
//	      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
//	      "env": { "DEBUG": "true" }
//	    },
//	    "weather": {
//	      "url": "http://localhost:3000/sse"
//	    }
//	  }
//	}

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPServerConfig 描述单个 MCP 服务器的连接配置。
//
// 三种传输模式（优先级 command > HTTPURL > URL）：
//  1. stdio —— 通过 command + args 启动子进程，stdin/stdout 传输 JSON-RPC。
//  2. HTTP  —— 通过 HTTPURL 连接 Streamable HTTP 传输。
//  3. SSE   —— 通过 URL 连接 Server-Sent Events 传输。
type MCPServerConfig struct {
	// --- stdio 传输 ---
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`

	// --- SSE 传输 ---
	URL string `json:"url,omitempty"`

	// --- Streamable HTTP 传输 ---
	HTTPURL string            `json:"httpUrl,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	// --- 通用选项 ---
	Timeout int  `json:"timeout,omitempty"` // 超时（毫秒），0 表示用默认值
	Trust   bool `json:"trust,omitempty"`  // 受信服务器，跳过确认对话框

	// --- 工具过滤 ---
	IncludeTools []string `json:"includeTools,omitempty"` // 仅注册这些工具（白名单）
	ExcludeTools []string `json:"excludeTools,omitempty"` // 排除这些工具（黑名单，优先级更高）

	// --- 元数据 ---
	Description string `json:"description,omitempty"`
}

// Transport 返回配置隐含的传输类型："stdio" / "http" / "sse"。
func (c MCPServerConfig) Transport() string {
	switch {
	case c.Command != "":
		return "stdio"
	case c.HTTPURL != "":
		return "http"
	case c.URL != "":
		return "sse"
	default:
		return ""
	}
}

// Validate 检查配置是否有效（至少指定了一种传输方式）。
func (c MCPServerConfig) Validate() error {
	if c.Transport() == "" {
		return fmt.Errorf("invalid MCP server config: must specify command (stdio), httpUrl, or url")
	}
	if c.Command != "" {
		// stdio 模式下 command 必须非空（args 可为空）
		return nil
	}
	return nil
}

// ---------------------------------------------------------------------------
// 配置文件加载
// ---------------------------------------------------------------------------

// MCPConfigFile 是 mcp.json 文件的顶层结构。
type MCPConfigFile struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// LoadConfig 从给定的 JSON 文件路径加载 MCP 服务器配置。
// 文件格式见本文件头部注释。
func LoadConfig(path string) (*MCPConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mcp config %q: %w", path, err)
	}

	var cfg MCPConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse mcp config %q: %w", path, err)
	}

	// 校验每个服务器配置
	for name, sc := range cfg.MCPServers {
		if err := sc.Validate(); err != nil {
			return nil, fmt.Errorf("server %q: %w", name, err)
		}
	}

	return &cfg, nil
}

// LoadConfigFromDir 在给定目录下查找 mcp.json 并加载。
// 典型用法：LoadConfigFromDir(".pi-go") → 查找 .pi-go/mcp.json。
func LoadConfigFromDir(dir string) (*MCPConfigFile, error) {
	path := filepath.Join(dir, "mcp.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 目录或文件不存在不算错误，返回空配置
		return &MCPConfigFile{MCPServers: map[string]MCPServerConfig{}}, nil
	}
	return LoadConfig(path)
}

// ParseConfig 从原始 JSON 字节解析 MCP 配置（方便测试）。
func ParseConfig(data []byte) (*MCPConfigFile, error) {
	var cfg MCPConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	for name, sc := range cfg.MCPServers {
		if err := sc.Validate(); err != nil {
			return nil, fmt.Errorf("server %q: %w", name, err)
		}
	}
	return &cfg, nil
}
