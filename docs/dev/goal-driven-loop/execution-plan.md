---
status: done
author: execution-agent
created: 2026-05-28
completed: 2026-05-28
---

# Goal-Driven Agent Loop 执行文档

## 完成的改动

### 1. `internal/agent/agent.go` — Agent 增加 goal 字段

- `Agent` 结构体增加 `goal string` 和 `goalMaxTurns int`
- `Options` 增加 `Goal string` 和 `GoalMaxTurns int`
- 新增 `Goal()`, `SetGoal()`, `ClearGoal()` 方法

### 2. `internal/agent/goal_evaluator.go` — **LLM 目标完成评估器**（核心新增）

- `evaluateGoalCompletion()` — 用独立 LLM 调用判断目标是否完成
- `buildGoalEvalPrompt()` — 参照 CC 的 `createGoalPromptHook` 设计评估 prompt
  - 精心设计的 prompt：只做判断，不执行目标
  - 要求返回 `{"ok": true/false, "reason": "..."}`
  - 保守策略：evidence ambiguous → false
- `parseGoalEvalResult()` — 解析 LLM JSON 响应，支持 markdown fence 清理
- **三级 fallback 链**：
  1. LLM 评估成功 → 使用 LLM 判断
  2. LLM 调用失败（provider 不可用/stream 错误）→ fallback 到关键词匹配
  3. JSON 解析失败 → fallback 到关键词匹配

### 3. `internal/agent/goal.go` — 关键词 fallback

- `goalCompleted(responseText string) bool` — 中英文关键词匹配
- 作为 LLM 评估器不可用时的降级方案

### 4. `internal/agent/goal_evaluator_test.go` — 评估器单测

- `TestEvaluateGoalCompletion_LLMTrue` — LLM 返回 `{"ok": true}`
- `TestEvaluateGoalCompletion_LLMFalse` — LLM 返回 `{"ok": false, "reason": "..."}`
- `TestEvaluateGoalCompletion_LLMWithMarkdownFence` — 处理 ```json 包裹
- `TestEvaluateGoalCompletion_NoProvider_Fallback` — 无 provider 时降级到关键词
- `TestEvaluateGoalCompletion_EmptyResponse` — 空回复直接返回 false
- `TestParseGoalEvalResult` — 6 个子用例覆盖 JSON 解析边界
- `TestBuildGoalEvalPrompt` — prompt 包含目标和响应
- `TestBuildGoalEvalPrompt_Truncation` — 长响应截断

### 5. `internal/agent/goal_test.go` — 关键词 fallback 单测

- 14 个测试用例覆盖中英文、大小写、误判保护

### 6. `internal/agent/event.go` — 新事件类型

- `EventGoalCompleted{Goal string}` — 目标完成时发出

### 7. `internal/agent/loop.go` — Goal-driven 循环核心

- `turnResult` 增加 `goalDone bool` 字段
- `runAgentLoop()` 中有 goal 时提升 effectiveMaxTurns（默认 20）
- `actionDone` 分支：goal 未完成 → 注入 follow-up 继续循环
- `processTurn()` 末尾：调用 `evaluateGoalCompletion()` 判断目标是否完成

### 8. `internal/slashcmd/registry.go` — CommandResult 扩展

- `CommandResult` 增加 `ShouldQuery bool` 字段

### 9. `internal/agents/coding/commands/builtins.go` — /goal 命令改造

- `/goal <text>` 返回 `ShouldQuery: true`，自动触发 Agent 执行

### 10. `internal/agents/coding/cli/interactive.go` — 处理 ShouldQuery

- 检测 `ShouldQuery=true` 时自动调用 `runPrompt(ctx, "Start working on the goal.")`

### 11. `internal/runtime/agent_session.go` — buildAgent 传入 goal

- `buildAgent()` 从 `s.ext.Goal()` 读取目标传入 `agent.Options.Goal`
- `GoalMaxTurns` 设为 20

## 验证结果

- `go build ./...` ✅
- `go vet ./...` ✅
- `go test ./internal/agent/...` ✅（含 22+ 个 goal 相关单测）
- `go test ./internal/...` ✅（全量零失败）

## 工作流程

```
用户: /goal 优化 app.go 的错误处理
  ↓
1. builtins.go: SetGoal() → rebuild agent (goal 注入 system prompt + agent.goal)
2. CommandResult{ShouldQuery: true}
  ↓
3. interactive.go: 检测 ShouldQuery → runPrompt("Start working on the goal.")
  ↓
4. Agent 循环开始:
   - effectiveMaxTurns = 20 (因为 goal != "")
   - Agent 执行工具调用 → 内层循环继续
   - Agent 返回纯文本 → actionDone
     ↓
     evaluateGoalCompletion() 用 LLM 评估:
       → {"ok": false, "reason": "..."} → 注入 follow-up → 继续循环
       → {"ok": true} → goal 清除 → EventGoalCompleted → 正常停止
       → LLM 失败 → fallback 关键词匹配
   - 或达到 20 轮 → 正常停止
```

## 与 CC 的对比

| 维度 | CC (Claude Code) | pi-go |
|------|-----------------|-------|
| 评估机制 | Stop Hook + 独立 LLM | 独立 LLM 评估器 |
| 评估 prompt | `createGoalPromptHook` | `buildGoalEvalPrompt`（同等设计） |
| 回复格式 | `{"ok": bool, "reason": string}` | 相同 |
| fallback | 无（Hook 失败就停） | 三级 fallback：LLM → 关键词 → 停 |
| maxTurns | 无上限（交互模式） | 20（有 goal 时） |
| Token Budget | 有 | 无 |
| Hook 系统 | 完整的 Stop/PreToolUse/PostToolUse | 无（只用 LLM 评估器） |
| 自动触发 | `shouldQuery: true` | `ShouldQuery: true`（相同） |
