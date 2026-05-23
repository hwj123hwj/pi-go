# Pi-Go Tool Lifecycle 执行文档

> 用途：指导执行 agent 为 `pi-go` 补齐最小可用的 tool lifecycle。  
> 本次目标不是做完整插件平台，而是把工具执行主链从“校验 -> 执行”升级成可拦截、可扩展、可流式反馈的结构。

## 1. 本次目标

本次任务要完成 4 件事：

1. 给工具执行链补上统一的 lifecycle 阶段
2. 让扩展系统可以在工具调用前后参与
3. 打通 tool partial result 到 agent event / stream 的回传
4. 保持现有工具、CLI、server、desktop 主链路不回归

## 2. 当前现状

当前工具执行主链在 [internal/agent/loop.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/agent/loop.go:287) 的 `executeOneTool()`。

现状流程是：

1. 找到 tool
2. `decodeToolArgs`
3. `tool.Execute(ctx, validated, nil)`
4. emit `EventToolExecutionEnd`

当前明确缺失的能力：

- 没有 `prepareToolCall`
- 没有 `beforeToolCall`
- 没有 `afterToolCall`
- 虽然有 [EventToolExecutionUpdate](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/agent/event.go:30)，但没有真正发出来
- 虽然 `Tool.Execute()` 带 `onUpdate func(PartialResult)`，但当前传的是 `nil`
- 扩展系统有 [Registry.EmitHook()](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/extensions/registry.go:58)，但没有接入工具执行主链

## 3. 本次边界

### 本次要做

- 定义最小 lifecycle 数据结构
- 增加 before / after hooks
- 增加参数预处理入口
- 增加 partial update 回传
- 把这些能力接进 `executeOneTool()`
- 增加测试

### 本次不要做

- 不做复杂权限系统
- 不做完整策略引擎
- 不做跨 turn 的 tool queue 编排
- 不做 UI 大改
- 不做完整的插件协议升级
- 不把所有 extension 机制一次性重构掉

## 4. 设计目标

本次设计要满足：

### 4.1 工具本身不必知道 hook 细节

工具继续只关心：

- 参数校验
- 具体执行
- 可选 partial update

tool lifecycle 的 orchestration 统一放在 agent 层。

### 4.2 扩展可以拦截工具调用

扩展后续至少能做到：

- 调用前审计
- 调用前阻断
- 参数修正
- 结果修正

### 4.3 不破坏现有工具接口

当前 [Tool](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/agent/tool.go:8) 接口已经包含：

```go
Execute(ctx context.Context, params json.RawMessage, onUpdate func(PartialResult)) (ToolResult, error)
```

这足够承接 partial update，所以本次优先不修改这个接口签名。

## 5. 推荐实现结构

建议新增：

```text
internal/agent/tool_lifecycle.go
internal/agent/tool_lifecycle_test.go
```

如果你觉得更合适，也可以拆成：

- `tool_hooks.go`
- `tool_lifecycle.go`

但不要把太多 lifecycle 逻辑继续堆到 `loop.go` 里。

## 6. 生命周期模型

建议把一次工具调用拆成这些阶段：

1. `prepare`
2. `before_execute`
3. `execute`
4. `partial_update`（零次到多次）
5. `after_execute`
6. `finish`

建议定义一个统一上下文结构，例如：

```go
type ToolCallContext struct {
    ToolCallID string
    ToolName   string
    RawArgs    json.RawMessage
    Args       json.RawMessage
}
```

再定义一个结果结构，例如：

```go
type ToolExecutionResult struct {
    Result agent.ToolResult
    Err    error
}
```

## 7. Hook 模型

本次先做最小集合，不要一次设计过重。

建议在 `extensions` 包中补两类 hook 事件：

- `tool.before`
- `tool.after`

也可以继续复用字符串事件名，不必现在就强行做类型系统。

### 7.1 before hook

before hook 至少应支持：

- 读取当前 tool name / args
- 返回修改后的 args
- 返回阻断错误

建议语义：

- hook 返回 `nil`：继续
- hook 返回一个“阻断错误”：终止执行
- hook 返回修改后的参数：后续执行使用新参数

为了实现这个能力，单纯复用当前 `EmitHook(ctx, event, data any) error` 不够。

建议新增一类更适合 tool lifecycle 的接口，例如：

```go
type BeforeToolCallHook func(ctx context.Context, call ToolCallContext) (ToolCallContext, error)
type AfterToolCallHook func(ctx context.Context, call ToolCallContext, result agent.ToolResult) (agent.ToolResult, error)
```

不要强行把“参数可修改”和“纯监听事件”塞到一个弱类型 `any` 回调里。

## 8. prepareArguments

本次建议把 prepareArguments 做成 **tool 可选接口**，而不是全局强制逻辑。

建议新增可选接口：

```go
type ToolWithPrepareArguments interface {
    Tool
    PrepareArguments(ctx context.Context, params json.RawMessage) (json.RawMessage, error)
}
```

执行顺序建议：

1. raw args
2. `tool.Validate(...)`
3. 如果实现了 `ToolWithPrepareArguments`，再执行 `PrepareArguments(...)`
4. before hooks

这样更稳，因为先有校验，再做格式标准化或补默认值。

## 9. partial result 打通

这是本次第二个重点。

当前 [EventToolExecutionUpdate](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/agent/event.go:30) 已经存在，但主链没发。

执行 agent 需要在 `executeOneTool()` 里：

- 构造 `onUpdate := func(pr PartialResult) { ... }`
- 把它传给 `tool.Execute(...)`
- 每次收到 partial update 时 emit `EventToolExecutionUpdate`

推荐行为：

```go
onUpdate := func(pr PartialResult) {
    a.emit(ctx, EventToolExecutionUpdate{
        ToolCallID: call.ID,
        ToolName:   call.Name,
        Args:       validated,
        PartialResult: pr,
    })
}
```

### 9.1 stream 链路要求

如果 `PromptStream()` 当前已经能转发 agent event 到 WebSocket / SSE / Desktop，就直接复用。

如果没有，需要至少保证：

- agent 层 event 已经完整发出
- server/desktop 后续能方便接入

本次不强制同时做完整前端消费，但 agent 侧必须打通。

## 10. executeOneTool 改造要求

本次执行主链建议改成：

1. 解析 tool
2. emit `EventToolExecutionStart`
3. validate args
4. optional `PrepareArguments`
5. run before hooks
6. build `onUpdate`
7. execute tool
8. run after hooks
9. emit `EventToolExecutionEnd`
10. append tool result

注意：

- before hook 阻断时，也要 emit `EventToolExecutionEnd`
- after hook 出错时，也要明确是否视为工具执行失败

建议策略：

- tool 本体成功、after hook 失败：整体视为失败更安全
- 但要把原始 tool result 保留在错误上下文里，方便排查

## 11. 扩展注册表改造建议

当前 [Registry](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/extensions/registry.go:12) 更偏“事件广播型”。

为了支持 lifecycle，建议最小增量改造：

### 11.1 新增生命周期 hook 注册能力

例如：

```go
type LifecycleHooks struct {
    BeforeToolCall []BeforeToolCallHook
    AfterToolCall  []AfterToolCallHook
}
```

Registry 暴露：

- `BeforeToolCallHooks() []BeforeToolCallHook`
- `AfterToolCallHooks() []AfterToolCallHook`

### 11.2 不要破坏现有通用 Hooks

当前 `Hooks() []Hook` 可以继续保留给非 tool lifecycle 事件。

本次只是在 registry 里补一条更适合工具调用链的专用通道。

## 12. Agent 与 Extension 的接线

本次需要确认 `agent.Agent` 是否能访问 extension registry。

如果当前 `agent.Agent` 还拿不到它，建议：

- 在 `agent.Options` 中新增 lifecycle hook 集合
- 由 runtime/buildAgent 时把 extension registry 聚合后的 hooks 注入 agent

不要让 `agent` 直接 import `extensions`，避免层次反转。

建议是：

- `extensions` 负责定义和聚合 hook
- `runtime` 负责装配
- `agent` 只消费抽象后的 hook 函数切片

## 13. 推荐执行顺序

执行 agent 必须按下面顺序做：

### Step 1：定义 lifecycle 抽象

包括：

- tool call context
- before / after hook 类型
- optional `ToolWithPrepareArguments`

完成标志：

- 编译通过
- 未改执行逻辑

### Step 2：agent 层接入 partial update

先不做 hooks，只把：

- `onUpdate`
- `EventToolExecutionUpdate`

打通。

完成标志：

- `tool.Execute(..., onUpdate)` 不再传 `nil`
- 测试能看到 update event

### Step 3：接入 prepareArguments

完成标志：

- 实现该接口的测试工具可以修改 validated args

### Step 4：接入 before hooks

要求：

- 支持只读
- 支持修改参数
- 支持阻断执行

完成标志：

- 测试覆盖这三种情况

### Step 5：接入 after hooks

要求：

- 支持修改结果
- 支持返回错误

完成标志：

- 测试覆盖结果改写和失败语义

### Step 6：runtime / extension 装配

把 extension registry 里的 lifecycle hooks 真的注入 agent。

完成标志：

- 一个最小假扩展能在工具调用前后生效

## 14. 测试要求

本次至少补这些测试：

### 14.1 agent 单元测试

- tool emits partial update
- before hook can block tool
- before hook can rewrite args
- after hook can rewrite result
- after hook error is surfaced
- prepareArguments is applied before execution

### 14.2 extension / runtime 测试

- registry 能聚合 lifecycle hooks
- buildAgent 后 hooks 真被注入 agent

### 14.3 回归测试

必须重新跑：

```bash
go test ./...
go vet ./...
```

## 15. 验收标准

本次完成后，必须满足：

### A. 结构上

- `executeOneTool()` 不再只是“校验 -> 执行”
- lifecycle 已成为明确的一层

### B. 能力上

- 工具可以发 partial update
- before hook 可以阻断或改参数
- after hook 可以改结果

### C. 集成上

- runtime 能把 hooks 注入 agent
- 现有工具与入口不回归

## 16. 本次明确不追求的高级能力

这些能力可以留到下一阶段：

- policy engine
- 细粒度权限矩阵
- tool sandbox approval UI
- 可配置 hook 优先级
- 跨 extension 的复杂排序
- hook tracing 可视化

## 17. 推荐提交拆分

建议拆成这几段：

1. lifecycle 抽象 + partial update
2. prepareArguments + before hooks
3. after hooks
4. runtime / extension 装配
5. 测试和文档

## 18. 给执行 agent 的一句话指令

先把 tool partial update 打通，再把 before / after hooks 和 prepareArguments 按最小闭环接入 `executeOneTool()`；不要一开始把整个扩展系统推翻重做。
