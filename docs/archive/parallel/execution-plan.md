---
status: approved
author: exec-agent
created: 2026-05-27
updated: 2026-05-27
depends-on:
  - docs/dev/parallel/proposal.md
  - docs/dev/parallel/review.md
---

# 并行工具调度：执行计划

## 1. 整体架构

### 1.1 改造后的数据流

```
LLM Response: [read("a.go"), grep("pattern", "b.go"), edit("a.go"), edit("c.go")]
     │
     ▼
partitionToolCalls (loop.go)
     │
     ├─ Batch 0: safe=true  → [read, grep]        → executeToolCallsParallel
     ├─ Batch 1: safe=false → [edit("a.go")]       → executeToolCallsSequential
     └─ Batch 2: safe=false → [edit("c.go")]       → executeToolCallsSequential
                                                     (若 edit 注入了 FileMutationQueue，
                                                      batch 1 和 batch 2 的不同文件可并行)
```

### 1.2 组件关系

```
┌──────────────────────────────────────────────────────────┐
│  loop.go                                                 │
│  ┌─────────────┐    ┌──────────────┐                     │
│  │ partition   │───→│ executeTool  │                     │
│  │ ToolCalls   │    │ CallsParallel│ (existing, unchanged)│
│  └─────────────┘    └──────────────┘                     │
│       │              ┌──────────────┐                     │
│       └─────────────→│ executeTool  │                     │
│                      │ CallsSeq     │ (existing, unchanged)│
│                      └──────┬───────┘                     │
│                             │                             │
│                             ▼                             │
│                      executeOneTool (existing, unchanged)  │
│                             │                             │
├─────────────────────────────┼─────────────────────────────┤
│  tool.go                    ▼                             │
│  ┌──────────────────────────────────┐                     │
│  │ ConcurrencySafeChecker (new)     │◄── ReadTool, GrepTool│
│  │ IsConcurrencySafe(params) bool   │    FindTool, LsTool  │
│  └──────────────────────────────────┘                     │
├──────────────────────────────────────────────────────────┤
│  agents/coding/tools/                                   │
│  ┌──────────────────────┐                                │
│  │ FileMutationQueue    │ (new file)                     │
│  │ per-file FIFO queue  │◄── EditTool, WriteTool inject  │
│  └──────────────────────┘                                │
│  ┌──────────────────────┐                                │
│  │ tools.go ListOptions │──+ FileMutationQueue * (new)   │
│  └──────────────────────┘                                │
├──────────────────────────────────────────────────────────┤
│  tools/edit.go                                           │
│  ┌──────────────────────┐                                │
│  │ EditEntry (new)      │                                │
│  │ EditParams.Edits []  │ (new field)                    │
│  │ applyEdits() (new)   │                                │
│  └──────────────────────┘                                │
└──────────────────────────────────────────────────────────┘
```

### 1.3 Review 建议采纳清单

| ID | 建议 | 采纳方式 |
|----|------|----------|
| S1 | `ConcurrencySafeChecker` 注释标注"可选接口" | ✅ 接口 doc comment 加 "可选接口" |
| S2 | `partitionToolCalls` 无需 validate，直接 `call.Args` → `json.RawMessage` | ✅ 采纳 |
| S3 | `FileMutationQueue` 移出 core agent 层，放 `internal/agents/coding/tools/` | ✅ 采纳 |
| S4 | `applyEdits` 增加重叠检测 | ✅ 采纳 |
| S5 | `edits[]` 与 `old_string`/`new_string` 互斥（Validate 中返回 error） | ✅ 采纳（返回 error，非优先） |
| M1 | 补充测试改造章节 | ✅ 本文档 §4 |
| M2 | `tools_test.go` 无需改动（nil 零值） | ✅ 确认无影响 |
| M3 | `edits[]` 模式下每个 `old_string` 必须唯一 | ✅ 在 `applyEdits` 中检查 |
| M4 | WriteTool 的 mutation key 是 `filePath` | ✅ 明确 |
| M5 | Extension 工具天然兼容 `ConcurrencySafeChecker` | ✅ 注释说明 |
| M6 | 更新 `PromptGuidelines()` | ✅ Step 8 |
| N1 | `continueOnError` 预留 | ✅ 注释中预留 |
| N2 | 批次间可观测事件 | ✅ Step 3 中 emit `EventToolBatchStart` |

---

## 2. 这次不做什么

| 不做 | 原因 |
|------|------|
| MaxConcurrentTools semaphore | tool call 数 < 10，goroutine 开销可忽略 |
| readFileState / queuedContextModifiers | 需跨批次 context 修改器，复杂度高 |
| ToolWithMode 废弃 | 保留向后兼容，分区调度同时检查两者 |
| Bash 并发安全判断 | 副作用不可预测，保守策略正确 |
| SSH 后端 mutation queue | 先覆盖 LocalOperations |
| `edits[]` 中单个 entry 的 `replace_all` 语义 | 批量模式下每个 old_string 必须唯一，如需 replace_all 用单编辑 |
| `FileMutationQueue` io.Closer / timeout | 当前 map entry 自动清理，context cancel 可传播 |

---

## 3. 实施步骤

### Step 1: 新增 `ConcurrencySafeChecker` 接口

**文件**: `internal/agent/tool.go`

**变更**: 在文件末尾（`ToolWithPromptInfo` 接口之后）新增：

```go
// ConcurrencySafeChecker 是一个可选接口（与 ToolWithMode 类似）。
// 工具可实现此接口以声明其是否可安全并发执行。
// 未实现此接口的工具默认视为不安全（保守策略）。
//
// 分区调度器 (partitionToolCalls) 在分批次时查询此接口。
// 若工具同时实现了 ToolWithMode 且 ExecutionMode() == Sequential，
// 即使 IsConcurrencySafe 返回 true，也会被保守地视为不安全。
//
// Extension 工具也可以实现此接口（Go duck typing 的自然结果，无需额外注册代码）。
type ConcurrencySafeChecker interface {
	Tool
	IsConcurrencySafe(params json.RawMessage) bool
}
```

**验证**:
- `go build ./internal/agent/` 编译通过
- 接口嵌入 `Tool`，与 `ToolWithMode`、`ToolWithPromptInfo` 风格一致
- doc comment 包含"可选接口"标注（S1）、Extension 兼容说明（M5）

---

### Step 2: 为只读工具实现 `ConcurrencySafeChecker`

**文件**: `internal/tools/read.go`

**变更**: 在文件末尾新增方法：

```go
// IsConcurrencySafe implements agent.ConcurrencySafeChecker.
// ReadTool is always safe to execute concurrently.
func (t *ReadTool) IsConcurrencySafe(params json.RawMessage) bool {
	return true
}
```

**文件**: `internal/tools/grep.go`

**变更**: 在文件末尾新增：

```go
func (t *GrepTool) IsConcurrencySafe(params json.RawMessage) bool {
	return true
}
```

**文件**: `internal/tools/find.go`

**变更**: 在文件末尾新增：

```go
func (t *FindTool) IsConcurrencySafe(params json.RawMessage) bool {
	return true
}
```

**文件**: `internal/tools/ls.go`

**变更**: 在文件末尾新增：

```go
func (t *LsTool) IsConcurrencySafe(params json.RawMessage) bool {
	return true
}
```

**不改动**: `edit.go`、`write.go`、`bash.go` — 不实现该接口，默认不安全。

**验证**:
- `go build ./internal/tools/` 编译通过
- 编译时类型检查确认 `ReadTool`/`GrepTool`/`FindTool`/`LsTool` 满足 `ConcurrencySafeChecker` 接口
- 添加编译期接口断言测试（Step 10）：

```go
var _ agent.ConcurrencySafeChecker = (*ReadTool)(nil)
var _ agent.ConcurrencySafeChecker = (*GrepTool)(nil)
var _ agent.ConcurrencySafeChecker = (*FindTool)(nil)
var _ agent.ConcurrencySafeChecker = (*LsTool)(nil)
```

---

### Step 3: 实现 `partitionToolCalls` + 重写 `executeToolCalls` + 批次事件

**文件**: `internal/agent/loop.go`

**变更 A**: 新增 `toolBatch` 类型和 `EventToolBatchStart` 事件

在文件顶部的 import 后、`consumeStreamFunc` 之前新增：

```go
// toolBatch 表示一组可一起执行的工具调用。
type toolBatch struct {
	safe  bool          // true = 批次内可并行执行
	calls []ai.ToolCall // 批次内的工具调用
}
```

**文件**: `internal/agent/event.go`

**变更**: 在 `EventCompactionFailed` 之后新增：

```go
// EventToolBatchStart 在每个工具批次开始执行时发出。
// 用于 UI 展示和调试，帮助区分并行批次和串行批次。
type EventToolBatchStart struct {
	BatchIndex int    // 批次序号（从 0 开始）
	Safe       bool   // true = 并行批次, false = 串行批次
	ToolNames  []string // 批次内工具名称列表
}

func (EventToolBatchStart) agentEventMarker() {}
```

**文件**: `internal/agent/loop.go`

**变更 B**: 新增 `partitionToolCalls` 函数（在 `executeToolCalls` 之前）

```go
// partitionToolCalls 将工具调用按并发安全性分区为有序的批次。
//
// 分区规则：
//  1. 查找每个 call 对应的 tool
//  2. 若 tool 实现了 ConcurrencySafeChecker 且 IsConcurrencySafe(params) == true → safe
//     若 tool 实现了 ToolWithMode 且 ExecutionMode() == Sequential → unsafe（保守策略）
//     两者都不满足 → unsafe（保守策略）
//  3. 连续的 safe call 合入同一并行批次
//  4. 每个 unsafe call 独占一个串行批次
//
// 注：此处直接将 call.Args（string）转为 json.RawMessage 传给 IsConcurrencySafe，
// 无需先 validate。对只读工具而言 IsConcurrencySafe 始终返回 true，不依赖参数内容。
// 参数参数预留用于未来可能的动态判断。
func partitionToolCalls(ctx context.Context, a *Agent, calls []ai.ToolCall) []toolBatch {
	var batches []toolBatch
	var currentBatch *toolBatch

	for _, call := range calls {
		safe := isToolCallSafe(a, call)

		if safe {
			if currentBatch == nil || !currentBatch.safe {
				// 开始新的并行批次
				batches = append(batches, toolBatch{safe: true})
				currentBatch = &batches[len(batches)-1]
			}
			currentBatch.calls = append(currentBatch.calls, call)
		} else {
			// 不安全：独占串行批次
			batches = append(batches, toolBatch{safe: false, calls: []ai.ToolCall{call}})
			currentBatch = nil
		}
	}

	return batches
}

// isToolCallSafe 判断单个 tool call 是否可安全并发执行。
func isToolCallSafe(a *Agent, call ai.ToolCall) bool {
	tool, ok := a.tools[call.Name]
	if !ok {
		return false // 未知工具，保守策略
	}

	// 优先级 1: ToolWithMode.Sequential → 保守不安全
	if tm, ok := tool.(ToolWithMode); ok && tm.ExecutionMode() == ExecutionModeSequential {
		return false
	}

	// 优先级 2: ConcurrencySafeChecker
	if csc, ok := tool.(ConcurrencySafeChecker); ok {
		return csc.IsConcurrencySafe(json.RawMessage(call.Args))
	}

	// 默认不安全
	return false
}
```

**变更 C**: 重写 `executeToolCalls` 函数（替换当前 `loop.go:243-263`）

```go
func executeToolCalls(ctx context.Context, a *Agent, calls []ai.ToolCall) ([]ai.Message, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	batches := partitionToolCalls(ctx, a, calls)
	var allResults []ai.Message

	for i, batch := range batches {
		// 批次开始事件
		names := make([]string, 0, len(batch.calls))
		for _, c := range batch.calls {
			names = append(names, c.Name)
		}
		a.emit(ctx, EventToolBatchStart{
			BatchIndex: i,
			Safe:       batch.safe,
			ToolNames:  names,
		})

		var results []ai.Message
		var err error

		if batch.safe {
			results, err = executeToolCallsParallel(ctx, a, batch.calls)
		} else {
			results, err = executeToolCallsSequential(ctx, a, batch.calls)
		}

		if err != nil {
			// 返回已执行的结果（支持未来 continueOnError 扩展）
			return allResults, err
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}
```

**验证**:
- `go build ./internal/agent/` 编译通过
- 现有测试 `TestRunLoop_ToolCallAndResponse` 通过（单个 echoTool call，不实现 `ConcurrencySafeChecker`，走串行批次，行为不变）
- 现有测试 `TestRunLoop_SimpleTextResponse`、`TestRunLoop_MaxTurns`、`TestRunLoop_ToolNotFound` 不受影响
- `partitionToolCalls` 的纯函数测试（Step 10）

---

### Step 4: 新增 `FileMutationQueue`

**文件**: `internal/agents/coding/tools/file_mutation.go`（**新建**）

```go
package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/earendil-works/pi-go/internal/agent"
)

// FileMutationQueue 提供 per-file 的 FIFO 串行化队列。
// 用于保护同一文件的并发写操作，不同文件的写操作可并行。
//
// 使用方式：
//
//	result, err := queue.Execute(ctx, filePath, func() (agent.ToolResult, error) {
//	    // read-modify-write 操作
//	})
//
// Mutation key 是 filePath（规范化后的绝对路径，文件级粒度）。
type FileMutationQueue struct {
	mu     sync.Mutex
	queues map[string]*queueEntry
}

type queueEntry struct {
	ch chan struct{} // 前一个操作完成后关闭
}

// NewFileMutationQueue 创建一个新的 FileMutationQueue。
func NewFileMutationQueue() *FileMutationQueue {
	return &FileMutationQueue{
		queues: make(map[string]*queueEntry),
	}
}

// Execute 在指定文件的 mutation queue 中执行 fn。
// 如果同一文件有正在执行的操作，会等待其完成后再执行。
// 不同文件的调用可并行。
//
// filePath 会被 filepath.EvalSymlinks 规范化（如果文件已存在），
// 以确保不同路径指向同一文件时共享同一个 queue entry。
func (q *FileMutationQueue) Execute(ctx context.Context, filePath string, fn func() (agent.ToolResult, error)) (agent.ToolResult, error) {
	key, err := q.normalizePath(filePath)
	if err != nil {
		// 规范化失败（如文件不存在但即将创建）——用原始路径作为 key
		key = filepath.Clean(filePath)
	}

	q.mu.Lock()
	prev, exists := q.queues[key]
	if !exists {
		q.queues[key] = &queueEntry{ch: make(chan struct{})}
	}
	q.mu.Unlock()

	// 等待前一个操作完成
	if exists {
		select {
		case <-prev.ch:
		case <-ctx.Done():
			return agent.ToolResult{IsError: true, Content: ctx.Err().Error()}, ctx.Err()
		}
	}

	// 创建当前操作的 channel
	q.mu.Lock()
	entry := &queueEntry{ch: make(chan struct{})}
	q.queues[key] = entry
	q.mu.Unlock()

	// 执行操作
	result, err := fn()

	// 释放等待的下一个操作
	close(entry.ch)

	// 清理：如果当前 entry 仍是最新且没有后续等待者，删除 map entry
	q.mu.Lock()
	if current, ok := q.queues[key]; ok && current == entry {
		delete(q.queues, key)
	}
	q.mu.Unlock()

	return result, err
}

// normalizePath 尝试规范化路径。如果文件不存在则返回原始路径和错误。
func (q *FileMutationQueue) normalizePath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path, fmt.Errorf("normalize path: %w", err)
	}
	return resolved, nil
}
```

**验证**:
- `go build ./internal/agents/coding/tools/` 编译通过
- 单元测试（Step 10）：同一文件并发写串行化、不同文件并行、路径规范化

---

### Step 5: `ListOptions` 新增 `FileMutationQueue` + 注入 EditTool/WriteTool

**文件**: `internal/agents/coding/tools/tools.go`

**变更 A**: `ListOptions` 结构体新增字段（在 `BlockedTools` 之后）：

```go
type ListOptions struct {
	Workspace         string
	MaxOutputLen      int
	EnableBash        bool
	BashOps           operations.BashOperations
	FileOps           operations.FileOperations
	ExtensionTools    []agent.Tool
	AllowedTools      []string
	BlockedTools      []string
	FileMutationQueue *FileMutationQueue // 可选：per-file 写操作串行化
}
```

**变更 B**: `BuildList` 函数中，EditTool 和 WriteTool 构造时注入 mutation queue

将当前的 EditTool 构造：

```go
basetools.NewEditTool(
    basetools.WithEditWorkspace(opts.Workspace),
    basetools.WithEditOperations(opts.FileOps),
),
```

替换为：

```go
basetools.NewEditTool(
    basetools.WithEditWorkspace(opts.Workspace),
    basetools.WithEditOperations(opts.FileOps),
    basetools.WithEditMutationQueue(opts.FileMutationQueue),
),
```

同理 WriteTool 构造：

```go
basetools.NewWriteTool(
    basetools.WithWriteWorkspace(opts.Workspace),
    basetools.WithWriteOperations(opts.FileOps),
    basetools.WithWriteMutationQueue(opts.FileMutationQueue),
),
```

**验证**:
- `go build ./internal/agents/coding/tools/` 编译通过
- `TestBuildList_FiltersAndExtensions` 通过（`FileMutationQueue` 零值为 nil，无影响）
- `TestBaseToolNames` 无影响

---

### Step 6: EditTool 接入 mutation queue + 新增 Option

**文件**: `internal/tools/edit.go`

**变更 A**: 结构体新增字段

```go
type EditTool struct {
	workspace      string
	ops            operations.FileOperations
	mutationQueue  MutationQueue // 可选：per-file 串行化
}
```

**变更 B**: 新增接口（放在 EditParams 结构体之前）

```go
// MutationQueue 是 EditTool/WriteTool 的 per-file 串行化抽象。
// 定义在此处以避免循环依赖（agent 包不应依赖 coding 包）。
type MutationQueue interface {
	Execute(ctx context.Context, filePath string, fn func() (agent.ToolResult, error)) (agent.ToolResult, error)
}
```

**变更 C**: 新增 Option 函数

```go
// WithEditMutationQueue sets the per-file mutation queue for serialized writes.
func WithEditMutationQueue(q MutationQueue) EditToolOption {
	return func(t *EditTool) { t.mutationQueue = q }
}
```

**变更 D**: 修改 `Execute` 方法

在当前的 `Execute` 中，将 read-modify-write 逻辑包裹在 mutation queue 中。关键改动：将 `Execute` 拆分为内部方法 `doExecute`（包含当前所有逻辑），然后 `Execute` 作为外层 wrapper：

```go
func (t *EditTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	// 若有 mutation queue，先解析 path 以确定 queue key
	if t.mutationQueue != nil {
		var params EditParams
		if err := json.Unmarshal(raw, &params); err != nil {
			return agent.ToolResult{IsError: true}, err
		}
		cleanPath := ResolvePath(t.workspace, params.Path)
		return t.mutationQueue.Execute(ctx, cleanPath, func() (agent.ToolResult, error) {
			return t.doExecute(ctx, raw, onUpdate)
		})
	}
	return t.doExecute(ctx, raw, onUpdate)
}
```

将当前 `Execute` 的全部内容（从 `var params EditParams` 到函数结尾）移到 `doExecute` 方法中，保持逻辑完全不变：

```go
func (t *EditTool) doExecute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	// ... 原有 Execute 的全部代码 ...
}
```

**验证**:
- `go build ./internal/tools/` 编译通过
- 现有 `TestEditTool_Replace`、`TestEditTool_NotFound`、`TestEditTool_NotUnique`、`TestEditTool_CreateNewFile`、`TestEditTool_Validate` 全部通过（`mutationQueue` 零值为 nil，走 else 分支，行为不变）

---

### Step 7: WriteTool 接入 mutation queue + 新增 Option

**文件**: `internal/tools/write.go`

**变更 A**: 结构体新增字段

```go
type WriteTool struct {
	workspace     string
	ops           operations.FileOperations
	mutationQueue MutationQueue // 可选：per-file 串行化
}
```

**变更 B**: 新增 Option 函数

```go
// WithWriteMutationQueue sets the per-file mutation queue for serialized writes.
func WithWriteMutationQueue(q MutationQueue) WriteToolOption {
	return func(t *WriteTool) { t.mutationQueue = q }
}
```

**变更 C**: 修改 `Execute` 方法

同理 EditTool 的模式，将 `MkdirAll` + `WriteFile` 包裹在 mutation queue 中。Mutation key 是 `filePath`（文件级粒度）：

```go
func (t *WriteTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	if t.mutationQueue != nil {
		var params WriteParams
		if err := json.Unmarshal(raw, &params); err != nil {
			return agent.ToolResult{IsError: true}, err
		}
		cleanPath := ResolvePath(t.workspace, params.Path)
		return t.mutationQueue.Execute(ctx, cleanPath, func() (agent.ToolResult, error) {
			return t.doExecute(ctx, raw, onUpdate)
		})
	}
	return t.doExecute(ctx, raw, onUpdate)
}

func (t *WriteTool) doExecute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	// ... 原有 Execute 的全部代码 ...
}
```

注意：`write.go` 的 import 需要新增 `tools` 包内部的引用（`MutationQueue` 在 `edit.go` 中定义，同属 `tools` 包，无需额外 import）。

**验证**:
- `go build ./internal/tools/` 编译通过
- WriteTool 无现有测试文件，无需回归

---

### Step 8: EditTool 多编辑支持

**文件**: `internal/tools/edit.go`

**变更 A**: `EditParams` 新增 `Edits` 字段

```go
type EditParams struct {
	Path       string      `json:"path"`
	OldString  string      `json:"old_string"`
	NewString  string      `json:"new_string"`
	ReplaceAll bool        `json:"replace_all,omitempty"`
	Edits      []EditEntry `json:"edits,omitempty"` // 多编辑模式：与 old_string/new_string 互斥
}

// EditEntry 表示多编辑模式中的一个替换操作。
type EditEntry struct {
	OldString string `json:"old_string"` // 在原始文件中必须唯一出现
	NewString string `json:"new_string"`
}
```

**变更 B**: `Validate` 方法增加互斥校验

在 `params.Path == ""` 校验之后新增：

```go
if len(params.Edits) > 0 {
    if params.OldString != "" || params.NewString != "" {
        return nil, fmt.Errorf("cannot provide both edits[] and old_string/new_string; use one or the other")
    }
    for i, e := range params.Edits {
        if e.OldString == "" {
            return nil, fmt.Errorf("edits[%d]: old_string is required", i)
        }
    }
}
```

**变更 C**: `doExecute` 方法增加批量编辑分支

在 `content := string(data)` 之后、`Check old_string exists` 之前，插入批量编辑分支：

```go
// 批量编辑模式
if len(params.Edits) > 0 {
    newContent, err := applyEdits(content, params.Edits)
    if err != nil {
        return agent.ToolResult{IsError: true, Content: err.Error()}, err
    }
    if err := t.ops.WriteFile(ctx, cleanPath, []byte(newContent), 0o644); err != nil {
        return agent.ToolResult{IsError: true}, err
    }
    return agent.ToolResult{
        Content: fmt.Sprintf("edited %s (%d edits applied)", cleanPath, len(params.Edits)),
    }, nil
}
```

**变更 D**: 新增 `applyEdits` 函数

```go
// applyEdits 对 content 应用多个编辑操作。
// 所有 old_string 匹配原始文件内容（非增量匹配）。
// 匹配失败或重叠时返回错误，成功时返回替换后的完整内容。
func applyEdits(content string, edits []EditEntry) (string, error) {
	type match struct {
		index int // 在 edits 中的下标
		start int // 在 content 中的起始位置
		end   int // 在 content 中的结束位置
	}

	var matches []match

	// 1. 校验所有 old_string 存在且唯一
	for i, e := range edits {
		idx := strings.Index(content, e.OldString)
		if idx < 0 {
			return "", fmt.Errorf("edits[%d]: old_string not found in file", i)
		}
		count := strings.Count(content, e.OldString)
		if count > 1 {
			return "", fmt.Errorf("edits[%d]: old_string appears %d times (must be unique)", i, count)
		}
		matches = append(matches, match{index: i, start: idx, end: idx + len(e.OldString)})
	}

	// 2. 按位置从大到小排序（从后往前替换）
	sort.Slice(matches, func(i, j int) bool { return matches[i].start > matches[j].start })

	// 3. 重叠检测：两个匹配区域不能有交集
	for i := 1; i < len(matches); i++ {
		if matches[i].end > matches[i-1].start {
			return "", fmt.Errorf("edits[%d] and edits[%d] have overlapping matches", matches[i].index, matches[i-1].index)
		}
	}

	// 4. 从后往前替换（后面的替换不影响前面文本的偏移量）
	result := content
	for _, m := range matches {
		result = result[:m.start] + edits[m.index].NewString + result[m.end:]
	}
	return result, nil
}
```

注意：`edit.go` 的 import 需要新增 `"sort"`。

**变更 E**: 更新 `Description` 和 `Parameters`

```go
func (t *EditTool) Description() string {
	return "Perform exact string replacements in files. Supports single replacement (old_string must be unique), replace_all mode, and multi-edit mode (edits array for multiple replacements in one call)."
}

func (t *EditTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":        map[string]any{"type": "string", "description": "Absolute path to the file to edit."},
			"old_string":  map[string]any{"type": "string", "description": "The text to replace (single-edit mode). Must match exactly, including whitespace and indentation."},
			"new_string":  map[string]any{"type": "string", "description": "The text to replace with (single-edit mode)."},
			"replace_all": map[string]any{"type": "boolean", "description": "Replace all occurrences (single-edit mode, default false)."},
			"edits": map[string]any{
				"type":        "array",
				"description": "Multi-edit mode: array of replacements to apply in one call. Each old_string must be unique in the original file. Mutually exclusive with old_string/new_string.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"old_string": map[string]any{"type": "string", "description": "The text to replace. Must be unique in the file."},
						"new_string": map[string]any{"type": "string", "description": "The replacement text."},
					},
					"required": []string{"old_string", "new_string"},
				},
			},
		},
		"required": []string{"path"},
	}
}
```

**验证**:
- `go build ./internal/tools/` 编译通过
- 现有单编辑测试全部通过（`params.Edits` 为 nil，走原有逻辑）

---

### Step 9: 更新 `EditTool.PromptGuidelines()`

**文件**: `internal/tools/prompt_info.go`

**变更**: 替换 `EditTool` 的 `PromptGuidelines` 实现

```go
func (t *EditTool) PromptGuidelines() []string {
	return []string{
		"Use edit for targeted modifications; it is safer and more precise than write for changes",
		"Always read a file before editing to ensure old_string matches exactly",
		"The old_string must be unique in the file; if not, include more surrounding context",
		"Multi-edit mode: use the edits[] array to apply multiple replacements in one call — more efficient than separate edit calls for the same file",
		"In multi-edit mode, each old_string must be unique in the original file and edits must not overlap",
		"Do not use both edits[] and old_string/new_string in the same call — they are mutually exclusive",
	}
}
```

**验证**:
- `go build ./internal/tools/` 编译通过
- `PromptGuidelines` 返回 6 条，新增多编辑相关说明

---

### Step 10: 单元测试

#### 10A: 分区调度测试

**文件**: `internal/agent/partition_test.go`（**新建**）

```go
package agent

import (
	"encoding/json"
	"testing"

	"github.com/earendil-works/pi-go/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// safeMockTool 是一个标记为并发安全的 mock 工具。
type safeMockTool struct {
	echoTool // embed echoTool to satisfy Tool
}

func (t *safeMockTool) IsConcurrencySafe(params json.RawMessage) bool { return true }

// sequentialMockTool 是一个标记为顺序执行的 mock 工具。
type sequentialMockTool struct {
	echoTool
}

func (t *sequentialMockTool) ExecutionMode() ExecutionMode { return ExecutionModeSequential }

func TestPartitionToolCalls_MixedSafeAndUnsafe(t *testing.T) {
	ag := &Agent{
		tools: map[string]Tool{
			"safe_read":   &safeMockTool{},
			"unsafe_edit": &echoTool{},
			"safe_grep":   &safeMockTool{},
		},
	}
	calls := []ai.ToolCall{
		{ID: "c1", Name: "safe_read", Args: `{}`},
		{ID: "c2", Name: "safe_grep", Args: `{}`},
		{ID: "c3", Name: "unsafe_edit", Args: `{}`},
	}
	batches := partitionToolCalls(nil, ag, calls)
	require.Len(t, batches, 2)
	assert.True(t, batches[0].safe)
	assert.Len(t, batches[0].calls, 2)
	assert.False(t, batches[1].safe)
	assert.Len(t, batches[1].calls, 1)
}

func TestPartitionToolCalls_AllSafe(t *testing.T) {
	ag := &Agent{
		tools: map[string]Tool{
			"read": &safeMockTool{},
			"grep": &safeMockTool{},
		},
	}
	calls := []ai.ToolCall{
		{ID: "c1", Name: "read", Args: `{}`},
		{ID: "c2", Name: "grep", Args: `{}`},
	}
	batches := partitionToolCalls(nil, ag, calls)
	require.Len(t, batches, 1)
	assert.True(t, batches[0].safe)
	assert.Len(t, batches[0].calls, 2)
}

func TestPartitionToolCalls_AllUnsafe(t *testing.T) {
	ag := &Agent{
		tools: map[string]Tool{
			"edit":  &echoTool{},
			"write": &echoTool{},
		},
	}
	calls := []ai.ToolCall{
		{ID: "c1", Name: "edit", Args: `{}`},
		{ID: "c2", Name: "write", Args: `{}`},
	}
	batches := partitionToolCalls(nil, ag, calls)
	require.Len(t, batches, 2)
	for _, b := range batches {
		assert.False(t, b.safe)
		assert.Len(t, b.calls, 1)
	}
}

func TestPartitionToolCalls_ToolWithModeSequentialOverridesSafe(t *testing.T) {
	// 实现 ConcurrencySafeChecker 返回 true 但也实现 ToolWithMode.Sequential
	type conflictingTool struct {
		echoTool
	}
	conflictingTool.IsConcurrencySafe = func(json.RawMessage) bool { return true }
	// 注意：Go 不支持在类型定义上直接设函数字段。
	// 用一个实现了两个接口的具体类型。
	type bothTool struct{}
	func (bothTool) Name() string        { return "both" }
	func (bothTool) Description() string { return "" }
	func (bothTool) Parameters() map[string]any { return nil }
	func (bothTool) Validate(raw json.RawMessage) (json.RawMessage, error) { return raw, nil }
	func (bothTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(PartialResult)) (ToolResult, error) {
		return ToolResult{}, nil
	}
	func (bothTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
	func (bothTool) ExecutionMode() ExecutionMode             { return ExecutionModeSequential }

	ag := &Agent{tools: map[string]Tool{"both": bothTool{}}}
	calls := []ai.ToolCall{{ID: "c1", Name: "both", Args: `{}`}}
	batches := partitionToolCalls(nil, ag, calls)
	require.Len(t, batches, 1)
	assert.False(t, batches[0].safe, "ToolWithMode.Sequential should override ConcurrencySafeChecker")
}
```

#### 10B: FileMutationQueue 测试

**文件**: `internal/agents/coding/tools/file_mutation_test.go`（**新建**）

```go
package tools

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileMutationQueue_SameFileSerialized(t *testing.T) {
	q := NewFileMutationQueue()
	ctx := context.Background()

	var order []int
	var mu sync.Mutex
	counter := int32(0)

	// 启动 3 个对同一文件的操作
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := q.Execute(ctx, "/tmp/same_file.go", func() (agent.ToolResult, error) {
				n := atomic.AddInt32(&counter, 1)
				mu.Lock()
				order = append(order, int(n))
				mu.Unlock()
				time.Sleep(10 * time.Millisecond) // 模拟耗时操作
				return agent.ToolResult{}, nil
			})
			require.NoError(t, err)
		}(i)
	}
	wg.Wait()

	// 验证串行执行：order 应该是 [1, 2, 3]
	assert.Equal(t, []int{1, 2, 3}, order)
}

func TestFileMutationQueue_DifferentFilesParallel(t *testing.T) {
	q := NewFileMutationQueue()
	ctx := context.Background()

	var running atomic.Int32
	var maxRunning atomic.Int32

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := q.Execute(ctx, "/tmp/file_"+string(rune('a'+idx))+".go", func() (agent.ToolResult, error) {
				r := running.Add(1)
				for {
					cur := maxRunning.Load()
					if r <= cur || maxRunning.CompareAndSwap(cur, r) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				running.Add(-1)
				return agent.ToolResult{}, nil
			})
			require.NoError(t, err)
		}(i)
	}
	wg.Wait()

	// 不同文件应该并行执行过
	assert.Greater(t, maxRunning.Load(), int32(1), "different files should run in parallel")
}

func TestFileMutationQueue_ContextCancel(t *testing.T) {
	q := NewFileMutationQueue()
	ctx, cancel := context.WithCancel(context.Background())

	// 第一个操作阻塞
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		q.Execute(ctx, "/tmp/blocking.go", func() (agent.ToolResult, error) {
			time.Sleep(200 * time.Millisecond)
			return agent.ToolResult{}, nil
		})
	}()

	// 等第一个操作开始
	time.Sleep(20 * time.Millisecond)

	// 第二个操作，然后 cancel context
	cancel()
	_, err := q.Execute(ctx, "/tmp/blocking.go", func() (agent.ToolResult, error) {
		return agent.ToolResult{}, nil
	})
	assert.Error(t, err)
	wg.Wait()
}
```

#### 10C: Edit 多编辑测试

**文件**: `internal/tools/edit_test.go`（追加测试函数）

```go
func TestEditTool_MultiEdit_Success(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\nline5\n"), 0o644))

	tool := NewEditTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","edits":[{"old_string":"line2","new_string":"REPLACED2"},{"old_string":"line4","new_string":"REPLACED4"}]}`))
	require.NoError(t, err)
	result, err := tool.Execute(ctx, validated, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "2 edits applied")

	data, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "line1\nREPLACED2\nline3\nREPLACED4\nline5\n", string(data))
}

func TestEditTool_MultiEdit_NotFound(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("line1\nline2\n"), 0o644))

	tool := NewEditTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","edits":[{"old_string":"line1","new_string":"x"},{"old_string":"NOT_EXIST","new_string":"y"}]}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "edits[1]: old_string not found")

	// 文件应未被修改（回滚）
	data, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "line1\nline2\n", string(data))
}

func TestEditTool_MultiEdit_Overlapping(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("abc\n"), 0o644))

	tool := NewEditTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","edits":[{"old_string":"abc","new_string":"x"},{"old_string":"bc","new_string":"y"}]}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "overlapping")
}

func TestEditTool_MultiEdit_NotUnique(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("aaa\nbbb\naaa\n"), 0o644))

	tool := NewEditTool()
	ctx := context.Background()
	validated, err := tool.Validate([]byte(`{"path":"` + testFile + `","edits":[{"old_string":"aaa","new_string":"x"}]}`))
	require.NoError(t, err)
	_, err = tool.Execute(ctx, validated, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be unique")
}

func TestEditTool_MultiEdit_MutuallyExclusive(t *testing.T) {
	tool := NewEditTool()
	_, err := tool.Validate([]byte(`{"path":"/tmp/test","old_string":"a","new_string":"b","edits":[{"old_string":"c","new_string":"d"}]}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestApplyEdits(t *testing.T) {
	// 纯函数测试，无需文件系统
	content := "hello world\nfoo bar\nbaz qux\n"

	result, err := applyEdits(content, []EditEntry{
		{OldString: "hello", NewString: "hi"},
		{OldString: "baz qux", NewString: "BAZ QUX"},
	})
	require.NoError(t, err)
	assert.Equal(t, "hi world\nfoo bar\nBAZ QUX\n", result)
}
```

#### 10D: ConcurrencySafeChecker 编译期断言

**文件**: `internal/tools/concurrency_safety_test.go`（**新建**）

```go
package tools

import (
	"github.com/earendil-works/pi-go/internal/agent"
)

// 编译期断言：只读工具实现 ConcurrencySafeChecker
var _ agent.ConcurrencySafeChecker = (*ReadTool)(nil)
var _ agent.ConcurrencySafeChecker = (*GrepTool)(nil)
var _ agent.ConcurrencySafeChecker = (*FindTool)(nil)
var _ agent.ConcurrencySafeChecker = (*LsTool)(nil)
```

**验证**:
- `go test ./internal/agent/... ./internal/tools/... ./internal/agents/coding/tools/...` 全部通过
- `go vet ./...` 无警告

---

### Step 11: 在 CodingApplication 中创建和注入 FileMutationQueue

**文件**: `internal/agents/coding/application.go`

**变更**: 在 `BuildTools` 或等效方法中，创建 `FileMutationQueue` 实例并传入 `ListOptions`

需要确认 `application.go` 中如何调用 `tools.BuildList`。在调用处新增：

```go
mutationQueue := tools.NewFileMutationQueue()
// ... 在 ListOptions 中 ...
FileMutationQueue: mutationQueue,
```

**验证**: `go build ./internal/agents/coding/...` 编译通过

---

## 4. 测试策略

### 4.1 现有测试影响分析

| 测试文件 | 影响 | 原因 |
|----------|------|------|
| `internal/agent/loop_test.go` | **低** | `echoTool` 不实现 `ConcurrencySafeChecker`，走串行批次。单 tool call 测试行为不变 |
| `internal/tools/edit_test.go` | **低** | `mutationQueue` 零值为 nil，走原有分支。新增测试函数不影响已有 |
| `internal/agents/coding/tools/tools_test.go` | **无** | `FileMutationQueue` 零值为 nil，`filterTools` 逻辑不变 |

### 4.2 新增测试清单

| 测试 | 文件 | 覆盖场景 |
|------|------|----------|
| `TestPartitionToolCalls_MixedSafeAndUnsafe` | `partition_test.go` | 混合读写 → 2 个批次 |
| `TestPartitionToolCalls_AllSafe` | `partition_test.go` | 全只读 → 1 个并行批次 |
| `TestPartitionToolCalls_AllUnsafe` | `partition_test.go` | 全不安全 → N 个串行批次 |
| `TestPartitionToolCalls_ToolWithModeSequentialOverridesSafe` | `partition_test.go` | ToolWithMode 优先级高于 ConcurrencySafeChecker |
| `TestFileMutationQueue_SameFileSerialized` | `file_mutation_test.go` | 同一文件 FIFO 串行 |
| `TestFileMutationQueue_DifferentFilesParallel` | `file_mutation_test.go` | 不同文件并行 |
| `TestFileMutationQueue_ContextCancel` | `file_mutation_test.go` | context cancel 不死锁 |
| `TestEditTool_MultiEdit_Success` | `edit_test.go` | 多编辑成功 |
| `TestEditTool_MultiEdit_NotFound` | `edit_test.go` | 部分失败回滚 |
| `TestEditTool_MultiEdit_Overlapping` | `edit_test.go` | 重叠检测 |
| `TestEditTool_MultiEdit_NotUnique` | `edit_test.go` | 唯一性检查 |
| `TestEditTool_MultiEdit_MutuallyExclusive` | `edit_test.go` | 互斥校验 |
| `TestApplyEdits` | `edit_test.go` | `applyEdits` 纯函数 |
| 编译期断言 | `concurrency_safety_test.go` | 接口实现检查 |

---

## 5. 迁移注意

### 5.1 向后兼容

所有变更是**增量式**的，无破坏性变更：

- `ConcurrencySafeChecker` 是新增可选接口，不影响现有工具
- `ToolWithMode` 保留，分区调度同时检查两者
- `EditParams` 的 `old_string`/`new_string` 字段保留，单编辑模式完全兼容
- `ListOptions.FileMutationQueue` 是可选字段，零值为 nil 时行为不变
- `FileMutationQueue` 注入通过 Option 模式，现有构造代码无需修改

### 5.2 `ai.ToolCall.Args` 类型注意

`ai.ToolCall.Args` 是 `string` 类型（`internal/ai/types.go:67`），传给 `IsConcurrencySafe(json.RawMessage)` 时需要 `json.RawMessage(call.Args)` 转换。这在 Go 中是安全的——`json.RawMessage` 就是 `[]byte` 的别名，而 Go 允许 `string` → `[]byte` 的显式转换。在 `partitionToolCalls` 中已完成此转换。

### 5.3 MutationQueue 接口定义位置

`MutationQueue` 接口定义在 `internal/tools/edit.go` 中（而非 `internal/agent/`），避免 coding 层对 tools 层的反向依赖。`FileMutationQueue`（`internal/agents/coding/tools/`）实现此接口。EditTool 和 WriteTool 在同包内直接引用，无需 import。

### 5.4 EventToolBatchStart 对 UI 的影响

新增的 `EventToolBatchStart` 事件是 additive 的。现有 UI 代码若不处理此事件，会被 `agent.go:164-165` 的 `default: return` 忽略，不影响运行。UI 层可后续选择消费此事件以展示批次信息。
