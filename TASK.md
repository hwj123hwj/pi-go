# 任务规划

> 本文档由主 Agent 在建群时自动生成，描述本群 agent 的工作目标。
> 文件持久保存在工作区根目录，可随时查看或编辑。

---

### 任务：移植 Hooks 和 Policy 引擎到 pi-go

**目标**：从 hwjcode 移植 Hooks 系统和 Policy 引擎，增强 pi-go 的工具执行管控能力。pi-go 已有基础的 tool_lifecycle hooks，需要升级为完整的 hooks 系统。

**参考源码（hwjcode TypeScript）**：
- `/home/q/hwjcode/packages/core/src/hooks/hookSystem.ts` — Hook 系统主入口
- `/home/q/hwjcode/packages/core/src/hooks/hookRegistry.ts` — Hook 注册表
- `/home/q/hwjcode/packages/core/src/hooks/hookRunner.ts` — Hook 执行器
- `/home/q/hwjcode/packages/core/src/hooks/hookAggregator.ts` — Hook 聚合器
- `/home/q/hwjcode/packages/core/src/hooks/hookPlanner.ts` — Hook 规划器
- `/home/q/hwjcode/packages/core/src/hooks/hookEventHandler.ts` — Hook 事件处理器
- `/home/q/hwjcode/packages/core/src/hooks/hookTranslator.ts` — Hook 翻译器
- `/home/q/hwjcode/packages/core/src/hooks/types.ts` — Hook 类型定义
- `/home/q/hwjcode/packages/core/src/policy/policy-engine.ts` — 策略引擎
- `/home/q/hwjcode/packages/core/src/policy/policy-updater.ts` — 策略更新器

**输出到 pi-go（Go）**：
1. `internal/hooks/` 目录：
   - `types.go` — Hook 类型定义（PreToolCall, PostToolCall, PreMessage, PostMessage, onError 等）
   - `registry.go` — Hook 注册表（支持按事件类型注册、优先级排序）
   - `runner.go` — Hook 执行器（顺序执行、错误处理、超时控制）
   - `system.go` — Hook 系统主入口（聚合 registry + runner + aggregator）
2. `internal/policy/` 目录：
   - `engine.go` — 策略引擎（Allow / AskUser / Deny 决策）
   - `rules.go` — 策略规则定义（按工具名、文件路径匹配）
   - `updater.go` — 策略更新器（持久化用户决策）

**pi-go 架构约定（必须遵守）**：
- 先 `read_file` 看 `internal/agent/tool_lifecycle.go` 了解现有的 BeforeToolCallHook / AfterToolCallHook 实现
- 新的 hooks 系统要与现有 tool_lifecycle 兼容，可以包装或替代它
- 先 `read_file` 看 `internal/agent/tool.go` 了解 Tool 接口
- 先 `read_file` 看 `internal/agent/agent.go` 了解 Agent 主循环（hooks 在哪里插入）
- Policy 引擎的决策类型：Allow（直接执行）、AskUser（需确认）、Deny（拒绝）
- 规则匹配支持：工具名通配符、文件路径 glob 模式
- 策略持久化到 `.pi-go/policy.json`
- 用 `slog` 做日志

**验收标准**：
- `go build ./...` 通过
- `go vet ./internal/hooks/ ./internal/policy/` 通过
- hooks registry 测试：注册、按优先级排序、按类型查询
- policy engine 测试：规则匹配、Allow/Deny/AskUser 决策
- 与现有 tool_lifecycle 的集成测试
