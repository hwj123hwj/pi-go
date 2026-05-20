# Coding Agent 实现规格

> 目标：在现有通用 Agent 框架之上，实现完整的 coding-agent 应用层。
> 本文档供执行 agent 使用，包含目录结构、文件清单、接口定义和实现要求。

## 1. 当前已有（不改或微调）

| 包 | 路径 | 说明 |
|---|---|---|
| ai 抽象层 | `internal/ai/` | 多 Provider LLM API，已完成 |
| agent 运行时 | `internal/agent/` | 循环、Tool 接口、状态机、事件、busy 检测，已完成 |
| session 管理 | `internal/session/` | JSONL 树状存储、分支、leaf，已完成 |
| compaction | `internal/compaction/` | 上下文压缩，已完成 |
| server | `internal/server/` | HTTP REST + SSE，已完成 |
| config | `internal/config/` | 配置加载，已完成 |
| skill | `internal/skill/` | SKILL.md 加载，已完成 |
| prompt | `internal/prompt/` | 系统提示构建，已完成 |
| tools（基础版） | `internal/tools/` | 7 个工具骨架已存在，需增强 |
| CLI 入口 | `cmd/pi-agent/main.go` | serve/chat/run 三种模式，需重构 |

## 2. 目标目录结构

```
pi-go/
├── cmd/
│   └── pi-agent/
│       └── main.go                    # 重构：薄入口，委托给 app
│
├── internal/
│   ├── ai/                            # [不改] LLM 抽象层
│   ├── agent/                         # [不改] Agent 运行时
│   ├── session/                       # [不改] 会话管理
│   ├── compaction/                    # [不改] 上下文压缩
│   ├── config/                        # [微调] 增加 coding-agent 配置项
│   ├── server/                        # [不改] HTTP 接口
│   ├── prompt/                        # [微调] 增强 coding 场景的系统提示
│   ├── skill/                         # [不改] 技能加载
│   │
│   ├── app/                           # [新建] coding-agent 应用核心
│   │   ├── app.go                     # Application 主结构：组装 agent、session、tools
│   │   └── app_test.go
│   │
│   ├── sessionmgr/                    # [新建] 会话生命周期管理
│   │   ├── manager.go                 # SessionManager：create/resume/fork/list/delete
│   │   └── manager_test.go
│   │
│   ├── tools/                         # [增强] 7 个内置工具
│   │   ├── bash.go                    # [增强] 输出截断、ANSI 清理、超时改进
│   │   ├── read.go                    # [增强] 行号格式、相对路径、截图支持
│   │   ├── write.go                   # [增强] 目录创建提示、diff 预览
│   │   ├── edit.go                    # [增强] 多处替换（replace_all）、diff 输出
│   │   ├── grep.go                    # [增强] 上下文行数、二进制文件跳过
│   │   ├── find.go                    # [增强] gitignore 支持、类型过滤
│   │   ├── ls.go                      # [增强] 递归列表、详细信息
│   │   ├── truncate.go                # [新建] 统一的输出截断工具
│   │   ├── path.go                    # [新建] 路径工具：相对路径解析、安全检查
│   │   └── tools_test.go              # [更新] 补充增强后的测试
│   │
│   ├── extensions/                    # [新建] 扩展系统
│   │   ├── types.go                   # Extension 接口定义
│   │   ├── loader.go                  # 扩展发现和加载
│   │   └── runner.go                  # 扩展执行：工具/命令/钩子
│   │
│   ├── slashcmd/                      # [新建] 斜杠命令
│   │   ├── registry.go                # 命令注册表
│   │   └── builtins.go                # 内置命令：/help, /clear, /compact, /branch
│   │
│   ├── mode/                          # [新建] 交互模式
│   │   ├── interactive.go             # 交互式 TUI 模式（改进现有 chat 模式）
│   │   ├── print.go                   # 单次执行 + 输出模式（改进现有 run 模式）
│   │   └── serve.go                   # HTTP 服务模式（改进现有 serve 模式）
│   │
│   └── util/                          # [新建] 通用工具
│       ├── git.go                     # Git 操作辅助
│       └── shell.go                   # Shell 环境检测
│
└── docs/
    └── archive/                       # 已有
```

## 3. 各模块详细规格

### 3.1 `internal/app/app.go` — 应用核心

**职责**：组装 coding-agent 的所有组件，提供统一的启动入口。把当前 `main.go` 中 `buildAgent()` 的逻辑搬到此处。

```go
package app

type App struct {
    agent       *agent.Agent
    sessionMgr  *sessionmgr.Manager
    config      config.Config
    registry    *providers.Registry
}

type AppOptions struct {
    Config     config.Config
    SessionID  string   // 空=新会话，非空=恢复会话
    SkillDirs  []string
}

// New 创建并初始化 coding-agent 应用
func New(opts AppOptions) (*App, error)

// Agent 返回底层 agent 实例
func (a *App) Agent() *agent.Agent

// Close 清理资源
func (a *App) Close() error
```

**实现要求**：
- 将 `main.go` 的 `buildAgent()` 逻辑搬到 `New()` 中
- 集成 `sessionmgr.Manager`，统一管理会话生命周期
- 加载工具时使用增强后的工具（传入 workspace、config 等参数）
- 构建 coding 场景的 system prompt

### 3.2 `internal/sessionmgr/manager.go` — 会话管理器

**职责**：管理会话的创建、恢复、分支、列表、删除。当前 server.go 里的 `listSessions`/`createSession` 等逻辑应提取到此处。

```go
package sessionmgr

type Manager struct {
    dataDir string  // 会话数据根目录，如 "./data/sessions"
}

func NewManager(dataDir string) *Manager

// Create 创建一个新会话，返回 session ID
func (m *Manager) Create(ctx context.Context) (string, error)

// Load 加载一个已有会话，返回 *session.Session
func (m *Manager) Load(ctx context.Context, id string) (*session.Session, error)

// Fork 从指定会话的某个 entry 分叉出新会话
func (m *Manager) Fork(ctx context.Context, sourceID string, entryID string) (string, error)

// List 列出所有会话的摘要信息
func (m *Manager) List(ctx context.Context) ([]SessionInfo, error)

// Delete 删除指定会话
func (m *Manager) Delete(id string) error

type SessionInfo struct {
    ID           string
    CreatedAt    int64
    MessageCount int
    LastActive   int64
}
```

**实现要求**：
- 会话存储在 `{dataDir}/{sessionID}/session.jsonl`
- ID 格式：`sess_{unix_nano}`
- `Fork` 需要复制源会话的 JSONL 文件，然后在新会话上调用 `MoveTo(entryID, "")` 创建分支点
- `List` 扫描目录，读取每个 session.jsonl 的行数作为 MessageCount

### 3.3 `internal/tools/` — 工具增强

#### 3.3.1 `truncate.go` — 统一输出截断

```go
package tools

// MaxOutputLen 工具输出的最大长度（字符数）
const MaxOutputLen = 30000

// TruncateOutput 截断过长输出，保留头部和尾部，中间插入省略标记
func TruncateOutput(output string, maxLen int) string
```

**实现要求**：
- 默认最大 30000 字符（参考 TS 原版）
- 截断时保留前 80% 和后 20%，中间插入 `\n... [truncated N characters] ...\n`
- 所有工具的 `Execute` 返回前都应调用此函数

#### 3.3.2 `path.go` — 路径工具

```go
package tools

// ResolvePath 将相对路径解析为绝对路径（基于 workspace）
func ResolvePath(workspace, path string) string

// IsPathSafe 检查路径是否在 workspace 内（防止路径穿越）
func IsPathSafe(workspace, path string) bool
```

**实现要求**：
- `ResolvePath`：如果 path 不是绝对路径，则拼接 workspace
- `IsPathSafe`：用 `filepath.Rel` 检查解析后的路径不会跳出 workspace
- 所有文件操作工具都应使用这两个函数

#### 3.3.3 各工具增强点

**bash.go 增强**：
- `Execute` 返回前截断输出
- 清理 ANSI 转义码（写一个 `StripANSI(s string) string`）
- 二进制输出检测：如果输出包含 `\x00`，返回 `[binary output, N bytes]` 而不是原始内容

**read.go 增强**：
- 输出格式改为带行号：`     1\t第一行内容`（参考 `cat -n` 格式，行号右对齐 6 位）
- 支持相对路径（通过 workspace 参数）
- 超大文件提示：如果文件行数超过 limit，末尾追加 `... (N more lines, use offset to continue reading)`

**write.go 增强**：
- 写入前检查目标路径是否在 workspace 内
- 返回写入的行数和字节数

**edit.go 增强**：
- 支持 `replace_all` 参数（bool），为 true 时不检查唯一性，替换所有匹配
- 返回替换的次数
- 输出前后各 3 行的 diff 上下文

**grep.go 增强**：
- `-i` 忽略大小写参数
- 二进制文件自动跳过
- 匹配行高亮（在输出中用 `>>` 标记匹配行）

**find.go 增强**：
- 支持 `type` 参数过滤：`f`（文件）/ `d`（目录）
- 支持 `pattern` 参数用 filepath.Match 匹配文件名
- 默认跳过 `.git`、`node_modules`、`vendor`、`__pycache__` 目录

**ls.go 增强**：
- 支持 `-l` 详细信息模式：显示大小、修改时间、权限
- 支持 `-R` 递归模式

### 3.4 `internal/extensions/` — 扩展系统

```go
package extensions

// Extension 扩展接口
type Extension interface {
    Name() string
    Init(ctx InitContext) error
    Tools() []agent.Tool          // 扩展提供的工具
    Commands() []Command          // 扩展提供的斜杠命令
    Hooks() []Hook                // 事件钩子
}

type InitContext struct {
    Workspace string
    Config    map[string]any
    Agent     *agent.Agent
}

type Command struct {
    Name        string
    Description string
    Handler     func(ctx context.Context, args string) error
}

type Hook struct {
    Event   string  // "agent:start", "turn:end", "tool:after" 等
    Handler func(ctx context.Context, data any) error
}

// Manager 扩展管理器
type Manager struct {
    extensions []Extension
}

func NewManager() *Manager

// LoadFromDir 从目录加载所有扩展
func (m *Manager) LoadFromDir(dir string) error

// AllTools 返回所有扩展提供的工具
func (m *Manager) AllTools() []agent.Tool

// AllCommands 返回所有扩展提供的命令
func (m *Manager) AllCommands() []Command

// EmitHook 触发指定事件的所有钩子
func (m *Manager) EmitHook(ctx context.Context, event string, data any) error
```

**实现要求**：
- MVP 阶段：支持从目录加载 Go plugin（`.so` 文件），或先实现为硬编码注册
- 如果 Go plugin 太复杂，可以先实现为一个简单的注册表，后续再支持动态加载

### 3.5 `internal/slashcmd/` — 斜杠命令

```go
package slashcmd

type Registry struct {
    commands map[string]Command
}

type Command struct {
    Name        string
    Description string
    Handler     func(ctx context.Context, args string) (string, error)
}

func NewRegistry() *Registry

// Register 注册一个命令
func (r *Registry) Register(cmd Command)

// Execute 执行命令，返回输出文本
func (r *Registry) Execute(ctx context.Context, input string) (string, error)

// Help 返回所有命令的帮助文本
func (r *Registry) Help() string

// IsSlashCommand 检查输入是否是斜杠命令（以 / 开头）
func IsSlashCommand(input string) bool
```

**内置命令**（在 `builtins.go` 中注册）：

| 命令 | 功能 |
|---|---|
| `/help` | 列出所有可用命令 |
| `/clear` | 清空当前会话历史 |
| `/compact` | 手动触发上下文压缩 |
| `/branch <entry_id>` | 从指定 entry 创建分支 |
| `/sessions` | 列出所有会话 |
| `/model` | 显示当前使用的模型信息 |
| `/tools` | 列出可用工具及说明 |

### 3.6 `internal/mode/` — 交互模式

#### 3.6.1 `interactive.go` — 交互式模式

**职责**：改进现有的 `runChat()` 函数，增加斜杠命令支持、更友好的输出格式。

```go
package mode

type InteractiveMode struct {
    app        *app.App
    slashCmds  *slashcmd.Registry
}

func NewInteractiveMode(app *app.App, cmds *slashcmd.Registry) *InteractiveMode

// Run 启动交互式会话循环
func (m *InteractiveMode) Run(ctx context.Context) error
```

**实现要求**：
- 从 stdin 读取输入
- 如果输入以 `/` 开头，交给 slash command registry 处理
- 否则作为用户消息发给 agent
- 流式输出（复用 agent.PromptStream）
- 显示格式改进：
  - `You>` 提示符
  - `Pi>` 前缀 + 流式文本
  - `[tool:bash]` 工具执行指示器
  - `[done]` 完成标记

#### 3.6.2 `print.go` — 单次执行模式

```go
package mode

type PrintMode struct {
    app *app.App
}

func NewPrintMode(app *app.App) *PrintMode

// Run 执行单次 prompt，输出结果到 stdout
func (m *PrintMode) Run(ctx context.Context, prompt string) error
```

#### 3.6.3 `serve.go` — HTTP 服务模式

```go
package mode

type ServeMode struct {
    app *app.App
}

func NewServeMode(app *app.App) *ServeMode

// Run 启动 HTTP 服务器
func (m *ServeMode) Run(ctx context.Context, listenAddr string) error
```

**实现要求**：
- 复用 `internal/server/` 的 Handler
- 将 sessionmgr 的 API 也暴露出去（会话列表、创建、删除）

### 3.7 `internal/util/` — 通用工具

#### 3.7.1 `git.go`

```go
package util

// CurrentBranch 返回当前 git 分支名
func CurrentBranch(dir string) string

// IsGitRepo 检查目录是否是 git 仓库
func IsGitRepo(dir string) bool

// DiffStats 返回当前未提交变更的统计
func DiffStats(dir string) (staged, unstaged int, err error)
```

#### 3.7.2 `shell.go`

```go
package util

// DetectShell 返回当前用户的默认 shell
func DetectShell() string

// IsCommandAvailable 检查命令是否可用
func IsCommandAvailable(name string) bool
```

### 3.8 `cmd/pi-agent/main.go` — 重构 CLI 入口

**重构后结构**：

```go
func main() {
    cfg := config.Load()           // 加载配置
    opts := parseFlags()           // 解析命令行参数

    application, err := app.New(app.AppOptions{
        Config:    cfg,
        SessionID: opts.SessionID,
        SkillDirs: opts.SkillDirs,
    })
    if err != nil {
        fatal(err)
    }
    defer application.Close()

    switch opts.Mode {
    case "interactive", "chat":
        mode.NewInteractiveMode(application, slashCmds).Run(ctx)
    case "run":
        mode.NewPrintMode(application).Run(ctx, opts.Prompt)
    case "serve":
        mode.NewServeMode(application).Run(ctx, opts.Listen)
    }
}
```

### 3.9 `internal/config/config.go` 微调

需要增加的配置项：

```go
type Config struct {
    // ... 现有字段 ...

    // 新增
    MaxOutputLen    int      // 工具输出截断长度，默认 30000
    AllowedTools    []string // 工具白名单，空=全部允许
    BlockedTools    []string // 工具黑名单
    HistoryFile     string   // 交互模式的历史记录文件路径
    PromptTemplate  string   // 自定义 prompt 模板文件路径
}
```

## 4. 实现优先级

### Phase 1：核心骨架（必做）

1. `internal/tools/truncate.go` — 输出截断
2. `internal/tools/path.go` — 路径解析
3. 增强 7 个现有工具（加入截断、路径安全检查等）
4. `internal/sessionmgr/manager.go` — 会话管理器
5. `internal/app/app.go` — 应用核心（组装）
6. 重构 `cmd/pi-agent/main.go`

### Phase 2：交互增强（建议做）

7. `internal/slashcmd/` — 斜杠命令系统
8. `internal/mode/interactive.go` — 改进交互模式
9. `internal/mode/print.go` — 单次执行模式
10. `internal/mode/serve.go` — HTTP 服务模式

### Phase 3：扩展能力（可选）

11. `internal/extensions/` — 扩展系统
12. `internal/util/` — Git/Shell 工具

## 5. 测试要求

每个新建包都必须有 `*_test.go`：

- `app/app_test.go`：测试 App 初始化、工具注册、session 集成
- `sessionmgr/manager_test.go`：测试 Create/Load/Fork/List/Delete
- `tools/truncate_test.go`：测试截断边界（刚好不超过、超过、空字符串）
- `tools/path_test.go`：测试相对路径解析、路径穿越检测
- `slashcmd/registry_test.go`：测试注册和执行

所有测试通过标准：`go test ./...` 零失败。

## 6. 约束

- **不改** `internal/agent/`、`internal/ai/`、`internal/session/`、`internal/compaction/` 的现有接口
- **不改** `internal/server/` 的现有路由（可新增路由）
- 所有工具必须满足 `agent.Tool` 接口
- 错误使用 `%w` 包装
- 不引入新的外部依赖（纯标准库实现）
