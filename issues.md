# Pi-Go 代码评审问题清单

> 基于 LLM 代码评审意见，逐条对照源码核实，并参考 pi TypeScript 原版实现对比。

## 评审结论

| # | 问题 | 严重度 | 状态 | 说明 |
|---|------|--------|------|------|
| 1 | BashTool 无沙箱隔离 | 🔴 高 | ✅ 已修复 | 添加 `cmd.Dir = workspace` 限制工作目录 |
| 2 | Session O(n²) 写入 | 🔴 高 | ✅ 已修复 | 只 append 当轮新增消息，不遍历全量历史 |
| 3 | containsAny 手写字符串匹配 | 🔴 高 | ✅ 已修复 | 改用标准库 `strings.Contains` |
| 4 | Context 取消处理不完整 | 🔴 高 | ✅ 已修复 | stream 消费循环加 `select + ctx.Done()` 检查 |
| 5 | Prompt/PromptStream 代码重复 | 🟡 中 | ✅ 已修复 | 抽取共享 `processTurn` + `runAgentLoop`，两入口只定义各自的 stream consumer |
| 6 | OpenAI Provider 丢弃图片 | 🟡 中 | ✅ 已修复 | 将 image block 转换为 OpenAI `image_url` content 格式 |
| 7 | Anthropic baseURL 拼接问题 | 🟡 中 | ❌ 误报 | 构造函数已规范化尾部斜杠 |
| 8 | JSONL 全量加载到内存 | 🟡 中 | ✅ 已注释 | 添加详细注释标记为已知限制，后续优化 |
| 9 | 并行 Tool 无 panic recovery | 🟡 中 | ✅ 已修复 | 并行和顺序执行均添加 `defer recover()` |
| 10 | 缺少 README | 🟢 低 | ✅ 已修复 | 添加 README.md，包含架构图、快速开始、API 文档、配置说明 |
| 11 | loadIgnorePatterns 定义未使用 | 🟢 低 | ✅ 已修复 | `loadFromDir` 中调用 `loadIgnorePatterns`，配合 `shouldIgnoreDir` |
| 12 | Error 缺少 %w wrap | 🟢 低 | ✅ 已修复 | 关键路径的 error 加上 `%w` 包装 |
| 13 | ContextWindow 硬编码 128000 | 🟢 低 | ✅ 已修复 | 添加模型元数据表 `contextWindowForModel` |
| 14 | Server 无中间件 | 🟢 低 | ✅ 已修复 | 添加 logging/recovery/CORS 中间件 |
| 15 | Channel 无背压控制 | 🟢 低 | ✅ 已修复 | 所有 `ch <-` 写入改用 `select + ctx.Done()` |

---

## 🔴 高优先级（必须修复）

### Issue 1: BashTool 无沙箱隔离

**位置**: `internal/tools/bash.go:47-48`

```go
cmd := exec.CommandContext(cmdCtx, "sh", "-c", params.Command)
out, err := cmd.CombinedOutput()
```

**问题**: 直接在宿主执行任意命令，无路径限制、无资源限制、无用户隔离。

**TS 原版对比**: 原版 (`packages/coding-agent/src/core/bash-executor.ts`) 也没有沙箱，但会检查 CWD 是否存在、使用 `detached` 管理进程树、有 ANSI 清理和二进制输出清理。

**修复建议**:
- 设置 `cmd.Dir = workspace` 限制工作目录
- 加 `cmd.SysProcAttr` 限制（Linux 可用 namespace 隔离）
- 考虑加 allowlist 模式或 `dangerouslyDisableSandbox` 开关

---

### Issue 2: Session O(n²) 写入

**位置**: `internal/agent/loop.go:54-58` 和 `internal/agent/agent.go:206-211`

```go
// 保存新消息到 session
if a.session != nil {
    for _, msg := range history {  // ← 每轮遍历全部历史
        _ = a.session.AppendMessage(ctx, msg)
    }
}
```

**问题**: 每轮循环把整个 `history` 逐条写入 session。第 n 轮写 n 条，总计 1+2+...+n = O(n²)。

**TS 原版对比**: 原版 (`packages/agent/src/harness/session/jsonl-storage.ts`) 用 append-only 模式：
```typescript
async appendEntry(entry: SessionTreeEntry): Promise<void> {
    await this.fs.appendFile(this.filePath, `${JSON.stringify(entry)}\n`);
}
```
只追加新消息，不重写历史。

**修复建议**: 只 append 当轮新增的消息（用户消息、assistant 消息、tool result），不要遍历整个 history。

---

### Issue 3: containsAny 手写字符串匹配

**位置**: `internal/ai/retry.go:114-125`

```go
func containsAny(s string, substrs ...string) bool {
    for _, sub := range substrs {
        if len(s) >= len(sub) {
            for i := 0; i <= len(s)-len(sub); i++ {
                if s[i:i+len(sub)] == sub {
                    return true
                }
            }
        }
    }
    return false
}
```

**问题**: 手写了 O(n×m) 的字符串搜索，性能差且不如标准库优化。用 `strings.Contains` 即可，标准库内部用 Rabin-Karp 等高效算法。

**修复建议**:
```go
func containsAny(s string, substrs ...string) bool {
    for _, sub := range substrs {
        if strings.Contains(s, sub) {
            return true
        }
    }
    return false
}
```

---

### Issue 4: Context 取消处理不完整

**位置**: 
- `internal/agent/agent.go:224-233` — PromptStream 中的 stream 消费循环
- `internal/agent/loop.go:71-78` — RunLoop 中的 stream 消费循环
- `internal/agent/loop.go:199-211` — 并行 tool 执行

**问题 A**: stream 消费循环不检查 ctx 取消：
```go
for event := range stream.Events() {
    // 没有 select + ctx.Done() 检查
    switch e := event.(type) { ... }
}
```
如果 ctx 被 cancel，goroutine 继续消费 LLM stream 直到自然结束。

**问题 B**: 并行 tool 执行时 `wg.Wait()` 会一直阻塞，即使 ctx 已取消，goroutine 仍继续执行 tool。

**TS 原版对比**: 原版也未在 `for await` 循环中显式检查 AbortSignal，但 signal 会传到底层 LLM API 和 bash 进程。即两个版本有相同的设计取舍。

**修复建议**:
- stream 消费循环加 `select { case <-ctx.Done(): break; default: }`
- 并行 tool 执行考虑用 `errgroup.WithContext` 或在 goroutine 中检查 ctx

---

## 🟡 中优先级（建议修复）

### Issue 5: Prompt 和 PromptStream 代码重复

**位置**: `internal/agent/loop.go:15-131` vs `internal/agent/agent.go:191-278`

**问题**: RunLoop 和 PromptStream 的核心循环逻辑 ~80% 重复：session 恢复、消息追加、session 保存、compaction 检查、LLM 调用、assistant 处理、tool 执行、tool result 保存。

**TS 原版对比**: 原版只有一个 `runLoop` 函数，通过 `emit` 回调统一处理事件分发，没有 streaming/non-streaming 的两套代码。

**修复建议**: 抽取公共逻辑为内部方法（如 `processTurn`），让 RunLoop 和 PromptStream 只处理各自特有的事件分发。

---

### Issue 6: OpenAI Provider 丢弃图片

**位置**: `internal/ai/providers/openai.go:297-311`

```go
case ai.UserMessage:
    var text string
    for _, block := range m.Content {
        if block.Type == "text" {
            text += block.Text
        }
        // image block 被静默忽略
    }
```

**问题**: UserMessage 中的 image block 被完全丢弃，无任何提示。GPT-4o 支持图片输入，应转换 image block 为 OpenAI 格式（base64 或 URL）。

**TS 原版对比**: Anthropic provider 把 image 转为 `[Image]` 占位符。但 OpenAI 的 API 原生支持 `image_url` 类型的 content block，应该直接转换。

**修复建议**: 将 image block 转换为 OpenAI 的 `image_url` content 格式（支持 base64 和 URL 两种方式）。

---

### ~~Issue 7: Anthropic Provider baseURL 拼接~~ ❌ 误报

**位置**: `internal/ai/providers/anthropic.go:25-34`

```go
func NewAnthropicProvider(apiKey, baseURL string) *AnthropicProvider {
    if !strings.HasSuffix(baseURL, "/") {
        baseURL += "/"
    }
    // ...
}
```

**结论**: 构造函数已规范化 baseURL 尾部斜杠，拼接结果始终正确。此问题不存在。

---

### Issue 8: JSONL 全量加载到内存

**位置**: `internal/session/jsonl.go:41-62`

```go
func (s *JSONLStorage) load() error {
    // ...
    for scanner.Scan() {
        var entry Entry
        json.Unmarshal(scanner.Bytes(), &entry)
        s.byID[entry.ID] = entry  // 全部加载到 map
    }
}
```

**问题**: `load()` 将所有 entry 加载到 `byID map`，长会话内存线性增长。

**评估**: MVP 阶段可接受。后续可考虑分页加载或 LRU 缓存。

**修复建议**: 标记为已知限制，后续优化时考虑：
- 只加载最近 N 条 entry
- 或按需加载（offset-based seek）

---

### Issue 9: 并行 Tool 无 panic recovery

**位置**: `internal/agent/loop.go:199-211`

```go
go func(idx int, call ai.ToolCall) {
    defer wg.Done()
    // 缺少 defer func() { recover() }() 
    results[idx] = executeOneTool(ctx, a, call)
}(i, call)
```

**问题**: 如果 `tool.Execute()` panic，整个 agent loop 会崩溃。

**TS 原版对比**: 原版在每个 tool 执行中有 try-catch：
```typescript
try {
    const result = await prepared.tool.execute(...);
    return { result, isError: false };
} catch (error) {
    return { result: createErrorToolResult(error.message), isError: true };
}
```

**修复建议**:
```go
go func(idx int, call ai.ToolCall) {
    defer wg.Done()
    defer func() {
        if r := recover(); r != nil {
            results[idx] = ai.ToolResultMessage{
                ToolCallID: call.ID,
                Content:    fmt.Sprintf("tool panic: %v", r),
                IsError:    true,
            }
        }
    }()
    results[idx] = executeOneTool(ctx, a, call)
}(i, call)
```

---

## 🟢 建议改进

### Issue 10: 缺少 README

**问题**: 项目根目录无 README.md。

**修复建议**: 添加 README，包含项目简介、架构图、快速开始、配置说明。

---

### Issue 11: loadIgnorePatterns 定义未使用

**位置**: `internal/skill/skill.go:412-430`

```go
func loadIgnorePatterns(dir string) []string {
    // 加载 .gitignore, .ignore, .fdignore
}
```

**问题**: 函数已实现但 `loadFromDir` 中未调用，用的是硬编码的跳过逻辑：
```go
if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__"
```

**修复建议**: 在 `loadFromDir` 中调用 `loadIgnorePatterns`，替换硬编码的跳过条件。

---

### Issue 12: Error 缺少 %w wrap

**问题**: 大部分 `fmt.Errorf` 调用未使用 `%w` 包装，导致错误链断裂，`errors.Is/As` 无法使用。

**示例**: `internal/agent/agent.go:302`
```go
fmt.Errorf("provider %q not found", a.model.Provider)
// 应改为
fmt.Errorf("provider %q not found: %w", a.model.Provider, err)
```

**修复建议**: 逐步将关键路径的 error 加上 `%w` 包装。

---

### Issue 13: ContextWindow 硬编码

**位置**: `cmd/pi-agent/main.go:120`

```go
Model: ai.Model{
    ContextWindow: 128000,  // 硬编码
    MaxTokens:     4096,
}
```

**问题**: 不同模型上下文窗口不同（Claude 3.5 Sonnet 200K、GPT-4o 128K、Gemini 1M）。

**修复建议**: 在 config 中配置，或建立 model 元数据表。

---

### Issue 14: Server 无中间件

**位置**: `internal/server/server.go:51-62`

**问题**: 直接 `http.NewServeMux()` 注册路由，无 logging、recovery、CORS、auth 中间件。

**修复建议**: 加标准中间件链（可用标准库 `http.Handler` 包装模式，无需引入框架）。

---

### Issue 15: Channel 无背压控制

**位置**: `internal/agent/agent.go:132`

```go
ch := make(chan AgentStreamEvent, 64)
```

**问题**: buffer=64，如果消费者（如 HTTP SSE client）消费慢，goroutine 会阻塞在 `ch <-` 上。极端情况可能死锁。

**修复建议**: 
- 增大 buffer 或改为动态计算
- 或在写入时用 `select` 检查 ctx 取消，避免无限阻塞

---

## 与 TS 原版的关键差异

| 维度 | TS 原版 | Go 重写 | 备注 |
|------|---------|---------|------|
| Session 写入 | append-only | 全量重写 | Go 版有 O(n²) 问题 |
| Agent 循环 | 单次流式 | RunLoop + PromptStream 两套 | Go 版有代码重复 |
| Tool panic | try-catch 恢复 | 无 recovery | Go 版需加 defer recover |
| Bash 安全 | 无沙箱，有 CWD 检查 | 无沙箱，无 CWD 检查 | 两者都需要加强 |
| Stream 取消 | 未在 for-await 中检查 | 未在 for range 中检查 | 两者设计取舍相同 |

---

## 修复优先级建议

1. **第一批**（核心正确性）：Issue 2 (Session O(n²))、Issue 9 (panic recovery)、Issue 3 (containsAny)
2. **第二批**（安全性）：Issue 1 (BashTool 沙箱)、Issue 4 (context 取消)
3. **第三批**（代码质量）：Issue 5 (代码重复)、Issue 12 (error wrap)
4. **第四批**（功能完善）：Issue 6 (OpenAI 图片)、Issue 11 (ignore patterns)、Issue 13 (ContextWindow)
5. **第五批**（工程化）：Issue 10 (README)、Issue 14 (中间件)、Issue 15 (背压)、Issue 8 (JSONL 内存)
