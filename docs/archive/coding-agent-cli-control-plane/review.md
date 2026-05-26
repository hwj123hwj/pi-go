---
status: done
author: review-agent
created: 2026-05-24
updated: 2026-05-25
reviewer: review-agent
review-status: approved
depends-on:
  - dev/coding-agent-cli-control-plane/execution-plan.md
---

# Coding Agent CLI Control Plane — 审核文档

> 审核对象：`dev/coding-agent-cli-control-plane/execution-plan.md`
> 审核方法：逐节对照源码验证问题描述的准确性、设计方案的可执行性、验收标准的完备性。

---

## 1. 总体评价

**审核结论：通过，建议执行前补充 3 处细节。**

执行计划整体质量高。目标聚焦、边界清晰、代码定位准确，执行 agent 拿着可以开工。但在 `/new` session 回写机制和 profile 切换生效范围两处，方案描述还不够具体，执行前需要补齐。

| 维度 | 评价 |
|------|------|
| 目标聚焦度 | ✅ 好。只做 CLI 控制面，不扩新领域，不过度抽象 |
| 边界意识 | ✅ 好。明确 profile 属于 coding application，不下沉 runtime |
| 执行顺序 | ✅ 合理。先补接口 → 再迁 `/new` → 再补命令 → 再补 profile → 最后测试 |
| 代码定位精度 | ✅ 好。准确指向 `slashcmd/context.go`、`interactive.go`、`builtins.go` 等关键文件 |
| 风险预判 | ⚠️ 中等。识别了 `/new` 特判问题，但低估了回写机制的设计复杂度 |
| 可执行性 | ✅ 好。改动面可控，不涉及 runtime 核心改动 |

---

## 2. 问题描述准确性审核

执行计划第 4 节列出的 4 个问题，逐条对照源码验证：

### 2.1 slash command 框架偏薄 ✅ 准确

计划说"缺更强的命令元数据、帮助结构、错误语义、上下文能力"。

源码验证：

- `internal/slashcmd/context.go` — `SessionContext` 只有 4 个方法：`SessionID()`、`ModelInfo()`、`SwitchModel()`、`ToolNames()`
- `AppContext` 只有 2 个方法：`ListSessionsInfo()`、`CreateSession()`
- 缺 profile 查询/切换、session 切换、workspace 摘要等控制面需要的接口

**结论：准确。**

### 2.2 `/new` 存在特判 ✅ 准确

计划说"`/new` 仍靠 interactive 层特判"。

源码验证：

`internal/agents/coding/cli/interactive.go` 第 58-65 行：

```go
if slashcmd.IsSlashCommand(input) {
    cmdName, _ := slashcmd.ParseSlashCommand(input)
    if cmdName == "new" {
        if err := m.handleNewSession(ctx); err != nil {
            fmt.Printf("error: %s\n", err)
        }
        continue
    }
    // ... 其他命令走 slash command 主链
}
```

确实存在硬编码的 `cmdName == "new"` 分支。

**结论：准确。** 但有一个计划未提及的重要细节——见第 3.1 节。

### 2.3 命令集不够像"控制面" ✅ 准确

计划说"缺 `/switch`、`/profiles`、`/profile`，`/compact` 和 `/branch` 是占位实现"。

源码验证：

- `builtins.go` 注册了 8 个命令：`help`、`compact`、`sessions`、`session`、`branch`、`new`、`tools`、`model`
- `/compact`（第 26 行）：返回 "Manual compaction is not yet implemented"
- `/branch`（第 84 行）：返回 "Branch navigation is not yet implemented"
- 无 `/switch`、`/profiles`、`/profile`

**结论：准确。**

### 2.4 profile 机制未成型 ✅ 准确

计划说"当前 coding-agent 还缺一个明确的 profile 承载点"。

源码验证：

- `internal/agents/coding/profile/` 目录不存在
- `CodingApplication.BuildTools()` 和 `BuildPrompt()` 均不涉及 profile 概念
- `runtime.ToolBuildOptions` 已有 `AllowedTools`/`BlockedTools` 字段，但 coding application 未使用

**结论：准确。**

---

## 3. 发现的问题与建议

### 3.1 ⚠️ `/new` 双重处理 — 回写机制需要明确方案

**问题**：计划说"消灭 `/new` 特判"，但代码中存在比计划描述更复杂的情况。

当前 `builtins.go` 第 88-103 行**已经注册了 `/new`**：

```go
registry.Register(slashcmd.Command{
    Name: "new",
    Handler: func(ctx slashcmd.Context, args string) (string, error) {
        newSession, err := ctx.App.CreateSession(ctx.Ctx)
        // ...
        ctx.Session = newSession  // ← 这行改的是 Context 值拷贝
        // ...
    },
})
```

而 `interactive.go` 第 102-123 行的 `handleNewSession()` 做的是：

```go
func (m *InteractiveMode) handleNewSession(ctx context.Context) error {
    newSession, err := m.app.NewSession(ctx)
    // ...
    m.session = newSession  // ← 这行改的是 interactive 持有的指针
    // ...
}
```

**关键差异**：`builtins.go` 的 `ctx.Session = newSession` 改的是 `slashcmd.Context` 的值拷贝字段，不会影响 `interactive.go` 持有的 `m.session` 指针。所以 `interactive.go` 的特判不是"多余重复"，而是**当前唯一能把 session 切换真正传递回 interactive 层的路径**。

计划 6.1 节提到了需要"轻量结果回写机制"，但没有给出具体方案。执行 agent 需要在以下方案中选一个：

| 方案 | 思路 | 优点 | 缺点 |
|------|------|------|------|
| **A：回调接口** | `AppContext` 增加 `OnSessionSwitch(func(SessionContext))` | 解耦，interactive 注册回调即可 | 回调链管理 |
| **B：Execute 返回结构化结果** | `Execute` 返回 `(CommandResult, error)`，`CommandResult` 携带 session 变更 | 显式、可测试 | 改 `Execute` 签名 |
| **C：Context 用指针** | `Session` 改为 `*SessionContext`，命令直接改指针指向 | 最小改动 | 语义不够清晰，`SessionContext` 是接口不能直接 `*SessionContext` |

**建议**：方案 B 更干净，且对现有测试的影响最小。

### 3.2 ⚠️ Profile 切换的生效范围需要明确

**问题**：计划说 profile 差异体现在 prompt 片段和工具过滤，但没说明 profile 切换后如何生效。

当前 `CodingApplication` 的 `BuildTools()` 和 `BuildPrompt()` 是在 session 创建时通过 `runtime.Application` 接口调用的：

```go
// application.go
func (CodingApplication) BuildTools(opts runtime.ToolBuildOptions) []agent.Tool { ... }
func (CodingApplication) BuildPrompt(opts runtime.PromptBuildOptions) string { ... }
```

这意味着：

- 如果 profile 只影响 prompt → 可以在 `PromptBuildOptions` 加 `Profile` 字段，每次 prompt 时生效
- 如果 profile 影响工具集 → 需要在 session 创建后热替换工具，或要求 profile 切换时重建 session

**建议**：执行计划应明确回答以下问题：

1. profile 切换是否需要重建 session？
2. 第一版是否只做 prompt 差异（不动工具集），降低复杂度？
3. 如果做工具过滤，是在 `BuildTools` 时根据 profile 过滤，还是运行时动态过滤？

对于第一版，我倾向于**只做 prompt 差异 + 预留工具过滤接口但不立即严格限制**——这也符合计划 5.2 节的建议（"如果你觉得直接禁用写工具风险太大，也可以第一版只做 prompt contract"）。

### 3.3 ⚠️ `AppContext` 接口扩展建议具体化

计划 6.1 节说"按需补充" profile 查询、profile 切换、session 切换等接口，但没有列出具体签名。建议在执行前明确新增的接口：

```go
// SessionContext 建议新增
Profile() string                                    // 查询当前 profile
SwitchProfile(profile string) error                 // 切换 profile

// AppContext 建议新增
SwitchSession(ctx context.Context, sessionID string) (SessionContext, error)  // /switch 需要
```

这样执行 agent 不需要自行推断接口设计，减少来回。

---

## 4. 设计方案审核

### 4.1 Profile 的位置 ✅ 合理

计划建议放在 `internal/agents/coding/profile/`，属于 coding application 内部，不下沉到 platform。

这与 `runtime.Application` 接口设计完全一致：
- `BuildTools` 已接受 `AllowedTools`/`BlockedTools`，profile 机制可以天然利用
- `BuildPrompt` 接受 `PromptBuildOptions`，可以加 profile 字段
- 不需要改 runtime 层

### 4.2 Application 边界 ✅ 清晰

计划的三层分工明确：

| 层 | 职责 | 不做什么 |
|----|------|---------|
| `slashcmd` | 通用命令框架 | 不感知 review/coding 等业务概念 |
| `coding/commands` | 命令定义 | 不承载 profile 逻辑 |
| `coding` application | profile 语义 | 不让 runtime 知道 profile |

### 4.3 执行顺序 ✅ 合理

五步顺序是正确的依赖链：

1. 补接口（不改行为）→ 为后续提供落点
2. 消灭 `/new` 特判 → 统一控制面入口
3. 补命令 → 丰富控制面
4. 补 profile 行为 → 利用前面建好的接口
5. 补测试 → 全链路验证

---

## 5. 验收标准审核

计划第 8 节列了 7 条验收标准。逐条评估：

| # | 验收标准 | 评估 | 建议 |
|---|---------|------|------|
| 1 | interactive 不再对 `/new` 做特殊分支处理 | ⚠️ 方向对，但不够 | 追加：验证切换后后续 prompt 流到新 session |
| 2 | 支持 `/switch <session-id>` | ✅ | — |
| 3 | 支持 `/profiles` 和 `/profile [name]` | ✅ | — |
| 4 | CLI 状态行能显示当前 profile | ✅ | — |
| 5 | review profile 在 prompt 行为上与 coding 有区别 | ✅ | 建议用测试证明，不要只靠人工验证 |
| 6 | `go test ./...` 通过 | ✅ | — |
| 7 | 新增测试存在 | ✅ | — |

**建议追加的验收标准：**

- **1b**：`/new` 执行后，`interactive` 持有的 session 指针确已切换（不是只改了值拷贝）
- **1c**：session 切换后，下一次 `runPrompt` 使用的是新 session（不是旧 session）
- **8**：`/branch` 如果仍为占位，从帮助主列表移除或标注 `[planned]`

---

## 6. "不要做"清单审核

计划第 3 节的"不要做"清单合理，对照代码现状确认无遗漏：

| 不要做 | 是否有风险违反 | 备注 |
|--------|--------------|------|
| 不引入新的大工具域 | ✅ 无风险 | 当前只在补控制面 |
| 不顺手做第二个 application | ✅ 无风险 | profile 是 coding 内部机制 |
| 不扩成通用万能 agent | ✅ 无风险 | 计划反复强调聚焦 |
| 不做新 GUI/TUI | ✅ 无风险 | 只改 interactive CLI |
| 不引入复杂插件市场 | ✅ 无风险 | — |
| 不为 profile 重写 prompt/skill 系统 | ✅ 无风险 | 只是加一层轻量配置 |

---

## 7. 依赖引用验证

| 依赖 | 路径 | 存在 |
|------|------|------|
| coding-agent spec | `docs/dev/coding-agent/spec.md` | ✅ |
| skills-vs-application | `docs/decisions/skills-vs-application.md` | ✅ |
| cc-haha core engine analysis | `docs/research/cc-haha-core-engine-analysis.md` | ✅ |
| deepv-code full analysis | `docs/research/deepv-code-full-analysis.md` | ✅ |

---

## 8. 审核总结

### 通过，建议执行前补充 3 处

1. **明确 `/new` 消灭后的 session 回写机制方案**（建议方案 B：Execute 返回结构化结果）
2. **明确 profile 切换时的生效范围**（建议第一版只做 prompt 差异 + 预留工具过滤接口）
3. **在验收标准中增加 "session 实际切换成功" 的验证点**

补充完毕后即可进入执行。计划整体设计方向正确，改动面可控，不需要调整架构层。
