# 任务规划

> 本文档由主 Agent 在建群时自动生成，描述本群 agent 的工作目标。
> 文件持久保存在工作区根目录，可随时查看或编辑。

---

### 任务：移植 MCP (Model Context Protocol) 客户端到 pi-go

**目标**：从 hwjcode 移植 MCP 客户端，让 pi-go Agent 能连接外部 MCP 服务器，动态加载和使用第三方工具。

**参考源码（hwjcode TypeScript）**：
- `/home/q/hwjcode/packages/core/src/tools/mcp-client.ts` — MCP 客户端管理器
- `/home/q/hwjcode/packages/core/src/tools/mcp-tool.ts` — MCP 工具包装器（将 MCP 工具适配为 Agent Tool 接口）
- `/home/q/hwjcode/packages/core/src/mcp/oauth-utils.ts` — OAuth 工具
- `/home/q/hwjcode/packages/core/src/mcp/login-provider.ts` — 登录提供器
- `/home/q/hwjcode/packages/core/src/mcp/oauth-provider.ts` — OAuth 提供器
- `/home/q/hwjcode/packages/core/src/mcp/oauth-token-storage.ts` — Token 存储

**MCP 协议参考**：
- MCP 使用 JSON-RPC 2.0 over stdio 或 SSE
- 核心方法：`initialize`, `tools/list`, `tools/call`, `resources/list`, `resources/read`, `prompts/list`
- 服务器配置：命令 + 参数 + 环境变量

**输出到 pi-go（Go）**：
1. `internal/mcp/` 目录：
   - `client.go` — MCP 客户端（JSON-RPC 2.0 通信，支持 stdio 和 SSE 传输）
   - `manager.go` — MCP 服务器管理器（多服务器管理、生命周期、自动发现）
   - `tool.go` — MCP 工具适配器（将 MCP 工具适配为 pi-go `agent.Tool` 接口）
   - `config.go` — MCP 配置（从 `.pi-go/mcp.json` 或项目配置加载服务器定义）
   - `types.go` — MCP 协议类型定义
2. `internal/mcp/client_test.go` — 基础测试

**pi-go 架构约定（必须遵守）**：
- 先 `read_file` 看 `internal/agent/tool.go` 了解 Tool 接口（MCP 工具需要适配这个接口）
- 先 `read_file` 看 `internal/agent/external_tool.go` 了解 pi-go 现有的外部工具注册机制
- 先 `read_file` 看 `internal/tools/grep.go` 了解标准工具注册模式
- MCP 工具适配器需要把 MCP 的 tool schema 转换为 pi-go 的 tool 定义
- 用子进程 + stdin/stdout 管道实现 stdio 传输
- 用 HTTP + SSE 实现 SSE 传输
- 配置格式参考 hwjcode 的 MCP 配置（服务器名 → command + args + env）
- 用 `slog` 做日志
- 需要处理 MCP 服务器崩溃后的自动重连

**验收标准**：
- `go build ./...` 通过
- `go vet ./internal/mcp/` 通过
- client.go 测试：JSON-RPC 消息编解码、initialize 握手
- manager.go 测试：服务器注册、列表、健康检查
- config.go 测试：从 JSON 加载配置
