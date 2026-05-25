---
status: done
author: plan-agent
created: 2026-05-23
updated: 2026-05-24
---

# Runtime 解耦执行文档

> 目标：让 `internal/runtime` 不再直接依赖 `internal/agents/coding`，使 `runtime` 真正成为可复用的 `Platform` 层。

本文档是给执行 agent 的直接施工说明，不是讨论稿。

---

## 1. 当前问题

虽然上一轮已经把 `coding-agent` 的大部分应用层能力抽到了：

- `internal/agents/coding/tools`
- `internal/agents/coding/commands`
- `internal/agents/coding/prompt`
- `internal/agents/coding/cli`
- `internal/agents/coding/deepv`

但当前仍有一个关键分层问题没有闭环：

- [internal/runtime/agent_session.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/runtime/agent_session.go) 还在直接 `import internal/agents/coding`

这意味着：

- `Platform -> Application` 依赖仍然存在
- `runtime` 还不是“application-agnostic”
- 如果未来要做第二个 agent，`runtime` 仍然需要改代码

这与 [LAYERING_REFACTOR_PROPOSAL.md](/Users/weijian/Desktop/develop/test/pi/pi-go/docs/LAYERING_REFACTOR_PROPOSAL.md) 的最终方向不一致。

---

## 2. 这次必须达成的目标

执行完成后，必须满足以下条件：

1. `internal/runtime` 不再直接 import `internal/agents/coding`
2. `runtime.AgentSession` 不再直接调用：
   - `coding.BuildToolList(...)`
   - `coding.BuildSystemPrompt(...)`
3. `runtime` 只依赖注入进来的 builder / factory / adapter 接口
4. `app.App` 负责选择并注入当前使用的 agent application
5. 当前 `coding-agent` 行为不变：
   - CLI 可运行
   - server 可运行
   - slash commands 正常
   - tools 正常
   - prompt 正常
6. `go test ./...` 与 `go vet ./...` 通过

---

## 3. 这次不要做的事

这次**不要**顺手做下面这些扩展：

- 不要重写 `internal/session` / `internal/sessionmgr`
- 不要改 `internal/agent` 主循环
- 不要新增第二个真实 agent 产品
- 不要大规模搬目录
- 不要改 Desktop 业务逻辑
- 不要顺手改 slashcmd 框架
- 不要在这次引入新的配置系统

重点只有一个：

**把 `runtime` 从 `coding-agent` 中解耦出来。**

---

## 4. 推荐设计

## 4.1 在 runtime 层定义注入接口

建议在 `internal/runtime` 新增一个小文件，例如：

- `internal/runtime/application.go`

定义类似这样的接口：

```go
type ToolBuilder interface {
    BuildTools(opts ToolBuildOptions) []agent.Tool
}

type PromptBuilder interface {
    BuildPrompt(opts PromptBuildOptions) string
}
```

也可以不拆成两个 interface，而是定义一个更高层的 application descriptor，例如：

```go
type Application interface {
    BuildTools(opts ToolBuildOptions) []agent.Tool
    BuildPrompt(opts PromptBuildOptions) string
}
```

两种都可以。

建议优先选：

**单个 `Application` 接口**

因为它更符合当前架构语义。

---

## 4.2 推荐 runtime.Application 接口形态

建议定义成：

```go
type Application interface {
    BuildTools(opts ToolBuildOptions) []agent.Tool
    BuildPrompt(opts PromptBuildOptions) string
}
```

其中 `opts` 要尽量表达平台层需要提供给 application 的上下文，而不是把整个 `AgentSession` 暴露出去。

例如：

```go
type ToolBuildOptions struct {
    Workspace      string
    MaxOutputLen   int
    EnableBash     bool
    BashOps        operations.BashOperations
    FileOps        operations.FileOperations
    ExtensionTools []agent.Tool
    AllowedTools   []string
    BlockedTools   []string
}

type PromptBuildOptions struct {
    CustomPrompt string
    CWD          string
    Tools        []agent.Tool
    ContextFiles []prompt.ContextFile
    Skills       []skill.Skill
}
```

注意：

- 这些 struct 应定义在 `runtime`，不是 `coding`
- 这样 `runtime` 才是接口拥有者
- `coding-agent` 只是实现者

---

## 4.3 AgentSession 改为依赖 runtime.Application

当前 `AgentSession` 里直接：

- `coding.BuildToolList(...)`
- `coding.BuildSystemPrompt(...)`

应改为：

- `s.application.BuildTools(...)`
- `s.application.BuildPrompt(...)`

建议改造点：

1. `runtime.Dependencies` 新增：

```go
Application runtime.Application
```

2. `AgentSession` 结构体持有：

```go
application Application
```

3. `NewAgentSession(...)` 从 `deps` 中接住这个依赖

4. `buildAgent(...)` / `buildToolList(...)` 全改成通过注入接口调用

---

## 4.4 app.App 负责装配 coding-agent

当前 `app.App` 是装配层，这一层最适合决定“本次跑的是哪个 application”。

建议：

- 在 `internal/app` 中创建并持有 `coding-agent` 的 application 实例
- 然后通过 `runtime.Dependencies` 注入给 `AgentSession`

也就是说，依赖方向应变成：

```text
cmd/main
  -> app
  -> runtime
  -> interface

app
  -> agents/coding
  -> runtime (interface owner)

runtime
  -> interface only

agents/coding
  -> core/platform
```

这是这次最关键的目标。

---

## 4.5 coding-agent 侧实现 runtime.Application

建议在：

- `internal/agents/coding/`

里新增一个真正的 application 对象，例如：

```go
type Application struct{}
```

实现：

```go
func (Application) BuildTools(opts runtime.ToolBuildOptions) []agent.Tool
func (Application) BuildPrompt(opts runtime.PromptBuildOptions) string
```

如果为了避免 import cycle，不能直接这么写，那就：

- 在 `runtime` 里定义更轻的 interface
- 在 `coding` 里做 adapter

总之最终效果必须是：

- `runtime` 不 import `coding`
- `coding` 可以 import `runtime` 或者与其通过中立 options package 协作

如果出现 import cycle：

- 优先把 options/interface 抽到一个更中立的小包
- 不要为了绕开 cycle 又把依赖方向倒回来

---

## 5. 推荐实施顺序

## Step 1

先在 `runtime` 定义接口和 options structs。

完成标准：

- `runtime` 新增 `Application` 抽象
- 暂时不删旧逻辑也可以，但接口文件要先落地

## Step 2

让 `agents/coding` 提供 `Application` 实现。

完成标准：

- `coding` 层有明确 application 对象
- 不只是若干 package-level helper function

## Step 3

改 `app.App` 注入 application。

完成标准：

- `runtime.Dependencies` 持有 application
- `NewSession/LoadSession` 全链路可把它传下去

## Step 4

改 `runtime.AgentSession` 通过接口调用。

完成标准：

- 删除 `runtime` 中对 `internal/agents/coding` 的直接 import
- `buildToolList` / `buildAgent` 全走注入接口

## Step 5

清理遗留 helper。

完成标准：

- 如果 `coding.NewToolOptions(...)` / `coding.NewPromptOptions(...)` 这种 wrapper 已经没有必要，可以顺手删掉
- 但前提是行为不变、代码更清晰

---

## 6. 验收标准

执行完成后，必须人工验证下面这些点：

## 6.1 分层验证

运行搜索，确认：

```bash
rg -n 'internal/agents/coding' internal/runtime
```

预期：

- **零命中**

这是本次最重要的验收条件。

## 6.2 行为验证

至少确认：

- CLI 仍能启动
- `/help`
- `/tools`
- `/model`
- `/new`

这些不出现行为回归。

## 6.3 DeepV 相关

确认：

- `app` 仍然能为 DeepV 注入 repo header provider
- `providers/deepv.go` 不重新吸收 Git 逻辑

## 6.4 自动化验证

必须跑：

```bash
go test ./...
go vet ./...
```

---

## 7. 完成后的理想状态

完成后，这个结构应更接近：

```text
internal/
  agent/
  ai/
  compaction/

  runtime/
    application.go
    agent_session.go
    session_registry.go

  prompt/
    context.go

  slashcmd/
    context.go
    registry.go

  agents/
    coding/
      application.go
      tools/
      commands/
      prompt/
      cli/
      deepv/
```

不要求目录名字完全一样，但**依赖方向必须接近这个结构**。

---

## 8. 这一步完成后，下一步才值得做什么

只有在 `runtime` 解耦完成后，才值得继续做：

1. 一个最小的第二 agent 验证样例
2. 更彻底的目录迁移
3. 更细的 application 生命周期抽象

否则继续往前做，`coding-agent` 仍然会被平台层“反向知道”。

---

## 9. 一句话要求

这次不是“再整理一下 coding 目录”，而是：

**把 `runtime` 从 `coding-agent` 中真正解耦出来，让 `runtime` 成为平台层，而 `coding-agent` 成为被装配进去的 application。**
