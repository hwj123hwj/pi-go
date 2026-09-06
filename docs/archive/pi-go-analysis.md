# pi-go 项目深度分析报告

> 分析日期：2026-05-25
> 项目路径：`/Users/weijian/Desktop/develop/test/pi/pi-go`
> 项目性质：Python Agent 框架 pi 的 Go 重实现 (reimplementation)
> 分析范围：65 个非测试源文件 + 30 个测试文件
> 基于 commit：`d3ed094`（共 36 个 commits）

---

## 整体评估

**成熟度：Beta / 功能完善期** — 4 层架构已稳定落地，核心模块完整，测试覆盖率约 43%。代码约 14,170 行 Go（非测试 9,909 行 + 测试 4,261 行）。

### 架构分层（4 层，从底向上）

```
Core（零领域知识）:
  agent/        — Agent 循环 + Tool 系统 + 事件
  ai/           — LLM 统一抽象层 + Provider 注册
  session/      — 会话持久化（树状 JSONL）
  compaction/   — 上下文压缩
  operations/   — 执行后端抽象（本地/SSH）
  prompt/       — 系统提示构建
  skill/        — 技能加载
  extensions/   — 扩展框架

Platform（领域无关运行时）:
  runtime/      — AgentSession 生命周期 + Application 接口

Application（可插拔领域应用）:
  agents/coding/ — 工具、提示、配置、命令、CLI

Entrypoints（组装与入口）:
  app/          — 依赖装配
  mode/         — 运行模式（interactive/print/serve）
  server/       — HTTP + WebSocket 服务
```

### 核心数据

| 指标 | 值 |
|------|-----|
| 非测试源文件 | 65 |
| 测试文件 | 30 |
| 非测试代码行 | 9,909 |
| 测试代码行 | 4,261 |
| 测试覆盖率（整体） | ~43% |
| 总 commits | 36 |
| 接口数 | 17 |
| 内置工具 | 7（read / write / edit / bash / grep / find / ls） |
| 注册的斜杠命令 | 11 |
| 零测试覆盖的包 | `app`, `mode`, `util` |
| 最低测试覆盖 | `runtime` (7%), `server` (20%) |

---

## 一、Core — Agent 层 (`internal/agent/`)

### 1. `agent.go` — Agent 结构 + Prompt/PromptStream 方法

**状态：已实现，稳定**

**功能：**
- `Agent` 结构体，包含状态机 (`StateIdle / StateRunning / StateWaiting / StateError`)
- 双消息队列：`steeringQueue`（主消息）和 `followUpQueue`（跟进消息）
- `Prompt()`：同步 API，调用 `RunLoop()` 返回完整回复
- `PromptStream()`：异步 API，返回 `<-chan AgentStreamEvent`，在 goroutine 中执行循环
- 订阅/事件分发 `Subscribe()` / `emit()` 模式
- 工具定义收集 `toolDefinitions()`、LLM 请求组装 `llmRequest()`
- 8 种流式事件类型：`text_delta / turn_end / tool_start / tool_update / tool_end / done / error / compacted`

**设计亮点：**
- `PromptStream` 的 goroutine 模式很好地实现了异步流式处理
- 双层队列设计（steering + followUp）比单一队列更灵活
- 事件系统使用 goroutine 安全的分发
- Goroutine 结束时一定会重置 `StateIdle`（已修复早期版本"永不清除 StateError"的 bug）

**已知问题：**
- `Subscribe()` 的 unsubscribe 函数使用 swap-delete（`append(a[:idx], a[idx+1:]...)`），但 `listen` 中的 `idx` 是 append 时的索引，如果之前有 unsubscribe 操作导致 `listen` 切片内部元素移位，索引会指向错误的元素
- `handleAssistantMessage`（288 行）仅做浅拷贝，实际是冗余方法

### 2. `loop.go` — Agent 循环核心逻辑

**状态：已实现，稳定**

**功能：**
- `RunLoop()`：外层调用入口，使用 consume 回调模式
- `runAgentLoop()`：**共享内核**，`RunLoop` 和 `PromptStream` 都调用它
- `processTurn()`：一轮处理的核心 — 追加 pending 消息、压缩检查、LLM 调用、工具执行
- `executeToolCalls()`：并行 vs 顺序执行判断
- `executeOneTool()`：完整的工具执行链路（找工具→发射 start→验证参数→PrepareArguments→before hooks→执行→after hooks→发射 end）
- `maybeCompact()`：上下文压缩触发 + 执行

**设计亮点：**
- `consumeStreamFunc` 回调模式让 `RunLoop` 和 `PromptStream` 共享同一循环内核
- 双层循环（tool call 内层 + follow-up 外层）职责清晰
- 压缩失败时优雅降级（使用完整历史）
- before/after hooks 集成在 `executeOneTool` 中

**已知问题：**
- 第 116 行：如果 `pending` 和 `history` 都为空，注入硬编码的 `"hello"` 消息——明显是调试残留
- 第 383 行：`marshalArgs()` 丢弃 `json.Marshal` 错误，如果传入不可序列化的值会 panic
- 第 83-85 行：达到 `maxTurns` 后静默返回 `lastAssistant`（可能为空），无错误提示
- 第 125、154、176 行：`session.AppendMessage()` 的错误被静默丢弃（`_ =`）
- `processTurn` 每次发送 LLM 请求前都构建完整历史，无增量缓存（这是架构上的选择，但仍有优化空间）
- `StreamWithRetry` 在 `ai/retry.go` 中定义了，但 loop 直接调用 `provider.Stream()`，未使用重试包装

### 3. `tool.go` — Tool 接口定义

**状态：已实现，简洁完整**

**功能：**
- `Tool` 接口：`Name() / Description() / Parameters() / Validate() / Execute()`
- `ToolResult` / `PartialResult` 结构体
- `ToolWithMode` 可选接口（并行/顺序执行控制）
- `ToolWithPromptInfo` 可选接口（工具提示信息注入）

**设计亮点：**
- `ExecutionMode` 枚举清晰，默认 `ExecutionModeParallel`
- 可选接口模式（`ToolWithMode` / `ToolWithPromptInfo`）避免了基类膨胀

### 4. `tool_lifecycle.go` — 工具生命周期

**状态：已实现，完整**

**功能：**
- `ToolCallContext` — 携带工具调用完整上下文
- `ToolExecutionResult` — 工具执行结果（**定义了但从未使用**，疑似死代码）
- `BeforeToolCallHook` / `AfterToolCallHook` — 函数类型定义
- `AfterHookError` — 保留 after hook 执行前原始结果的错误包装
- `LifecycleHooks` — 聚合 before/after hook 切片
- `ToolWithPrepareArguments` — 可选接口，在 before hooks 之前标准化参数

### 5. `event.go` — 事件类型

**状态：已定义，完整**

7 种事件类型覆盖 Agent 全生命周期：Agent 生命周期（start/end）、轮次生命周期（turn_start/turn_end）、工具执行（start/update/end）、压缩（compacted/compaction_failed）。

**注意：** `EventToolExecutionUpdate` 现在会在工具执行时通过 `onUpdate` 回调正确发出（早期版本定义了但不发出）。

### 6. `message.go` — MessageQueue

**状态：简单但完整**

线程安全的 FIFO 队列。`Drain()` 通过复制 + 清空实现原子性。

**注意：** `Drain()` 后如果消费者处理失败，消息丢失（无重试/回滚机制）。

### 7. `errors.go` — 错误类型

**状态：极简骨架**

仅定义 `ErrAgentBusy`。缺少工具未找到、无效参数、超时、provider 错误等业务错误类型。

---

## 二、Core — AI 层 (`internal/ai/`)

### 8. `types.go` — 核心类型定义

**状态：已实现，较完整**

**关键设计：**
- `Message` 接口 + `messageMarker()` 模式（类型安全联合类型）
- `UserMessage` 支持多 `ContentBlock`（text / image）
- `AssistantMessage` 支持 text / thinking / tool_calls / stop_reason
- `ToolResultMessage` 独立类型
- `StreamRequest` / `SimpleStreamRequest` 两级 API 设计

**注意：** `ToolResultMessage.Content` 是纯字符串，不支持结构化返回（如 JSON 对象）。

### 9. `stream.go` — 事件流

**状态：已实现，完整**

`EventStream` 封装了带缓冲的 channel + 结果 + 错误。7 种事件类型：`Start / TextStart / TextDelta / TextEnd / ToolCallStart / ToolCallDelta / ToolCallEnd / Done / Error`。

**注意：** `Result()` 在 `done == false` 时仍返回 `result` 和错误（早期版本已修复增加了 `done` 检查）。

### 10. `transform.go` — 消息转换

**状态：已实现，中等完整度**

`TransformMessages()` 支持图片降级、ToolCall ID 规范化、连续同角色消息合并。

**已知问题：**
- `ValidateMessageSequence` 定义了但从未在 agent loop 中调用
- 图片降级只是把 `image` 换成 `[Image]` 字符串，丢失了所有元信息

### 11. `cost.go` — 成本计算

**状态：简单工具函数，完整**

简单的 `input * cost + output * cost` 计算。不支持缓存命中成本（`CacheReadPerMega`, `CacheWritePerMega` 定义了但从未使用）。

### 12. `retry.go` — 重试逻辑

**状态：已实现，但未集成到循环中**

`RetryConfig`、`IsRetryableError()`、`StreamWithRetry()` 都已定义。

**已知问题：**
- `StreamWithRetry` **未在 Agent 循环中使用**——`loop.go` 直接调用 `provider.Stream()`
- `IsRetryableError()` 使用字符串匹配判断错误类型，很脆弱
- 重试只在 HTTP 请求层面发生，流中间错误无法中途重试

### 13. Provider 系统

#### 13a. `providers/interface.go` — Provider 接口 + 注册表

**状态：简洁完整**

`Provider` 接口只要求 `Stream()` 和 `StreamSimple()` 两个方法。`Registry` 是线程安全的 `map[string]Provider`。

#### 13b. `providers/anthropic.go` — Anthropic Provider（411 行）

**状态：已实现，较完整**

直接通过 HTTP 调用 Anthropic Messages API，不依赖 SDK。支持流式 SSE 事件解析。

**已知问题：**
- **图片降级：** `image` block 被降级为 `[Image]` 文本（第 368 行），禁用了 Anthropic 的原生图片能力（**高影响**）
- 硬编码 `anthropic-version: 2023-06-01`（第 97 行）
- 没有 `ToolChoice` 支持
- 没有 `anthropic-beta` 头（如 `output-128k-2025-02-19`）
- System prompt 长度未检查

#### 13c. `providers/openai.go` — OpenAI Provider（395 行）

**状态：已实现，较完整**

直接通过 HTTP 调用 OpenAI Chat Completions API。

**已知问题：**
- `baseURL` 构造可能在已含 `/v1/` 的 URL 上再追加 `/v1/`，导致双路径（第 121 行）（**中影响**）
- ToolCall 参数在流结束后一次性拼装，不是真正的增量
- ToolCall index 处理假设文本在先、工具在后，但 OpenAI 可能以任意顺序发送
- `Temperature: 0.7` 硬编码，无配置覆盖

#### 13d. `providers/deepv.go` — DeepV Provider

**状态：已实现，较完整**

通过 DeepVcode Server API 提供 LLM 服务，从 `~/.deepv/jwt-token.json` 读取认证 token。

**已知问题：**
- JWT token 不会在会话期间自动刷新（如果 token 在会话中过期）（**中影响**）

#### 13e. `providers/mock.go` — Mock Provider

**状态：测试工具，较完整**

基于文本匹配触发 mock tool call。

#### 13f. `providers/register.go` — 注册骨架

**状态：骨架，注册逻辑实际在 `app/app.go`**

---

## 三、Core — 工具实现 (`internal/tools/`)

7 个内置工具，均通过 `Operations` 抽象与执行后端解耦。

### 14. `read.go` — ReadTool

**状态：已实现，完整**

支持 `path`、`offset`、`limit` 参数，工作目录路径解析 + 路径安全检查，1-indexed 行号输出，大文件自动截断。

### 15. `write.go` — WriteTool

**状态：已实现，完整**

创建父目录、路径安全检查。**没有覆盖确认机制。**

### 16. `edit.go` — EditTool

**状态：已实现，完整（179 行）**

精确字符串替换、`ReplaceAll` 模式、新文件创建、唯一性检查、Diff 上下文输出。

### 17. `bash.go` — BashTool

**状态：已实现，完整**

`sh -c` 执行、超时控制、ANSI escape 清除、二进制输出检测、输出截断。

**注意：** 没有持久 Shell 会话（每次调用独立 `sh -c`），没有 `run_in_background` 支持。

### 18. `grep.go` — GrepTool

**状态：已实现，完整（332 行）**

纯 Go 实现的正则表达式文件搜索，支持 glob 文件名过滤、上下文行显示。不依赖系统 `grep` 命令。

**注意：** `globToRegex()` 不支持 `**` 等高级模式；`readLines()` 读取整个文件到内存。

### 19. `find.go` — FindTool

**状态：已实现，完整**

文件名 glob 搜索，递归/深度控制，类型过滤，跳过隐藏目录。**没有 `.gitignore` 感知。**

### 20. `ls.go` — LsTool

**状态：已实现，完整**

目录列表，按类型+名称排序，文件大小格式化，递归列表。

### 21. `truncate.go` — 输出截断

默认 30000 字符截断，保留前 80% + 后 20%。

### 支持文件

- `path.go` — 路径解析与安全检查
- `prompt_info.go` — 工具提示信息

---

## 四、Core — Session (`internal/session/`)

### 22. `interface.go` — SessionStorage 接口

`SessionStorage` 接口：`Init / Close / Append / GetPathToRoot / SetLeaf / GetLeaf / Fork`

### 23. `session/session.go` — Session 实现

**状态：已实现，完整**

基于 `SessionStorage` 的会话管理封装，支持 `AppendMessage`、`BuildContext`、`MoveTo`。

**注意：** `BuildContext` 每次调用全量读取无缓存。

### 24. `jsonl.go` — JSONL 存储

**状态：已实现，但有多处隐患**

基于 JSONL 文件的日志式存储，全部加载到内存的 `byID` map。

**已知问题：**
- 没有 `fsync()` 保证——写操作可能在内核缓冲区丢失（**中影响**）
- `load()` 在首个格式错误的行即失败，导致整个 session 文件不可读（**中影响**）
- `Fork()` 未实现，返回 `"fork not implemented in MVP"`
- `newID()` 使用 `time.Now().UnixNano()`，高并发下可能冲突
- 内存占用随会话增长线性增长

### 25. `sessionmgr/manager.go` — 会话管理器

**状态：已实现，较完整（232 行）**

文件系统会话存储管理，支持 Create / Open / Fork / List / Delete / Exists。`List` 的 `LastActive` 使用目录修改时间而非消息时间戳。

---

## 五、Core — Operations 抽象 (`internal/operations/`)

### 26. `interface.go` — 接口定义

两个核心接口：
- `BashOperations` — 命令执行（`Run`、`RunBackground`、`Walk`）
- `FileOperations` — 文件读写（`Read`、`Write`、`Edit`、`Grep`、`Find`、`List`）

### 27. `local.go` — 本地实现

**状态：已实现，完整**

### 28. `ssh.go` — SSH 实现

**状态：已实现，基础可用**

**已知问题：**
- 没有 SSH 连接池——每个操作创建新的 SSH 进程（**低影响**）
- `Walk` 超时硬编码为 30s
- `parseWalkLine` 在文件名含 `|` 时解析错误

---

## 六、Core — 其他

### 29. `compaction/` — 上下文压缩

**状态：已实现，中等完整度**

`Settings`（Enabled / ReserveTokens / KeepRecentTokens）、`ShouldCompact()`（基于 token 估算）、`Compact()`（调用 LLM 生成摘要）、`LLMSummarizer()`（使用 provider/model 调用 LLM）。

**注意：** 4 字符 / token 估算对中文不准确（中文 ~1.5-2 字符/token）；摘要调用消耗额外 token 但未跟踪。

### 30. `prompt/` — 系统提示构建

**状态：已实现，较完整**

8 个区域的提示构建、工具指南智能生成、运行时环境信息注入。

**注意：** git 分支信息在构建时计算一次，不动态更新。

### 31. `skill/` — 技能加载

**状态：已实现，较完整（453 行）**

遵循 agentskills.io 格式的 SKILL.md 文件加载。远超简单 MVP 需求的实现。

### 32. `extensions/` — 扩展框架

**状态：接口完整，但无实际扩展实现**

`Extension` 接口支持工具、命令、事件钩子。`Registry.Register()` 没有调用 `ext.Init()`。

---

## 七、Platform — Runtime (`internal/runtime/`)

### 33. `application.go` — Application 接口

`Application` 接口定义了 Platform ↔ Application 的解耦点：
```go
type Application interface {
    BuildTools(opts ToolBuildOptions) []agent.Tool
    BuildPrompt(opts PromptBuildOptions) string
}
```

### 34. `agent_session.go` — AgentSession（412 行）

**状态：已实现，完整**

`AgentSession` 是所有运行模式的统一抽象。功能涵盖：创建/加载会话、Agent 构建、工具列表装配、模型切换、profile 切换、Operations 后端选择、SSH 模式支持、上下文文件与技能加载。

**已知问题：**
- `Compact()` 是空实现（仅日志，不执行任何操作）——注释说 compaction 在 loop 中自动处理
- `contextWindowForModel` 硬编码有限模型映射，不可扩展
- `SwitchModel` 在未知 provider 时静默默认到 `"deepv"`（第 180 行）

### 35. `session_registry.go` — 会话注册表

**状态：已实现，完整**

`session_id → AgentSession` 运行时映射。`List` 也获取写锁（小规模下影响不大）。

---

## 八、Application — Coding Agent (`internal/agents/coding/`)

### 36. `application.go` — CodingApplication

实现 `runtime.Application` 接口，组装 coding-agent 的工具 + 提示。

### 37. `cli/interactive.go` — 交互式 CLI

**状态：已实现，基本可用**

`InteractiveMode` 是 coding-agent 的 CLI 对话循环。支持：
- 斜杠命令处理（通过 `slashcmd.Registry`）
- 会话切换（`SessionSwitchTo`）
- `ui.Presenter` 流式输出呈现

**已知问题：**
- `bufio.Scanner` 不支持行编辑、历史、补全（没有 readline）
- `5 * time.Minute` 硬编码超时
- 没有信号处理（Ctrl-C 不会优雅退出）

### 38. `commands/builtins.go` — 内置斜杠命令

**状态：已实现，11 个命令注册**

`help`, `compact`（stub）, `sessions`, `session`, `branch`（stub）, `new`, `switch`, `tools`, `model`, `profiles`, `profile`

**已知问题：** `/compact` 和 `/branch` 是存根，返回 `"not yet implemented"`。

### 39. `profile/profile.go` — Profile 系统

**状态：已实现**

支持 `coding` 和 `review` 两种 profile，不同 profile 使用不同的 system prompt。

### 40. 其他文件

- `prompt/builder.go` — coding-agent 专属提示构建
- `tools/tools.go` — coding-agent 工具装配
- `deepv/headers.go` — DeepV 自定义 HTTP 头
- `coding.go` — coding-agent 初始化

---

## 九、Entrypoints

### 41. `app/app.go` — 应用装配层

**状态：已实现，较完整**

创建 Provider Registry、Session Manager、Extension Registry。支持 `Application` 接口注入（通过 `Dependencies`）。

**已知问题：**
- 只有一个真实 provider 注册在 switch 中生效（`cfg.Provider` 控制）
- `app`、`mode`、`util` 三个包零测试覆盖

### 42. `mode/interactive.go` — 交互模式（旧版）

**状态：已实现，极简**（已被 `agents/coding/cli/interactive.go` 取代为主入口）

### 43. `mode/print.go` / `mode/serve.go`

两者都是极简代理——print 代理到 `session.Prompt()`，serve 代理到 `server.New()`。

### 44. `server/server.go` — HTTP 服务

**状态：已实现，较完整（372 行）**

REST API：`GET /health`、`POST /chat`、`POST /chat/stream`、`GET /sessions`、`POST /sessions`、`GET /sessions/{id}/messages`、`DELETE /sessions/{id}`、`GET /tools`

**已知问题：**
- 所有端点都**没有身份认证**（**高影响**）
- SSE 没有心跳 ping，长连接可能被代理断开
- `deleteSession` 在 sessionmgr 已删除但 registry 清理失败时留下不一致状态
- `listModels` 返回硬编码的模型目录，而非真实已注册的 provider

### 45. `server/websocket.go` — WebSocket 支持

**状态：已实现**

---

## 十、全量已知 Bug / 问题清单

| # | 位置 | 描述 | 严重度 |
|---|------|------|--------|
| 1 | `agent/agent.go` Subscribe | Unsubscribe 使用 swap-delete，索引在并发场景下可能指向错误元素 | Medium |
| 2 | `agent/loop.go:116` | 空历史注入硬编码 "hello" 消息 | Low |
| 3 | `agent/loop.go:383` | `marshalArgs` 静默忽略 json.Marshal 错误 | Low |
| 4 | `agent/loop.go` | Session AppendMessage 错误被静默丢弃（`_ =`） | Medium |
| 5 | `agent/tool_lifecycle.go` | `ToolExecutionResult` 定义了但从未使用（死代码） | Low |
| 6 | `runtime/agent_session.go` | `Compact()` 是空实现 | Medium |
| 7 | `runtime/agent_session.go` | `contextWindowForModel` 硬编码，不可扩展 | Medium |
| 8 | `runtime/agent_session.go:180` | `SwitchModel` 对未知 provider 静默默认到 "deepv" | Medium |
| 9 | `ai/providers/anthropic.go:368` | 图片 block 降级为 "[Image]" 文本，不发送到 API | **High** |
| 10 | `ai/providers/anthropic.go:97` | 硬编码 `anthropic-version: 2023-06-01` | Low |
| 11 | `ai/providers/openai.go:121` | baseURL 构造可能产生 `/v1/v1/` 双路径 | Medium |
| 12 | `ai/providers/openai.go` | ToolCall 参数累积后从未验证为合法 JSON | Low |
| 13 | `ai/providers/deepv.go` | JWT token 在会话期间过期时不会刷新 | Medium |
| 14 | `ai/retry.go` | `StreamWithRetry` 定义了但未在 agent loop 中使用 | **High** |
| 15 | `ai/retry.go` | `IsRetryableError` 使用脆弱的字符串匹配 | Low |
| 16 | `ai/transform.go` | `ValidateMessageSequence` 定义了但从未调用 | Low |
| 17 | `session/jsonl.go` | 写操作没有 fsync——崩溃时数据丢失 | Medium |
| 18 | `session/jsonl.go` | `load()` 在第一个格式错误的行即失败，整个 session 不可读 | Medium |
| 19 | `session/jsonl.go` | `Fork()` 未实现 ("not implemented in MVP") | Low |
| 20 | `session/jsonl.go` | `newID` 使用 UnixNano——高并发下可能冲突 | Low |
| 21 | `operations/ssh.go` | 没有 SSH 连接池——每个操作创建新进程 | Low |
| 22 | `operations/ssh.go` | `Walk` 超时硬编码 30s；`parseWalkLine` 在文件名含 `\|` 时中断 | Low |
| 23 | `server/server.go` | **所有端点没有身份认证** | **High** |
| 24 | `server/server.go` | `deleteSession` 可能留下不一致状态 | Medium |
| 25 | `server/server.go` | SSE 无心跳，无客户端断开检测 | Medium |
| 26 | `server/server.go` | `listModels` 返回硬编码目录，非真实注册的 provider | Low |
| 27 | `app/app.go` | 一次只有一个真实 provider 能注册（switch 语句） | Medium |
| 28 | `commands/builtins.go` | `/compact` 和 `/branch` 是存根 | Low |
| 29 | `cli/interactive.go` | 无 readline、无信号处理、5 分钟硬编码超时 | Medium |

### Bug 分类统计

| 严重度 | 数量 | 占比 |
|--------|------|------|
| **High** | 3 | 10.3% |
| **Medium** | 13 | 44.8% |
| **Low** | 13 | 44.8% |

### 自旧报告（05-21）以来的变化

旧报告追踪了 24 个 bug，自 2026-05-21 至 2026-05-25 期间的状态变化：

- **已修复（5 个）**: #1 (PromptStream 状态永不恢复) ✅, #5 (Result() 未关闭时返回数据) ✅, #8 (Anthropic 重复 EventDone) ✅, #15 (EventToolExecutionUpdate 从不发出) ✅, #16 (ToolNames 逻辑重复) ✅
- **部分修复（1 个）**: #18 (contextWindow 扩充了但依然硬编码)
- **架构选择（1 个）**: #6 (每次全量历史重建——设计有意如此)
- **仍存在（17 个）**: 见上表

---

## 十一、架构评分

| 维度 | 评分 | 说明 |
|------|------|------|
| **架构清晰度** | ⭐⭐⭐⭐⭐ | 4 层分层清晰，职责分明，Application 接口成功解耦 |
| **代码质量** | ⭐⭐⭐⭐ | 大部分代码质量可接受，Go 风格良好，少量明显 bug |
| **完整度** | ⭐⭐⭐ | 核心路径坚固（Agent 循环 + Provider + 工具 + 会话），边缘功能存根 |
| **错误处理** | ⭐⭐ | 基本覆盖，但多处静默失败、空实现、未使用的函数 |
| **测试覆盖** | ⭐⭐⭐ | 30 个测试文件，43% 整体覆盖，但 app/mode/util 三个包为零 |
| **可扩展性** | ⭐⭐⭐⭐ | Application 接口 + Operations 抽象 + LifecycleHooks 设计合理 |
| **生产就绪度** | ⭐⭐ | 缺认证、审计、配置持久化、信号处理、会话 fork |

### 相比旧报告（05-21）的提升

| 维度 | 旧评分 | 新评分 | 变化原因 |
|------|--------|--------|---------|
| 架构清晰度 | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 4 层架构明确落地，Application 接口证明可扩展 |
| 代码质量 | ⭐⭐⭐ | ⭐⭐⭐⭐ | 新增测试、修复 5 个 bug、Operations 抽象质量高 |
| 完整度 | ⭐⭐ | ⭐⭐⭐ | CLI 控制面、profile 系统、WS 支持等新功能 |
| 测试覆盖 | ⭐ | ⭐⭐⭐ | 从 0 到 30 个测试文件、4,261 行测试代码 |
| 可扩展性 | ⭐⭐⭐ | ⭐⭐⭐⭐ | Application 接口 + LifecycleHooks + Operations 抽象 |

---

## 十二、总结

**pi-go 已从一个可运行的原型演进为一个有良好架构设计的 Agent 框架 Beta 版。** 4 层架构（Core → Platform → Application → Entrypoints）已稳定落地，`Application` 接口成功解耦了运行时与 coding-agent。测试覆盖从 0 增长到 43%，代码量增长到约 14,000 行。

**主要优点：**
- 清晰的 4 层架构，接口设计合理
- `Application` 接口成功解耦 Platform ↔ Application
- Operations 抽象（Local + SSH）使工具与执行后端解耦
- 测试覆盖从 0 到 43%（30 个测试文件）
- Agent 双层循环设计清晰，事件系统支持流式消费
- 完整的 7 件套内置工具
- CLI 控制面（profile 切换、session 管理、model 切换）

**主要不足：**
- 3 个 High 级别 bug（Anthropic 图片降级、无 retry 集成、无认证）
- `Compact()` 和 `Fork()` 等关键功能仍为存根
- 3 个包（app/mode/util）零测试覆盖
- 会话存储缺少 fsync 保证和 Fork 实现
- 命令行模式缺少 readline 支持和信号处理
- 无生产级别的监控和审计能力

**总体定位：** pi-go 已超越"学习项目"阶段，具备了作为个人生产力工具的可用性。对于需要 Agent 框架的 Go 项目，它是一个值得评估的轻量级选择。要用于生产环境或团队服务，还需要解决安全认证、会话 Fork、配置持久化和关键路径重试等核心问题。
