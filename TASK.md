# 任务规划

> 本文档由主 Agent 在建群时自动生成，描述本群 agent 的工作目标。
> 文件持久保存在工作区根目录，可随时查看或编辑。

---

### 任务：移植 LSP 工具到 pi-go

**目标**：从 hwjcode (TypeScript) 移植 LSP (Language Server Protocol) 工具到 pi-go (Go)，让 Agent 能做 go-to-definition、find-references、hover、workspace-symbols 等代码导航操作。

**参考源码（hwjcode TypeScript）**：
- `/home/q/hwjcode/packages/core/src/tools/lsp.ts` — LSP 工具注册入口
- `/home/q/hwjcode/packages/core/src/tools/lsp/lsp-provider.ts` — LSP Manager 单例
- `/home/q/hwjcode/packages/core/src/tools/lsp/lsp-goto-definition.ts`
- `/home/q/hwjcode/packages/core/src/tools/lsp/lsp-find-references.ts`
- `/home/q/hwjcode/packages/core/src/tools/lsp/lsp-hover.ts`
- `/home/q/hwjcode/packages/core/src/tools/lsp/lsp-document-symbols.ts`
- `/home/q/hwjcode/packages/core/src/tools/lsp/lsp-workspace-symbols.ts`
- `/home/q/hwjcode/packages/core/src/tools/lsp/lsp-implementation.ts`
- `/home/q/hwjcode/packages/core/src/lsp/` — LSP Manager 核心实现

**输出到 pi-go（Go）**：
1. `internal/lsp/` — LSP Manager 核心（启动/管理语言服务器进程，JSON-RPC 通信）
2. `internal/tools/lsp.go` — 注册以下工具到 Agent tool registry：
   - `lsp_go_to_definition` — 跳转到定义
   - `lsp_find_references` — 查找引用
   - `lsp_hover` — 悬停信息
   - `lsp_document_symbols` — 文档符号
   - `lsp_workspace_symbols` — 工作区符号搜索
   - `lsp_go_to_implementation` — 跳转到实现

**pi-go 架构约定（必须遵守）**：
- 工具接口定义在 `internal/agent/tool.go`，先 `read_file` 看接口签名
- 工具注册方式参考 `internal/tools/grep.go` 或 `internal/tools/find.go`（先读这些文件了解模式）
- 每个 LSP 工具实现 `agent.Tool` 接口（Name()、Description()、Execute()）
- 用 `slog` 做日志，不要用 `log`
- LSP Manager 需要支持至少 Go (gopls) 语言服务器，设计成可扩展多语言
- 用 JSON-RPC 2.0 与语言服务器进程通信
- 需要处理 LSP 进程的生命周期（启动、关闭、超时）

**验收标准**：
- `go build ./...` 通过
- `go vet ./internal/lsp/ ./internal/tools/` 通过
- 写基础测试 `internal/lsp/lsp_test.go`（至少测试 Manager 初始化和进程启动）
- 工具能正确注册到 tool registry
