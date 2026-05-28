---
status: draft
author: plan-agent
created: 2026-05-26
updated: 2026-05-26
---

# 分层深化：消除 Platform 层中的 coding-agent 语义泄露

## 1. 目标

将 `runtime.AgentSession` 中硬编码的 coding-agent 语义（profile、goal、model 切换、operations 构建）抽到 Application 层，使 Platform 层真正做到领域无关。

## 2. 为什么现在做

上一轮分层重构（`docs/dev/layering-refactor/`）已经完成了目录搬迁和 `runtime.Application` 接口的落地。coding-agent 的 tools、commands、prompt builder 已经归位到 `internal/agents/coding/`。这很好。

但现在有一个明确的问题：**`runtime.AgentSession` 仍然是 coding-agent 的"胖 session"**。它直接持有 `profile`、`goal` 字段，硬编码了 `SwitchProfile` / `SetGoal` / `SwitchModel` 的实现，还在 `buildAgent` 里做 skill 加载和 context files 加载。这些不是 Platform 层该管的事。

具体问题清单：

| 问题 | 位置 | 影响 |
|------|------|------|
| `profile` / `goal` 字段直接在 Platform 层 | `internal/runtime/agent_session.go:55-56` | 第二个 agent 不一定需要 profile/goal 概念 |
| `SwitchProfile` 硬编码 coding/review 两个 profile | `agent_session.go:214-231` | 新 profile 需要改 runtime 包 |
| `contextWindowForModel` 硬编码模型表 | `agent_session.go:422-445` | 这应该是 model registry 的事 |
| Skill 加载逻辑在 `buildAgent` 里 | `agent_session.go:324-340` | 不是所有 agent 都需要 skill |
| Context files 加载在 `buildAgent` 里 | `agent_session.go:318` | 不是所有 agent 都读 CLAUDE.md |
| `buildOperations` 在 Platform 层做 SSH/Local 判断 | `agent_session.go:409-420` | operations 的选择策略应该是 Application 层决定的 |
| `ToolBuildOptions` 包含 `EnableBash` | `internal/runtime/application.go:27` | bash 开关是 coding-agent 的业务策略，不是所有 agent 都有 bash |
| `app.App` 硬编码 `CodingApplication` | `internal/app/app.go:124` | 组装层直接依赖具体实现，没有多 Application 的空间 |
| `mode/interactive.go` 是 coding CLI 的 type alias | `internal/mode/interactive.go` | Entrypoints 层被 coding-agent 耦合 |
| `server/server.go` 注释写 "coding agent" | `internal/server/server.go:21` | 文档层面泄露 |

这些问题现在不改，后面做第二个 Application（比如 feishu-agent）时会遇到两种选择：要么在 `AgentSession` 里加 `if agentType == "feishu"` 的分支，要么绕过 `AgentSession` 自己造一套。两条路都不好。

## 3. 这次做什么

### 3.1 AgentSession 瘦身

**核心思路**：`AgentSession` 只管 session 生命周期（创建、加载、对话、compaction），不管 Application 级别的配置。

**具体变更**：

1. **移除 `profile` / `goal` 字段和对应方法**：`Profile()`、`SwitchProfile()`、`Goal()`、`SetGoal()`、`ClearGoal()` 从 `AgentSession` 移到 `CodingApplication` 的 session state。

2. **`buildAgent` 不再做 skill 加载和 context files 加载**：这些传入 `BuildPrompt` 的数据由 Application 层准备。`PromptBuildOptions` 保持字段但由调用方填充。

3. **`buildOperations` 移出 AgentSession**：operations 后端的选择策略移到 `CodingApplication`。`ToolBuildOptions` 保留 `BashOps` / `FileOps` 接口，但 `AgentSession` 不再负责构建具体实现。

4. **`EnableBash` 从 `ToolBuildOptions` 移除**：bash 开关是 coding-agent 的业务决策，应由 `CodingApplication.BuildTools()` 内部判断，而不是 Platform 层传递。

5. **`contextWindowForModel` 移到 model registry**：当前是 `AgentSession` 里的硬编码 map，移到 `internal/ai/providers/` 或独立的 model metadata 包。

### 3.2 Application 接口扩展

当前接口只有两个方法：

```go
type Application interface {
    BuildTools(opts ToolBuildOptions) []agent.Tool
    BuildPrompt(opts PromptBuildOptions) string
}
```

需要扩展以承担从 AgentSession 移出的职责：

```go
type Application interface {
    // BuildTools assembles the tool list for the agent session.
    BuildTools(opts ToolBuildOptions) []agent.Tool

    // BuildPrompt constructs the system prompt for the agent session.
    BuildPrompt(opts PromptBuildOptions) string

    // SessionState creates a new per-session state holder.
    // Used for application-specific session data (e.g. profile, goal).
    SessionState() ApplicationSessionState
}

// ApplicationSessionState holds application-specific per-session state.
// AgentSession stores this opaquely and returns it for slash commands.
type ApplicationSessionState interface {
    // SwitchModel is delegated to the application for model switching logic.
    // The application can decide whether to rebuild the agent.
    ApplyStateChange(ctx context.Context, change StateChange) error
}

// StateChange represents a generic state change request from slash commands.
type StateChange struct {
    Type  string // "profile", "goal", etc.
    Key   string
    Value string
}
```

但这个设计有过度工程化的风险。**我推荐更简单的方案**：

不扩展 `Application` 接口，而是让 `AgentSession` 接受一个可选的 `SessionExt` 接口：

```go
// SessionExt is an optional extension for AgentSession to support
// application-specific session operations (profile, goal, etc.).
// If not provided, profile/goal features are unavailable.
type SessionExt interface {
    SwitchProfile(ctx context.Context, profile string) error
    Profile() string
    SetGoal(goal string)
    Goal() string
    ClearGoal()
}
```

`CodingApplication` 在创建 session 时注入 `SessionExt` 实现。Platform 层的 `AgentSession` 通过 `ext.SessionExt` 可选地暴露这些能力。如果 `ext` 为 nil，相关 slash command 返回 "not supported"。

**推荐简单方案**，理由：
- 不改变现有 `Application` 接口的两个方法签名
- Profile/Goal 是 coding-agent 特有概念，不应该污染 Platform 接口
- 第二个 agent 不实现 `SessionExt` 就自然没有这些功能
- 渐进式：可以后续再决定是否做通用的 `StateChange` 机制

### 3.3 App 层的多 Application 支持

当前 `app.App` 直接硬编码 `CodingApplication`：

```go
func (a *App) deps() runtime.Dependencies {
    return runtime.Dependencies{
        // ...
        Application: coding.CodingApplication{},
    }
}
```

改为构造时注入：

```go
type AppOptions struct {
    Config      config.Config
    SkillDirs   []string
    Application runtime.Application // 注入
}
```

`cmd/pi-agent/main.go` 负责选择具体 Application：

```go
application, err := app.New(app.AppOptions{
    Config:      cfg,
    SkillDirs:   skillDirs(*skillDir),
    Application: coding.CodingApplication{},
})
```

### 3.4 Slash command 上下文调整

`slashcmd.SessionContext` 当前直接定义了 `Profile()` / `SwitchProfile()` / `Goal()` / `SetGoal()` / `ClearGoal()`。这些方法不是所有 agent 都有。

两个选择：

**A. 保留接口但拆分**：把 `SessionContext` 拆成基础接口 + 可选扩展：

```go
type SessionContext interface {
    SessionID() string
    ModelInfo() (provider string, modelID string)
    SwitchModel(ctx context.Context, modelID string, provider string) error
    ToolNames() []string
    Compact(ctx context.Context, customInstructions string) (string, int, int, error)
}

type ProfileContext interface {
    Profile() string
    SwitchProfile(ctx context.Context, profile string) error
}

type GoalContext interface {
    Goal() string
    SetGoal(goal string)
    ClearGoal()
}
```

Slash command handler 通过 type assertion 检查：

```go
if pc, ok := ctx.Session.(slashcmd.ProfileContext); ok {
    // use pc.Profile()
}
```

**B. 保留当前接口不变，不满足的方法返回 error**。

**推荐方案 A**：接口分离更干净，符合 Go 的接口组合风格。不破坏现有代码——coding-agent 的 `AgentSession` + `SessionExt` 自然满足所有子接口。

### 3.5 Entrypoints 层清理

- `internal/mode/interactive.go` 的 type alias 改为真正的入口分发
- `internal/server/server.go` 注释从 "coding agent" 改为 "agent"
- `internal/mode/` 包改为接受 `runtime.Application` 而不是硬编码 coding 类型

### 3.6 模型元数据集中化

把 `contextWindowForModel` 的硬编码 map 提取到 `internal/ai/` 下的 model metadata 包：

```go
// internal/ai/models/registry.go
package models

var contextWindows = map[string]int{
    "claude-sonnet-4-6": 200000,
    // ...
}

func ContextWindow(modelID string) int {
    if w, ok := contextWindows[modelID]; ok {
        return w
    }
    return 128000
}
```

## 4. 这次不做什么

1. **不做目录重命名**：不把 `internal/runtime` 改成 `internal/platform`。当前名称语义已够清晰，重命名是纯机械变更，收益不大。

2. **不做第二个 Application 实现**：这次只清理边界，不实现 feishu-agent 或 review-agent 作为验证。但设计时要确保第二个 Application 能自然落位。

3. **不做通用的 `StateChange` 消息机制**：profile/goal 用简单的 `SessionExt` 接口解决，不做 event bus 式的通用状态变更机制。

4. **不改 agent core**（`internal/agent/`、`internal/ai/`）：Core 层已经是通用的，这次不动。

5. **不做 `App` 的多 Application 路由**：一个进程仍然只跑一个 Application，不做按请求路由到不同 Application 的机制。

6. **不改 server 的 API 路径设计**：当前 `/chat`、`/sessions`、`/models` 等路径保持不变。多 Application 路由是以后的事。

## 5. 技术方案

### 变更的文件

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/runtime/application.go` | 修改 | 新增 `SessionExt` 接口，调整 `ToolBuildOptions`（移除 `EnableBash`） |
| `internal/runtime/agent_session.go` | 修改 | 移除 profile/goal 字段和方法，移除 `buildOperations`，移除 `contextWindowForModel`，通过 `SessionExt` 委托 |
| `internal/ai/models/registry.go` | 新增 | 模型元数据集中化 |
| `internal/agents/coding/session_ext.go` | 新增 | `CodingSessionExt` 实现 `SessionExt`，持有 profile/goal 状态 |
| `internal/agents/coding/application.go` | 修改 | `CodingApplication` 提供 `SessionExt`，内部决定 `EnableBash` |
| `internal/agents/coding/tools/tools.go` | 修改 | `ListOptions` 移除 `EnableBash`，改为内部常量或从 config 读取 |
| `internal/slashcmd/context.go` | 修改 | 拆分 `SessionContext` 为基础 + 可选子接口 |
| `internal/app/app.go` | 修改 | `AppOptions` 接受 `Application` 参数，移除硬编码 |
| `internal/mode/interactive.go` | 修改 | 不再是 type alias，改为接受 `runtime.Application` |
| `internal/mode/serve.go` | 修改 | 同上 |
| `internal/mode/print.go` | 微调 | 仅调整依赖 |
| `cmd/pi-agent/main.go` | 修改 | 显式注入 `CodingApplication` |
| `internal/server/server.go` | 微调 | 注释更新 |
| `internal/agents/coding/commands/builtins.go` | 修改 | profile/goal 命令通过 type assertion 使用子接口 |

### 数据流变化

**Before**：
```
cmd/main → app.New() → [hardcoded CodingApplication]
                            ↓
AgentSession (holds profile, goal, builds operations, loads skills, loads context files)
                            ↓
Application.BuildTools() / BuildPrompt()
```

**After**：
```
cmd/main → app.New(Application: coding.CodingApplication{})
                            ↓
AgentSession (thin: session lifecycle + agent rebuild only)
                            ↓
Application.BuildTools() / BuildPrompt()
                            ↓
SessionExt (coding-specific: profile, goal, skill loading)
```

### 关键接口变更

```go
// runtime/application.go — 新增

// SessionExt is an optional extension for application-specific session features.
// CodingApplication provides profile and goal management via this interface.
// Other applications may leave this as nil.
type SessionExt interface {
    SwitchProfile(ctx context.Context, profile string) error
    Profile() string
    SetGoal(goal string)
    Goal() string
    ClearGoal()
}

// Dependencies 新增 SessionExt 字段
type Dependencies struct {
    Registry    *providers.Registry
    SessionMgr  *sessionmgr.Manager
    ExtRegistry *extensions.Registry
    Application Application
    SessionExt  SessionExt  // optional, nil if not supported
}
```

```go
// slashcmd/context.go — 拆分

type SessionContext interface {
    SessionID() string
    ModelInfo() (provider string, modelID string)
    SwitchModel(ctx context.Context, modelID string, provider string) error
    ToolNames() []string
    Compact(ctx context.Context, customInstructions string) (string, int, int, error)
}

// Optional: profile support
type ProfileSupport interface {
    Profile() string
    SwitchProfile(ctx context.Context, profile string) error
}

// Optional: goal support
type GoalSupport interface {
    Goal() string
    SetGoal(goal string)
    ClearGoal()
}
```

## 6. 依赖关系

| 依赖 | 状态 | 说明 |
|------|------|------|
| `runtime.Application` 接口 | ✅ 已存在 | 不改签名，只新增 `SessionExt` |
| `slashcmd` 框架 | ✅ 已存在 | 拆分子接口是向后兼容的（type assertion） |
| `operations` 抽象 | ✅ 已存在 | 不变，只是构建位置从 Platform 移到 Application |
| 模型元数据 registry | ❌ 需新建 | `internal/ai/models/` 包 |
| coding-agent 目录结构 | ✅ 已就位 | `internal/agents/coding/` 已经有独立子包 |

## 7. 风险和取舍

### 风险

1. **Slash command 的 type assertion 会在运行时才发现不支持**：如果 coding-agent 的 `/profile` 命令跑在非 coding session 上，会得到 "profile not supported" 的运行时错误。这是可接受的——slash command 本来就是 application-specific 的，注册时就知道在哪个 application 下。

2. **`SessionExt` 可能膨胀**：如果未来 agent 需要更多 session-level 状态（比如 workspace 路径、权限级别），`SessionExt` 可能变成另一个大接口。缓解方式：保持 `SessionExt` 小，只放真正跨 command 共享的状态；其余用 command 自己的 state。

3. **重构期间需要同步改很多文件**：agent_session.go、builtins.go、app.go、context.go 都有变动。建议一个 PR 完成，不要分多个 PR 以免中间状态不可用。

### 取舍

1. **没有做通用的 `StateChange` 消息机制**：因为当前只有两个 Application-level 状态（profile、goal），不值得做通用机制。等第三个或第四个状态出现时再抽象。

2. **`ToolBuildOptions` 保留 `BashOps`/`FileOps`**：虽然 operations 的具体能力集合偏 coding，但 `BashOperations` 和 `FileOperations` 作为 Platform 级接口是合理的——它们足够通用（任何需要文件操作的 agent 都可以用）。只是"选择哪个后端"的逻辑应该由 Application 决定。

3. **没有改目录结构**：`internal/runtime` 保持原名，不改成 `internal/platform`。重命名收益低于风险。

## 8. 完成标志

- [ ] `AgentSession` 不持有 `profile`、`goal` 字段，不包含 `SwitchProfile`/`SetGoal`/`ClearGoal` 方法
- [ ] `AgentSession` 不包含 `buildOperations` 方法，operations 构建由 Application 层负责
- [ ] `AgentSession` 不包含 `contextWindowForModel` 函数，模型元数据在独立包中
- [ ] `ToolBuildOptions` 不包含 `EnableBash` 字段
- [ ] `Application` 接口签名不变（`BuildTools` + `BuildPrompt`），`SessionExt` 作为可选扩展
- [ ] `app.App` 通过构造参数接受 `Application`，不硬编码 `CodingApplication`
- [ ] `slashcmd.SessionContext` 拆分出 `ProfileSupport` 和 `GoalSupport` 子接口
- [ ] `cmd/pi-agent/main.go` 显式构造并注入 `CodingApplication`
- [ ] `go test ./...` 全部通过
- [ ] `internal/server/` 和 `internal/mode/` 中不再有 "coding agent" 的注释或假设
