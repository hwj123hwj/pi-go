---
status: approved
author: plan-agent
created: 2026-05-28
updated: 2026-05-28
depends-on:
  - docs/research/cc-haha-core-engine-analysis.md
  - src/goals/goalState.ts (cc-haha reference)
  - src/query/stopHooks.ts (cc-haha reference)
---

# Goal-Driven Agent Loop：让 /goal 真正驱动 Agent 持续工作直到目标完成

## 1. 目标

改造 `/goal` 命令和 Agent 循环，使设置目标后 Agent 自动持续工作直到目标完成，
而不是像现在一样 LLM 说停就停。

## 2. 为什么现在做

**用户痛点**：设置 `/goal 继续阅读我的代码，提出优化方案` 后发消息"开始吧"，
Agent 执行几步就停了——因为当前 Agent 循环只在有 tool call 时自动继续，
LLM 返回纯文本后循环立刻终止，即使目标远未完成。

**根本原因**：pi-go 的 Agent 循环是"反应式"的（LLM 说停就停），
不是"目标驱动"的（检查目标是否达成才决定停不停）。

**CC 参考实现**：Claude Code 通过 Stop Hook 机制实现——每轮结束时用独立 LLM 评估
"目标是否达成"，未达成则注入 follow-up 让 Agent 继续。
pi-go 没有 Hook 系统，但可以用更简单的方式实现等效效果。

## 3. 设计方案

### 3.1 核心思路

不引入 CC 那样复杂的 Hook 系统，而是在 Agent 循环的 `actionDone` 分支中
加入"目标检查 + 自动 follow-up"：

```
processTurn 返回 actionDone
  ↓
如果 goal != ""
  → 将 goal 注入为 follow-up 消息
  → action 改为 actionFollowUp
  → Agent 继续下一轮
否则
  → 正常停止
```

### 3.2 架构选择：为什么不用 CC 的 Stop Hook

| 维度 | CC Stop Hook | pi-go 方案 |
|------|-------------|-----------|
| 复杂度 | 需要 Hook 系统、独立 LLM 评估、prompt 工程 | 直接在循环中检查 |
| 额外 LLM 调用 | 每轮一次（评估"目标是否达成"） | 无（由 Agent 自行判断） |
| 可靠性 | 依赖评估 LLM 的判断质量 | 依赖 Agent 自身判断 |
| 实现成本 | 需要先建 Hook 基础设施 | 改动集中在 2-3 个文件 |

**选择理由**：pi-go 当前没有 Hook 系统，建一套只为 `/goal` 服务的 Hook 过重。
简单方案是把 goal 作为 follow-up 注入，Agent 自然会"看到"目标还没完成而继续工作。

### 3.3 具体改动

#### 改动 1：Agent 增加 goal 字段和 setter

`internal/agent/agent.go`：
- `Agent` 结构体增加 `goal string` 字段
- 增加 `SetGoal(string)` / `Goal() string` / `ClearGoal()` 方法
- `Options` 增加 `Goal string`

#### 改动 2：Agent 循环 goal-driven 逻辑

`internal/agent/loop.go` `runAgentLoop()`：

```go
case actionDone:
    // 如果有活跃 goal，注入 follow-up 让 Agent 继续
    if a.goal != "" {
        pending = []ai.Message{ai.NewTextUserMessage(
            fmt.Sprintf("Reminder: your current goal is \"%s\". Continue working on it. If the goal is fully achieved, say so explicitly.", a.goal),
        )}
        // 继续循环而不是返回
    } else {
        return lastAssistant, nil
    }
```

#### 改动 3：Agent 识别目标完成

在 `processTurn` 中，当 assistant 回复包含"goal achieved"语义时清除 goal：

```go
// 检测 Agent 是否表示目标已完成
if a.goal != "" && message.StopReason == ai.StopReasonStop {
    if goalCompleted(message.Text, a.goal) {
        a.goal = "" // 清除 goal，下一轮正常停止
    }
}
```

`goalCompleted` 使用简单的关键词匹配（"goal achieved"、"任务完成"等），
不做额外的 LLM 调用。

#### 改动 4：/goal 命令增加 ShouldQuery

`internal/slashcmd/registry.go`：
- `CommandResult` 增加 `ShouldQuery bool` 字段
- 当 `ShouldQuery=true` 时，交互模式自动触发一轮 Agent 调用

`internal/agents/coding/commands/builtins.go`：
- `/goal <text>` 返回 `ShouldQuery: true`

#### 改动 5：interactive.go 处理 ShouldQuery

`internal/agents/coding/cli/interactive.go`：
- 检测到 `ShouldQuery=true` 时自动调用 `m.runPrompt(ctx, "...")`
- 使用 goal 文本作为初始 prompt

#### 改动 6：Goal 注入 system prompt（保留现有行为）

`internal/agents/coding/session_ext.go` 和 `prompt/builder.go` 的现有逻辑保留，
goal 同时出现在 system prompt 和 follow-up 中，双重保障。

### 3.4 安全限制

- **MaxTurns 兜底**：goal-driven 循环仍受 `maxTurns` 限制（默认 8），不会无限循环
- **自动提升 MaxTurns**：当有 active goal 时，自动将 MaxTurns 提升到 20（或配置值）
- **Goal 完成**：Agent 明确表示完成时清除 goal 并停止
- **用户中断**：Ctrl-C / context cancel 仍然有效

## 4. 不做什么

- 不实现完整的 Hook 系统（留到未来）
- 不做独立的 LLM 评估器判断目标是否达成（用简单关键词匹配）
- 不改 Agent 循环的核心结构（双层循环不变）
- 不改 session 持久化格式（goal 只存内存，重启后丢失）

## 5. 新增/修改文件清单

```
修改:
  internal/agent/agent.go       — 增加 goal 字段和方法
  internal/agent/loop.go        — goal-driven 循环逻辑
  internal/agent/event.go       — 新增 EventGoalCompleted 事件（可选）
  internal/slashcmd/registry.go — CommandResult 增加 ShouldQuery
  internal/agents/coding/commands/builtins.go — /goal 返回 ShouldQuery
  internal/agents/coding/cli/interactive.go — 处理 ShouldQuery
  internal/agents/coding/session_ext.go — SetGoal 时同步到 Agent
  internal/runtime/agent_session.go — buildAgent 时传入 goal

新增:
  internal/agent/goal.go        — goalCompleted() 检测函数 + 测试
  internal/agent/goal_test.go   — goalCompleted 单测
```

## 6. 验收标准

1. `/goal 优化 app.go 的错误处理` → 显示 "Goal set" → Agent 自动开始工作
2. Agent 持续执行多个 tool call 轮次，不会中途停下
3. Agent 明确表示"任务完成"后自动停止，显示 "Goal completed"
4. 达到 maxTurns 后正常停止，不会无限循环
5. `/goal clear` 可以中途取消目标
6. `/goal` (无参数) 显示当前目标状态
7. `go test ./internal/agent/...` 通过
8. `go test ./internal/agents/coding/...` 通过
