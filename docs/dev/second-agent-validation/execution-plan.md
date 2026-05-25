---
status: deferred
author: plan-agent
created: 2026-05-24
updated: 2026-05-25
depends-on:
  - dev/layering-refactor/proposal.md
  - archive/runtime-decoupling/execution-plan.md
---

# 第二个最小 Agent 验证执行文档

> 状态：**暂缓预案，不是当前主线。**
>
> 目标：在未来确有必要时，新增一个最小可运行的第二 agent，用来验证 `core / platform / application / entrypoint` 分层是否真的成立，而不是只对 `coding-agent` 特例成立。

本文档保留为预案，不建议执行 agent 立即开工。

---

## 1. 为什么现在暂缓

当前分层重构和 runtime 解耦已经完成，这意味着架构上**有能力**承接第二个 application。

但基于后续讨论，当前更清楚的判断是：

- 短期产品主线仍然是一个强的 `coding-agent`
- 很多角色差异更适合先走 `skills / prompt profile / tool filtering`
- 只有在“执行循环明显变化”或“默认行为契约严重冲突且成为一等公民产品形态”时，第二个 application 才会真正产生高价值

也就是说，这份文档保留的价值主要是：

- 作为未来架构验证预案
- 不是当前开发优先级

---

## 2. 什么时候再启用这份文档

只有在出现下面任一触发条件时，才建议重新把这份文档拉回主线：

1. 明确要做一个新的、平级的一等公民 application
2. 新能力已经不适合继续塞在 `CodingApplication` 里
3. 出现了明显不同的执行模型，例如：
   - 事件驱动
   - 多租户 bot
   - DAG / multi-agent orchestration
   - 自治 daemon
4. 或者要做一个默认行为契约明显不同的独立产品形态，例如真正独立的 browser agent

在此之前，优先继续沿着：

- `coding-agent`
- `profile`
- `skills`
- `tool filtering`

这条路推进。

---

## 3. 这份预案保留什么价值

虽然现在暂缓，但文档不删除。

保留原因：

1. 它仍然记录了“第二个 application 最小验证”应怎么做
2. 它可以作为未来检验分层是否真的站住的执行模板
3. 后面如果要做 browser agent / bot agent / orchestration agent，可以拿这里的最小实现思路做参考

---

## 4. 未来启用时的最小验证思路

如果未来重新启用，推荐仍然采用“最小第二 agent”策略，而不是直接做完整产品。

原始建议保留：

- 选择一个极小 application
- 尽量不改 `internal/agent`
- 尽量不改 `internal/ai`
- 尽量不改 `internal/runtime`
- 只靠新的 application 实现来跑通主链

文档下面原有的最小实现建议，继续保留供将来参考。

`review-lite-agent` 的职责非常简单：

- 面向代码审阅场景
- 可以读取文件、搜索、列目录
- 不允许修改文件
- 不需要 bash

所以它的最小工具集建议是：

- `read`
- `grep`
- `find`
- `ls`

不包含：

- `write`
- `edit`
- `bash`

这正好可以验证：

- application 可以定义与 `coding-agent` 不同的工具组合
- platform 不需要假设所有 agent 都有 bash/edit/write

## 3.3 prompt 风格

它应有自己的 system prompt，强调：

- 以 review / analysis 为主
- 默认输出 findings
- 不主动改代码
- 优先指出风险、回归、测试缺口

但不要把它做成复杂产品文案。

---

## 5. 推荐目录结构

建议最小结构如下：

```text
internal/agents/reviewlite/
  application.go
  prompt/
    builder.go
  tools/
    tools.go
```

如果你觉得太重，也可以更薄：

```text
internal/agents/reviewlite/
  application.go
```

在 `application.go` 里直接实现 prompt 和 tools 组装。

这次我更建议第二种。

原因：

- 目标是验证架构，不是堆目录层次
- 保持样例够小，能更快看清平台边界

---

## 6. Application 设计要求

必须实现：

- `runtime.Application`

也就是：

```go
type Application interface {
    BuildTools(opts ToolBuildOptions) []agent.Tool
    BuildPrompt(opts PromptBuildOptions) string
}
```

建议形态：

```go
type ReviewLiteApplication struct{}
```

实现要求：

### BuildTools

- 只返回 `read/grep/find/ls`
- 可以直接复用 `internal/tools` 中已有工具
- 但工具组合逻辑必须属于 `reviewlite` application，不要放回 `runtime`

### BuildPrompt

- 构造 review-lite 专属系统提示
- 不要复用 `coding-agent` 的 prompt builder
- 允许适度复用平台层 `prompt.ContextFile` 等通用输入

---

## 7. App 装配方式

这次需要让 `app.App` 能装配不同 application。

当前大概率还是默认装配 `coding.CodingApplication{}`。

这次建议最小扩展成：

1. 在 `AppOptions` 里新增一个字段，例如：

```go
Application runtime.Application
```

或：

```go
AgentKind string
```

### 推荐方案

优先推荐：

```go
Application runtime.Application
```

原因：

- 更符合当前 runtime 已经接口化的方向
- 不会把 `app` 又写死成字符串分支工厂
- 后续测试更容易注入假的 application

### 行为要求

- 如果 `Application` 为空，默认仍使用 `coding.CodingApplication{}`
- 这样保证当前主链不回归
- 第二个 agent 样例则显式注入 `reviewlite.ReviewLiteApplication{}`

---

## 8. CLI 验证方式

这次不要求让 `main.go` 做复杂多 agent 选择系统，但至少要能验证第二 agent 真能跑。

推荐二选一：

### 方案 A：先加一个临时 flag

例如：

```bash
pi-agent -agent review-lite
```

由 `main.go` 根据 flag 选择注入：

- `coding.CodingApplication{}`
- `reviewlite.ReviewLiteApplication{}`

### 方案 B：先在测试里验证，不暴露 CLI flag

也可以不先改最终 CLI，而是：

- 在 `app.New(...)` 或测试 helper 中显式传 `Application`
- 通过 runtime / app / server 测试验证它能跑

### 推荐

我更建议：

**方案 A，但保持很薄。**

原因：

- 用户能真跑一下
- 验证更直接
- 改动面也不大

---

## 8. 必须满足的验证标准

完成后必须证明以下几点：

## 8.1 不改 core

执行前后对照确认：

- 不需要改 `internal/agent` 来适配第二 agent
- 不需要改 `internal/ai` 来适配第二 agent

允许的例外：

- 如果只是测试 helper 或注释，尽量避免，但不是重点

## 8.2 runtime 基本不变

理想目标：

- `internal/runtime` 不需要为 `reviewlite` 特判

如果你发现必须在 `runtime` 里加 `if reviewlite { ... }`，说明这次实现方向错了。

## 8.3 tool 组合差异真实存在

必须证明：

- `coding-agent` 仍有 `write/edit/bash`
- `review-lite-agent` 不包含这些工具

最好有测试或运行输出直接证明。

## 8.4 prompt 差异真实存在

必须证明：

- `review-lite-agent` 的系统提示不是 `coding-agent` 那套 prompt

至少应有一个单元测试检查关键文案差异。

## 8.5 自动化通过

必须跑：

```bash
go test ./...
go vet ./...
```

---

## 9. 推荐测试清单

至少补这些测试：

1. `reviewlite.Application` 实现 `runtime.Application`
2. `BuildTools` 只返回只读工具
3. `BuildPrompt` 返回 review-lite 专属提示
4. `app.New(...)` 在显式注入 reviewlite application 时能创建 session
5. 如果加了 CLI flag，至少补一个解析/装配测试

---

## 10. 明确不要做错的几件事

1. 不要把 `review-lite` 做成 `coding-agent` 的一个 mode
2. 不要把 `review-lite` 逻辑塞回 `runtime`
3. 不要在 `internal/tools` 里加 review-lite 专属分支
4. 不要为了赶进度复制一整套 `coding-agent` 大文件
5. 不要新增大量暂时没用的抽象

这次是最小验证，不是大而全平台化。

---

## 11. 完成后的理想状态

理想上，完成后项目会更接近：

```text
internal/
  agents/
    coding/
      application.go
      ...
    reviewlite/
      application.go
```

并且：

- `app` 能选择注入哪一个 application
- `runtime` 不关心具体是哪一个
- `mode` / `server` / `session` 主链不需要知道 reviewlite 细节

---

## 12. 一句话要求

这次的目标不是“再做一个功能”，而是：

**用一个最小的第二 agent 样例，验证 `pi-go` 现在的分层已经从 `coding-agent 特例架构` 走向 `可承载多个 agent 的平台架构`。**
