# pi-go 项目代码审查与改进建议

> 基于对整个项目（包括 `internal/agent`、`internal/ai`、`internal/tools`、`internal/runtime`、`internal/app` 等核心模块以及 CLI/TUI 集成和飞书桥接）的全面阅读。

---

## 目录

1. [代码质量与健壮性](#1-代码质量与健壮性)
2. [架构与设计](#2-架构与设计)
3. [工程实践与可维护性](#3-工程实践与可维护性)
4. [功能与用户体验](#4-功能与用户体验)
5. [安全与运维](#5-安全与运维)
6. [总结与优先级](#6-总结与优先级)

---

## 1. 代码质量与健壮性

### 1.1 `Goal()` 读操作存在并发安全问题

**文件**: `internal/agent/agent.go` (第 122 行)

**问题**: `Agent` 结构体使用了 `sync.RWMutex`，`SetGoal()` 和 `ClearGoal()` 都正确加写锁，但 `Goal()` 方法虽然当前已加 `RLock`——**实际上已在之前的一次提交中已修复**。这是好的。但需确认 `Goal()` 的调用者是否在所有读路径上都通过 `a.mu.RLock()` 保护。

**建议**: ✅ 已修复。保持使用 `sync.RWMutex` 模式，确保所有新增的读操作也通过 `RLock` 保护。

### 1.2 `runAgentLoop` 中 goal-driven 模式的行为可预测性

**文件**: `internal/agent/loop.go` (第 73-142 行)

**问题**: Goal-driven 模式的核心逻辑在 `runAgentLoop` 中：当 `actionDone` 且 `goal != "" && !result.goalDone` 时，注入一条固定的 follow-up 消息 `"Reminder: your current goal is \"...\". Continue working on it."`。这个机制的缺陷在于：
1. **无退避策略**：如果 LLM 持续返回 text-only response（无 tool call、无 goal completion 信号），循环可能会无限注入 follow-up 消息，浪费 token。
2. **固定消息内容**：每次注入的提示词完全一样，LLM 可能产生重复输出，形成 "ping-pong" 循环。

**建议**：
- 增加连续 follow-up 轮次计数器，超过 3 轮后自动退出（或 alert 用户）。
- 考虑改善 follow-up 消息的多样性，例如添加类似 "你已经在第 X 轮回复中，当前目标仍然是 Y，请评估是否已完成或继续需要更多工作" 的上下文感知提示。
- 将 `maxFollowUpTurns` 参数化到配置中。

### 1.3 Goal 评估器没有超时控制

**文件**: `internal/agent/goal_evaluator.go` (第 54-105 行)

**问题**: `evaluateGoalCompletion` 函数使用了传入的 `ctx`，但调用方 `processTurn` 并没有为评估器设置独立的超时。如果 LLM 评估器调用卡住，它会阻塞整个 `processTurn`，进而阻塞整个 agent 循环。当前循环使用用户输入的 `ctx`（可能是 5 分钟或 30 分钟超时），评估器可能占用很长时间。

**建议**：
- 为 goal 评估调用创建一个带独立超时的子 context（例如 10 秒）。如果评估超时，回退到关键词匹配。

```go
evalCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()
done, reason := evaluateGoalCompletion(evalCtx, ...)
```

- 将超时值提取到常量或配置中。

### 1.4 `processTurn` 中 `pending` 消息的初始值容易混淆

**文件**: `internal/agent/loop.go` (第 148-151 行)

```go
if len(pending) == 0 && len(history) == 0 {
    pending = []ai.Message{ai.NewTextUserMessage("hello")}
}
```

**问题**: 当 `history` 和 `pending` 都为空时，默认注入 `"hello"`。这是一个比较隐晦的 fallback，可能导致在特定场景下（如 session 恢复后 history 为空）发送无意义的 "hello" 消息。这个分支仅在 `runAgentLoop` 第一步且 session 为空时触发。

**建议**：将此 fallback 改为返回错误（`fmt.Errorf("no messages to process")`），让调用方明确需要提供初始消息。或者在文档中明确标注此行为。

### 1.5 大量使用 `_` 忽略 session 持久化错误

**问题**: `internal/agent/loop.go` 中多处使用 `_ = a.session.AppendMessage(ctx, msg)` 和 `_ = a.session.AppendMessage(ctx, message)` 忽略 session 持久化错误。同样在 `processTurn` 中工具调用结果的持久化也被忽略。

**建议**：
- 至少记录日志（`slog.Warn`），以便运维时发现持久化问题。
- 考虑是否在某些场景下（如 compaction entry 写入失败）应该向上返回错误，而不是静默忽略。

### 1.6 `goalLog` 调试日志的竞态与资源泄漏

**文件**: `internal/agent/goal_evaluator.go` (第 24-44 行)

**问题**: `goalLog` 函数使用了包级别的文件句柄 `goalLogFile`，但没有任何关闭机制。在长时间运行的 server 模式中，日志文件会持续增长。同时，函数的 `sync.Mutex` 保护和 `os.Stderr` 的并发写入是安全的，但包级变量在多 session 共享时可能产生不可预期的交错输出。

**建议**：
- 考虑使用 `slog.Debug` 替代或补充文件日志。
- 如果保留文件日志，提供 `CloseGoalLog()` 函数用于优雅关闭。
- 或将日志路径移到配置中，而不是硬编码为 `/tmp/pi-goal-debug.log`。

---

## 2. 架构与设计

### 2.1 Provider 注册表已是好的设计，但仍有改进空间

**文件**: `internal/app/app.go` (第 205-241 行)

**问题**: `registerProviders` 使用 `switch-case` 硬编码各 provider 的初始化逻辑。虽然 `internal/ai/providers/register.go` 中已有 `Registry` 结构体（提供 `Register`/`Get` 方法），但 provider 的创建并没有使用注册模式——每个 provider 的初始化参数（API key、base URL 等）都从 `config.Config` 直接读取。

**建议**：采用 **工厂注册模式**，让每个 provider 自行注册工厂函数：

```go
// providers/register.go
type Factory func(cfg config.Config) (Provider, error)

var factories = map[string]Factory{}

func RegisterProvider(name string, factory Factory) {
    factories[name] = factory
}

func CreateProvider(name string, cfg config.Config) (Provider, error) {
    factory, ok := factories[name]
    if !ok {
        return nil, fmt.Errorf("unknown provider: %s", name)
    }
    return factory(cfg)
}
```

然后在各 provider 的 `init()` 中注册：
```go
// providers/anthropic.go
func init() {
    RegisterProvider("anthropic", func(cfg config.Config) (Provider, error) {
        if cfg.AnthropicAPIKey == "" {
            return nil, fmt.Errorf("ANTHROPIC_API_KEY is empty")
        }
        return NewAnthropicProvider(cfg.AnthropicAPIKey, cfg.AnthropicBaseURL), nil
    })
}
```

### 2.2 `StreamSimple` 方法的存在意义不明确

**文件**: `internal/ai/providers/interface.go` (第 13 行) + 多个 provider 实现

**问题**: `Provider` 接口同时定义了 `Stream` 和 `StreamSimple` 方法。`StreamSimple` 只是将 `SimpleStreamRequest` 转换为 `StreamRequest` 后调用 `Stream`。但 `SimpleStreamRequest` 与 `StreamRequest` 的唯一区别是缺少 `ToolChoice` 字段。这种冗余可以通过内部转换消除。

**建议**：合并两个方法，或只保留 `Stream` 并让调用方自行构造 `StreamRequest`。`StreamSimple` 可以保留但不暴露在接口中（作为各 provider 的内部辅助函数）。

### 2.3 `SessionExt` 的 `rebuild` 回调模式较为脆弱

**文件**: `internal/runtime/agent_session.go` (第 77-85 行) + `internal/agents/coding/session_ext.go` (第 33-35 行)

**问题**: `SetRebuild` 使用接口断言注入回调，回调类型为 `func() error`。如果回调未设置（nil），`SetGoal` 等操作不会重建 agent，但不会返回错误——用户调用 `/goal xxx` 后可能以为 goal 已生效，但实际上 agent 并未重建。

**建议**：
- `SetGoal` 在 `rebuild` 为 nil 时返回错误（或至少记录 warn 日志——当前已有）。
- 考虑将 `rebuild` 作为构造函数参数传入，避免运行时断言和延迟注入。

### 2.4 `agent/agent.go` 中的 `EventAgentEnd` 携带 `Messages` 而 `PromptStream` 版本不携带

**文件**: `internal/agent/agent.go` (第 155 行 vs 第 260 行)

**问题**: `Prompt()` 方法在 `EventAgentEnd` 中携带了 `[]ai.Message{msg}`（第 162 行），而 `PromptStream()` 中触发的 `EventAgentEnd` 不带 `Messages`（第 260 行 `a.emit(ctx, EventAgentEnd{})`）。这个不一致可能导致事件订阅者（如飞书桥接或 UI）无法完整追踪消息流。

**建议**：统一行为，让两种模式的 `EventAgentEnd` 都携带相同的消息元数据。

### 2.5 `Serve` 模式下没有 goal-driven loop 支持

**文件**: `internal/server/server.go`

**问题**: Server 模式的 `POST /chat` 和 `POST /chat/stream` 端点都使用了 `5*time.Minute` 的超时（第 114 行和第 160 行）。但 goal-driven 会话可能需要更长时间（CLI 模式在 goal 活跃时使用 30 分钟超时）。Server 模式无法感知 goal 状态来动态调整超时。

**建议**：
- 在 Server 的 chat handler 中检查 session 是否有活跃 goal，动态调整超时。
- 或者在配置中统一 `GoalTimeout` 参数，供 CLI 和 Server 共同使用。

---

## 3. 工程实践与可维护性

### 3.1 测试覆盖缺口

#### 3.1.1 Goal-driven 循环的集成测试

**问题**: `internal/agent/goal_evaluator_test.go` 测试了 `evaluateGoalCompletion` 的独立单元，但 `loop.go` 中的 `runAgentLoop` 整体 goal-driven 流程（actionDone → 注入 follow-up → 继续循环 → 评估完成 → 停止）没有被测试。关键的代码路径包括：
- `processTurn` 中的 `evaluateGoalCompletion` 调用
- `runAgentLoop` 中的 follow-up 注入
- `EventGoalCompleted` 的触发

**建议**：使用 `MockProvider` 模拟 LLM 返回不同场景：
- 返回 text-only（触发 follow-up 注入）
- 返回 tool_use（继续内层循环）
- 返回停下 + 评估器判完成（触发 `EventGoalCompleted`）
- 返回停下 + 评估器判未完成（注入 follow-up）

#### 3.1.2 DeepV Provider 的多工具调用合并逻辑

**问题**: `internal/ai/providers/deepv.go` 中的多工具调用处理（`mergeParallelFunctionResults` 的逻辑在 `convertRequest` 中对 `ToolResultMessage` 的处理：第 328-363 行）没有单元测试。这个逻辑涉及多个 `FunctionResponse` 合并到同一个 user message 中，是容易出错的地方。

**建议**：添加单元测试覆盖：
- 单个 tool result 的转换
- 多个 tool results 的合并
- 空 content 的处理
- 混合 text + tool results 的场景

#### 3.1.3 `edit` 工具的多编辑模式

**问题**: `internal/tools/edit.go` 中的 `applyEdits` 函数（第 285-323 行）实现了批量替换和重叠检测，但 `edit_test.go` 对此功能的测试覆盖不完整。

**建议**：补充测试用例覆盖：
- 重叠编辑的拒绝
- 不重叠编辑的成功应用
- 空 edits 的处理
- 编辑后文件位置正确的验证

### 3.2 文档与源码不同步

#### 3.2.1 高层文档未反映 Goal-Driven Loop 的完成状态

**文件**: `docs/PROJECT_CONTEXT.md`, `docs/PRODUCT_ROADMAP.md`

**问题**: 这些高层文档中 Goal-Driven Loop 仍显示为"规划中"或"进行中"，但实际上该功能已在 `feat/cli-tui-enhancement` 分支完整实现。

**建议**：PR 合入时同步更新，标记 goal-driven loop 为 ✅ 已完成。

#### 3.2.2 Server 模式的 API 文档缺失

**问题**: `internal/server/server.go` 已经实现了丰富的 REST API（包括 `/chat`, `/chat/stream`, `/sessions`, `/sessions/{id}/messages`, `/sessions/{id}/compact`, `/sessions/{id}/model`, `/tools`, `/models` 等端点），但没有对应的 API 文档（OpenAPI/Swagger 或 Markdown）。

**建议**：生成 `docs/api.md` 或集成 OpenAPI 注释，描述各端点的请求/响应格式。

### 3.3 `.gitignore` 不够全面

**文件**: `.gitignore`

**问题**: 当前只忽略了 `pi-agent` 二进制文件，但 `pi-feishu-bridge` 和其他可能的二进制产物未被忽略。此外，Go 编译产生的临时文件（如 `go-build` 缓存）也没有包含。

**建议**：
```
# Binary artifacts
pi-agent
pi-feishu-bridge
pi-*

# Go build cache
*.out
*.test
*.test.exe

# IDE
.idea/
.vscode/
*.swp
*.swo
```

### 3.4 没有 CI/CD 配置

**问题**: 项目根目录有 `.github/workflows/deploy.yml`，但缺少基础的 PR/推送 CI 工作流来自动运行测试和 lint。

**建议**：创建 `.github/workflows/ci.yml`：

```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go vet ./...
      - run: go test ./... -race -count=1 -timeout=60s
```

---

## 4. 功能与用户体验

### 4.1 Goal Completion 在 CLI 中没有用户反馈

**文件**: `internal/agents/coding/cli/interactive.go`

**问题**: 当 goal 被判定为完成时，`runAgentLoop` 会触发 `EventGoalCompleted` 事件，`loop.go` 中也会打印 `slog.Info`。但在 CLI 终端中，用户只会看到 agent 停止输出，没有醒目的 "目标已完成" 提示。用户可能困惑是 agent 卡住了还是完成了。

**建议**：在 `InteractiveMode.runPrompt` 或 `eventLoop` 中监听 `EventGoalCompleted` 事件，打印一条明显的分隔消息：

```go
// 在 Presenter 或 InteractiveMode 中：
case agent.EventGoalCompleted:
    fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
    fmt.Printf("✅ 目标已完成：%s\n", e.Goal)
    fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
```

### 4.2 Presenter 未展示 `EventGoalCompleted` 事件

**文件**: `internal/ui/presenter.go`

**问题**: `Presenter` 的 `convertEvent` 函数处理了 `StreamEventTextDelta`、`StreamEventToolStart` 等多种事件，但 `EventGoalCompleted` 事件在 `PromptStream` 模式中未被映射到 `AgentStreamEvent`。PromptyStream 中的事件订阅（`agent.go` 第 173-195 行）没有处理 `EventGoalCompleted`。

**建议**：在 `agent.go` 的 `PromptStream` 事件转发中添加 `EventGoalCompleted` 的处理：

```go
case EventGoalCompleted:
    ev = AgentStreamEvent{Type: StreamEventGoalCompleted, Goal: e.Goal}
```

同时在 `AgentStreamEvent` 和 `StreamEventType` 中添加对应的类型和字段。

### 4.3 `/help` 命令没有汇总所有注册命令

**文件**: `internal/agents/coding/commands/builtins.go` (第 342-393 行)

**问题**: `formatHelp` 目前通过 `registry.Names()` 获取所有命令名称，然后按类别硬编码分类（sessionCmds、infoCmds、actionCmds）。当一个新命令通过扩展注册时，如果它的名称不在分类 switch-case 中，会被归入 `actionCmds`。这虽然能工作，但分类不准确。

**建议**：
- 为 `slashcmd.Command` 增加 `Category string` 字段（如 `"session"`, `"info"`, `"action"`）。
- `formatHelp` 按分类动态分组显示，消除硬编码的分类逻辑。

### 4.4 Server 模式不支持 goal 设置

**文件**: `internal/server/server.go`

**问题**: Server 的 API 端点没有暴露 goal 的设置/清除功能。REST API 只提供了 session 管理、模型切换、上下文压缩等，但没有 goal 相关接口。

**建议**：添加 `POST /sessions/{id}/goal` 端点，允许通过 API 设置和清除 goal：
```go
// 请求体
{ "goal": "optimize error handling" }   // 设置
{ "goal": "" }                          // 清除

// 或使用 DELETE /sessions/{id}/goal
```

### 4.5 飞书桥接 `/goal` 命令缺失

**文件**: `internal/feishu/handler.go` (第 73-87 行)

**问题**: 飞书 handler 的 `handleSlashCommand` 只支持 `/new`, `/compact`, `/status`, `/help`，不支持 `/goal`。用户无法通过飞书设置目标。

**建议**：添加 `/goal <text>` 命令支持，通过 pi-agent 的 HTTP API 设置 session goal。简化方案是发送原始文本到 agent 的 chat 端点，由 agent 自然处理。

---

## 5. 安全与运维

### 5.1 敏感信息启动校验不足

**文件**: `internal/app/app.go` (第 205-241 行) + 各入口文件

**问题**: `registerProviders` 在检测到配置的 provider 缺少 API key 时会返回错误，这是好的。但其他敏感信息（如 `FEISHU_APP_ID`、`FEISHU_APP_SECRET`）没有在启动时校验。

**建议**：在 `cmd/pi-feishu-bridge/main.go` 和 `cmd/pi-agent/main.go` 中添加启动时校验，确保所有必需的配置项已设置：

```go
func validateRequiredConfig(cfg config.Config) error {
    if cfg.Provider != "mock" {
        if cfg.AnthropicAPIKey == "" && cfg.OpenAIAPIKey == "" && !cfg.DeepVEnabled {
            return fmt.Errorf("no API key configured for provider %q", cfg.Provider)
        }
    }
    // 其他校验...
}
```

### 5.2 文件路径安全校验可以更严格

**文件**: `internal/tools/path.go` (IsPathSafe)

**问题**: 工具层有 `IsPathSafe` 函数来防止路径逃逸，这是一个好的安全设计。但该函数仅在 edit 和 grep 工具中被调用（通过 `doExecute` 和 `Execute` 中的检查），需要确认所有文件操作工具（read、write、find、ls）是否都做了同样的检查。

**建议**：审计所有工具的 `Execute` 方法，确保 `IsPathSafe` 的调用覆盖完整。

### 5.3 缺乏 Docker 部署支持

**问题**: 项目有 `deploy/pi-feishu-bridge.service` systemd 服务文件，但没有 Dockerfile 或 docker-compose.yml。对于生产部署，Docker 容器化是标准做法。

**建议**：提供多阶段构建的 Dockerfile：

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o pi-agent ./cmd/pi-agent
RUN CGO_ENABLED=0 go build -o pi-feishu-bridge ./cmd/pi-feishu-bridge

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/pi-agent .
COPY --from=builder /app/pi-feishu-bridge .
ENTRYPOINT ["./pi-agent"]
```

同时补充 `docker-compose.yml` 用于本地开发和测试。

### 5.4 `options` 模式无法通过配置关闭

**问题**: `Config` 中没有对应的字段来控制很多内部行为，例如：
- `GoalMaxTurns`（goal-driven 模式的最大轮次限制）
- `GoalEvalTimeout`（goal 评估器超时）
- `EnableGoalEval`（是否启用 LLM 评估器，或只使用关键词回退）
- `CompactionReserveTokens` 和 `KeepRecentTokens`（压缩参数）

这些参数当前在代码中硬编码。

**建议**：在 `Config` 中添加高级配置字段，提供默认值，允许通过环境变量覆盖。例如：

```go
type Config struct {
    // ... 现有字段 ...
    
    // Goal-driven loop
    GoalMaxTurns    int  `env:"PI_GO_GOAL_MAX_TURNS"`     // 0 = 无限
    GoalEvalTimeout int  `env:"PI_GO_GOAL_EVAL_TIMEOUT"`  // 秒
}
```

---

## 6. 总结与优先级

### 6.1 总体评价

pi-go 项目整体代码质量较高，展现出良好的 Go 工程实践：

- **模块化清晰**：`agent` → `runtime` → `app` 三层分离，依赖注入方向正确。
- **接口设计合理**：`Tool`、`Provider`、`Application`、`SessionExt` 等接口职责明确。
- **测试覆盖较好**：核心模块（agent loop、tools、compaction、session）都有单元测试。
- **并发安全**：`sync.RWMutex` 正确使用，`FileMutationQueue` 的 per-file 串行化设计精巧。
- **扩展性**：Extension 系统、Skill 系统、Slash Command 注册表都体现了良好的扩展性。

### 6.2 优先级矩阵

| 优先级 | 领域 | 建议 | 影响 |
|--------|------|------|------|
| 🔴 Critical | 测试 | Goal-driven 集成测试（1.1） | 防止回归，确保新功能可靠性 |
| 🔴 Critical | 健壮性 | Goal 评估器超时控制（1.3） | 防止 agent 循环卡死 |
| 🟠 High | 架构 | Provider 工厂注册模式（2.1） | 提升可扩展性 |
| 🟠 High | 用户体验 | Goal 完成反馈（4.1, 4.2） | 提升用户感知 |
| 🟠 High | 健壮性 | Follow-up 退避策略（1.2） | 防止无限循环 |
| 🟡 Medium | 配置 | 硬编码参数可配置化（5.4） | 提升部署灵活性 |
| 🟡 Medium | 文档 | 文档同步（3.2） | 降低新人上手成本 |
| 🟡 Medium | CI | GitHub Actions CI（3.4） | 保证代码质量 |
| 🟢 Low | 部署 | Dockerfile（5.3） | 简化部署 |
| 🟢 Low | 工程 | `.gitignore` 完善（3.3） | 代码库整洁 |
| 🟢 Low | API | Server API 文档（3.2.2） | 便于集成 |

### 6.3 建议优先处理

1. **Goal-driven 集成测试**：确保整个 goal 循环路径（从 `/goal` 设置到完成停止）被自动化测试覆盖。
2. **Goal 评估器超时**：增加 10 秒独立超时，防止评估调用阻塞 agent 循环。
3. **Follow-up 退避策略**：防止在 LLM 不配合时无限循环。
4. **Present `EventGoalCompleted`**：让 CLI 用户明确知道目标已完成。
5. **Provider 工厂注册模式**：长期来看，这是使 pi-go 成为"可扩展 Agent 底座"的关键架构改进。

---

*审查日期：2025-07*
*审查范围：pi-go 全项目代码（含 `feat/cli-tui-enhancement` 分支的 Goal-Driven Loop 实现）*
