---
status: reviewed
author: review-agent
created: 2026-05-26
updated: 2026-05-26
reviewer: review-agent
review-status: pending
depends-on:
  - docs/dev/layering/proposal.md
---

# Review: 分层深化 — 消除 Platform 层中的 coding-agent 语义泄露

## 1. 总体评价

**needs-revision**

提案方向正确，问题诊断精准，技术方案大体可行。但存在几个需要修正的不准确之处和一个架构层面的遗漏，需要在实施前修订。

---

## 2. 准确性验证

逐条核对提案中的关键声明与源代码的实际状态：

### 2.1 `profile` / `goal` 字段位置

| 提案声明 | 实际代码 | 判定 |
|---------|---------|------|
| `profile`/`goal` 字段在 `agent_session.go:55-56` | 实际在 `agent_session.go:54-55`（`profile string` / `goal string`） | ✅ 正确（行号偏差 1 行，无影响） |
| `SwitchProfile` 硬编码 coding/review | `agent_session.go:215-216` 确认 `case "coding", "review"` | ✅ 正确 |
| `SetGoal`/`ClearGoal` 触发 agent rebuild | `agent_session.go:241-248` 和 `252-260` 确认 | ✅ 正确 |

### 2.2 `contextWindowForModel` 硬编码

| 提案声明 | 实际代码 | 判定 |
|---------|---------|------|
| `agent_session.go:422-445` | 实际在 `agent_session.go:423-445` | ✅ 正确（行号偏差 1 行） |
| 硬编码模型表 | 确认，`map[string]int` 包含 14 个模型 | ✅ 正确 |

### 2.3 Skill 加载逻辑

| 提案声明 | 实际代码 | 判定 |
|---------|---------|------|
| `agent_session.go:324-340` | 实际在 `agent_session.go:321-340` | ✅ 正确（行号偏差） |
| 加载 context files 在 `agent_session.go:318` | 实际在 `agent_session.go:318`（`prompt.LoadProjectContextFiles(cwd, "")`） | ✅ 正确 |

### 2.4 `buildOperations` 位置

| 提案声明 | 实际代码 | 判定 |
|---------|---------|------|
| `agent_session.go:409-420` | 实际在 `agent_session.go:409-420` | ✅ 完全正确 |

### 2.5 `ToolBuildOptions` 包含 `EnableBash`

| 提案声明 | 实际代码 | 判定 |
|---------|---------|------|
| `internal/runtime/application.go:27` | 实际在 `application.go:26`（`EnableBash bool`） | ✅ 正确 |

### 2.6 `app.App` 硬编码 `CodingApplication`

| 提案声明 | 实际代码 | 判定 |
|---------|---------|------|
| `internal/app/app.go:124` | 实际在 `app.go:123`：`Application: coding.CodingApplication{}` | ✅ 正确 |
| `app.App` 注释写 "coding-agent" | `app.go:17` 确认 `// App is the thin assembly layer for the coding-agent.` | ✅ 正确 |

### 2.7 `mode/interactive.go` 是 type alias

| 提案声明 | 实际代码 | 判定 |
|---------|---------|------|
| type alias | `mode/interactive.go:11`：`type InteractiveMode = codingcli.InteractiveMode` | ✅ 正确 |

### 2.8 `server/server.go` 注释写 "coding agent"

| 提案声明 | 实际代码 | 判定 |
|---------|---------|------|
| `server.go:21` | 实际在 `server.go:20`：`// Server provides HTTP REST + SSE endpoints for the coding agent.` | ✅ 正确 |

### 2.9 `Application` 接口只有两个方法

| 提案声明 | 实际代码 | 判定 |
|---------|---------|------|
| `BuildTools` + `BuildPrompt` | `application.go:14-19` 确认 | ✅ 正确 |

### 2.10 `cmd/pi-agent/main.go` 已经使用 `app.AppOptions`

| 提案声称 `main.go` 需要改为接受 `Application` 参数 | 实际 `main.go:39-42` 已经使用 `app.New(app.AppOptions{...})` | ⚠️ 提案正确描述了改动方向，但暗示当前代码没有 `AppOptions`，而实际上已经有了 |

### 2.11 `Dependencies` 结构

| 提案代码片段中的 `Dependencies` | 实际 `agent_session.go:26-31` | ⚠️ **不一致** — 提案显示 `Dependencies` 有 `ExtRegistry` 字段（✅ 正确），但提案 §3.2 关键接口变更中新增的 `Dependencies` 代码片段把 `SessionExt` 放进了 `Dependencies`，这意味着 `SessionExt` 是全局共享而非 per-session 的。这与 per-session state 的设计目标矛盾。详见下方问题。 |

---

## 3. 发现的问题

### 🔴 Blockers（必须修正）

#### B1: `SessionExt` 放错位置 — 放进了 `Dependencies` 而非 per-session 结构

提案 §3.2 关键接口变更中写道：

```go
type Dependencies struct {
    // ...
    SessionExt  SessionExt  // optional, nil if not supported
}
```

`Dependencies` 是 App 级别的共享结构（在 `app.deps()` 中构造一次，所有 session 复用）。而 `SessionExt` 需要持有 per-session 的可变状态（`profile`、`goal`）。如果放在 `Dependencies` 里：

1. **所有 session 共享同一个 `SessionExt` 实例**，profile/goal 状态会跨 session 串扰。
2. **线程安全问题**：多个 session 并发修改同一个 `SessionExt` 里的 `profile`/`goal` 字段。

**正确做法**：`SessionExt` 应该由 `AgentSession` 在创建时自行构造（通过一个工厂方法），或者 `Application` 接口提供一个 `NewSessionExt() SessionExt` 方法来创建 per-session 实例。

#### B2: `SwitchModel` 留在 `AgentSession` 但未讨论

提案聚焦 profile/goal，但 `SwitchModel`（`agent_session.go:166-195`）同样包含 provider-specific 的硬编码逻辑（`switch providerName` 分支处理 `openai`/`deepv`/`anthropic`）。这不比 profile/goal 的耦合程度低。

虽然提案 §4 "不做的事" 中提到不改 agent core，但 `SwitchModel` 在 Platform 层而非 core 层。当前提案遗漏了 `SwitchModel` 的处置方案。

**需要补充**：要么说明 `SwitchModel` 为什么这次不改（附带理由），要么给出与 profile/goal 平行的提取方案。

#### B3: `PromptBuildOptions` 中 `Profile`/`Goal` 字段的归属问题未闭环

提案说 profile/goal 从 `AgentSession` 移到 `CodingSessionExt`，但 `PromptBuildOptions` 仍然有 `Profile` 和 `Goal` 字段（`application.go:41-42`）。如果这些字段由 `SessionExt` 管理，那么 `buildAgent` 仍然需要从 `SessionExt` 读取 profile/goal 来填充 `PromptBuildOptions`。

提案没有说明这个桥接路径。`AgentSession.buildAgent` 在移除 profile/goal 后如何获取这些值传给 `BuildPrompt`？

### 🟡 Strong Suggestions（强烈建议修正）

#### S1: `App.Profiles()` 和 `App.AvailableModels()` 也是 coding-agent 硬编码

`app.go:244-246`：`Profiles()` 硬编码返回 `["coding", "review"]`。
`app.go:250-263`：`AvailableModels()` 硬编码模型列表。

这些实现了 `slashcmd.AppContext` 接口。如果 `App` 要支持多 Application，这些方法也需要委托给 Application。提案遗漏了 `AppContext` 接口本身的调整。

#### S2: `App.ToolNames()` 直接调用 `coding.BaseToolNames()`

`app.go:132`：`baseNames := coding.BaseToolNames(cfg.EnableBash)` — `App` 层直接依赖 `coding` 包的具体函数。如果 `App` 要做成 Application 无关的组装层，`ToolNames()` 应该委托给 `Application` 接口。

#### S3: WebSocket handler 也调用 `SwitchModel`

`server/websocket.go:251`：`sess.SwitchModel(ctx, msg.Model, msg.Provider)` — WebSocket 路径直接调用 `AgentSession.SwitchModel`。如果 `SwitchModel` 未来要提取，WebSocket handler 也需要同步调整。提案的变更文件列表中漏掉了 `websocket.go`。

#### S4: `InteractiveMode` 中直接访问 `session.Profile()`

`cli/interactive.go:47,86`：直接调用 `m.session.Profile()` 来格式化状态显示。如果 `Profile()` 从 `AgentSession` 移到 `SessionExt`，`InteractiveMode` 需要 type assertion 或新的访问路径。提案未提及这个影响。

#### S5: 测试文件影响范围未评估

以下测试文件会受影响但提案未提及：
- `internal/slashcmd/registry_test.go:107-120`：`mockSessionContext` 实现了完整 `SessionContext`（含 `Profile`/`Goal`/`SwitchProfile`/`SetGoal`/`ClearGoal`）
- `internal/agents/coding/commands/builtins_test.go:84-101`：`mockApp` 和 session mock
- `internal/runtime/session_registry_test.go`

拆分 `SessionContext` 接口后，这些 mock 需要同步更新。

### 🟢 Nice-to-haves

#### N1: `Config.EnableBash` 是否也该从 `Config` 中移除？

`config.Config.EnableBash`（`config.go:40`）是全局配置，但 bash 开关是 coding-agent 的业务策略。如果追求彻底的分层，`Config` 层也不应该知道 bash。不过这可能是过度拆分，可以留到未来。

#### N2: `registerProviders` 在 `app.go` 中硬编码 provider 注册逻辑

`app.go:169-202` 的 `registerProviders` 包含 coding-agent 特有的 DeepV header provider 逻辑（`coding.NewDeepVHeaderProvider(workDir)`）。如果未来有非 coding agent，这个函数也需要重构。

#### N3: `operations.NewSSHOperations` 的配置来源

`buildOperations` 从 `Config` 读取 SSH 配置。如果移到 Application 层，Application 需要访问 SSH 配置。目前 `Config` 作为通用配置结构，SSH 配置的存在是否合理需要考量。

---

## 4. 遗漏检查

### 4.1 `Compact` 方法在 `SessionContext` 中

`slashcmd.SessionContext` 包含 `Compact()` 方法（`context.go:29`）。这个方法由 `agent.Agent.CompactNow` 实现，属于 Platform 层的通用能力。提案没有提到它，但拆分 `SessionContext` 时需要确保 `Compact` 留在基础接口中。这一点提案 §3.4 方案 A 的基础接口已经包含了 `Compact`，✅ 没有遗漏。

### 4.2 `SessionSwitchTo` 的类型断言

`cli/interactive.go:83`：`m.session = result.SessionSwitchTo.(*runtime.AgentSession)` — 这是一个硬类型断言。如果 `SessionContext` 拆分后返回的是接口而非具体类型，这个断言可能会失败。需要确保拆分方案保留了具体的 `*runtime.AgentSession` 类型断言路径。

### 4.3 `buildAgent` 中 skill 加载的默认路径逻辑

`agent_session.go:322-332` 中 skill 加载有默认路径逻辑（`.claude/skills` 目录），这是 coding-agent 特有的。移到 Application 层后，`CodingApplication` 需要自己负责这些默认路径。提案提到了但不详细。建议 `CodingApplication.BuildPrompt` 或一个新方法（如 `PreparePromptData`）来封装这个逻辑。

### 4.4 `app.go` 中 `homeDir()` 函数

`app.go:205-210` 有一个未被使用的 `homeDir()` 函数。清理时可以一并移除。

---

## 5. 修改建议

### 针对每个问题的具体建议

**B1 修复**：将 `SessionExt` 从 `Dependencies` 移到 `AgentSessionOptions`，或让 `Application` 提供 `NewSessionExt() SessionExt` 工厂方法：

```go
// 方案 A：由 Application 创建 per-session ext
type Application interface {
    BuildTools(opts ToolBuildOptions) []agent.Tool
    BuildPrompt(opts PromptBuildOptions) string
    NewSessionExt() SessionExt  // 返回 nil 表示不支持
}

// AgentSession 在 NewAgentSession 中调用
s.ext = deps.Application.NewSessionExt()
```

```go
// 方案 B：通过 AgentSessionOptions 传入
type AgentSessionOptions struct {
    SessionID string
    Config    config.Config
    SkillDirs []string
    Ext       SessionExt  // 由调用方（App）创建 per-session 实例
}
```

推荐方案 A，因为 `AgentSession` 不应该知道 `CodingSessionExt` 的构造细节。

**B2 修复**：在提案中明确 `SwitchModel` 的处置：
- 选项 1：这次不动，在 §4 "不做的事" 中显式列出，并说明理由（`SwitchModel` 的 model dispatch 逻辑虽然包含 provider 分支，但 model 切换是 Platform 级能力而非 coding-agent 特有能力）
- 选项 2：将 model dispatch 逻辑提取到 provider registry 中（让 registry 根据 model ID 自动路由 provider），`SwitchModel` 只做 config 更新 + rebuild

**B3 修复**：在提案中补充 `PromptBuildOptions` 的 profile/goal 字段填充路径：

```go
// buildAgent 中
var profile, goal string
if s.ext != nil {
    profile = s.ext.Profile()
    goal = s.ext.Goal()
}
systemPrompt := s.application.BuildPrompt(PromptBuildOptions{
    // ...
    Profile: profile,
    Goal:    goal,
})
```

**S1 修复**：在 `AppContext` 接口中增加 `Profiles()` 和 `AvailableModels()` 的来源说明，或将这些方法委托给 `Application`：

```go
// App 层
func (a *App) Profiles() []string {
    if p, ok := a.application.(ProfileLister); ok {
        return p.Profiles()
    }
    return nil
}
```

**S2 修复**：`ToolNames()` 委托给 Application，或在 `Application` 接口增加 `ToolNames(enableBash bool) []string` 方法。

**S3 修复**：将 `internal/server/websocket.go` 加入变更文件列表。

**S4 修复**：在变更文件列表中加入 `internal/agents/coding/cli/interactive.go`，并说明 `Profile()` 访问路径的调整方案。

**S5 修复**：增加 §5.变更的文件 中的测试文件：
- `internal/slashcmd/registry_test.go`
- `internal/agents/coding/commands/builtins_test.go`

---

## 6. 总结

| 维度 | 评级 | 说明 |
|------|------|------|
| **准确性** | ⭐⭐⭐⭐ | 行号有微小偏差（1-2 行），不影响理解。但 §3.2 接口代码片段与实际 `Dependencies` 用法有冲突（B1） |
| **完整性** | ⭐⭐⭐ | 遗漏了 `SwitchModel`（B2）、`websocket.go`、测试文件、`App.ToolNames()`/`App.Profiles()` 的耦合、`InteractiveMode` 对 `Profile()` 的直接访问 |
| **可行性** | ⭐⭐⭐⭐ | 整体方向可行，`SessionExt` 的 per-session 实例化问题（B1）是实施前必须解决的架构问题 |
| **一致性** | ⭐⭐⭐⭐⭐ | 与现有分层架构的思路高度一致，推荐方案（简单 `SessionExt` 而非通用 `StateChange`）符合 Go 的最小抽象风格 |
| **风险盲点** | ⭐⭐⭐ | 低估了 `SessionExt` 的 per-session 生命周期管理复杂度，以及拆分 `SessionContext` 对 mock/test 的影响范围 |
| **范围控制** | ⭐⭐⭐⭐⭐ | "不做的事" 定义清晰，没有暗中扩大范围 |

**修订后预期评级**：approve — 修正 B1/B2/B3 后即可进入实施。
