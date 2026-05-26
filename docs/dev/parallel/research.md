---
status: complete
author: research-agent
created: 2026-05-27
updated: 2026-05-27
---

# 并行工具调研报告：parallel-tools

## 1. 调研目标

**问题**：pi-go 当前的工具执行模型是全量并行或全量顺序，缺乏细粒度的并发安全控制。LLM 发出多个 tool call 时（如同时读 5 个文件、或同时读 2 个文件 + 编辑 1 个文件），pi-go 要么全部并行（可能导致写冲突），要么全部顺序（浪费读操作的并行性）。

**目标**：参考 Claude Code (cc-haha) 和 Codex 的实现，为 pi-go 设计：

1. **并行读工具**：多个 read/grep/find/ls 调用可安全并行执行
2. **串行写保护**：对同一文件的 edit/write 操作串行化，不同文件可并行
3. **批次分区调度**：将 LLM 返回的 tool call 列表按并发安全分区，安全批次并行执行，不安全批次串行执行

## 2. 参考项目分析

### 2.1 Claude Code (cc-haha)

#### 核心架构：toolOrchestration.ts 的分区调度

**文件**：`cc-haha/src/services/tools/toolOrchestration.ts`

Claude Code 的并行调度核心是 `partitionToolCalls` + `runTools` 的两级结构：

```typescript
// toolOrchestration.ts:84-116
type Batch = { isConcurrencySafe: boolean; blocks: ToolUseBlock[] }

function partitionToolCalls(
  toolUseMessages: ToolUseBlock[],
  toolUseContext: ToolUseContext,
): Batch[] {
  return toolUseMessages.reduce((acc: Batch[], toolUse) => {
    const tool = findToolByName(toolUseContext.options.tools, toolUse.name)
    const parsedInput = tool?.inputSchema.safeParse(toolUse.input)
    const isConcurrencySafe = parsedInput?.success
      ? Boolean(tool?.isConcurrencySafe(parsedInput.data))
      : false
    if (isConcurrencySafe && acc[acc.length - 1]?.isConcurrencySafe) {
      acc[acc.length - 1]!.blocks.push(toolUse)
    } else {
      acc.push({ isConcurrencySafe, blocks: [toolUse] })
    }
    return acc
  }, [])
}
```

**设计要点**：
- 每个 tool call 独立判断 `isConcurrencySafe(input)`
- 连续的安全调用合并为一个并行批次
- 不安全的调用独占一个串行批次
- 批次间严格顺序执行

#### Tool 接口的 isConcurrencySafe 方法

**文件**：`cc-haha/src/Tool.ts:402`

```typescript
export type Tool<Input, Output, P> = {
  // ...
  isConcurrencySafe(input: z.infer<Input>): boolean
  isReadOnly(input: z.infer<Input>): boolean
  // ...
}
```

关键：`isConcurrencySafe` 接收 **具体输入** 作为参数，允许根据参数动态判断。默认值为 `false`（保守策略）：

```typescript
// cc-haha/src/Tool.ts:759
const TOOL_DEFAULTS = {
  isConcurrencySafe: (_input?: unknown) => false,
  isReadOnly: (_input?: unknown) => false,
  // ...
}
```

#### 各工具的 isConcurrencySafe 实现

| 工具 | isConcurrencySafe | 说明 |
|------|-------------------|------|
| **FileReadTool** | `() => true` | 纯读操作，始终安全 |
| **FileEditTool** | 未覆盖，默认 `false` | 写操作，不安全 |
| **FileWriteTool** | 未覆盖，默认 `false` | 写操作，不安全 |
| **BashTool** | 未覆盖，默认 `false` | 可能有副作用，不安全 |
| **GlobTool** | `() => true` | 纯搜索，安全 |
| **GrepTool** | `() => true` | 纯搜索，安全 |

#### 并行执行的具体实现

**文件**：`cc-haha/src/services/tools/toolOrchestration.ts:152-177`

```typescript
async function* runToolsConcurrently(
  toolUseMessages: ToolUseBlock[],
  // ...
): AsyncGenerator<MessageUpdateLazy, void> {
  yield* all(
    toolUseMessages.map(async function* (toolUse) {
      toolUseContext.setInProgressToolUseIDs(prev =>
        new Set(prev).add(toolUse.id),
      )
      yield* runToolUse(toolUse, ..., canUseTool, toolUseContext)
      markToolUseAsComplete(toolUseContext, toolUse.id)
    }),
    getMaxToolUseConcurrency(), // 默认 10
  )
}
```

`all()` 是一个带并发上限的 generator 合并器（`cc-haha/src/utils/generators.ts`），用 `Promise.race` 实现背压控制，保证最多 N 个工具同时运行。

#### FileEditTool 的文件修改保护

**文件**：`cc-haha/src/tools/FileEditTool/FileReadTool.ts:375`

FileReadTool 的 `isConcurrencySafe() => true` 意味着多个读操作可并行。但 FileEditTool 使用了 **readFileState** 追踪文件读取时间戳：

```typescript
// FileEditTool.ts:275-311
const readTimestamp = toolUseContext.readFileState.get(fullFilePath)
if (!readTimestamp || readTimestamp.isPartialView) {
  return { result: false, message: 'File has not been read yet...' }
}
if (readTimestamp) {
  const lastWriteTime = getFileModificationTime(fullFilePath)
  if (lastWriteTime > readTimestamp.timestamp) {
    return { result: false, message: 'File has been modified since read...' }
  }
}
```

这是一个重要的"先读后写"约束。

#### 并发上下文修改器

**文件**：`cc-haha/src/services/tools/toolOrchestration.ts:36-53`

并行批次中，工具可能需要修改共享 context（如 readFileState）。CC 使用 **queuedContextModifiers** 延迟应用修改：

```typescript
const queuedContextModifiers: Record<string, ((context: ToolUseContext) => ToolUseContext)[]> = {}
// 并行执行期间收集 context modifiers
for await (const update of runToolsConcurrently(...)) {
  if (update.contextModifier) {
    const { toolUseID, modifyContext } = update.contextModifier
    if (!queuedContextModifiers[toolUseID]) queuedContextModifiers[toolUseID] = []
    queuedContextModifiers[toolUseID].push(modifyContext)
  }
}
// 批次结束后统一应用
for (const block of blocks) {
  const modifiers = queuedContextModifiers[block.id]
  for (const modifier of modifiers) currentContext = modifier(currentContext)
}
```

### 2.2 Pi TypeScript Monorepo (packages/)

#### Agent Loop 的并行/顺序调度

**文件**：`packages/agent/src/agent-loop.ts:373-388`

```typescript
async function executeToolCalls(
  currentContext: AgentContext,
  assistantMessage: AssistantMessage,
  config: AgentLoopConfig,
  signal: AbortSignal | undefined,
  emit: AgentEventSink,
): Promise<ExecutedToolCallBatch> {
  const toolCalls = assistantMessage.content.filter((c) => c.type === "toolCall")
  const hasSequentialToolCall = toolCalls.some(
    (tc) => currentContext.tools?.find((t) => t.name === tc.name)?.executionMode === "sequential",
  )
  if (config.toolExecution === "sequential" || hasSequentialToolCall) {
    return executeToolCallsSequential(...)
  }
  return executeToolCallsParallel(...)
}
```

**设计要点**：
- 粒度是 **per-tool type**（不是 per-input），不如 CC 细
- 任一 tool call 的工具声明了 `executionMode: "sequential"` 则全批串行
- 没有分批概念——要么全并行要么全串行

#### AgentTool 的 executionMode 属性

**文件**：`packages/agent/src/types.ts:377-384`

```typescript
export interface AgentTool<TParameters extends TSchema = AnySchema> {
  // ...
  executionMode?: ToolExecutionMode;
}
```

```typescript
export type ToolExecutionMode = "sequential" | "parallel"
```

#### 并行执行实现

**文件**：`packages/agent/src/agent-loop.ts:447-506`

```typescript
async function executeToolCallsParallel(...): Promise<ExecutedToolCallBatch> {
  const finalizedCalls: FinalizedToolCallEntry[] = []

  for (const toolCall of toolCalls) {
    // 预处理（validate + before hooks）— 顺序执行
    const preparation = await prepareToolCall(...)
    if (preparation.kind === "immediate") {
      finalizedCalls.push(preparation) // 验证失败等立即完成
      continue
    }
    // 将实际执行包装为 lazy thunk
    finalizedCalls.push(async () => {
      const executed = await executePreparedToolCall(preparation, signal, emit)
      const finalized = await finalizeExecutedToolCall(...)
      return finalized
    })
  }

  // 并行执行所有 thunk
  const orderedFinalizedCalls = await Promise.all(
    finalizedCalls.map(entry => typeof entry === "function" ? entry() : Promise.resolve(entry))
  )
  // ...
}
```

**设计亮点**：
- 预处理（validate + hooks）是串行的，保证 beforeToolCall 能按顺序看到每个 tool call
- 实际执行（`executePreparedToolCall`）并行
- 结果按原始顺序返回（保持 tool call ID 顺序）

#### File Mutation Queue — 文件级串行化

**文件**：`packages/coding-agent/src/core/tools/file-mutation-queue.ts`

```typescript
const fileMutationQueues = new Map<string, Promise<void>>()

export async function withFileMutationQueue<T>(filePath: string, fn: () => Promise<T>): Promise<T> {
  const key = getMutationQueueKey(filePath)  // realpath 去重
  const currentQueue = fileMutationQueues.get(key) ?? Promise.resolve()

  let releaseNext!: () => void
  const nextQueue = new Promise<void>((resolveQueue) => { releaseNext = resolveQueue })
  const chainedQueue = currentQueue.then(() => nextQueue)
  fileMutationQueues.set(key, chainedQueue)

  await currentQueue  // 等前面的操作完成
  try {
    return await fn()
  } finally {
    releaseNext()
    if (fileMutationQueues.get(key) === chainedQueue) {
      fileMutationQueues.delete(key)
    }
  }
}
```

**设计要点**：
- 每个 **文件路径** 一个 Promise chain（不是全局锁）
- 不同文件的写操作可以并行
- 同一文件的写操作严格串行（FIFO）
- 用 `realpathSync.native` 处理符号链接去重
- Edit 工具在 `withFileMutationQueue` 回调内执行完整的 read-modify-write

#### Edit 工具的多编辑支持

**文件**：`packages/coding-agent/src/core/tools/edit.ts:30-51`

```typescript
const editSchema = Type.Object({
  path: Type.String({ description: "Path to the file to edit" }),
  edits: Type.Array(replaceEditSchema, {
    description: "One or more targeted replacements. Each edit is matched against the original file, not incrementally.",
  }),
})
```

Pi 的 edit 工具支持 **一次调用多个 edits**（类似 diff 的 hunks），每个 edit 的 `oldText` 匹配原始文件内容（非增量匹配），避免重叠问题。

### 2.3 OpenAI Codex (codex-rs)

Codex 的并行策略更简单——它将 `parallel_tool_calls` 直接传给 API 层，由模型/服务端控制并发：

**文件**：`codex-rs/core/src/client.rs:473`

```rust
let ResponsesApiRequest {
    parallel_tool_calls,
    ..
} = request;
```

Codex 在 MCP 层有 `supports_parallel_tool_calls` 标记（`codex-rs/codex-mcp/src/tools.rs:33`），但并行决策主要由 OpenAI Responses API 的服务端处理。

**对 pi-go 的启发有限**：Codex 的并行控制依赖服务端调度，而非客户端分区。

## 3. pi-go 现状分析

### 3.1 当前工具接口

**文件**：`pi-go/internal/agent/tool.go`

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any
    Validate(params json.RawMessage) (json.RawMessage, error)
    Execute(ctx context.Context, params json.RawMessage, onUpdate func(PartialResult)) (ToolResult, error)
}

type ToolWithMode interface {
    Tool
    ExecutionMode() ExecutionMode
}
```

**现状**：
- `ToolWithMode` 是 **per-tool type** 的静态模式（与 Pi TS 相同）
- 没有类似 CC 的 `IsConcurrencySafe(input)` 按输入动态判断
- 没有 `IsReadOnly()` 概念

### 3.2 当前工具执行

**文件**：`pi-go/internal/agent/loop.go:243-305`

```go
func executeToolCalls(ctx context.Context, a *Agent, calls []ai.ToolCall) ([]ai.Message, error) {
    hasSequential := false
    for _, call := range calls {
        if tool, ok := a.tools[call.Name]; ok {
            if tm, ok := tool.(ToolWithMode); ok && tm.ExecutionMode() == ExecutionModeSequential {
                hasSequential = true
                break
            }
        }
    }
    if hasSequential {
        return executeToolCallsSequential(ctx, a, calls)
    }
    return executeToolCallsParallel(ctx, a, calls)
}
```

**现状**：
- 只有两个极端：全部并行或全部串行
- 没有批次分区概念
- 没有 per-file 串行化

### 3.3 当前 Read/Edit/Write 工具

**文件**：`pi-go/internal/tools/read.go`, `edit.go`, `write.go`

- **ReadTool**：简单文件读取，无 read-state 追踪
- **EditTool**：单次 old→new 替换，不支持多 edits
- **WriteTool**：全量写入
- **均无文件级锁**

### 3.4 当前工具组装

**文件**：`pi-go/internal/agents/coding/tools/tools.go`

所有工具通过 `BuildList(opts)` 组装，没有标记哪些是只读的。

## 4. Gap 分析

| 维度 | Claude Code | Pi TS | Codex | pi-go 现状 | 差距 |
|------|------------|-------|-------|-----------|------|
| **并发安全判断** | `isConcurrencySafe(input)` 按输入动态判断 | `executionMode` 按 tool type 静态判断 | API 层处理 | `ToolWithMode` 静态判断 | 缺少动态判断能力 |
| **批次分区** | `partitionToolCalls` 分安全/不安全批次 | 全量并行或全量串行 | 无 | 全量并行或全量串行 | 缺少混合批次 |
| **只读标记** | `isReadOnly()` 方法 | 无显式标记 | 无 | 无 | 缺少只读语义 |
| **文件级串行化** | readFileState + mtime 校验 | `withFileMutationQueue` per-file | 无 | 无 | **完全缺失** |
| **多编辑支持** | 单次 edit 调用（一次一个替换） | `edits[]` 数组，一次多个替换 | `apply_patch` | 单次 edit（一次一个替换） | 缺少多 edit |
| **上下文修改延迟** | `queuedContextModifiers` 批次后统一应用 | 无 | 无 | 无 | 低优先（pi-go 暂无 readFileState） |
| **并发上限** | `getMaxToolUseConcurrency()` 默认 10 | `Promise.all` 无上限 | API 层控制 | `sync.WaitGroup` 无上限 | 缺少并发控制 |

### 核心缺失清单（按优先级）

1. **P0 — 批次分区调度**：将 tool call 列表分为只读批次（并行）和写操作批次（串行）
2. **P0 — IsConcurrencySafe 接口**：工具可声明自己是否并发安全（静态或按输入动态）
3. **P1 — 文件级串行化**：同一文件的 edit/write 操作串行执行
4. **P1 — Edit 多编辑**：支持一次 edit 调用中包含多个替换
5. **P2 — 并发上限**：控制最大并行工具数
6. **P2 — Read state 追踪**：记录已读文件的 mtime，edit 时校验

## 5. 移植建议

### 5.1 直接移植

| 来源 | 移植内容 | 目标位置 |
|------|---------|---------|
| CC `partitionToolCalls` | 批次分区逻辑 | `internal/agent/loop.go` |
| CC `isConcurrencySafe` | 工具接口方法 | `internal/agent/tool.go` 新增 `ToolConcurrencyChecker` |
| Pi TS `withFileMutationQueue` | 文件级串行化队列 | `internal/agent/file_mutation.go`（新文件） |
| Pi TS `edits[]` schema | Edit 多编辑支持 | `internal/tools/edit.go` 改造 |

### 5.2 需要适配

| 来源 | 适配内容 | 原因 |
|------|---------|------|
| CC `isConcurrencySafe(input)` | Go 版本需要用 `json.RawMessage` 做参数 | Go 没有泛型 schema 推断 |
| CC `queuedContextModifiers` | pi-go 暂无 readFileState，可先不实现 | 非阻塞依赖 |
| Pi TS `realpathSync` | 用 `filepath.EvalSymlinks` 替代 | Go 标准库等价物 |

### 5.3 建议跳过

| 来源 | 跳过内容 | 原因 |
|------|---------|------|
| CC `all()` generator 并发合并器 | Go 用 `sync.WaitGroup` + channel 已足够 | Go 天生支持并发 |
| Codex API 层 `parallel_tool_calls` | pi-go 用本地调度 | 架构不同 |
| CC `FileReadTool` 的 dedup cache | 复杂度高，收益延迟 | 后续优化项 |

### 5.4 推荐实施顺序

```
Step 1: IsConcurrencySafe 接口 + 批次分区调度
        ├── 新增 ToolConcurrencyChecker 接口
        ├── 改造 executeToolCalls → partitionToolCalls
        ├── read/grep/find/ls 标记并发安全
        └── 验证并行读场景

Step 2: 文件级串行化（file mutation queue）
        ├── 新增 FileMutationQueue
        ├── edit/write 工具接入队列
        └── 验证同一文件并发写

Step 3: Edit 多编辑支持
        ├── 改造 EditParams 增加 Edits[] 数组
        ├── 实现 applyEditsToContent 逻辑
        └── 更新工具描述和提示

Step 4: 并发上限 + 细节打磨
        ├── 添加 MaxConcurrentTools 配置
        ├── 串联 session 事件
        └── 集成测试
```

## 6. 关键发现

### 发现 1：CC 的分区粒度是 per-input，而非 per-tool-type

CC 的 `isConcurrencySafe(input)` 接收具体输入参数，理论上同一个工具的不同调用可以有不同的并发安全性。虽然当前实现中 FileReadTool 始终返回 true、FileEditTool 始终返回 false，但接口设计预留了按输入动态判断的空间。

**对 pi-go 的建议**：接口设计应支持 `IsConcurrencySafe(params json.RawMessage) bool`，即使初期只实现静态判断。

### 发现 2：文件级串行化比全局串行更高效

Pi TS 的 `withFileMutationQueue` 证明了一个重要模式：不同文件的写操作可以安全并行（因为操作的是不同的 inode），只有同一文件的多个写操作需要串行。这比"遇到写操作就全批串行"高效得多。

**对 pi-go 的建议**：这是最值得投入的优化点。当 LLM 同时编辑 3 个不同文件时，用 per-file queue 可以 3 倍加速。

### 发现 3：批次分区 + per-file queue 是互补的两层

- **批次分区**（CC 模式）：宏观调度，决定哪些 tool call 可以同时开始
- **per-file queue**（Pi TS 模式）：微观保护，在并行执行中保护文件一致性

两者不冲突，应该组合使用：
1. 先用 `partitionToolCalls` 分出只读批次和混合批次
2. 只读批次直接并行
3. 混合批次中也允许并行执行，但 edit/write 走 per-file queue

### 发现 4：Edit 多编辑能减少 tool call 轮次

Pi TS 的 `edits[]` 设计让 LLM 在一次 edit 调用中同时修改同一文件的多处位置，减少 tool call 轮次。这对 LLM 效率和 token 消耗都有帮助。

### 发现 5：Go 实现比 TypeScript 简单

- Go 的 `sync.WaitGroup` 天然支持并行等待，无需 `Promise.all` 或 generator
- Go 的 `chan` 可以实现带缓冲的并发控制（替代 `all()` 的 concurrency cap）
- Go 的 `sync.Mutex` + `map` 可以高效实现 per-file queue
- Go 的接口隐式实现让 `ToolConcurrencyChecker` 易于扩展

### 发现 6：当前 pi-go 的并行执行缺少 panic 恢复

```go
// pi-go/internal/agent/loop.go:271-276
go func(idx int, call ai.ToolCall) {
    defer wg.Done()
    defer func() {
        if r := recover(); r != nil {
            results[idx] = ai.ToolResultMessage{...}
        }
    }()
    results[idx] = executeOneTool(ctx, a, call)
}(i, call)
```

已有 panic 恢复，但缺少并发上限控制。大量 tool call 时可能产生过多 goroutine。

---

## 附录：参考文件索引

| 项目 | 文件路径 | 关键内容 |
|------|---------|---------|
| CC | `src/services/tools/toolOrchestration.ts` | 批次分区调度核心 |
| CC | `src/Tool.ts` | `isConcurrencySafe` 接口定义、默认值 |
| CC | `src/tools/FileReadTool/FileReadTool.ts:373` | `isConcurrencySafe() => true` |
| CC | `src/tools/FileEditTool/FileEditTool.ts` | 无覆盖（默认 false），readFileState 校验 |
| CC | `src/utils/generators.ts` | `all()` 带并发上限的 generator 合并 |
| Pi TS | `packages/agent/src/agent-loop.ts:373-506` | 并行/串行执行 + prepare-then-execute 模式 |
| Pi TS | `packages/agent/src/types.ts:377-384` | `executionMode?: ToolExecutionMode` |
| Pi TS | `packages/coding-agent/src/core/tools/file-mutation-queue.ts` | per-file 串行化队列 |
| Pi TS | `packages/coding-agent/src/core/tools/edit.ts:30-51` | `edits[]` 多编辑 schema |
| Codex | `codex-rs/core/src/client.rs:473` | `parallel_tool_calls` API 参数 |
| Codex | `codex-rs/codex-mcp/src/tools.rs:33` | `supports_parallel_tool_calls` 标记 |
| pi-go | `internal/agent/tool.go` | 当前 Tool 接口 + ToolWithMode |
| pi-go | `internal/agent/loop.go:243-305` | 当前 executeToolCalls 实现 |
| pi-go | `internal/tools/read.go` | ReadTool 实现 |
| pi-go | `internal/tools/edit.go` | EditTool 实现（单编辑） |
