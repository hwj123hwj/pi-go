# Pi-Go 项目上下文

> 本文档是 pi-go 项目的高层快照，供调研/分析任务快速了解项目全貌，无需重读源码。
> 架构变更时应同步更新本文档。

---

## 定位

pi-go 是一个用 Go 实现的通用 Agent 框架，核心目标是：**可扩展的 Agent 底座 + 可插拔的应用层**。当前主要应用是 coding-agent（代码编辑助手）。

## 架构

```
┌─────────────────────────────────────────────────────┐
│  Entrypoints（组装与入口）                           │
│  app/ CLI/ server/                                   │
├─────────────────────────────────────────────────────┤
│  Application（领域应用层，可插拔）                    │
│  agents/coding/ — coding-agent 的工具集、提示、命令   │
├─────────────────────────────────────────────────────┤
│  Platform（运行时平台层，领域无关）                   │
│  runtime/ — AgentSession 生命周期、Application 接口  │
├─────────────────────────────────────────────────────┤
│  Core（核心层，零领域知识）                           │
│  ai/ agent/ session/ compaction/ operations/         │
│  prompt/ skill/ extensions/                          │
└─────────────────────────────────────────────────────┘
```

### 层间依赖规则

- **Core** → 不依赖上层任何包
- **Platform** → 只依赖 Core，通过 `runtime.Application` 接口与上层解耦
- **Application** → 实现 `runtime.Application` 接口，依赖 Core + Platform
- **Entrypoints** → 组装所有依赖，注入 Application 实例

### 关键接口

| 接口 | 位置 | 作用 |
|------|------|------|
| `runtime.Application` | `internal/runtime/application.go` | Platform 与 Application 的解耦点：`BuildTools()` + `BuildPrompt()` |
| `agent.Tool` | `internal/agent/` | 工具系统：泛型 schema + 执行函数 |
| `providers.Provider` | `internal/ai/providers/` | LLM Provider 抽象：注册制 + 懒加载 |
| `operations.Operations` | `internal/operations/` | 执行后端：本地 / SSH 切换 |
| `slashcmd.SessionContext` | `internal/slashcmd/context.go` | Slash command 可操作的 session 接口（model/profile/switch） |
| `slashcmd.AppContext` | `internal/slashcmd/context.go` | Slash command 可操作的 app 接口（session CRUD/profiles） |

## 核心能力

| 能力 | 实现位置 | 状态 |
|------|---------|------|
| 统一 LLM API 抽象 | `internal/ai/` | ✅ 多 Provider（Anthropic/OpenAI/Mock/DeepV） |
| Agent 双层循环 | `internal/agent/` | ✅ 外层 follow-up + 内层 tool call |
| 7 个内置工具 | `internal/tools/` + 组装于 `internal/agents/coding/tools/` | ✅ read/write/edit/bash/grep/find/ls |
| 工具过滤 | config AllowedTools/BlockedTools | ✅ |
| Tool Lifecycle hooks | `internal/agent/tool_lifecycle.go` | ✅ Before/After hook 接口 + PrepareArguments |
| Operations 抽象 | `internal/operations/` | ✅ Local + SSH 执行后端 |
| 事件系统 + 流式输出 | `internal/agent/` + SSE | ✅ |
| 上下文压缩 | `internal/compaction/` | ✅ LLM 摘要 + 保留最近消息 |
| JSONL 树状会话 | `internal/session/` | ✅ 分支 + MoveTo |
| 扩展系统 | `internal/extensions/` | ✅ 工具/命令/事件钩子 |
| 技能系统 | `internal/skill/` | ✅ Markdown 格式加载 |
| HTTP API Server | `internal/server/` | ✅ REST + SSE |
| 多执行模式 | app 层 | ✅ interactive/print/serve |
| SSH 远程执行 | `internal/operations/ssh.go` | ✅ |
| Slash Commands 框架 | `internal/slashcmd/` | ✅ 注册制 + 结构化 CommandResult + Session 交接 |
| CLI 控制面 | `internal/agents/coding/commands/` + `cli/` | ✅ /new /switch /sessions /model /tools /profiles /profile |
| Profile 机制 | `internal/agents/coding/profile/` | ✅ coding/review 双 profile，切换即重建 agent |

## 技术栈

- **语言**：Go 1.24+
- **外部依赖**：极简（标准库为主，少量第三方）
- **存储**：文件系统（JSONL 会话、JSON 配置）
- **构建**：Go modules，无复杂构建链

## 与 TypeScript Agent 项目的典型差异

| 维度 | Go (pi-go) | TypeScript (常见) |
|------|-----------|-------------------|
| 并发 | goroutine + channel | async/await + Promise |
| 错误处理 | 多返回值 error | try/catch + Error 对象 |
| 泛型 | 1.18+ 类型参数，较简洁 | 更灵活（条件类型等） |
| 接口 | 结构化类型，隐式实现 | 同为结构化类型但更灵活 |
| 工具链 | go build/test/vet，单一 | npm/tsc/bun，碎片化 |
| 分发 | 单二进制 | Node.js 运行时依赖 |

## 当前进展与方向

- **已完成**：runtime 从 coding-agent 解耦（Application 接口注入）
- **已完成**：CLI 控制面（session 切换、model 切换、profile 切换、结构化 slash command）
- **规划中**：Desktop App（Electron + React）、更多内置工具、Agent 协作
- **文档**：`docs/` 下有架构提案、产品路线图、编码规范

## 关键文件速查

| 文件 | 职责 |
|------|------|
| `internal/ai/stream.go` | 统一 LLM 流式 API 入口 |
| `internal/agent/agent.go` | Agent 核心状态机 |
| `internal/agent/agent-loop.go` | Agent 双层循环实现 |
| `internal/runtime/agent_session.go` | AgentSession 生命周期 |
| `internal/runtime/application.go` | Application 接口定义 |
| `internal/agents/coding/application.go` | CodingApplication 实现 |
| `internal/app/app.go` | 依赖组装 |
| `internal/session/session.go` | 树状会话存储 |
| `internal/compaction/compaction.go` | 上下文压缩策略 |
| `internal/config/config.go` | 配置结构定义 |
| `internal/slashcmd/registry.go` | Slash command 注册与执行框架 |
| `internal/agents/coding/commands/builtins.go` | Coding-agent 内置命令（/new /switch /model /profile 等） |
| `internal/agents/coding/profile/profile.go` | Profile 类型定义与 prompt 片段 |
| `internal/agents/coding/cli/interactive.go` | CLI 交互模式（命令分发 + session 交接） |
