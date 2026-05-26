---
status: approved
author: exec-agent
created: 2026-05-26
updated: 2026-05-26
depends-on:
  - docs/dev/layering/proposal.md
  - docs/dev/layering/review.md
---

# Execution Plan: 分层深化 — 消除 Platform 层中的 coding-agent 语义泄露

## 1. 整体架构

### 1.1 目标架构

```
┌──────────────────────────────────────────────────────────────────┐
│  Entrypoints (cmd/, mode/)                                      │
│  — 接受 runtime.Application 接口，不再硬编码 coding 依赖          │
├──────────────────────────────────────────────────────────────────┤
│  App (internal/app/)                                            │
│  — 组装层：构造 Dependencies，注入 Application                   │
│  — AppOptions.Application 字段接受任何 Application 实现           │
│  — ToolNames/Profiles/AvailableModels 委托给 Application         │
├──────────────────────────────────────────────────────────────────┤
│  Application: CodingApplication (internal/agents/coding/)       │
│  — BuildTools(): 内部决定 EnableBash，接收 BashOps/FileOps      │
│  — BuildPrompt(): 接收 Profile/Goal，构建 coding 特有 prompt      │
│  — NewSessionExt(): 创建 per-session 的 CodingSessionExt        │
│  — ToolNames/Profiles/AvailableModels: 通过可选接口暴露           │
├──────────────────────────────────────────────────────────────────┤
│  SessionExt: CodingSessionExt (internal/agents/coding/)         │
│  — 持有 per-session 的 profile/goal 状态                         │
│  — 由 CodingApplication.NewSessionExt() 工厂方法创建             │
│  — AgentSession 通过接口可选访问                                  │
├──────────────────────────────────────────────────────────────────┤
│  Platform / Runtime (internal/runtime/)                          │
│  — AgentSession: session 生命周期 + agent rebuild               │
│  — 不持有 profile/goal 字段                                     │
│  — 不包含 buildOperations / contextWindowForModel               │
│  — 通过 SessionExt 接口可选地暴露 profile/goal 能力              │
├──────────────────────────────────────────────────────────────────┤
│  SlashCmd (internal/slashcmd/)                                  │
│  — SessionContext: 基础接口（session/model/compact/tool）        │
│  — ProfileSupport: 可选子接口（profile）                         │
│  — GoalSupport: 可选子接口（goal）                               │
├──────────────────────────────────────────────────────────────────┤
│  Model Metadata (internal/ai/models/)                            │
│  — contextWindowForModel 提取为独立函数                          │
│  — 由 Platform 层调用，不再硬编码在 AgentSession 中               │
└──────────────────────────────────────────────────────────────────┘
```

### 1.2 关键组件关系

```
Dependencies (共享, App 级)
├── Registry      *providers.Registry
├── SessionMgr    *sessionmgr.Manager
├── ExtRegistry   *extensions.Registry
└── Application   runtime.Application       ← CodingApplication{}

AgentSession (per-session, 由 SessionRegistry 管理)
├── ext           SessionExt                ← 由 Application.NewSessionExt() 创建
│                                              每个 session 独立实例
├── application  Application
└── buildAgent()
    ├── Application.BuildTools()
    │     └── EnableBash 从 Config 传入，不再通过 ToolBuildOptions
    └── Application.BuildPrompt()
          ├── profile/goal 从 ext 读取 (nil-safe)
          └── 传入 PromptBuildOptions{Profile, Goal}
```

### 1.3 数据流变化

**Before:**
```
cmd/main → app.New() [hardcoded CodingApplication]
            ↓
AgentSession [holds profile/goal, builds operations, loads skills/context files, contextWindowForModel]
            ↓
Application.BuildTools(EnableBash) / BuildPrompt(Profile, Goal)
```

**After:**
```
cmd/main → app.New(AppOptions{Application: CodingApplication{}})
            ↓
AgentSession [thin: session lifecycle + agent rebuild]
  ├── ext = Application.NewSessionExt()  [per-session, holds profile/goal]
  ├── builds operations via Application.BuildOperations()
  ├── loads context window via ai/models.ContextWindow()
  └── buildAgent()
        ├── profile/goal from ext (nil-safe)
        ├── Application.BuildTools(BashOps, FileOps) [no EnableBash param]
        └── Application.BuildPrompt(PromptBuildOptions{Profile, Goal})
```

## 2. 这次不做什么

1. **不改 `SwitchModel` 逻辑**：`SwitchModel` 虽然包含 provider-specific 分支，但 model 切换是 Platform 级能力（任何 agent 都需要切换模型）。这次保留在 `AgentSession` 中。后续可通过让 provider registry 自动路由 model ID → provider 来简化。
2. **不做第二个 Application 实现**：只清理边界，不实现新 Application 作为验证。
3. **不做通用的 `StateChange` 消息机制**：profile/goal 用简单的 `SessionExt` 接口解决。
4. **不改 agent core**（`internal/agent/`、`internal/ai/`）：Core 层保持不变。
5. **不做 `App` 的多 Application 路由**：一个进程仍只跑一个 Application。
6. **不改 server API 路径**：当前路由保持不变。
7. **不改 `Config` 中的 `EnableBash` 字段位置**：`config.Config.EnableBash` 保留，但从 `ToolBuildOptions` 中移除，改由 `CodingApplication.BuildTools` 内部读取 Config。
8. **不做目录重命名**：`internal/runtime` 保持原名。

## 3. 实施步骤

### Step 1: 创建 `internal/ai/models/registry.go` — 模型元数据提取

**文件**: `internal/ai/models/registry.go`（新建）

**内容**: 将 `agent_session.go:422-445` 中的 `contextWindowForModel` 函数提取为独立包。

```go
package models

// ContextWindow returns the context window size for a given model ID.
// Returns 128000 as the default for unknown models.
func ContextWindow(modelID string) int {
    windows := map[string]int{
        "claude-3-5-sonnet": 200000,
        "claude-3-5-haiku":  200000,
        "claude-3-opus":     200000,
        "claude-sonnet-4":   200000,
        "claude-sonnet-4-5": 200000,
        "gpt-4o":            128000,
        "gpt-4o-mini":       128000,
        "gpt-4-turbo":       128000,
        "gpt-4":             8192,
        "o1":                200000,
        "o1-mini":           128000,
        "o3-mini":           200000,
        "claude-sonnet-4-6": 200000,
        "glm-5":             128000,
        "deepseek-v4-flash": 128000,
    }
    if w, ok := windows[modelID]; ok {
        return w
    }
    return 128000
}
```

**验证**: `go build ./internal/ai/models/` 编译通过。

**依赖**: 无。

---

### Step 2: 定义 `SessionExt` 接口 — `internal/runtime/application.go`

**文件**: `internal/runtime/application.go`（修改）

**变更**:

1. 新增 `SessionExt` 接口定义（在文件末尾）：

```go
// SessionExt is an optional extension interface for application-specific
// session features (e.g., profile, goal management).
// If not provided (nil), these features are unavailable.
// Each session gets its own SessionExt instance via Application.NewSessionExt().
type SessionExt interface {
    // Profile returns the current profile name.
    Profile() string

    // SwitchProfile changes the active profile and triggers an agent rebuild.
    SwitchProfile(ctx context.Context, profile string) error

    // Goal returns the current session goal.
    Goal() string

    // SetGoal sets the session goal and triggers an agent rebuild.
    SetGoal(goal string)

    // ClearGoal clears the session goal and triggers an agent rebuild.
    ClearGoal()
}
```

2. 从 `ToolBuildOptions` 移除 `EnableBash` 字段：

```go
type ToolBuildOptions struct {
    Workspace      string
    MaxOutputLen   int
    // EnableBash removed — Application decides internally
    BashOps        operations.BashOperations
    FileOps        operations.FileOperations
    ExtensionTools []agent.Tool
    AllowedTools   []string
    BlockedTools   []string
}
```

3. 从 `PromptBuildOptions` 移除 `Profile` 和 `Goal` 字段：

```go
type PromptBuildOptions struct {
    CustomPrompt string
    CWD          string
    Tools        []agent.Tool
    ContextFiles []prompt.ContextFile
    Skills       []skill.Skill
    // Profile and Goal removed — read from SessionExt if needed
}
```

4. 在 `Application` 接口中新增 `NewSessionExt()` 方法：

```go
type Application interface {
    BuildTools(opts ToolBuildOptions) []agent.Tool
    BuildPrompt(opts PromptBuildOptions) string

    // NewSessionExt creates a per-session extension for application-specific state.
    // Returns nil if the application does not use session extensions.
    NewSessionExt(cfg Config) SessionExt
}
```

**注意**: `NewSessionExt(cfg Config)` 接受 `Config` 是因为 `CodingSessionExt` 需要读取 `Config.EnableBash` 和 `Config.ExecutionMode` 等。

**验证**: `go build ./internal/runtime/` 编译通过（此时 AgentSession 会报编译错误，因为还没改——预期行为）。

**依赖**: 无。

---

### Step 3: 创建 `CodingSessionExt` — `internal/agents/coding/session_ext.go`

**文件**: `internal/agents/coding/session_ext.go`（新建）

**变更**: 实现 `runtime.SessionExt`，持有 per-session 的 profile/goal 状态。需要实现 rebuild 回调。

```go
package coding

import (
    "context"
    "fmt"

    "github.com/earendil-works/pi-go/internal/runtime"
    "github.com/earendil-works/pi-go/internal/agents/coding/profile"
)

// RebuildFunc is a callback to trigger agent rebuild after state change.
type RebuildFunc func() error

// CodingSessionExt implements runtime.SessionExt for the coding-agent.
// It holds per-session application state (profile, goal).
type CodingSessionExt struct {
    profile string
    goal    string
    rebuild RebuildFunc
}

// NewCodingSessionExt creates a new CodingSessionExt with default profile "coding".
func NewCodingSessionExt(rebuild RebuildFunc) *CodingSessionExt {
    return &CodingSessionExt{
        profile: string(profile.ProfileCoding),
        rebuild: rebuild,
    }
}

func (e *CodingSessionExt) Profile() string { return e.profile }

func (e *CodingSessionExt) SwitchProfile(ctx context.Context, p string) error {
    if !profile.Valid(p) {
        return fmt.Errorf("unknown profile: %q (available: %v)", p, profile.All())
    }
    e.profile = p
    if e.rebuild != nil {
        if err := e.rebuild(); err != nil {
            return fmt.Errorf("rebuild agent with profile %q: %w", p, err)
        }
    }
    return nil
}

func (e *CodingSessionExt) Goal() string { return e.goal }

func (e *CodingSessionExt) SetGoal(goal string) {
    e.goal = goal
    if e.rebuild != nil {
        if err := e.rebuild(); err != nil {
            slog.Error("failed to rebuild agent after goal set", "error", err)
        }
    }
}

func (e *CodingSessionExt) ClearGoal() {
    e.goal = ""
    if e.rebuild != nil {
        if err := e.rebuild(); err != nil {
            slog.Error("failed to rebuild agent after goal clear", "error", err)
        }
    }
}

// Compile-time check.
var _ runtime.SessionExt = (*CodingSessionExt)(nil)
```

**验证**: `go build ./internal/agents/coding/` 编译通过。

**依赖**: Step 2（需要 `runtime.SessionExt` 接口）。

---

### Step 4: 更新 `CodingApplication` — `internal/agents/coding/application.go`

**文件**: `internal/agents/coding/application.go`（修改）

**变更**:

1. `CodingApplication` 需要持有 `Config`（用于 `BuildTools` 内部读取 `EnableBash`）：

```go
type CodingApplication struct {
    Cfg Config
}

func NewCodingApplication(cfg Config) CodingApplication {
    return CodingApplication{Cfg: cfg}
}
```

2. `BuildTools`: 内部读取 `cfg.EnableBash`，不再从 `ToolBuildOptions` 获取：

```go
func (a CodingApplication) BuildTools(opts runtime.ToolBuildOptions) []agent.Tool {
    return codingtools.BuildList(codingtools.ListOptions{
        Workspace:      opts.Workspace,
        MaxOutputLen:   opts.MaxOutputLen,
        EnableBash:     a.Cfg.EnableBash,   // 从自身 Config 读取
        BashOps:        opts.BashOps,
        FileOps:        opts.FileOps,
        ExtensionTools: opts.ExtensionTools,
        AllowedTools:   opts.AllowedTools,
        BlockedTools:   opts.BlockedTools,
    })
}
```

3. `BuildPrompt`: 直接接收 Profile/Goal 参数，不从 PromptBuildOptions 读取：

```go
func (a CodingApplication) BuildPrompt(opts runtime.PromptBuildOptions, profile, goal string) string {
    return codingprompt.BuildSystemPrompt(codingprompt.Options{
        CustomPrompt: opts.CustomPrompt,
        CWD:          opts.CWD,
        Tools:        opts.Tools,
        ContextFiles: opts.ContextFiles,
        Skills:       opts.Skills,
        Profile:      profile,
        Goal:         goal,
    })
}
```

4. 新增 `NewSessionExt`:

```go
func (a CodingApplication) NewSessionExt() *CodingSessionExt {
    return NewCodingSessionExt(nil) // rebuild callback set later by AgentSession
}
```

**等等 — 这里有一个接口签名问题**。`Application` 接口要求 `NewSessionExt(cfg Config) SessionExt`（返回 `SessionExt` 接口），但 `CodingSessionExt` 需要在创建后注入 `RebuildFunc`。解决方案：使用 setter 方法。

```go
func (a CodingApplication) NewSessionExt(cfg Config) runtime.SessionExt {
    return NewCodingSessionExt(nil) // rebuild set via SetRebuild
}
```

`CodingSessionExt` 新增：

```go
func (e *CodingSessionExt) SetRebuild(fn RebuildFunc) {
    e.rebuild = fn
}
```

**Application 接口的 Config 参数问题**: 重新考虑——`NewSessionExt` 不需要 `Config`。`CodingSessionExt` 不直接读 Config。Config 只在 `BuildTools`/`BuildPrompt` 时通过 `ToolBuildOptions`/`PromptBuildOptions` 传入。修正接口：

```go
type Application interface {
    BuildTools(opts ToolBuildOptions) []agent.Tool
    BuildPrompt(opts PromptBuildOptions) string
    NewSessionExt() SessionExt
}
```

**验证**: `go build ./internal/agents/coding/` 编译通过。

**依赖**: Step 2, Step 3。

---

### Step 5: 重构 `AgentSession` — `internal/runtime/agent_session.go`

**文件**: `internal/runtime/agent_session.go`（修改）

这是最核心的步骤。需要做以下变更：

#### 5a. 移除 profile/goal 字段，添加 ext 字段

```go
type AgentSession struct {
    agent       *agent.Agent
    session     *session.Session
    sessionID   string
    sessionMgr  *sessionmgr.Manager
    cfg         config.Config
    extRegistry *extensions.Registry
    sessionPath string
    deps        Dependencies
    skillDirs   []string
    application Application
    ext         SessionExt  // 新增：per-session 应用扩展
    // profile, goal 字段删除
}
```

#### 5b. 在 `NewAgentSession` 中创建 SessionExt

```go
func NewAgentSession(ctx context.Context, opts AgentSessionOptions, deps Dependencies) (*AgentSession, error) {
    s := &AgentSession{
        cfg:         opts.Config,
        sessionMgr:  deps.SessionMgr,
        extRegistry: deps.ExtRegistry,
        deps:        deps,
        skillDirs:   opts.SkillDirs,
    }

    // 创建 per-session 扩展
    if deps.Application != nil {
        s.ext = deps.Application.NewSessionExt()
        if cse, ok := s.ext.(interface{ SetRebuild(func() error) }); ok {
            cse.SetRebuild(func() error {
                _, err := s.rebuildAgent(ctx, s.deps.Registry, s.skillDirs)
                return err
            })
        }
    }

    // 移除: s.profile = "coding"

    // ... 其余不变（session 创建/加载逻辑）
}
```

#### 5c. 添加 Profile()/Goal() 的 ext 代理方法

保持 `AgentSession` 上有 `Profile()`、`Goal()` 方法，但委托给 ext：

```go
func (s *AgentSession) Profile() string {
    if s.ext != nil {
        return s.ext.Profile()
    }
    return ""
}

func (s *AgentSession) Goal() string {
    if s.ext != nil {
        return s.ext.Goal()
    }
    return ""
}

func (s *AgentSession) SwitchProfile(ctx context.Context, profile string) error {
    if s.ext == nil {
        return fmt.Errorf("profile switching not supported")
    }
    return s.ext.SwitchProfile(ctx, profile)
}

func (s *AgentSession) SetGoal(goal string) {
    if s.ext == nil {
        return
    }
    s.ext.SetGoal(goal)
}

func (s *AgentSession) ClearGoal() {
    if s.ext == nil {
        return
    }
    s.ext.ClearGoal()
}
```

**为什么保留这些方法在 AgentSession 上**：`slashcmd.SessionContext` 需要它们，`cli/interactive.go` 也直接调用 `m.session.Profile()`。保留代理方法避免大面积改动调用方。如果未来要完全移除，需要先让所有调用方通过 type assertion 使用 `ProfileSupport`/`GoalSupport` 子接口——这是后续优化。

#### 5d. 移除 `buildOperations` 方法，将 operations 构建移到 `toolBuildOptions`

`buildOperations` 方法（`agent_session.go:409-420`）的功能需要由调用方提供。将 `BashOps` 和 `FileOps` 通过 `Dependencies` 或 `Application` 传入。

**方案**：在 `Dependencies` 中新增 `OperationsBuilder` 回调：

```go
type Dependencies struct {
    Registry          *providers.Registry
    SessionMgr        *sessionmgr.Manager
    ExtRegistry       *extensions.Registry
    Application       Application
    BuildOperations   func(cfg config.Config, workspace string) *operations.Operations
}
```

`app.deps()` 中设置 `BuildOperations`:

```go
func (a *App) deps() runtime.Dependencies {
    return runtime.Dependencies{
        Registry:    a.registry,
        SessionMgr:  a.sessionMgr,
        ExtRegistry: a.extRegistry,
        Application: coding.NewCodingApplication(a.cfg),
        BuildOperations: func(cfg config.Config, workspace string) *operations.Operations {
            switch cfg.ExecutionMode {
            case "ssh":
                return operations.NewSSHOperations(operations.SSHConfig{
                    Host:    cfg.SSHHost,
                    Port:    cfg.SSHPort,
                    WorkDir: cfg.SSHWorkDir,
                })
            default:
                return operations.NewLocalOperations()
            }
        },
    }
}
```

`toolBuildOptions` 中调用 `BuildOperations`:

```go
func (s *AgentSession) toolBuildOptions(cwd string) ToolBuildOptions {
    cfg := s.cfg
    workspace := cfg.Workspace
    if workspace == "" {
        workspace = cwd
    }

    var ops *operations.Operations
    if s.deps.BuildOperations != nil {
        ops = s.deps.BuildOperations(cfg, workspace)
    } else {
        ops = operations.NewLocalOperations()
    }

    if cfg.ExecutionMode == "ssh" && cfg.SSHWorkDir != "" {
        workspace = cfg.SSHWorkDir
    }

    var extTools []agent.Tool
    if s.extRegistry != nil {
        extTools = s.extRegistry.Tools()
    }

    return ToolBuildOptions{
        Workspace:      workspace,
        MaxOutputLen:   cfg.MaxOutputLen,
        BashOps:        ops.Bash,
        FileOps:        ops.Files,
        ExtensionTools: extTools,
        AllowedTools:   cfg.AllowedTools,
        BlockedTools:   cfg.BlockedTools,
    }
}
```

**删除** `buildOperations` 方法（原 `agent_session.go:409-420`）。

#### 5e. 替换 `contextWindowForModel` 调用

```go
import "github.com/earendil-works/pi-go/internal/ai/models"

// buildAgent 中:
ContextWindow: models.ContextWindow(modelID),
```

**删除** `contextWindowForModel` 函数（原 `agent_session.go:422-445`）。

#### 5f. `buildAgent` 中 Profile/Goal 从 ext 读取

```go
func (s *AgentSession) buildAgent(ctx context.Context, registry *providers.Registry, skillDirs []string) (*agent.Agent, error) {
    // ... 工具构建和模型构建不变 ...

    // 从 ext 读取 profile/goal
    var profile, goal string
    if s.ext != nil {
        profile = s.ext.Profile()
        goal = s.ext.Goal()
    }

    // Build system prompt
    systemPrompt := s.application.BuildPrompt(PromptBuildOptions{
        CustomPrompt: cfg.PromptTemplate,
        CWD:          cwd,
        Tools:        toolList,
        ContextFiles: contextFiles,
        Skills:       skills,
    }, profile, goal)
    // ...
}
```

**验证**:
- `go build ./internal/runtime/` 编译通过
- `go vet ./internal/runtime/` 无警告
- 运行现有测试（部分可能失败，Step 6 修复）

**依赖**: Step 1, Step 2, Step 3, Step 4。

---

### Step 6: 拆分 `slashcmd.SessionContext` 接口 — `internal/slashcmd/context.go`

**文件**: `internal/slashcmd/context.go`（修改）

**变更**:

```go
// SessionContext is the base interface for session operations available to slash commands.
type SessionContext interface {
    SessionID() string
    ModelInfo() (provider string, modelID string)
    SwitchModel(ctx context.Context, modelID string, provider string) error
    ToolNames() []string
    Compact(ctx context.Context, customInstructions string) (summary string, trimmedFrom int, trimmedTo int, err error)
}

// ProfileSupport is an optional extension for applications that support profiles.
type ProfileSupport interface {
    Profile() string
    SwitchProfile(ctx context.Context, profile string) error
}

// GoalSupport is an optional extension for applications that support session goals.
type GoalSupport interface {
    Goal() string
    SetGoal(goal string)
    ClearGoal()
}
```

**注意**：`AgentSession` 仍保留 `Profile()`/`Goal()`/`SwitchProfile()`/`SetGoal()`/`ClearGoal()` 方法（代理到 ext），所以它同时满足 `SessionContext`、`ProfileSupport` 和 `GoalSupport`。向后兼容——旧的 `ctx.Session.Profile()` 调用仍然可以工作，因为 `AgentSession` 实现了所有方法。

但等等——如果 `SessionContext` 不再包含 `Profile()` 等方法，那么 `builtins.go` 中的 `ctx.Session.Profile()` 编译会失败。

**解决方案 A**（推荐）：**不拆分 `SessionContext`**，保持当前完整接口。`SessionExt` 机制已经解决了 Platform 层的语义泄露（`AgentSession` 中的 profile/goal 字段被移到 ext）。`slashcmd.SessionContext` 是给 command handler 用的 convenience 接口，它依赖 `AgentSession` 的代理方法，不存在"非 coding agent 实现不了"的问题——因为任何不需要 profile/goal 的 agent 的 `AgentSession` 代理方法会返回空值。

**决策**：保留 `SessionContext` 完整接口不变。理由：
1. `AgentSession` 上的代理方法保证了所有 session 都满足完整接口
2. 拆分接口增加 type assertion 复杂度，但实际收益低——所有 slash command 都注册在特定 application 下
3. review 的建议（S4）提到的 `InteractiveMode` 对 `Profile()` 的直接访问也通过代理方法解决

**验证**: `go build ./internal/slashcmd/` 编译通过。

**依赖**: Step 5。

---

### Step 7: 更新 `app.App` — `internal/app/app.go`

**文件**: `internal/app/app.go`（修改）

**变更**:

#### 7a. `AppOptions` 新增 `Application` 字段

```go
type AppOptions struct {
    Config      config.Config
    SkillDirs   []string
    Application runtime.Application  // 注入具体的 Application 实现
}
```

#### 7b. `App` 存储注入的 Application

```go
type App struct {
    cfg          config.Config
    skillDirs    []string
    sessionMgr   *sessionmgr.Manager
    registry     *providers.Registry
    sessionStore *runtime.SessionRegistry
    extRegistry  *extensions.Registry
    application  runtime.Application  // 新增
}
```

#### 7c. `New()` 使用注入的 Application

```go
func New(opts AppOptions) (*App, error) {
    // ...
    application := opts.Application
    if application == nil {
        application = coding.NewCodingApplication(opts.Config)
    }
    return &App{
        // ...
        application: application,
    }, nil
}
```

#### 7d. `deps()` 使用存储的 Application

```go
func (a *App) deps() runtime.Dependencies {
    return runtime.Dependencies{
        Registry:    a.registry,
        SessionMgr:  a.sessionMgr,
        ExtRegistry: a.extRegistry,
        Application: a.application,
        BuildOperations: func(cfg config.Config, workspace string) *operations.Operations {
            switch cfg.ExecutionMode {
            case "ssh":
                return operations.NewSSHOperations(operations.SSHConfig{
                    Host:    cfg.SSHHost,
                    Port:    cfg.SSHPort,
                    WorkDir: cfg.SSHWorkDir,
                })
            default:
                return operations.NewLocalOperations()
            }
        },
    }
}
```

#### 7e. `ToolNames()` 委托给 Application

定义可选接口 `ToolNamer`：

```go
// ToolNamer is an optional Application interface for listing tool names.
type ToolNamer interface {
    ToolNames(enableBash bool) []string
}
```

放在 `internal/runtime/application.go` 中或直接在 `app.go` 中：

```go
func (a *App) ToolNames() []string {
    cfg := a.cfg

    // 尝试委托给 Application
    type toolNamer interface {
        ToolNames(enableBash bool) []string
    }
    var baseNames []string
    if tn, ok := a.application.(toolNamer); ok {
        baseNames = tn.ToolNames(cfg.EnableBash)
    } else {
        baseNames = coding.BaseToolNames(cfg.EnableBash)
    }

    // Extension tools
    if a.extRegistry != nil {
        for _, t := range a.extRegistry.Tools() {
            baseNames = append(baseNames, t.Name())
        }
    }

    // Apply filtering (不变)
    // ...
}
```

#### 7f. `Profiles()` 和 `AvailableModels()` 委托给 Application

定义可选接口：

```go
// ProfileLister is an optional Application interface for listing profiles.
type ProfileLister interface {
    Profiles() []string
}

// ModelLister is an optional Application interface for listing available models.
type ModelLister interface {
    AvailableModels() []slashcmd.ModelInfo
}
```

```go
func (a *App) Profiles() []string {
    type profileLister interface{ Profiles() []string }
    if pl, ok := a.application.(profileLister); ok {
        return pl.Profiles()
    }
    return nil
}

func (a *App) AvailableModels() []slashcmd.ModelInfo {
    type modelLister interface{ AvailableModels() []slashcmd.ModelInfo }
    if ml, ok := a.application.(modelLister); ok {
        return ml.AvailableModels()
    }
    return nil
}
```

#### 7g. 删除未使用的 `homeDir()` 函数

删除 `app.go:205-210`。

#### 7h. `CodingApplication` 实现可选接口

在 `internal/agents/coding/coding.go` 或新文件中添加：

```go
func (CodingApplication) Profiles() []string {
    return profile.All()
}

func (CodingApplication) AvailableModels() []slashcmd.ModelInfo {
    return []slashcmd.ModelInfo{
        {Provider: "anthropic", ModelID: "claude-sonnet-4-6"},
        {Provider: "anthropic", ModelID: "claude-sonnet-4-5"},
        {Provider: "anthropic", ModelID: "claude-sonnet-4"},
        {Provider: "openai", ModelID: "gpt-4o"},
        {Provider: "openai", ModelID: "gpt-4o-mini"},
        {Provider: "deepv", ModelID: "glm-5"},
        {Provider: "deepv", ModelID: "deepseek-v4-flash"},
        {Provider: "deepv", ModelID: "deepseek-v4-pro"},
        {Provider: "deepv", ModelID: "kimi-k2.6"},
        {Provider: "mock", ModelID: "mock"},
    }
}

func (CodingApplication) ToolNames(enableBash bool) []string {
    return codingtools.BaseToolNames(enableBash)
}
```

#### 7i. `registerProviders` 中的 DeepV 逻辑保持不变

`app.go:169-202` 中的 `registerProviders` 依赖 `coding.NewDeepVHeaderProvider`，这是 coding-agent 特有的。暂时保留——后续如果要做非 coding Application，可以将 provider 注册也移到 Application 层。

**验证**: `go build ./internal/app/` 编译通过。

**依赖**: Step 2, Step 4, Step 5。

---

### Step 8: 更新 `cmd/pi-agent/main.go`

**文件**: `cmd/pi-agent/main.go`（修改）

**变更**: 显式注入 `CodingApplication`：

```go
import (
    "github.com/earendil-works/pi-go/internal/agents/coding"
    // ...
)

func main() {
    // ...
    application, err := app.New(app.AppOptions{
        Config:      cfg,
        SkillDirs:   skillDirs(*skillDir),
        Application: coding.NewCodingApplication(cfg),
    })
    // ...
}
```

**验证**: `go build ./cmd/pi-agent/` 编译通过。

**依赖**: Step 7。

---

### Step 9: 更新 Entrypoints 层 — `internal/mode/`

**文件**: `internal/mode/interactive.go`（修改）

**变更**：保留 type alias（因为它确实只是 coding CLI 的薄包装），但更新注释：

```go
// InteractiveMode is the interactive CLI entrypoint.
// Currently wraps the coding-agent CLI; will become mode-agnostic when
// multiple Applications exist.
type InteractiveMode = codingcli.InteractiveMode
```

函数签名不变——它已经接受 `*runtime.AgentSession` 和 `*app.App`。

**文件**: `internal/mode/serve.go`（无变更）

`ServeMode` 已经接受 `*app.App`，无需修改。

**文件**: `internal/mode/print.go`（无变更）

`PrintMode` 只接受 `*runtime.AgentSession`，无需修改。

**验证**: `go build ./internal/mode/` 编译通过。

**依赖**: Step 8。

---

### Step 10: 更新注释 — `internal/server/server.go`

**文件**: `internal/server/server.go`（修改）

**变更**:

```go
// Line 20: 注释更新
// Server provides HTTP REST + SSE endpoints for the agent.
```

**文件**: `internal/server/websocket.go`（无代码变更）

`websocket.go:251` 中的 `sess.SwitchModel` 调用保留——SwitchModel 这次不动（见"不做的事"第1条）。将 `websocket.go` 加入变更文件列表仅为了确认审查。

**验证**: `go build ./internal/server/` 编译通过。

**依赖**: Step 8。

---

### Step 11: 更新 `InteractiveMode` 中的 Profile 访问 — `internal/agents/coding/cli/interactive.go`

**文件**: `internal/agents/coding/cli/interactive.go`（无变更）

`interactive.go:47,86` 调用 `m.session.Profile()` — 由于 `AgentSession` 保留了代理方法（Step 5c），这些调用无需修改。

**验证**: `go build ./internal/agents/coding/cli/` 编译通过。

**依赖**: Step 5。

---

### Step 12: 更新测试 — slashcmd

**文件**: `internal/slashcmd/registry_test.go`（修改）

**变更**: `mockSessionContext` 需要保留所有方法（因为 `SessionContext` 接口保持不变，Step 6 决策）。

无变更需要。✅

**文件**: `internal/agents/coding/commands/builtins_test.go`（修改）

**变更**: `mockSession` 需要保留所有方法。无变更需要。✅

**验证**: `go test ./internal/slashcmd/ ./internal/agents/coding/commands/ -v` 全部通过。

**依赖**: Step 6。

---

### Step 13: 新增测试

**文件**: `internal/ai/models/registry_test.go`（新建）

```go
package models

import "testing"

func TestContextWindow_KnownModels(t *testing.T) {
    tests := []struct {
        model string
        want  int
    }{
        {"claude-sonnet-4-6", 200000},
        {"claude-sonnet-4-5", 200000},
        {"claude-3-5-sonnet", 200000},
        {"gpt-4o", 128000},
        {"gpt-4", 8192},
        {"o1", 200000},
        {"glm-5", 128000},
    }
    for _, tt := range tests {
        got := ContextWindow(tt.model)
        if got != tt.want {
            t.Errorf("ContextWindow(%q) = %d, want %d", tt.model, got, tt.want)
        }
    }
}

func TestContextWindow_UnknownModel(t *testing.T) {
    got := ContextWindow("nonexistent-model-v1")
    if got != 128000 {
        t.Errorf("ContextWindow(unknown) = %d, want 128000", got)
    }
}
```

**文件**: `internal/agents/coding/session_ext_test.go`（新建）

```go
package coding

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCodingSessionExt_DefaultProfile(t *testing.T) {
    ext := NewCodingSessionExt(nil)
    assert.Equal(t, "coding", ext.Profile())
}

func TestCodingSessionExt_SwitchProfile_Valid(t *testing.T) {
    rebuildCalled := false
    ext := NewCodingSessionExt(func() error {
        rebuildCalled = true
        return nil
    })
    err := ext.SwitchProfile(context.Background(), "review")
    require.NoError(t, err)
    assert.Equal(t, "review", ext.Profile())
    assert.True(t, rebuildCalled)
}

func TestCodingSessionExt_SwitchProfile_Invalid(t *testing.T) {
    ext := NewCodingSessionExt(nil)
    err := ext.SwitchProfile(context.Background(), "nonexistent")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "unknown profile")
    assert.Equal(t, "coding", ext.Profile()) // 未变
}

func TestCodingSessionExt_Goal(t *testing.T) {
    rebuildCalled := false
    ext := NewCodingSessionExt(func() error {
        rebuildCalled = true
        return nil
    })
    assert.Equal(t, "", ext.Goal())

    ext.SetGoal("fix auth bug")
    assert.Equal(t, "fix auth bug", ext.Goal())
    assert.True(t, rebuildCalled)

    ext.ClearGoal()
    assert.Equal(t, "", ext.Goal())
}
```

**验证**: `go test ./internal/ai/models/ ./internal/agents/coding/ -v` 全部通过。

**依赖**: Step 1, Step 3。

---

### Step 14: 全量编译和测试验证

```bash
go build ./...
go vet ./...
go test ./...
```

修复任何遗漏的编译错误或测试失败。

**依赖**: All previous steps.

---

## 4. 测试策略

### 新增测试

| 测试文件 | 覆盖内容 |
|---------|---------|
| `internal/ai/models/registry_test.go` | `ContextWindow()` 对已知/未知模型的返回值 |
| `internal/agents/coding/session_ext_test.go` | `CodingSessionExt` 的 profile/goal 状态管理、rebuild 回调触发、无效 profile 拒绝 |

### 现有测试影响

| 测试文件 | 影响 | 需要改动 |
|---------|------|---------|
| `internal/slashcmd/registry_test.go` | `SessionContext` 接口不变 | ❌ 无需改动 |
| `internal/agents/coding/commands/builtins_test.go` | `SessionContext` 接口不变 | ❌ 无需改动 |
| `internal/runtime/session_registry_test.go` | `Dependencies` 新增 `BuildOperations` 字段 | ✅ 需检查（`NewAgentSession` 的 `deps` 构造可能需要更新，但这些测试不创建完整 session） |
| `internal/app/` 测试 | `AppOptions` 新增 `Application` 字段 | ✅ 需检查是否有 app-level 测试 |

### 集成验证

- `go build ./cmd/pi-agent/` 编译通过
- `go test ./...` 全部通过
- 手动验证：`./pi-agent -mode run -prompt "hello"` 运行正常

## 5. 迁移注意

### 零破坏性变更

本次重构对所有外部调用方是**向后兼容**的：

1. **`AgentSession` 的 `Profile()`/`Goal()`/`SwitchProfile()`/`SetGoal()`/`ClearGoal()` 方法保留**，只是内部代理到 `ext`。所有调用方无需修改。
2. **`Application` 接口新增 `NewSessionExt()` 方法**——这是一个 breaking change，但当前只有 `CodingApplication` 实现了该接口，且实现由我们控制。
3. **`ToolBuildOptions` 移除 `EnableBash`**——当前只有 `CodingApplication.BuildTools` 消费该字段，由我们控制。
4. **`PromptBuildOptions` 的 `Profile`/`Goal` 字段移除**——改为 `BuildPrompt` 的额外参数。当前只有 `AgentSession.buildAgent` 调用 `BuildPrompt`，由我们控制。
5. **`Dependencies` 新增 `BuildOperations`**——由 `app.deps()` 设置，内部代码。

### 顺序依赖

实施必须按 Step 顺序进行。核心依赖链：

```
Step 1 (models/registry.go)
Step 2 (SessionExt interface) ← Step 1 无依赖
Step 3 (CodingSessionExt) ← Step 2
Step 4 (CodingApplication update) ← Step 2, 3
Step 5 (AgentSession refactor) ← Step 1, 2, 4  [核心步骤]
Step 6 (SessionContext decision) ← Step 5
Step 7 (App update) ← Step 2, 4, 5
Step 8 (main.go) ← Step 7
Step 9 (mode/) ← Step 8
Step 10 (server/) ← Step 8
Step 11 (cli/) ← Step 5
Step 12 (test updates) ← Step 6
Step 13 (new tests) ← Step 1, 3
Step 14 (full validation) ← All
```

建议在 Step 5 完成后做一次中间编译验证，因为 Step 5 是改动最大的文件。

### Review 阻塞项解决确认

| 阻塞项 | 解决方案 | 对应 Step |
|--------|---------|----------|
| **B1**: SessionExt 不能放 Dependencies | 通过 `Application.NewSessionExt()` 工厂方法创建 per-session 实例 | Step 2, 3, 5 |
| **B2**: SwitchModel 遗漏 | 在"不做的事"第1条显式说明理由 | §2 |
| **B3**: PromptBuildOptions profile/goal 填充路径 | `buildAgent` 中从 `s.ext` 读取，传给 `BuildPrompt` 的额外参数 | Step 5f |

### Review 建议采纳确认

| 建议 | 采纳情况 | 对应 Step |
|------|---------|----------|
| **S1**: Profiles/AvailableModels 委托 | ✅ 通过可选接口委托 | Step 7f |
| **S2**: ToolNames 委托 | ✅ 通过可选接口委托 | Step 7e |
| **S3**: WebSocket SwitchModel | ✅ 确认不受影响（SwitchModel 不动） | §2 |
| **S4**: InteractiveMode Profile() 访问 | ✅ AgentSession 保留代理方法，无需改动 | Step 11 |
| **S5**: 测试文件影响评估 | ✅ 确认无需改动（SessionContext 接口不变） | Step 12 |
