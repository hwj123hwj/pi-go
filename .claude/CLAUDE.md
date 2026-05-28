# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 行为准则

减少常见 LLM 编码错误。**权衡：偏谨慎而非速度。简单任务自行判断。**

### 先想后写

- 不要假设。有困惑先问，不要默默选一个方案。
- 有更简单的方案直接说，该反驳就反驳。

### 简洁优先

- 最少代码解决问题，不写需求之外的功能。
- 不为单次调用创建抽象，不为不可能的场景做错误处理。
- 自问："一个 senior 工程师会觉得这过度设计了吗？"

### 精准改动

- 只碰必须改的，不要顺手优化相邻代码、注释、格式。
- 匹配现有风格，即使你觉得自己的写法更好。
- 删除你的改动产生的孤儿代码（import/变量/函数），但不要删之前就存在的死代码。

### 目标驱动执行

- 把任务转为可验证目标："加校验" → "先写测试用例，再让它通过"
- 多步骤任务先列计划，每步带验证标准。

---

## 项目文档

- 项目介绍 & 架构：`README.md` / `docs/PROJECT_CONTEXT.md`
- 开发流程 & 编码规范（分支命名、commit 格式等）：`docs/CONTRIBUTING.md`
- 架构决策：`docs/decisions/`
- 竞品调研：`docs/research/`

## 项目架构

四层分层，依赖方向：`Entrypoints → Application → Platform → Core`

```
cmd/pi-agent  cmd/pi-feishu-bridge     ← 入口
internal/agents/coding/                ← 应用层（工具、提示、Profile、命令）
internal/runtime/                      ← 平台层（AgentSession、Application 接口）
internal/agent/ ai/ session/ ...       ← 核心层（零领域知识）
```

层间规则：Core 不依赖上层；Platform 只依赖 Core；Application 通过 `runtime.Application` 接口解耦。

## 核心接口

| 接口 | 文件 | 作用 |
|------|------|------|
| `agent.Tool` | `internal/agent/tool.go` | 工具系统，可选接口：`ToolWithMode`、`ConcurrencySafeChecker`、`ToolWithPrepareArguments` |
| `providers.Provider` | `internal/ai/providers/interface.go` | LLM Provider 注册制（Name + Stream + StreamSimple） |
| `runtime.Application` | `internal/runtime/application.go` | Platform↔App 解耦点：`BuildTools()` + `BuildPrompt()` + `NewSessionExt()` |
| `operations.Operations` | `internal/operations/interface.go` | 本地/SSH 执行后端切换 |

## 关键设计

### Agent 双层循环 (`internal/agent/loop.go`)

外层处理 follow-up，内层处理 tool call。`runAgentLoop()` 是 `RunLoop` 和 `PromptStream` 的共享核心，通过 `consumeStreamFunc` 回调区分行为。

### Tool 执行流程

分区（`partitionToolCalls`：连续 safe call→并行批次，unsafe→串行批次）→ 每批执行：Validate → PrepareArguments → Before hooks → Execute → After hooks

### Provider 注册

`internal/ai/providers/register.go` 注册 builtins，`internal/app/app.go` 组装注入。新增 Provider：实现接口 → `registerProvider()` 注册 → 添加配置 env。

### 会话持久化

JSONL append-only（`internal/session/jsonl.go`）：message/compaction/checkpoint 三种 entry，通过 `BuildContext()` 重建历史，支持 `MoveTo(entryID)` 分支导航。

### Goal-Driven Loop

当 goal 非空：取消 maxTurns 限制、LLM 评估完成度、自动注入 follow-up reminder。

## 命令

```bash
go build -o pi-agent ./cmd/pi-agent        # 构建
go test ./...                               # 全量测试
go test ./internal/tools/ -v                # 单包测试
./pi-agent -mode chat                       # 交互式
./pi-agent -mode serve -listen :8080        # HTTP 服务
```

Provider 开发用 `mock`（`PI_GO_PROVIDER=mock`），不调真实 LLM。

## 环境变量

核心变量（完整列表见 `docs/CONTRIBUTING.md`）：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PI_GO_PROVIDER` | `mock` | `anthropic` / `openai` / `deepv` / `mock` |
| `PI_GO_ENABLE_BASH` | `false` | 启用 Bash 工具 |
| `PI_GO_DATA_DIR` | `./data` | 会话数据目录 |
