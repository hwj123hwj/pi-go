# 通用 Agent 框架的可扩展性分析

> 核心观点：pi-go 已经搭建了一个通用的 Agent 底座（ai 层 + agent 运行时 + 会话管理），通过替换工具集和系统提示，可以快速构建不同领域的 Agent 应用。

## 1. 当前已实现的架构

```
┌──────────────────────────────────────┐
│  application layer（待实现）          │
│  coding-agent / browser-agent / ...  │
├──────────────────────────────────────┤
│  agent runtime（已实现）              │
│  循环 / 状态机 / Tool 调度 / 事件    │
├──────────────────────────────────────┤
│  ai layer（已实现）                   │
│  统一 LLM API / 多 Provider / 流式   │
├──────────────────────────────────────┤
│  session layer（已实现）              │
│  树状 JSONL 存储 / 分支 / leaf 指针  │
├──────────────────────────────────────┤
│  server layer（已实现）               │
│  HTTP REST + SSE 流式接口            │
└──────────────────────────────────────┘
```

### 各层对应包路径

| 层 | 包路径 | 职责 |
|---|---|---|
| ai 抽象层 | `internal/ai/` | 统一多 Provider LLM 流式 API、Message 类型、EventStream 协议 |
| agent 运行时 | `internal/agent/` | Agent 循环、Tool 系统接口、状态机（Idle/Running/Waiting/Error）、事件订阅、busy 检测 |
| 会话管理 | `internal/session/` | 树状 JSONL 存储、分支（MoveTo）、leaf 指针追踪 |
| 上下文压缩 | `internal/compaction/` | 长对话的上下文窗口管理 |
| HTTP 接口 | `internal/server/` | REST 端点 + SSE 流式推送 |

## 2. 为什么底座是通用的

底座的每一层都做到了 **领域无关**：

- **ai 层**：不知道上层在做什么，只负责"发消息给 LLM，拿回响应"
- **agent 运行时**：不知道具体有哪些工具，只负责"循环调 LLM → 拿到 tool calls → 执行 → 回传结果 → 再调 LLM"
- **session 层**：不知道存的是什么内容，只负责"树状结构追加、分支、持久化"
- **server 层**：不知道业务语义，只负责"HTTP 请求 → agent.Prompt() → HTTP 响应"

这种分层意味着：**换一个领域，只需要换应用层，底座一行不改。**

## 3. 不同 Agent 的差异点只有三样

### 3.1 工具集（Tool）

每种 Agent 操纵的外部世界不同：

```
coding-agent   → 文件系统（read/write/edit/bash/grep/find/ls）
browser-agent  → 浏览器（navigate/click/screenshot/fill_form/extract）
research-agent → 互联网（web_search/crawl/extract/summarize）
data-agent     → 数据库（query/insert/update/export/visualize）
```

但它们都满足同一个接口：

```go
// internal/agent/ 中的 Tool 接口
type Tool interface {
    Name() string
    Description() string
    Parameters() ai.ToolDefinition  // JSON Schema
    Validate(raw json.RawMessage) (json.RawMessage, error)
    Execute(ctx context.Context, params json.RawMessage) (ToolResult, error)
}
```

### 3.2 系统提示（System Prompt）

决定了 Agent 的"人格"和行为准则：

```go
// coding
system := "你是一个编码助手，帮助用户编写、修改、调试代码..."

// browser
system := "你是一个浏览器操作助手，通过工具与网页交互完成任务..."

// research
system := "你是一个研究助手，帮助用户搜索、收集和分析信息..."
```

### 3.3 压缩策略（Compaction）

不同场景关注的上下文重点不同：

- coding：保留关键代码片段，压缩重复文件内容
- browser：保留关键页面状态，压缩中间操作过程
- research：保留核心结论，压缩原始搜索结果

通过 `SummarizeFunc` 可插拔：

```go
type SummarizeFunc func(ctx context.Context, messages []ai.Message) (string, error)
```

## 4. 组装方式

`agent.Options` 天然支持组合：

```go
type Options struct {
    Model              ai.Model
    Registry           *providers.Registry
    System             string                      // ← 换 prompt
    Tools              []Tool                      // ← 换工具集
    MaxTurns           int
    Session            *session.Session
    CompactionSettings compaction.Settings
    SummarizeFunc      compaction.SummarizeFunc    // ← 换压缩策略
}
```

构建不同 Agent 就是不同的 `New()` 调用：

```go
// ─── coding-agent ───
codingAgent := agent.New(agent.Options{
    Model:    ai.Model{Provider: "anthropic", Name: "claude-sonnet-4-20250514"},
    Registry: reg,
    System:   "你是一个编码助手...",
    Tools:    []agent.Tool{&ReadTool{}, &WriteTool{}, &EditTool{}, &BashTool{}},
})

// ─── browser-agent ───
browserAgent := agent.New(agent.Options{
    Model:    ai.Model{Provider: "anthropic", Name: "claude-sonnet-4-20250514"},
    Registry: reg,
    System:   "你是一个浏览器操作助手...",
    Tools:    []agent.Tool{&NavigateTool{}, &ClickTool{}, &ScreenshotTool{}},
})

// ─── 两个 agent 共享同一套运行时逻辑 ───
```

底层循环、状态机、会话管理、流式输出——全部复用，零修改。

## 5. 具体扩展示例：browser-agent

如果要造一个 browser-agent，需要做的事情：

### 5.1 实现工具

```go
// internal/browser-tools/navigate.go
type NavigateTool struct{}

func (t *NavigateTool) Name() string { return "navigate" }
func (t *NavigateTool) Description() string { return "导航到指定 URL" }
func (t *NavigateTool) Parameters() ai.ToolDefinition {
    return ai.ToolDefinition{
        Name: "navigate",
        Description: "导航到指定 URL",
        Parameters: /* JSON Schema: { url: string } */,
    }
}
func (t *NavigateTool) Execute(ctx context.Context, params json.RawMessage) (agent.ToolResult, error) {
    // 调用 Playwright / Chrome DevTools Protocol /chromedp 等
    // 实际执行页面导航
    return agent.ToolResult{Content: "已导航到 ..."}, nil
}
```

类似地实现 `ClickTool`、`ScreenshotTool`、`FillFormTool`、`ExtractTextTool` 等。

### 5.2 编写 System Prompt

```
你是一个浏览器操作助手。你可以通过以下工具与网页交互：
- navigate: 导航到 URL
- click: 点击页面元素
- screenshot: 截取当前页面
- fill_form: 填写表单字段
- extract_text: 提取页面文本内容

操作原则：
1. 每次操作前先截图确认页面状态
2. 使用 CSS 选择器精确定位元素
3. 操作后验证结果
...
```

### 5.3 组装启动

```go
browserAgent := agent.New(agent.Options{
    Model:    model,
    Registry: registry,
    System:   browserSystemPrompt,
    Tools: []agent.Tool{
        &NavigateTool{}, &ClickTool{}, &ScreenshotTool{},
        &FillFormTool{}, &ExtractTextTool{},
    },
    MaxTurns: 20,
})

// 通过 server 暴露 HTTP 接口，完全复用现有 server 代码
srv := server.New(browserAgent)
```

## 6. 与原版 Pi 架构的对比

原版 TypeScript 项目中，`packages/agent/` 是通用运行时，`packages/coding-agent/` 是一个应用层实现。理论上任何人都可以基于 `packages/agent/` 造自己的 agent。

pi-go 的 Go 实现遵循了同样的设计哲学，并且在类型安全（Go 接口）和并发控制（mutex + ErrAgentBusy）方面做了更严格的保障。

## 7. 小结

| 概念 | 说明 |
|---|---|
| 底座 | agent-core + ai + session + server，领域无关 |
| 差异化 | 工具集 + 系统提示 + 压缩策略，三者可插拔 |
| 扩展方式 | 实现 `agent.Tool` 接口 + 写 prompt + `agent.New()` 组装 |
| 核心价值 | 一次实现运行时，多种 Agent 复用 |
