# pi-go 核心层架构偏离分析报告

> 日期：2026-06-07
> 问题：`internal/agent/` 核心层被应用层逻辑严重污染，分层名存实亡

## 1. 背景：原始 pi 的分层设计

原始 pi（TypeScript）的三层架构清晰：

```
coding-agent    ← 知道"编码"：bash、文件、技能、工具策略
agent-core      ← 不知道"编码"：循环、Tool 抽象、事件、会话
ai              ← 纯基础设施：LLM Provider 抽象
```

**核心原则：agent-core 是领域无关的。** 它不知道什么是 bash 命令、什么是技能、什么是文件路径。

## 2. pi-go 的现状：核心层被污染

### 2.1 文件体量对比

```
internal/agent/ (核心层)        行数
├── tool_policy.go               2,202   ← 严重污染
├── loop.go                        800   ← 严重污染
├── agent.go                       588   ← 中度污染
├── tool.go                        121   ← 中度污染
├── goal_evaluator.go              162   ← 轻微污染
├── tool_lifecycle.go               73   ← 干净
├── external_tool.go                90   ← 干净
├── event.go                       112   ← 基本干净
├── message.go                      43   ← 干净
└── errors.go                        6   ← 干净

internal/agents/coding/ (应用层)  行数
├── tools/tools.go                 113
├── tools/file_mutation.go         104
├── application.go                  90
├── session_ext.go                  78
└── coding.go                       20
```

**核心层 4,397 行 vs 应用层 405 行。** 核心层的体量是应用层的 10 倍以上，说明大量应用逻辑下沉到了核心。

### 2.2 tool_policy.go：核心污染重灾区（2,202 行）

这个文件名义上是"工具策略执行"，实际上内置了一整套领域特定的解析器和处理器：

| 污染内容 | 大约行数 | 本应归属 |
|---|---|---|
| bash 命令解析（`shellFields`、`parseBashCommandSpecs`、`splitShellSegments`） | ~500 | coding-agent 应用层 |
| bash 命令 spec 匹配（`bashCommandSpecMatches`、`bashCommandAllowedBySpecs`） | ~200 | coding-agent 应用层 |
| 技能路径策略（`resolveSkillPath`、`extractCommandSkillPathCandidates`） | ~400 | skill 子系统 |
| 技能结果处理（`skillResultChange`、`skillResultDiff`、`skillOperationSummary`） | ~200 | skill 子系统 |
| shell 重定向解析（`isRedirectionOperator`） | ~80 | coding-agent 应用层 |
| 文件路径推断（`isLikelyCommandPath`、`isPathArgumentCommand`） | ~100 | coding-agent 应用层 |
| 路径模式匹配（`pathPatternMatches`、`pathSuffixPatternMatches`） | ~100 | 通用工具层 |

**总计约 1,600 行是领域特定逻辑，占文件的 73%。** 真正属于核心层的通用策略框架只有 ~600 行。

### 2.3 loop.go：fork 逻辑硬编码（800 行）

loop.go 包含了大量 skill fork 的硬编码逻辑：

| 函数 | 职责 | 问题 |
|---|---|---|
| `runActiveForkChild()` | 运行 skill 的 fork 子 Agent | 应通过接口注入，不应硬编码在循环中 |
| `forkSkillRequestHistory()` | 裁剪 skill 隔离历史 | 应用层关注点 |
| `isSkillInvocationMessage()` | 检测 `<skill name="...">` 标签 | 应用层关注点 |
| `previewSkillResult()` | 截断 skill 结果展示 | 应用层关注点 |
| `hasActionIntent()` | 中英文意图短语检测 | 应用层关注点 |

`runAgentLoop()` 函数（第 76-165 行）的主流程中充斥着 fork 判断：

```go
// loop.go:109 - 在核心循环中硬编码 fork 分支
if !a.isForkChild && a.activeForkSkillPolicy() && a.currentForkSession() != nil {
    forkAssistant, err := runActiveForkChild(ctx, a, provider, consume, pending)
    ...
}

// loop.go:181-196 - 核心循环中多处 fork 特判
if a.activeForkSkillPolicy() {
    a.appendForkMessage(ctx, msg)
}
if a.activeForkSkillPolicy() {
    requestHistory = forkSkillRequestHistory(history)
}
```

**核心循环不应知道"技能"的存在。** fork 子 Agent 是一种应用层的扩展模式，应通过 hook/callback 注入，而不是在循环里 if-else。

### 2.4 agent.go：状态字段污染

Agent 结构体包含了大量应用层状态字段：

```go
type Agent struct {
    // ... 核心字段 ...
    activePolicy    *activeToolPolicy       // 技能策略状态
    forkSession     *session.Session        // fork 会话
    activeForkCancel context.CancelFunc     // fork 取消
    isForkChild     bool                    // 是否是 fork 子 Agent
}
```

以及应用层方法：`ActivateToolPolicyWithContext()`、`toolAllowedByPolicy()`、`policyToolDescription()` 等。

### 2.5 tool.go：数据结构污染

`ToolPolicyActivation` 结构体包含了技能特定字段：

```go
type ToolPolicyActivation struct {
    SkillRoot, AllowedSkillPaths, BlockedSkillPaths  // 技能路径
    Branch, ExecutionContext, CompactContext           // 技能上下文
    FilePath, Args                                    // 技能执行参数
}
```

核心层的类型定义不应包含 `SkillRoot` 这样的字段。

## 3. 对比：原始 pi 的做法

| 机制 | 原始 pi (TS) | pi-go (当前) |
|---|---|---|
| 工具策略 | coding-agent 应用层，通过 hook 注入 agent-core | 硬编码在 `internal/agent/tool_policy.go` |
| 技能 fork | harness 层管理，agent-core 只提供子 Agent 机制 | 硬编码在 `internal/agent/loop.go` |
| bash 解析 | coding-agent 的内置工具层 | 下沉到 `internal/agent/tool_policy.go` |
| 技能路径 | harness/skills.ts 管理 | 下沉到 `internal/agent/tool_policy.go` |
| Goal 评估 | harness 层通过回调注入 | 部分硬编码在 `internal/agent/goal_evaluator.go` |

## 4. 根本原因

pi-go 在实现时，将原始 pi 的 coding-agent 应用层逻辑（工具策略、技能系统、bash 解析）直接放入了 agent 核心包，而不是通过接口/回调注入。可能的原因：

1. **快捷实现**：直接写在一起比定义接口+注入更省事
2. **缺乏 harness 层**：原始 pi 有一个 agent-harness 层作为中间层，pi-go 没有对应的抽象
3. **渐进式腐化**：先加一个功能，再加一个，逐步累积，没有及时重构

## 5. 影响

1. **不可复用**：`internal/agent/` 无法被非编码场景（如对话助手、数据分析 Agent）复用，因为它内置了 bash/文件/技能假设
2. **不可测试**：核心循环的测试需要 mock 技能系统，增加测试复杂度
3. **耦合严重**：修改技能系统需要同时改核心层，违反开闭原则
4. **学习误导**：作为学习项目，当前架构传递了"核心层可以包含领域逻辑"的错误信号

## 6. 纠偏方案

### 6.1 目标状态

```
┌──────────────────────────────────────────────┐
│  coding-agent（应用层）                       │
│  bash 解析、技能路径、工具策略、fork 生命周期  │
├──────────────────────────────────────────────┤
│  agent-core（核心层）                         │
│  循环、Tool 抽象、通用策略接口、事件、会话     │
├──────────────────────────────────────────────┤
│  ai（基础设施层）                             │
│  LLM Provider 抽象                           │
└──────────────────────────────────────────────┘
```

### 6.2 具体步骤

| 步骤 | 内容 | 从 | 到 |
|---|---|---|---|
| 1 | 定义通用策略接口 | - | `internal/agent/tool_policy.go`（仅保留接口） |
| 2 | 迁移 bash 解析 | `internal/agent/tool_policy.go` | `internal/tools/bash/` 或 `internal/agents/coding/` |
| 3 | 迁移技能路径逻辑 | `internal/agent/tool_policy.go` | `internal/skill/` |
| 4 | 迁移技能结果处理 | `internal/agent/tool_policy.go` | `internal/skill/` |
| 5 | 将 fork 逻辑改为接口注入 | `internal/agent/loop.go` | 通过 `LoopHook` 或 `ForkHandler` 接口 |
| 6 | 清理 Agent 结构体状态字段 | `internal/agent/agent.go` | 通过接口持有应用层状态 |
| 7 | 迁移 ToolPolicyActivation | `internal/agent/tool.go` | 核心层只保留通用策略字段 |

### 6.3 关键接口设计（示例）

```go
// internal/agent/tool_policy.go - 核心层只保留接口
type ToolPolicyChecker interface {
    IsToolAllowed(name string, args json.RawMessage) (bool, string)
    TransformToolDescription(name, desc string) string
}

type ForkHandler interface {
    ShouldFork(message ai.Message) bool
    RunFork(ctx context.Context, parent *Agent, message ai.Message) (ai.AssistantMessage, error)
}
```

应用层实现这些接口，通过 `Agent` 构造函数注入。

## 7. 结论

**pi-go 的分层确实严重偏离了原始 pi 的设计。** 核心层 `internal/agent/` 被约 2,800 行的领域特定逻辑污染（占核心层总量的 64%），导致分层名存实亡。应用层 `internal/agents/coding/` 反而只有 405 行的薄壳。

这不是一个架构洁癖问题——它直接导致了核心层不可复用、不可独立测试、修改成本高。如果 pi-go 要成为一个通用 Agent 框架（而非只服务编码场景），纠偏是必要的。
