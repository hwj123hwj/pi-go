---
status: revised
author: plan-agent
created: 2026-05-27
updated: 2026-05-27
revision-note: 处理 review S1-S5, M1-M6 建议
---

# 并行工具调度：批次分区 + 文件级串行化 + Edit 多编辑

## 1. 目标

将 pi-go 的工具执行从"全量并行/全量串行"两极模式升级为"批次分区调度 + per-file 串行化"的混合并发模型，并补充 Edit 多编辑支持，使 LLM 同时发起多个读操作可安全并行、多个写操作按文件粒度串行化、一次 edit 可修改同一文件的多处位置。

## 2. 为什么现在做

**Roadmap 对齐**：PRODUCT_ROADMAP §4.4 明确列出"edit 多 edit batch"和"文件变更串行保护或 mutation queue"为 Phase 1 P0 工具增强项。当前已有 7 工具和 ToolWithMode 接口，是做并行调度的正确时机——工具体系稳定，但调度逻辑是明显的短板。

**性能瓶颈**：当前 `executeToolCalls`（`internal/agent/loop.go:243`）只有两个极端——全部并行或全部串行。实际场景中 LLM 常发出 3~5 个 read + 1~2 个 edit，全串行浪费读的并行性，全并行有写冲突风险。Pi TS 的 `withFileMutationQueue` 已证明 per-file queue 模式在 LLM 同时编辑 3 个不同文件时可获约 3 倍加速。

**竞品差距**：CC 的 `partitionToolCalls` + `isConcurrencySafe(input)` 提供了成熟的参考实现，且研究报告中已确认移植路径清晰。

## 3. 这次做什么

分三个层次，按依赖顺序实施：

### 3.1 批次分区调度（P0）

改造 `executeToolCalls` 为分区调度：

1. **新增 `ConcurrencySafeChecker` 接口**（`internal/agent/tool.go`）

   ```go
   // ConcurrencySafeChecker 是一个可选接口（与 ToolWithMode 类似）。
   // 工具可实现此接口以声明其是否可安全并发执行。
   // 未实现此接口的工具默认视为不安全（保守策略）。
   type ConcurrencySafeChecker interface {
       Tool
       IsConcurrencySafe(params json.RawMessage) bool
   }
   ```

   - 接收 `json.RawMessage` 参数，允许按输入动态判断（与 CC 的 `isConcurrencySafe(input)` 对齐）
   - 默认不实现此接口 → 等价于 `false`（保守策略）
   - Extension 工具也可以实现此接口（Go duck typing 的自然结果，无需额外注册代码）

2. **实现 `partitionToolCalls`**（`internal/agent/loop.go`）

   将 `[]ai.ToolCall` 分为有序的 `[]toolBatch`：

   ```go
   type toolBatch struct {
       safe   bool        // true=可并行，false=需串行
       calls  []ai.ToolCall
   }
   ```

   分区规则：
   - 遍历 calls，对每个 call 查找对应 tool
   - 若 tool 实现了 `ConcurrencySafeChecker` 且 `IsConcurrencySafe(params)` 返回 true → 标记为安全
   - 连续的安全 call 合入同一并行批次；不安全的 call 独占一个串行批次
   - 批次间严格按顺序执行（批次内并行/串行取决于 `safe` 标记）

3. **改造 `executeToolCalls`**（`internal/agent/loop.go`）

   ```go
   func executeToolCalls(ctx context.Context, a *Agent, calls []ai.ToolCall) ([]ai.Message, error) {
       batches := partitionToolCalls(ctx, a, calls)
       var allResults []ai.Message
       for _, batch := range batches {
           var results []ai.Message
           var err error
           if batch.safe {
               results, err = executeToolCallsParallel(ctx, a, batch.calls)
           } else {
               results, err = executeToolCallsSequential(ctx, a, batch.calls)
           }
           if err != nil {
               return allResults, err
           }
           allResults = append(allResults, results...)
       }
       return allResults, nil
   }
   ```

4. **为只读工具实现 `ConcurrencySafeChecker`**

   | 工具 | `IsConcurrencySafe` | 改动位置 |
   |------|---------------------|----------|
   | `ReadTool` | `() => true` | `internal/tools/read.go` |
   | `GrepTool` | `() => true` | `internal/tools/grep.go` |
   | `FindTool` | `() => true` | `internal/tools/find.go` |
   | `LsTool` | `() => true` | `internal/tools/ls.go` |
   | `EditTool` | 不实现（默认 false） | — |
   | `WriteTool` | 不实现（默认 false） | — |
   | `BashTool` | 不实现（默认 false） | — |

### 3.2 文件级串行化（P1）

新增 per-file mutation queue，让不同文件的写操作可并行，同一文件的写操作串行：

1. **新增 `FileMutationQueue`**（`internal/agents/coding/tools/file_mutation.go`，新文件）

   ```go
   type FileMutationQueue struct {
       mu     sync.Mutex
       queues map[string]*queueEntry  // filepath → promise chain
   }

   type queueEntry struct {
       ch chan struct{}  // 前一个操作完成后关闭
   }

   func (q *FileMutationQueue) Execute(ctx context.Context, filePath string, fn func() (ToolResult, error)) (ToolResult, error)
   ```

   - 使用 `filepath.EvalSymlinks` 做路径规范化（等价于 Pi TS 的 `realpathSync.native`）
   - 每个 normalized path 一个 FIFO channel chain
   - 等前一个操作完成 → 执行当前操作 → 释放下一个

2. **EditTool 和 WriteTool 接入 mutation queue**

   EditTool 和 WriteTool 的 mutation key 均为 `filePath`（规范化后的绝对路径，不含目录路径——即文件级粒度）。EditTool 的 `Execute` 在 read-modify-write 全过程包裹在 mutation queue 中。WriteTool 的 `Execute` 同理（创建新文件和覆盖已有文件都走 queue）。

   需要在工具构造时注入 `FileMutationQueue` 实例（通过新的 Option）。

3. **Agent 持有 FileMutationQueue 实例**

   在 `Options` 和 `Agent` 中新增 `fileMutationQueue *FileMutationQueue` 字段，由 `New()` 初始化。工具通过 `ToolWithPrepareArguments` 或构造时注入获取 queue 引用。

   **推荐方案**：在 `ListOptions`（`internal/agents/coding/tools/tools.go`）中新增 `FileMutationQueue` 字段，EditTool/WriteTool 构造时通过 Option 注入。这样 mutation queue 的生命周期由 application 层管理，不侵入 core 层的 Tool 接口。

### 3.3 Edit 多编辑支持（P1）

改造 EditTool 的 schema 和执行逻辑，支持一次调用中包含多个替换：

1. **改造 `EditParams`**

   ```go
   type EditParams struct {
       Path       string `json:"path"`
       OldString  string `json:"old_string"`   // 保留向后兼容：单编辑
       NewString  string `json:"new_string"`   // 保留向后兼容：单编辑
       ReplaceAll bool   `json:"replace_all,omitempty"`
       Edits      []EditEntry `json:"edits,omitempty"`  // 新增：多编辑
   }

   type EditEntry struct {
       OldString string `json:"old_string"`
       NewString string `json:"new_string"`
   }
   ```

2. **执行逻辑**

   - `edits[]` 与 `old_string`/`new_string` **互斥**：`Validate` 中若 `edits` 非空，返回 error 要求不能同时提供 `old_string`/`new_string`
   - 如果 `Edits` 非空 → 批量编辑模式；否则 → 单编辑模式（向后兼容）
   - 批量模式下，每个 `EditEntry` 的 `old_string` 必须在原始文件中**唯一出现**（与单编辑行为一致）。如需 `replace_all` 语义，仍应使用单编辑模式
   - 所有 `OldString` 匹配原始文件内容（非增量匹配，与 Pi TS 的 `edits[]` 设计一致）
   - 匹配失败时回滚全部，返回错误和第一个失败的位置
   - 成功时一次性写入

3. **更新工具 Description 和 Parameters schema**，告知 LLM 可用多编辑模式
4. **更新 `EditTool.PromptGuidelines()`**（`internal/tools/edit.go`），在工具指引中说明多编辑模式的用法和注意事项

## 4. 这次不做什么

| 不做 | 原因 |
|------|------|
| **并发上限（MaxConcurrentTools）** | 当前 goroutine 数量等于 tool call 数（通常 < 10），不是实际瓶颈。放在后续迭代 |
| **Read state 追踪（readFileState）** | pi-go 没有 CC 那样的 readFileState 上下文，且这个功能需要跨批次的 context 修改器支持，复杂度高、收益延迟 |
| **queuedContextModifiers** | 依赖 readFileState，当前无需求 |
| **ToolWithMode 接口废弃** | 保留向后兼容，`ConcurrencySafeChecker` 是增量接口。`ToolWithMode` 仍可用于强制串行（如 bash 的副作用场景） |
| **Bash 的并发安全判断** | Bash 可能有任意副作用，保守策略（始终不安全）是正确的 |
| **SSH 后端的 mutation queue** | 当前 SSHOperations 已有独立的文件操作，mutation queue 先覆盖 LocalOperations 路径 |

## 5. 技术方案

### 5.1 核心接口变更

**`internal/agent/tool.go`** — 新增一个可选接口：

```go
type ConcurrencySafeChecker interface {
    Tool
    IsConcurrencySafe(params json.RawMessage) bool
}
```

这是增量变更，不影响现有 `Tool`、`ToolWithMode`、`ToolWithPromptInfo` 接口。工具可选择实现，不实现则默认不安全。

### 5.2 调度流程变更

**`internal/agent/loop.go`** — 替换 `executeToolCalls` 的全量判断逻辑：

```
当前流程:
  calls → 检查是否有 sequential tool → 全并行 or 全串行

新流程:
  calls → partitionToolCalls → []batch
    batch.safe=true  → executeToolCallsParallel（已有）
    batch.safe=false → executeToolCallsSequential（已有）
    批次间顺序串联
```

已有的 `executeToolCallsParallel` 和 `executeToolCallsSequential` 函数**不需要改动**，只改调度入口。

### 5.3 FileMutationQueue 数据流

```
EditTool.Execute("foo.go", ...)
  → FileMutationQueue.Execute("foo.go", func() {
      content = ops.ReadFile("foo.go")
      newContent = applyEdits(content)
      ops.WriteFile("foo.go", newContent)
    })
```

并发场景：
- EditTool("a.go") 和 EditTool("b.go") → 并行执行（不同 queue entry）
- EditTool("a.go") 和 EditTool("a.go") → FIFO 串行（同一 queue entry）

### 5.4 Edit 多编辑的 applyEdits 逻辑

```go
func applyEdits(content string, edits []EditEntry) (string, error) {
    type match struct {
        index int    // 在 edits 中的下标
        start int    // 在 content 中的起始位置
        end   int    // 在 content 中的结束位置
    }
    var matches []match
    // 1. 校验所有 old_string 存在且唯一
    for i, e := range edits {
        idx := strings.Index(content, e.OldString)
        if idx < 0 {
            return "", fmt.Errorf("edits[%d]: old_string not found", i)
        }
        count := strings.Count(content, e.OldString)
        if count > 1 {
            return "", fmt.Errorf("edits[%d]: old_string appears %d times (must be unique)", i, count)
        }
        matches = append(matches, match{index: i, start: idx, end: idx + len(e.OldString)})
    }
    // 2. 按位置从大到小排序（从后往前）
    sort.Slice(matches, func(i, j int) bool { return matches[i].start > matches[j].start })
    // 3. 重叠检测：两个匹配区域不能有交集
    for i := 1; i < len(matches); i++ {
        if matches[i].end > matches[i-1].start {
            return "", fmt.Errorf("edits[%d] and edits[%d] have overlapping matches", matches[i].index, matches[i-1].index)
        }
    }
    // 4. 从后往前替换（保证前面的偏移不变）
    result := content
    for _, m := range matches {
        result = result[:m.start] + edits[m.index].NewString + result[m.end:]
    }
    return result, nil
}
```

从后往前替换是因为：后面的替换不影响前面文本的偏移量。这是 Pi TS 的 `edits[]` 每个都匹配原始文件的关键实现细节。重叠检测确保两个 edits 不会修改同一段文本，避免歧义。

### 5.5 关键文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/agent/tool.go` | 修改 | 新增 `ConcurrencySafeChecker` 接口 |
| `internal/agent/loop.go` | 修改 | 重写 `executeToolCalls`，新增 `partitionToolCalls`、`toolBatch` |
| `internal/agents/coding/tools/file_mutation.go` | **新建** | `FileMutationQueue` 实现 |
| `internal/tools/read.go` | 修改 | 实现 `ConcurrencySafeChecker` |
| `internal/tools/grep.go` | 修改 | 实现 `ConcurrencySafeChecker` |
| `internal/tools/find.go` | 修改 | 实现 `ConcurrencySafeChecker` |
| `internal/tools/ls.go` | 修改 | 实现 `ConcurrencySafeChecker` |
| `internal/tools/edit.go` | 修改 | 实现 `EditEntry`、`applyEdits`、接入 mutation queue |
| `internal/tools/write.go` | 修改 | 接入 mutation queue |
| `internal/agents/coding/tools/tools.go` | 修改 | `ListOptions` 新增 `FileMutationQueue`，注入 EditTool/WriteTool |

### 5.6 与 ToolWithMode 的关系

`ToolWithMode.ExecutionMode()` 是 per-tool-type 的静态声明。`ConcurrencySafeChecker.IsConcurrencySafe(params)` 是 per-call 的动态判断。两者不冲突：

- 分区调度优先使用 `ConcurrencySafeChecker`（如果工具实现了）
- 如果工具没实现 `ConcurrencySafeChecker` 但 `ToolWithMode.ExecutionMode() == Sequential`，则保守地视为不安全
- 两者都未实现 → 默认不安全（保守）

## 6. 依赖关系

| 依赖项 | 状态 | 说明 |
|--------|------|------|
| Tool 接口体系 | ✅ 已完成 | `Tool` + `ToolWithMode` + `ToolWithPromptInfo` 已稳定 |
| Tool lifecycle hooks | ✅ 已完成 | `BeforeToolCallHook` / `AfterToolCallHook` 不受影响，仍在 `executeOneTool` 内部 |
| Operations 抽象 | ✅ 已完成 | `FileOperations` 接口不变，mutation queue 是在上层的编排 |
| 7 个内置工具 | ✅ 已完成 | 改造是对现有工具的增量增强 |

**无阻塞依赖**。可立即开始。

## 7. 风险和取舍

### 风险

1. **Edit 多编辑的原子性**：多个 edits 中间某个失败时，方案是"整体失败、文件不修改"。这是正确的保守策略，但 LLM 可能需要重试。在工具描述中明确说明"所有 old_string 必须同时匹配才能成功"。

2. **partitionToolCalls 无需 validate 开销**：分区时直接将 `call.Args`（`string`）转为 `json.RawMessage` 传给 `IsConcurrencySafe`，无需 validate。对于只读工具（Read/Grep/Find/Ls），`IsConcurrencySafe` 始终返回 `true`，不依赖参数内容。参数只预留用于未来可能的动态判断。

3. **FileMutationQueue 的内存泄漏**：queue map 中的 entry 如果不清理会累积。方案是：操作完成后检查当前 entry 是否仍是最新，若是则删除（与 Pi TS 的 `withFileMutationQueue` 相同策略）。

### 取舍

1. **不引入并发上限**：当前 tool call 数通常 < 10，goroutine 开销可忽略。如果未来 LLM 开始发出 20+ tool call，再加 semaphore 限流。

2. **Edit 的 edits[] 与现有单编辑参数并存**：不破坏向后兼容。LLM 可选用任一模式。如果两者同时提供，优先使用 `edits[]`。

3. **mutation queue 不侵入 core 层 Tool 接口**：通过 application 层的构造注入实现，保持 core 层的领域无关性。如果未来非 coding-agent 的 application 也需要 mutation queue，可再考虑提升到 core 层。

## 8. 测试策略

### 8.1 现有测试影响分析

| 测试文件 | 影响 | 说明 |
|----------|------|------|
| `internal/agent/loop_test.go` | 低影响 | `echoTool` 不实现 `ConcurrencySafeChecker`，走串行批次。`TestRunLoop_ToolCallAndResponse` 只有一个 tool call，行为不变 |
| `internal/agents/coding/tools/tools_test.go` | 无影响 | `FileMutationQueue` 的零值为 `nil`，Go 的 nil check 机制保证现有测试无需改动 |

### 8.2 新增测试用例

**分区调度测试**（`internal/agent/loop_test.go` 或新建 `internal/agent/partition_test.go`）：
- 混合读写批次：验证 read calls 并入同一并行批次，edit calls 独占串行批次
- 全安全批次：全部为只读工具时只有一个并行批次
- 全不安全批次：全部为写/bash 时每个 call 独占串行批次
- ToolWithMode 兼容：实现了 `ToolWithMode.Sequential` 但未实现 `ConcurrencySafeChecker` 的工具仍走串行

**FileMutationQueue 测试**（`internal/agents/coding/tools/file_mutation_test.go`）：
- 同一文件并发写：验证 FIFO 串行执行
- 不同文件并发写：验证真正并行
- 路径规范化：symlink 路径映射到同一 queue entry

**Edit 多编辑测试**（`internal/tools/edit_test.go`）：
- 批量成功：多个 edits 全部匹配，按位置从后往前替换
- 部分失败回滚：某个 `old_string` 不匹配时整体失败，文件不变
- 重叠检测：两个 edits 匹配区域重叠时返回错误
- edits[] 与 old_string 互斥：同时提供时返回 validation error
- 唯一性检查：`old_string` 在文件中出现多次时返回错误

## 9. 完成标志

- [ ] `ConcurrencySafeChecker` 接口在 `internal/agent/tool.go` 中定义
- [ ] `partitionToolCalls` 函数实现，`executeToolCalls` 改为批次调度
- [ ] ReadTool、GrepTool、FindTool、LsTool 实现 `ConcurrencySafeChecker` 返回 `true`
- [ ] EditTool、WriteTool、BashTool 不实现该接口（默认不安全）
- [ ] `FileMutationQueue` 在 `internal/agent/file_mutation.go` 中实现
- [ ] EditTool 和 WriteTool 通过 Option 注入 mutation queue，写操作走 per-file 串行化
- [ ] EditTool 支持 `edits[]` 多编辑，所有 old_string 匹配原始文件
- [ ] 单元测试覆盖：
  - 分区调度：混合读写批次的正确分区
  - mutation queue：同一文件并发写的串行化、不同文件并行
  - Edit 多编辑：批量成功、部分失败回滚
- [ ] `go test ./...` 全部通过
