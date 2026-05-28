---
status: reviewed
author: review-agent
created: 2026-05-27
updated: 2026-05-27
reviewer: review-agent
review-status: addressed
depends-on:
  - docs/dev/parallel/proposal.md
---

# Review：并行工具调度 Proposal

## 1. 总体评价

**approve（附建议）**

Proposal 对当前代码状态描述准确，技术方案可行且与现有架构对齐。三个层次的划分（批次分区 → 文件级串行化 → Edit 多编辑）依赖关系清晰，增量实施策略合理。以下按维度展开详细评审。

---

## 2. 准确性验证

逐项交叉验证 proposal 中的关键声明：

| # | Proposal 声明 | 验证结果 | 详情 |
|---|---|---|---|
| 1 | `executeToolCalls` 在 `internal/agent/loop.go:243` | ✅ 准确 | `loop.go:243` 正是 `executeToolCalls` 函数定义处 |
| 2 | 当前只有"全并行或全串行"两极模式 | ✅ 准确 | `loop.go:249-262`：遍历 calls 检查 `ToolWithMode`，有 sequential 就全串行，否则全并行 |
| 3 | `Tool` + `ToolWithMode` + `ToolWithPromptInfo` 接口已稳定 | ✅ 准确 | `tool.go:8-47` 三个接口定义清晰，无其他扩展接口 |
| 4 | `ToolWithMode` 只在 `loop.go:252` 一处使用 | ✅ 准确 | `grep -rn "ToolWithMode"` 确认仅 `loop.go` 的类型断言处使用 |
| 5 | `BeforeToolCallHook` / `AfterToolCallHook` 在 `executeOneTool` 内部调用 | ✅ 准确 | `loop.go:344-383`，hooks 在 `executeOneTool` 内执行，不受调度改造影响 |
| 6 | `Operations` 抽象在 `internal/operations/` | ✅ 准确 | `interface.go` 定义了 `BashOperations` 和 `FileOperations`，有 `local.go` 和 `ssh.go` 实现 |
| 7 | 7 个内置工具在 `internal/tools/` | ✅ 准确 | `bash.go`, `edit.go`, `find.go`, `grep.go`, `ls.go`, `read.go`, `write.go` |
| 8 | `ListOptions` 在 `internal/agents/coding/tools/tools.go` | ✅ 准确 | `tools.go:10-19` 定义了 `ListOptions`，`tools.go:31` 定义了 `BuildList` |
| 9 | PRODUCT_ROADMAP §4.4 列出"edit 多 edit batch"和"mutation queue" | ✅ 准确 | ROADMAP 第 207 行明确列出这两个条目，标注为 Phase 1 工具增强 |
| 10 | `executeToolCallsParallel` 和 `executeToolCallsSequential` 已存在 | ✅ 准确 | `loop.go:265-305`，两个函数实现完整 |
| 11 | `ToolWithPrepareArguments` 接口在 `tool_lifecycle.go` | ✅ 准确 | `tool_lifecycle.go:70-73` |
| 12 | `Agent` struct 有 `tools map[string]Tool` | ✅ 准确 | `agent.go:44` |
| 13 | EditTool 使用 `ReadFile` → `Replace` → `WriteFile` 的 read-modify-write 模式 | ✅ 准确 | `edit.go:100-150` |
| 14 | EditTool 支持 `replace_all` 模式 | ✅ 准确 | `edit.go:127-136` |
| 15 | 工具通过 Option 模式构造（functional options） | ✅ 准确 | 所有工具均使用 `XxxToolOption func(*XxxTool)` 模式 |

**准确性结论**：Proposal 中所有关键事实性声明与源码一致，无发现错误描述。

---

## 3. 发现的问题

### 🔴 Blockers（必须修复）

无。

### 🟡 Strong Suggestions（强烈建议）

#### S1: `ConcurrencySafeChecker` 接口嵌入 `Tool` 可能导致歧义

**问题**：Proposal 中 `ConcurrencySafeChecker` 嵌入了 `Tool` 接口：

```go
type ConcurrencySafeChecker interface {
    Tool
    IsConcurrencySafe(params json.RawMessage) bool
}
```

这与项目现有的可选接口模式（`ToolWithMode`、`ToolWithPromptInfo`、`ToolWithPrepareArguments`）一致——它们都嵌入了 `Tool`。但要注意：分区调度代码中的类型断言 `tool.(ConcurrencySafeChecker)` 要求工具必须同时满足 `Tool` 的全部方法。由于所有工具都实现了 `Tool`，这在实践中不是问题，但建议在注释中明确说明这是**可选接口**（与 `ToolWithMode` 的 doc comment 风格保持一致）。

**建议**：保持嵌入设计（与现有模式一致），但在接口注释中加上 "可选接口" 标注。

#### S2: `partitionToolCalls` 中 validate 的双重调用问题

**问题**：Proposal 风险分析（§7.2）提到 `partitionToolCalls` 需要 validate 参数来判断 `IsConcurrencySafe`，而 `executeOneTool` 内部也会再次 validate。但 proposal 提到"分区时需要 validate 每个 tool call 的参数"——仔细看 proposal 的伪代码，`partitionToolCalls` 实际上**没有调用 validate**，它直接把 `call.Args`（`string` 类型）转成 `json.RawMessage` 传给 `IsConcurrencySafe`。

但这里有个类型问题：`ai.ToolCall.Args` 是 `string` 类型（`types.go:67`），而 `IsConcurrencySafe` 接收 `json.RawMessage`。两者可以直接转换，但需要明确。

**更重要的是**：`IsConcurrencySafe` 对于 Read/Grep/Find/Ls 这些只读工具总是返回 `true`，不依赖参数内容。因此实际实现中可以跳过 validate，直接调用 `IsConcurrencySafe`（参数只用于未来可能的动态判断）。

**建议**：
1. 在 `partitionToolCalls` 中直接将 `call.Args` 转为 `json.RawMessage` 传给 `IsConcurrencySafe`，无需 validate
2. 在 proposal 的风险分析 §7.2 中更新描述，反映这个简化

#### S3: `FileMutationQueue` 应放在 `internal/agent/` 还是别处？

**问题**：Proposal 将 `FileMutationQueue` 放在 `internal/agent/file_mutation.go`。但从分层角度看，mutation queue 是**编码场景**特有的需求（保护文件写入），而非 agent 运行时的通用能力。Agent core 层（`internal/agent/`）的设计目标是领域无关的。

Proposal 的 §5.6 取舍分析中也提到"mutation queue 不侵入 core 层 Tool 接口"，但把 `FileMutationQueue` 的类型定义放在了 `internal/agent/`——这存在一定的矛盾。

**建议**：将 `FileMutationQueue` 定义放在 `internal/agents/coding/` 或 `internal/agents/coding/tools/` 下（与 `ListOptions` 同层），而非 `internal/agent/`。这样更符合 "core 层不感知具体应用场景" 的分层原则。如果放在 `internal/agent/`，至少应该以可选的、generic 的方式命名和设计（例如 `ResourceLock` 或 `KeyedMutex`），避免与 "file" 这个具体概念绑定。

#### S4: `applyEdits` 的"从后往前替换"策略需要更精确的描述

**问题**：Proposal 的 `applyEdits` 伪代码说"按在文件中出现的位置从后往前排序"，但实际上需要：
1. 先找到每个 `old_string` 在原始文件中的位置（offset）
2. 按 offset 从大到小排序
3. 从后往前替换

这里有个微妙的问题：如果两个 `old_string` 的匹配区域有**重叠**怎么办？例如 `old_string_1 = "abc"` 和 `old_string_2 = "bc"` 在同一位置重叠。

**建议**：
1. 在 `applyEdits` 中增加重叠检测：两个匹配区域不能重叠
2. 重叠时返回明确错误："edits have overlapping matches"
3. 在 proposal 中补充这个边界条件的处理策略

#### S5: `edits[]` 与 `old_string`/`new_string` 同时提供时的行为

**问题**：Proposal §7 取舍 3 说"如果两者同时提供，优先使用 `edits[]`"。但 `Validate` 方法需要明确处理这种情况，且 LLM 可能混淆两种模式。

**建议**：在 `Validate` 中，如果 `edits` 非空，清空 `old_string` 和 `new_string`（或返回 validation error 要求不同时提供），并在工具 Description 中明确说明互斥关系。

### 🟢 Nice-to-haves（可考虑）

#### N1: 批次间错误处理的粒度

当前 proposal 中，任何批次出错就直接返回已执行的结果。考虑增加一个 `continueOnError` 选项（或者至少在注释中预留这个可能性），以便未来支持"某个批次失败后继续执行后续批次"。

#### N2: 分区调度的可观测性

建议在分区完成后 emit 一个事件（如 `EventToolBatchStart`），包含批次信息和是否并行。这对 UI 展示和调试都很有帮助。当前事件体系只有 `EventToolExecutionStart/End`，无法表达"这批工具是并行执行的"。

#### N3: `FileMutationQueue` 的超时和死锁防护

`Execute` 方法使用 channel 等待前一个操作完成。如果前一个操作因为 context cancel 长期阻塞，后续操作也会被卡住。建议增加 per-operation 的 timeout 或 context propagation。

---

## 4. 遗漏检查

### 4.1 已正确排除的项

| 项 | 评估 |
|---|---|
| MaxConcurrentTools | ✅ 合理排除，当前规模不需要 |
| readFileState 追踪 | ✅ 合理排除，复杂度高 |
| SSH 后端 mutation queue | ✅ 合理排除，先覆盖本地路径 |
| ToolWithMode 废弃 | ✅ 合理保留，向后兼容 |

### 4.2 可能遗漏的项

#### M1: `internal/agent/loop_test.go` 的测试改造

现有 `loop_test.go` 使用 `echoTool`（不实现 `ToolWithMode`），在当前逻辑下走并行路径。改造后，`echoTool` 也不实现 `ConcurrencySafeChecker`，会走串行批次——但这对单工具调用的测试没有影响。**但如果测试中有多个 tool call 的场景**，行为可能变化。

当前测试中 `TestRunLoop_ToolCallAndResponse` 只有一个 tool call，不受影响。但建议在 proposal 中补充"测试改造"章节。

#### M2: `internal/agents/coding/tools/tools_test.go` 的潜在影响

`tools_test.go` 需要检查是否需要为新的 `ListOptions` 字段（`FileMutationQueue`）更新测试。由于 Go 的零值机制（`nil *FileMutationQueue`），现有测试不需要立即改动，但建议在 proposal 中注明。

#### M3: `replace_all` 与 `edits[]` 的交互

当前 `EditParams` 有 `ReplaceAll` 字段。如果 `edits[]` 的某个 entry 的 `old_string` 在文件中出现多次，是否应该报错？Proposal 没有明确说明 `edits[]` 模式下是否仍然要求每个 `old_string` 唯一。

**建议**：明确 `edits[]` 模式下，每个 `EditEntry` 的 `old_string` 必须在原始文件中唯一出现（与当前单编辑的行为一致）。如果需要 replace_all 语义，仍应使用单编辑模式。

#### M4: WriteTool 的文件创建场景

WriteTool 可以创建新文件（`MkdirAll` + `WriteFile`）。两个并发 WriteTool 创建不同文件是安全的，但如果创建同一文件（父目录相同），需要 mutation queue 保护。Proposal 说"WriteTool 的 Execute 同理接入 mutation queue"，这是正确的，但建议在 proposal 中明确 WriteTool 的 mutation key 是 `filePath`（与 EditTool 一致），而非包含目录路径。

#### M5: Extension 工具的兼容性

`ListOptions.ExtensionTools` 允许外部注入自定义工具。这些工具不实现 `ConcurrencySafeChecker`，默认视为不安全——这是正确的保守策略。但 proposal 没有提到 extension 工具是否可以自行实现 `ConcurrencySafeChecker`。

**建议**：在 proposal 中注明，extension 工具也可以实现 `ConcurrencySafeChecker`，这是接口设计的自然结果（Go 的 duck typing），无需额外代码。

#### M6: Prompt info 更新

`prompt_info.go` 为 EditTool 提供了 guidelines。增加多编辑支持后，需要更新 `EditTool.PromptGuidelines()` 来告知 LLM 可用多编辑模式。Proposal §3.3 第 3 点提到了"更新工具 Description 和 Parameters schema"，但没有提到 `PromptGuidelines()`。

---

## 5. 修改建议

### 对 proposal 的修改建议（按优先级）

| # | 建议 | 对应问题 |
|---|---|---|
| 1 | 考虑将 `FileMutationQueue` 从 `internal/agent/` 移到 `internal/agents/coding/tools/`，或重命名为 generic 名称如 `KeyedMutex` | S3 |
| 2 | 在 `applyEdits` 描述中增加重叠检测逻辑 | S4 |
| 3 | 明确 `Validate` 中 `edits[]` 与 `old_string`/`new_string` 互斥（或优先）的处理 | S5 |
| 4 | 明确 `edits[]` 模式下每个 `old_string` 仍需唯一匹配 | M3 |
| 5 | 补充"测试改造"章节：现有测试影响分析 + 新增测试用例清单 | M1 |
| 6 | 更新 §7.2 关于 validate 开销的描述（实际不需要 validate） | S2 |
| 7 | 补充 `PromptGuidelines()` 更新 | M6 |
| 8 | 注明 extension 工具天然兼容 `ConcurrencySafeChecker` | M5 |

### 对技术方案的补充建议

1. **`partitionToolCalls` 伪代码建议增加 `ToolWithMode` 兼容逻辑**：如果工具没有实现 `ConcurrencySafeChecker`，但实现了 `ToolWithMode` 且 `ExecutionMode() == Sequential`，应视为不安全。Proposal §5.6 已经描述了这个逻辑，但建议在 `partitionToolCalls` 的伪代码中体现出来。

2. **`FileMutationQueue` 建议**：
   - 在 `Execute` 方法中传播 `ctx`（用于 cancellation）
   - 考虑使用 `sync.Map` 替代 `map[string]*queueEntry` + `sync.Mutex`（因为 queue key 可能被并发访问）
   - 考虑实现 `io.Closer` 接口用于清理（虽然当前 map entry 有自动清理逻辑）

3. **`applyEdits` 建议实现骨架**：

```go
func applyEdits(content string, edits []EditEntry) (string, error) {
    type match struct {
        index int    // 在 edits 中的下标
        start int    // 在 content 中的起始位置
        end   int    // 在 content 中的结束位置
    }
    var matches []match
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
    // 按位置排序（从大到小）
    sort.Slice(matches, func(i, j int) bool { return matches[i].start > matches[j].start })
    // 检测重叠
    for i := 1; i < len(matches); i++ {
        if matches[i].end > matches[i-1].start {
            return "", fmt.Errorf("edits[%d] and edits[%d] have overlapping matches", matches[i].index, matches[i-1].index)
        }
    }
    // 从后往前替换
    result := content
    for _, m := range matches {
        result = result[:m.start] + edits[m.index].NewString + result[m.end:]
    }
    return result, nil
}
```

---

## 6. 总结

Proposal 质量很高，对代码现状的把握准确，技术方案与现有架构风格一致（可选接口 + functional options + 分层隔离）。三个层次的增量实施策略合理，依赖关系清晰。

主要改进空间：
1. `FileMutationQueue` 的位置（建议移出 core agent 层）
2. Edit 多编辑的边界条件（重叠检测、与 replace_all 的关系）
3. 测试改造章节的补充

以上建议均为非阻塞性改进，不影响 proposal 的整体可行性。
