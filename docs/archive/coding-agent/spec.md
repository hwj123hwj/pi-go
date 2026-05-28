---
status: approved
author: plan-agent
created: 2026-05-22
updated: 2026-05-24
---

# Coding Agent 实现规格

> 目标：在现有通用 Agent 框架之上，实现完整的 coding-agent 应用层。  
> 本文档供执行 agent 使用，包含目录结构、核心抽象、接口定义和实现要求。  
> 重要参考实现：`/Users/weijian/Desktop/develop/test/pi/packages/coding-agent`

> 说明：本规格聚焦 `pi-go` 的共用内核与应用层抽象，不直接规划产品 UI。  
> 当前产品主线以 `CLI` 为开发者主入口，`Desktop` 为可视化工作台，`Server + Feishu` 为服务化入口；不再单独规划 `TUI` 产品线。

## 0. 设计原则

这份规格不是简单给 `pi-go` 补一些命令和工具，而是要让它在架构上逐步对齐参考实现 `packages/coding-agent` 的核心思路。

最重要的设计判断：

1. **核心抽象不是 `App`，而是 `AgentSession` 式运行时**
   - 参考实现真正的中心在 `src/core/agent-session.ts`
   - 它统一承接 agent 生命周期、会话、事件、compaction、tree navigation、mode 复用
   - Go 版也应先建立这一层，再让 `app`、`mode`、`slashcmd`、`server` 围绕它展开

2. **`sessionmgr` 负责“持久化与索引”，不负责“运行时行为”**
   - `sessionmgr` 应管理 session 文件、header、list、fork、load、delete
   - 但 prompt、stream、切 branch、compact、切 model、slash command 上下文，都应属于运行时对象

3. **`server` 不能再只持有一个全局 `*agent.Agent`**
   - HTTP 模式必须支持 `session_id -> runtime` 的路由
   - 否则 session API 只是“能列目录”，真正聊天仍然共用一个 agent 状态

4. **扩展系统先做“进程内注册 + hook/event 模型”，不要把 `.so` 当第一优先级**
   - 参考实现的价值不在动态库，而在 runtime/event bus/resource loader 这一套组合能力
   - MVP 阶段优先保证扩展能注册工具、命令和钩子

5. **终端场景由 CLI 承担，不额外引入独立 TUI 抽象**
   - `mode/interactive` 的目标是成为强 CLI，而不是演进出一套单独终端 UI 产品
   - 如果后续需要更丰富的终端展示，也应作为 CLI 呈现层增强，而不是另起产品线

## 1. 当前已有（不改或微调）

| 包 | 路径 | 说明 |
|---|---|---|
| ai 抽象层 | `internal/ai/` | 多 Provider LLM API，已完成 |
| agent 运行时 | `internal/agent/` | 循环、Tool 接口、状态机、事件、busy 检测，已完成 |
| session 存储 | `internal/session/` | JSONL 树状存储、leaf、MoveTo，已完成 |
| compaction | `internal/compaction/` | 上下文压缩，已完成 |
| server | `internal/server/` | HTTP REST + WebSocket，多 session 路由是主要求 |
| config | `internal/config/` | 配置加载，需扩展 |
| skill | `internal/skill/` | SKILL.md 加载，已完成 |
| prompt | `internal/prompt/` | 系统提示构建，需增强 coding 场景 |
| tools（基础版） | `internal/tools/` | 7 个工具已存在，需增强 |
| CLI 入口 | `cmd/pi-agent/main.go` | serve/chat/run 三种模式，CLI 是终端主入口 |

## 2. 目标目录结构

```
pi-go/
├── cmd/
│   └── pi-agent/
│       └── main.go                    # 薄入口：解析参数，创建 app，进入 mode
│
├── internal/
│   ├── ai/                            # [不改]
│   ├── agent/                         # [不改]
│   ├── session/                       # [不改] 底层 JSONL storage
│   ├── compaction/                    # [不改]
│   ├── config/                        # [微调]
│   ├── prompt/                        # [微调]
│   ├── skill/                         # [不改]
│   │
│   ├── app/                           # [新建] 应用装配层（薄）
│   │   ├── app.go
│   │   └── app_test.go
│   │
│   ├── runtime/                       # [新建] 核心运行时（最重要）
│   │   ├── agent_session.go           # AgentSession：统一生命周期、prompt、branch、compact
│   │   ├── session_registry.go        # session_id -> runtime 的管理与复用
│   │   └── agent_session_test.go
│   │
│   ├── sessionmgr/                    # [新建] 会话持久化与索引
│   │   ├── manager.go
│   │   └── manager_test.go
│   │
│   ├── tools/                         # [增强] 7 个内置工具
│   │   ├── bash.go
│   │   ├── read.go
│   │   ├── write.go
│   │   ├── edit.go
│   │   ├── grep.go
│   │   ├── find.go
│   │   ├── ls.go
│   │   ├── truncate.go
│   │   ├── path.go
│   │   └── tools_test.go
│   │
│   ├── extensions/                    # [新建] 扩展注册与事件钩子
│   │   ├── types.go
│   │   ├── registry.go
│   │   ├── hooks.go
│   │   └── extensions_test.go
│   │
│   ├── slashcmd/                      # [新建] 斜杠命令
│   │   ├── registry.go
│   │   ├── context.go                 # 命令上下文，绑定 runtime/app 能力
│   │   └── builtins.go
│   │
│   ├── mode/                          # [新建] 运行模式（CLI / print / serve）
│   │   ├── interactive.go
│   │   ├── print.go
│   │   └── serve.go
│   │
│   └── util/                          # [新建] 通用工具
│       ├── git.go
│       └── shell.go
│
└── docs/
    └── archive/
```

## 3. 核心抽象与职责边界

### 3.1 `internal/runtime/agent_session.go` — 核心运行时

**职责**：这是 Go 版 coding-agent 的中心抽象，对齐参考实现的 `AgentSession` 思路。  
它统一承接：

- 底层 `agent.Agent`
- 当前 session
- prompt / prompt stream
- branch/tree navigation
- compaction
- model / thinking level 之类的运行时设置
- 斜杠命令所需的操作上下文
- mode 间复用

这层能力要同时服务于：

- `CLI` 交互模式
- `Desktop` 通过 server/websocket 的访问
- `Server + Feishu` 的服务化调用

```go
package runtime

type AgentSession struct {
    agent          *agent.Agent
    session        *session.Session
    sessionID      string
    sessionMgr     *sessionmgr.Manager
    cfg            config.Config
    slashCtx       *slashcmd.Context
    extRegistry    *extensions.Registry
}

type AgentSessionOptions struct {
    SessionID   string
    Config      config.Config
    SkillDirs   []string
}

func NewAgentSession(opts AgentSessionOptions, deps Dependencies) (*AgentSession, error)
func (s *AgentSession) Prompt(ctx context.Context, input string) (ai.AssistantMessage, error)
func (s *AgentSession) PromptStream(ctx context.Context, input string) (<-chan agent.AgentStreamEvent, error)
func (s *AgentSession) SessionID() string
func (s *AgentSession) Session() *session.Session
func (s *AgentSession) Agent() *agent.Agent
func (s *AgentSession) Compact(ctx context.Context, reason string) error
func (s *AgentSession) MoveTo(ctx context.Context, entryID string, summary string) error
func (s *AgentSession) Close() error
```

**实现要求**：
- 这里承接当前 `main.go` 里的 `buildAgent()` 主逻辑
- `interactive`、`print`、`serve` 三种 mode 都只能依赖这个对象，不直接拼装 `agent.Agent`
- slash command 的处理上下文也从这里拿，不在 mode 里各自实现
- 如果后面要支持模型切换、branch summary、tree view，这一层必须能承接

### 3.2 `internal/runtime/session_registry.go` — 会话运行时注册表

**职责**：解决 HTTP 模式和多 session 模式下，`session_id -> runtime` 的路由问题。

```go
package runtime

type SessionRegistry struct {
    mu       sync.Mutex
    sessions map[string]*AgentSession
}

func NewSessionRegistry() *SessionRegistry
func (r *SessionRegistry) Get(id string) (*AgentSession, bool)
func (r *SessionRegistry) Create(ctx context.Context, opts AgentSessionOptions, deps Dependencies) (*AgentSession, error)
func (r *SessionRegistry) Load(ctx context.Context, id string, opts AgentSessionOptions, deps Dependencies) (*AgentSession, error)
func (r *SessionRegistry) Delete(id string) error
```

**实现要求**：
- `serve` 模式不能继续只持有一个全局 `*agent.Agent`
- 每个会话应对应自己的 `AgentSession`
- REST/SSE 请求必须明确绑定 session
- 如果请求没带 session id，可由服务端创建并返回

## 4. 应用层与会话层

### 4.1 `internal/app/app.go` — 薄装配层

**职责**：组装依赖，不承载具体会话行为。

```go
package app

type App struct {
    cfg          config.Config
    sessionMgr   *sessionmgr.Manager
    registry     *providers.Registry
    sessionStore *runtime.SessionRegistry
}

type AppOptions struct {
    Config    config.Config
    SkillDirs []string
}

func New(opts AppOptions) (*App, error)
func (a *App) NewSession(ctx context.Context, sessionID string) (*runtime.AgentSession, error)
func (a *App) LoadSession(ctx context.Context, sessionID string) (*runtime.AgentSession, error)
func (a *App) SessionManager() *sessionmgr.Manager
func (a *App) Close() error
```

**实现要求**：
- `App` 负责 provider、sessionmgr、runtime registry、技能目录等依赖装配
- 但不再自己暴露一个唯一 `agent.Agent`
- `main.go` 和 `serve.go` 以后都通过 `App` 创建/加载 `AgentSession`

### 4.2 `internal/sessionmgr/manager.go` — 会话持久化与索引

**职责**：管理 session 文件、header、fork、list、delete。  
这里是存储与索引层，不是 prompt/runtime 层。

```go
package sessionmgr

type Manager struct {
    dataDir string
}

func NewManager(dataDir string) *Manager
func (m *Manager) Create(ctx context.Context) (string, string, error) // id, sessionFile
func (m *Manager) Open(ctx context.Context, id string) (*session.Session, string, error)
func (m *Manager) Fork(ctx context.Context, sourceID string, entryID string) (string, string, error)
func (m *Manager) List(ctx context.Context) ([]SessionInfo, error)
func (m *Manager) Delete(id string) error
```

```go
type SessionInfo struct {
    ID           string
    CreatedAt    int64
    MessageCount int
    LastActive   int64
}
```

**实现要求**：
- 会话路径：`{dataDir}/sessions/{sessionID}/session.jsonl`
- `Create` 不仅创建目录，也要初始化 storage
- `MessageCount` 必须按 `EntryTypeMessage` 计数，不能简单按 JSONL 行数
- `Fork` 基于现有 session 文件复制，再用 `MoveTo(entryID, "")` 切到分支点
- 后续如果加入 header / version，也由这层负责初始化

## 5. 工具增强

### 5.1 `truncate.go`

```go
package tools

const DefaultMaxOutputLen = 30000
func TruncateOutput(output string, maxLen int) string
```

**实现要求**：
- 默认最大 30000 字符
- 保留前 80% 和后 20%
- 中间插入 `\n... [truncated N characters] ...\n`
- 所有工具最终输出前都调用

### 5.2 `path.go`

```go
package tools

func ResolvePath(workspace, path string) string
func IsPathSafe(workspace, path string) bool
```

**实现要求**：
- 所有文件工具都走这层，不要各写一套 `filepath.Clean`
- `workspace` 为空时可退化为当前行为
- 一旦启用 workspace，读写编辑必须做路径越界检查

### 5.3 各工具增强点

**bash.go**：
- 截断输出
- ANSI 清理
- 二进制输出检测
- 支持 workspace

**read.go**：
- 输出带行号
- 支持相对路径解析
- 超大文件提示剩余行数

**write.go**：
- 写入前检查路径安全
- 返回字节数与行数

**edit.go**：
- `replace_all`
- 返回替换次数
- 输出 diff 上下文

**grep.go**：
- 忽略大小写
- 二进制文件跳过
- 匹配行标记

**find.go**：
- `type=f/d`
- `pattern` 匹配
- 默认跳过 `.git`、`node_modules`、`vendor`、`__pycache__`

**ls.go**：
- `-l`
- `-R`

## 6. 斜杠命令

### 6.1 `internal/slashcmd/context.go`

参考实现里 slash command 不只是字符串命令表，而是运行时控制入口。  
Go 版也要先把命令上下文抽出来：

```go
package slashcmd

type Context struct {
    Session      *runtime.AgentSession
    App          *app.App
}
```

### 6.2 `internal/slashcmd/registry.go`

```go
package slashcmd

type Registry struct {
    commands map[string]Command
}

type Command struct {
    Name        string
    Description string
    Handler     func(ctx context.Context, cmdCtx *Context, args string) (string, error)
}

func NewRegistry() *Registry
func (r *Registry) Register(cmd Command)
func (r *Registry) Execute(ctx context.Context, cmdCtx *Context, input string) (string, error)
func (r *Registry) Help() string
func IsSlashCommand(input string) bool
```

### 6.3 内置命令

MVP 阶段优先支持：

| 命令 | 功能 |
|---|---|
| `/help` | 列出所有命令 |
| `/compact` | 手动触发 compact |
| `/sessions` | 列出会话 |
| `/session` | 显示当前会话信息 |
| `/branch <entry_id>` | 切到指定 entry |
| `/new` | 创建新会话 |
| `/tools` | 显示当前工具 |
| `/model` | 显示当前模型信息（后续可扩展切换） |

**实现要求**：
- 命令逻辑尽量调用 `AgentSession` / `App`，不要直接操作底层文件
- 后续若要扩成 `/tree`、`/fork`、`/clone`，也应该自然落在这层

## 7. 扩展系统

### 7.1 目标

这里要对齐参考实现的思想，但不直接照搬 TypeScript 扩展运行时。

MVP 重点不是 `.so` 动态加载，而是：
- 能注册额外工具
- 能注册 slash command
- 能监听运行时事件

### 7.2 `internal/extensions/types.go`

```go
package extensions

type Extension interface {
    Name() string
    Init(ctx InitContext) error
    Tools() []agent.Tool
    Commands() []slashcmd.Command
    Hooks() []Hook
}

type InitContext struct {
    Workspace string
    Config    map[string]any
}

type Hook struct {
    Event   string
    Handler func(ctx context.Context, data any) error
}
```

### 7.3 `internal/extensions/registry.go`

```go
package extensions

type Registry struct {
    extensions []Extension
}

func NewRegistry() *Registry
func (r *Registry) Register(ext Extension)
func (r *Registry) Tools() []agent.Tool
func (r *Registry) Commands() []slashcmd.Command
func (r *Registry) EmitHook(ctx context.Context, event string, data any) error
```

**实现要求**：
- MVP 先用进程内注册表
- 不强求 `.so`
- 后续要做动态扩展时，再在注册表外层包 loader

## 8. 模式层

### 8.1 `interactive.go`

**职责**：强 CLI 交互层，不承载会话业务。

```go
type InteractiveMode struct {
    session   *runtime.AgentSession
    slashCmds *slashcmd.Registry
}

func NewInteractiveMode(session *runtime.AgentSession, cmds *slashcmd.Registry) *InteractiveMode
func (m *InteractiveMode) Run(ctx context.Context) error
```

**要求**：
- 输入 `/` 命令时走 slash registry
- 普通输入走 `AgentSession.PromptStream`
- 只负责 stdin/stdout 呈现

### 8.2 `print.go`

```go
type PrintMode struct {
    session *runtime.AgentSession
}

func NewPrintMode(session *runtime.AgentSession) *PrintMode
func (m *PrintMode) Run(ctx context.Context, prompt string) error
```

### 8.3 `serve.go`

```go
type ServeMode struct {
    app *app.App
}

func NewServeMode(app *app.App) *ServeMode
func (m *ServeMode) Run(ctx context.Context, listenAddr string) error
```

**关键要求**：
- HTTP 请求必须能指定或创建 `session_id`
- `chat` / `chat/stream` 不再直接绑定一个全局 agent
- `internal/server/` 如果保留，也应改成依赖 `App + SessionRegistry`
- 它应同时作为 `Desktop` 与 `Server + Feishu` 的共用接入底座

## 9. CLI 入口

### 9.1 `cmd/pi-agent/main.go`

目标：尽可能变薄。

```go
func main() {
    cfg := config.Default()
    _ = config.LoadDotEnv(".env")
    _ = config.LoadDotEnv(".env.local")
    cfg.LoadFromEnv()

    opts := parseFlags()

    application, err := app.New(app.AppOptions{
        Config:    cfg,
        SkillDirs: opts.SkillDirs,
    })
    if err != nil {
        fatal(err)
    }
    defer application.Close()

    switch opts.Mode {
    case "interactive", "chat":
        sess, err := application.LoadOrCreateSession(context.Background(), opts.SessionID)
        must(err)
        cmds := buildSlashRegistry(application, sess)
        must(mode.NewInteractiveMode(sess, cmds).Run(context.Background()))
    case "run":
        sess, err := application.LoadOrCreateSession(context.Background(), opts.SessionID)
        must(err)
        must(mode.NewPrintMode(sess).Run(context.Background(), opts.Prompt))
    case "serve":
        must(mode.NewServeMode(application).Run(context.Background(), opts.Listen))
    }
}
```

## 10. 配置扩展

`internal/config/config.go` 可增加以下字段：

```go
type Config struct {
    // ... existing ...
    MaxOutputLen   int
    AllowedTools   []string
    BlockedTools   []string
    HistoryFile    string
    PromptTemplate string
}
```

**要求**：
- `MaxOutputLen` 真正传入工具层
- `AllowedTools/BlockedTools` 在 `AgentSession` 构建工具集时生效
- `PromptTemplate` 在系统提示生成前生效

## 11. 实现优先级

### Phase 1：运行时骨架（必做）

1. `internal/runtime/agent_session.go`
2. `internal/runtime/session_registry.go`
3. `internal/sessionmgr/manager.go`
4. `internal/app/app.go`
5. `cmd/pi-agent/main.go` 变薄

### Phase 2：工具与命令（建议做）

6. `tools/truncate.go`
7. `tools/path.go`
8. 7 个内置工具增强
9. `slashcmd/context.go`
10. `slashcmd/registry.go`
11. `slashcmd/builtins.go`

### Phase 3：交互模式与 HTTP 路由

12. `mode/interactive.go`
13. `mode/print.go`
14. `mode/serve.go`
15. `internal/server/` 改成按 session 路由 runtime

### Phase 4：扩展能力（可选）

16. `extensions/registry.go`
17. `extensions/hooks.go`
18. `util/git.go`
19. `util/shell.go`

## 12. 测试要求

每个新建包都必须带 `*_test.go`，但测试重点要围绕真实主线：

- `runtime/agent_session_test.go`
  - prompt / stream 是否能驱动 session 持久化
  - branch / move 是否正确影响上下文

- `runtime/session_registry_test.go`
  - `session_id -> runtime` 路由是否正确

- `sessionmgr/manager_test.go`
  - Create / Open / Fork / List / Delete
  - `MessageCount` 必须按 message entry 计算

- `tools/*_test.go`
  - 路径安全
  - 输出截断
  - 大文件 / 大输出边界

- `slashcmd/registry_test.go`
  - 注册、执行、上下文绑定

最终标准：

```bash
go test ./...
```

零失败。

## 13. 约束

- **不改** `internal/agent/`、`internal/ai/`、`internal/session/`、`internal/compaction/` 的现有核心接口，除非确实被运行时抽象阻塞
- **优先围绕 `AgentSession` 式运行时设计，不要把逻辑散进 mode/app/server**
- **不把 `.so` 动态加载当 MVP 阶段目标**
- `interactive` 的增强方向是强 CLI，而不是分化出独立 TUI 产品线
- 所有工具必须满足 `agent.Tool` 接口
- 错误使用 `%w` 包装
- 尽量不引入新的外部依赖

## 14. 给执行 Agent 的一句话目标

`先把 Go 版 coding-agent 的核心抽象收敛成一个可复用的 AgentSession 运行时，再围绕它补 session 管理、工具增强、slash command 和 serve 路由，而不是继续把逻辑堆在 main.go 和 mode 里。`
