---
status: done
author: plan-agent
created: 2026-05-24
updated: 2026-05-25
depends-on:
  - dev/coding-agent/spec.md
  - references/skills-vs-application.md
  - research/cc-haha-core-engine-analysis.md
  - research/deepv-code-full-analysis.md
---

# Coding Agent CLI Control Plane 执行文档

> 目标：继续加厚 `coding-agent`，但不是继续无上限加工具和模式，而是把 `CLI` 打造成稳定、清晰、可控的控制面。  
> 本文档供执行 agent 直接施工使用。

---

## 1. 为什么现在做这件事

最近几轮重构已经把几块底座理顺了：

1. `coding-agent` 已从 `runtime/platform` 中显式抽成 application
2. `runtime` 已通过 `runtime.Application` 解耦，不再直接依赖 `internal/agents/coding`
3. `SSH operations`、`tool lifecycle hooks`、`session routing` 这些地基已经起来了

但站在真实可用性的角度，`coding-agent` 现在最薄弱的地方已经不是底层循环，而是 **CLI 控制面**：

- slash commands 还偏 MVP
- `/new` 仍靠 interactive 层特判
- session / model / tools / compact / profile 这类控制动作还不够完整
- CLI 状态反馈还不够像一个成熟 coding agent

商业产品调研给了两个很一致的启发：

1. **它们真正做厚的，不只是工具数量，而是控制面**
   - 会话切换
   - 模型切换
   - 状态反馈
   - 工具编排
   - 运行时治理
2. **它们真正值得学的，不是继续变通用，而是把 coding 主线做扎实**
   - 默认身份清晰
   - 工具集可控
   - prompt 不膨胀
   - 用户能明确掌控当前 session / model / profile / tools

所以这一步的目标非常明确：

**继续完善 `coding-agent`，优先把 CLI 做成稳定的控制平面，而不是去扩一堆新领域能力。**

---

## 2. 这次要做什么

本次主题聚焦 4 件事：

1. **补强 slash command 框架和命令集**
2. **引入轻量 profile 机制**
3. **改进交互式 CLI 的状态与反馈**
4. **补齐对应测试**

本次做完后，用户应能更自然地在 CLI 中：

- 看见当前 session / model / profile / cwd
- 创建、列出、切换 session
- 查询和切换 model
- 查询当前工具集
- 查询和切换 profile
- 通过一致的 slash command 行为控制运行时

---

## 3. 这次不要做什么

这一步要保持非常克制。

### 不要做

- 不要引入新的大工具域（浏览器、数据库、通知等）
- 不要顺手做第二个产品级 application
- 不要把 `coding-agent` 扩成“通用万能 agent”
- 不要做新的 GUI / TUI 路线
- 不要在这一步引入复杂插件市场
- 不要为了 profile 机制重写整套 prompt/skill 系统

### 特别注意

这一步不是为了“命令越多越好”，而是为了：

**让 `coding-agent` 的 CLI 更像一个成熟的操作面板。**

---

## 4. 当前问题总结

结合当前代码，主要问题有这些：

### 4.1 slash command 框架还偏薄

当前 [internal/slashcmd/registry.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/slashcmd/registry.go) 的能力还比较基础：

- 能注册
- 能执行
- 能列 help

但还缺：

- 更强的命令元数据
- 更好的帮助结构
- 更一致的错误语义
- 为 profile / session 切换预留的上下文能力

### 4.2 命令与 interactive CLI 还有临时拼接

当前 [internal/agents/coding/cli/interactive.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/agents/coding/cli/interactive.go) 对 `/new` 做了特判，说明 slash command 还没有完全成为统一控制面。

目标应该是：

- interactive 只负责读输入和展示输出
- slash command 自己完成行为
- session 切换通过明确接口回写到 interactive 状态

### 4.3 当前命令集还不够像“控制面”

现有内置命令主要有：

- `/help`
- `/compact`
- `/sessions`
- `/session`
- `/branch`
- `/new`
- `/tools`
- `/model`

问题不在于没有骨架，而在于：

- 缺 `/switch`
- 缺 `/profiles`
- 缺 `/profile`
- `/compact` 还是占位实现
- `/branch` 还是占位实现
- 命令输出还不够稳定和产品化

### 4.4 profile 机制还没有成型

根据 [skills-vs-application.md](/Users/weijian/Desktop/develop/test/pi/pi-go/docs/references/skills-vs-application.md) 的判断，很多“角色差异”现在不应该拆成新 application，而应该优先走：

- prompt profile
- tool filtering
- 轻量 role contract

但当前 `coding-agent` 还缺一个明确的 profile 承载点。

---

## 5. 目标设计

## 5.1 本次完成后的用户体验目标

交互式 CLI 至少应支持这样的操作流：

1. 用户启动 CLI，看到清晰状态行
2. 输入 `/help`，看到分组后的命令帮助
3. 输入 `/session`，看到当前 session / model / profile / tools 摘要
4. 输入 `/sessions`，列出全部 session，并标记当前会话
5. 输入 `/new`，创建并切换到新 session，不再依赖 interactive 特判
6. 输入 `/switch <session-id>`，切换到已有 session
7. 输入 `/model` 查看当前模型，输入 `/model provider:model` 切换模型
8. 输入 `/profiles` 查看所有 profile，输入 `/profile review` 切到 review profile
9. 输入 `/tools` 看当前 profile 下的有效工具集

---

## 5.2 Profile 的定位

这次的 profile 不要做成大系统。

它只是一个 **coding-agent 内部的轻量角色配置层**。

推荐至少支持两个 profile：

- `coding`：默认编程模式
- `review`：偏审查模式

这两个 profile 的差异应尽量只体现在：

- system prompt 片段
- 工具过滤
- 默认输出契约

不要在这一步引入：

- 复杂 profile 继承
- 用户自定义 profile DSL
- profile marketplace

### 推荐契约

#### `coding`

- 工具集：完整 coding 工具集
- 默认目标：完成任务、修改代码、运行命令

#### `review`

- 工具集：优先只读工具，可评估是否禁用 `write/edit/bash`
- 默认目标：指出风险、回归、测试缺口

如果你觉得直接禁用写工具风险太大，也可以第一版只做 prompt contract，不立即做严格工具过滤。  
但接口设计要为工具过滤预留位置。

---

## 5.3 Application 边界要求

profile 机制属于 `coding-agent` 应用层能力。

所以这次实现时要注意：

- 不要把 profile 逻辑塞回 `runtime`
- 不要让 `slashcmd` 框架知道 “review” 或 “coding” 这样的具体业务概念
- `runtime` 只承接通用运行时状态和接口
- `coding-agent` application 决定有哪些 profile、怎么构造 prompt、怎么过滤工具

也就是说：

- `slashcmd` 是框架
- `coding/commands` 是 application 命令定义
- `coding` application 决定 profile 语义

---

## 6. 推荐实现方案

## 6.1 先把 slash command 变成真正的控制面

建议优先扩展 [internal/slashcmd/context.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/slashcmd/context.go) 和相关接口。

当前 `SessionContext` 只有：

- `SessionID()`
- `ModelInfo()`
- `SwitchModel()`
- `ToolNames()`

这对更完整的 CLI 控制面不够。

建议按需补充：

- 当前 profile 查询
- profile 切换
- 当前工作目录或 workspace 摘要
- 当前 session 的更多基础状态

推荐直接补成可执行签名，不要只写抽象意图：

```go
type SessionContext interface {
    SessionID() string
    ModelInfo() (provider string, modelID string)
    SwitchModel(ctx context.Context, modelID string, provider string) error
    ToolNames() []string
    Profile() string
    SwitchProfile(profile string) error
}
```

同时 `AppContext` 也应支持：

- 创建 session
- 加载/切换 session
- 列出 session

推荐最小签名：

```go
type AppContext interface {
    ListSessionsInfo() ([]SessionInfo, error)
    CreateSession(ctx context.Context) (SessionContext, error)
    SwitchSession(ctx context.Context, sessionID string) (SessionContext, error)
}
```

### 关键要求

slash command 执行完成后，**interactive mode 必须能感知 session 切换结果**。  
不要再保留 `/new` 的命令外特判。

这里不要让执行 agent 自己猜实现方向。  
推荐明确采用：

### 推荐方案：`Execute` 返回结构化结果

不要再让 slash command 只能返回 `(string, error)`。  
建议扩展为显式结果对象，例如：

```go
type Result struct {
    Output          string
    ReplacedSession SessionContext
}
```

或等价结构：

```go
type CommandResult struct {
    Output          string
    SessionSwitchTo SessionContext
}
```

然后：

- `Registry.Execute(...)` 返回 `(Result, error)`
- `/new` 和 `/switch` 在命令内创建或加载 session，并把新 session 放进结果对象
- `interactive` 统一消费结果对象并更新自己持有的 `m.session`

这样做的好处是：

- 不需要在 `interactive` 里继续特判 `/new`
- 不依赖“修改 `slashcmd.Context` 值拷贝”这类隐式行为
- 命令执行结果可测试、可扩展

目标要非常清楚：

**slash command 是唯一入口，interactive 不再单独理解 `/new`，session 切换通过结构化结果回传。**

---

## 6.2 命令集增强建议

### 必做命令

- `/help`
- `/session`
- `/sessions`
- `/new`
- `/switch <session-id>`
- `/model`
- `/tools`
- `/profiles`
- `/profile [name]`

### 可选命令

- `/compact`
- `/context`

### 这次不做或只保留占位

- `/branch`

如果 `branch navigation` 现在没有明确实现路径，建议这次不要继续把它放在主要命令集中假装可用。  
可以：

- 临时移出帮助主列表
- 或明确标注 `planned`

---

## 6.3 CLI 展示层改进

当前 [internal/ui/presenter.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/ui/presenter.go) 已经有基础 presenter，这是个好起点。

这次建议重点提升：

### 状态行

当前状态行建议至少展示：

- session id
- provider/model
- 当前 profile
- cwd（可短路径）

### slash command 输出风格

命令输出要做到：

- 结构清晰
- 语义稳定
- 错误信息可区分
- 多行输出格式统一

### tool 执行展示

如果当前 `tool_update` 已经能进入 presenter，就继续保持；
如果还只是极简点状输出，也先不要过度包装，但至少要保证：

- start / update / end 的表现一致
- error 呈现清楚

---

## 6.4 profile 的实现位置

推荐把 profile 机制放在 `internal/agents/coding/` 内部，不要下沉到 platform。

建议新增轻量模块，例如：

```text
internal/agents/coding/profile/
  profile.go
```

可选设计：

```go
type Profile string

const (
    ProfileCoding Profile = "coding"
    ProfileReview Profile = "review"
)
```

然后由 coding application 在构建 prompt / tools 时读取当前 profile。

### 第一版的生效范围要明确

这一步不要把 profile 机制做得过重。  
第一版建议明确采用：

- **profile 切换先只影响 prompt contract**
- **工具过滤先预留接口，但不要求第一版严格热切换**

原因很实际：

- 当前 `runtime.Application.BuildPrompt()` 只要在 `PromptBuildOptions` 增加 `Profile` 字段，就能比较自然地在后续 prompt 中生效
- 但工具集是 session 创建时构建的，若第一版就要求 profile 热切换后同时重建工具集，会明显抬高复杂度

所以本次建议执行边界是：

1. `PromptBuildOptions` 增加 `Profile`
2. `coding` application 按 profile 输出不同 prompt
3. `ToolBuildOptions` 可为后续 profile-aware filtering 预留位置
4. 第一版不要求 profile 切换后立即热替换整套工具集

如果执行 agent 评估后发现“按 profile 切换工具集”改动很小，也可以顺手做；  
但验收不以此为硬要求。

### 第一版可以简单存在哪里

优先建议放到 `runtime.AgentSession` 持有的 application-level state 中，但通过 coding application 提供接口暴露。  
不要为了这个小功能重做 session storage 格式，除非你确认 profile 需要跨重启持久化。

第一版如果需要简化：

- profile 只在当前运行时 session 中生效

这是可接受的。

---

## 7. 推荐执行顺序

### 第一步：补接口，不改行为

先补齐 `slashcmd.Context` / `SessionContext` / `AppContext` 所需接口，保证后续命令扩展有落点。

目标：

- session 切换可以被统一表达
- profile 查询/设置有接口
- interactive 可以通过统一方式同步状态

### 第二步：消灭 `/new` 特判

把 `/new` 从 interactive 特判迁回 slash command 主链。

这是本次很重要的验收点之一。
同时把 `/switch` 也走同样的结构化结果回写链路，不要做第二套特殊处理。

### 第三步：补 session/model/profile 命令

优先补：

- `/switch`
- `/profiles`
- `/profile`

同时把：

- `/session`
- `/sessions`
- `/model`
- `/tools`

统一成更稳定的输出风格。

### 第四步：补 profile 驱动的 prompt/tools 行为

至少先让 `review` profile 在 prompt 上有明确差异。  
第一版以 **prompt 差异** 为硬要求，工具过滤只做预留或轻量实现。

执行 agent 可以根据当前改动成本选择：

- 第一版只做 prompt contract
- 或第一版顺手加上有限工具过滤

但必须在代码和测试里把实际行为写清楚，不要留下“文档说会过滤、实现其实没过滤”的模糊状态。

### 第五步：补测试

测试要覆盖：

- slash registry / parsing
- `/new` 经 slash command 完成 session 切换
- `/switch` 切换已有 session
- `/model` 查询与切换
- `/profiles` / `/profile`
- review profile 对 prompt 或工具集的影响

---

## 8. 验收标准

本次完成后，至少应满足：

1. `interactive` 中不再对 `/new` 做特殊分支处理
2. `/new` 执行后，interactive 持有的 session 已实际切换，不是只修改了命令上下文值拷贝
3. session 切换后，下一次 `runPrompt` 使用的是新 session
4. `coding-agent` 支持 `/switch <session-id>`
5. `coding-agent` 支持 `/profiles` 和 `/profile [name]`
6. CLI 状态行能显示当前 profile
7. `review` profile 至少在 prompt 行为上与默认 `coding` 有区别
8. 如果 `/branch` 仍是占位实现，则从主帮助分组中移除，或明确标注 `planned`
9. `go test ./...` 通过
10. 与本次改动相关的新增测试存在，且不是只靠人工验证

---

## 9. 实现时的判断原则

执行过程中，如果遇到“要不要把更多领域能力也顺手塞进来”，请回到这几个原则：

1. 这一步服务的是 **coding-agent 控制面**
2. 不要因为商业产品很厚，就跟着把 `pi-go` 做成万能工具箱
3. 默认身份必须保持清晰：当前主角仍然是 coding-agent
4. 角色差异优先走 profile，而不是立刻拆新 application
5. 只有当控制面真正稳定后，再继续扩更高层能力

---

## 10. 一句话总结

这一步不是“再加几个 slash 命令”，而是：

**把 `pi-go` 的 `coding-agent` 从“能聊天、能调工具”的原型，推进到“有明确控制面、会话掌控、模型掌控、角色切换能力”的成熟 CLI。**
