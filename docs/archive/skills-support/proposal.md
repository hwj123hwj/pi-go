---
status: draft
author: plan-agent
created: 2026-05-28
updated: 2026-05-28
---

# Skills 完整支持提案（v2）

> Goal: 让 skills 从"系统提示中的展示列表"变成"可被 LLM 和用户实际调用的指令集"

## 目标

1. LLM 能通过 SkillTool 主动调用 skill（inline 模式，skill 内容注入当前对话）
2. 用户能通过 `/skill-name args` 斜杠命令手动触发 skill
3. 两个入口共享同一套底层逻辑（`FindByName` + `FormatInvocation`）

## 现状

### 已实现

| 功能 | 文件 | 状态 |
|------|------|------|
| 从目录加载 SKILL.md | `internal/skill/skill.go` `LoadFromDirs` | 完成，未充分使用 |
| Frontmatter 解析 (name/description) | `internal/skill/skill.go` `parseFrontmatter` | 完成 |
| 系统提示中展示 skill 列表 | `internal/skill/skill.go` `FormatForSystemPrompt` | 完成 |
| 格式化 skill 调用内容 | `internal/skill/skill.go` `FormatInvocation` | 完成，**从未使用** |
| 按名称查找 skill | `internal/skill/skill.go` `FindByName` | 完成，**从未使用** |
| 默认 skill 目录发现 (.claude/skills) | `internal/runtime/agent_session.go:337-357` | 完成 |
| Slash 命令框架 | `internal/slashcmd/registry.go` | 完成 |

### 缺失能力

| # | 缺失能力 | 影响 | 优先级 |
|---|---------|------|--------|
| 1 | SkillTool（LLM 调用 skill 的工具） | LLM 知道有 skill 但无法读取内容 | P0 |
| 2 | Skills 传递到 Tool 层 | ToolBuildOptions 不含 skills | P0 |
| 3 | Slash 命令触发 skill | 用户无法手动 `/name` 触发 | P1 |

> **不做**：丰富 frontmatter (model/effort/hooks/paths)、fork 模式、条件激活、MCP/Plugin skills、GUI 前端 `/` 自动补全、serve 模式 slash 支持（后续再做）。

## 这次做什么

### Phase 1：SkillTool（LLM 触发）

#### 1.1 新增 SkillTool 实现

新增 `internal/tools/skill.go`：

```go
type SkillTool struct {
    skills []skill.Skill
}

func NewSkillTool(skills []skill.Skill) *SkillTool

func (t *SkillTool) Name() string        { return "skill" }
func (t *SkillTool) Description() string {
    return "Invoke a skill by name to load specialized instructions. " +
        "Available skills are listed in the system prompt under <available_skills>."
}
func (t *SkillTool) Parameters() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "skill": map[string]any{
                "type":        "string",
                "description": "Name of the skill to invoke.",
            },
            "args": map[string]any{
                "type":        "string",
                "description": "Optional arguments for the skill.",
            },
        },
        "required": []string{"skill"},
    }
}
```

Execute 逻辑：
1. `FindByName` 查找 skill → 找不到返回错误
2. `FormatInvocation` 格式化 skill 内容
3. 返回 `ToolResult`，Content 为格式化后的 skill 指令

#### 1.2 Skills 传递到 Tool 层

修改 `internal/runtime/application.go`：`ToolBuildOptions` 新增字段：

```go
type ToolBuildOptions struct {
    // ... 现有字段 ...
    Skills []skill.Skill  // 新增
}
```

#### 1.3 接入 Tool 构建链

修改 `internal/agents/coding/tools/tools.go`：`ListOptions` 新增 `Skills` 字段，`BuildList` 中创建并追加 SkillTool。

修改 `internal/agents/coding/application.go`：`BuildTools` 从 `opts.Skills` 传入 `ListOptions`。

修改 `internal/runtime/agent_session.go`：`toolBuildOptions` 方法中把已加载的 skills 填入 `ToolBuildOptions.Skills`。

### Phase 2：Slash 命令触发（用户触发，仅 interactive 模式）

#### 2.1 CommandResult 支持 QueryInput

修改 `internal/slashcmd/registry.go`：`CommandResult` 新增字段：

```go
type CommandResult struct {
    // ... 现有字段 ...
    QueryInput string // ShouldQuery=true 时，发给 agent 的 prompt；空则用默认值
}
```

设计说明：
- 比 ContextInjection 更简单明确——直接指定发给 agent 的完整文本
- 向后兼容：`QueryInput` 为空时保持现有行为（`/goal` 等命令不受影响）
- serve 模式不需要改动

#### 2.2 Interactive 模式 fallback 查找 skill

修改 `internal/agents/coding/cli/interactive.go`：在 slash 命令处理逻辑中，当 `Execute` 返回 unknown command 错误时，fallback 尝试 `skill.FindByName`：

```go
result, err := m.slashCmds.Execute(cmdCtx, input)
if err != nil {
    // fallback: 尝试 skill 命令
    name, args := slashcmd.ParseSlashCommand(input)
    if s := skill.FindByName(m.skills, name); s != nil {
        result = slashcmd.CommandResult{
            Output:     fmt.Sprintf("Loaded skill: %s", s.Name),
            ShouldQuery: true,
            QueryInput:  skill.FormatInvocation(*s, args),
        }
    } else {
        // 真正的 unknown command
    }
}
```

优势：
- 不改 `RegisterBuiltins` 签名
- 不需要 session 访问 registry
- 不需要动态注册/注销
- 所有 skill 都对用户可用，不需要额外的 `user-invocable` 字段

#### 2.3 传递 skills 到 Interactive 模式

skills 列表需要从 `AgentSession` 传递到 `InteractiveMode`。有两种方式：
- 方案 A：`InteractiveMode` 构造时接收 `[]skill.Skill`（简单直接）
- 方案 B：通过 `SessionContext` 接口暴露 `Skills()` 方法

建议方案 A，改动最小。

#### 2.4 顺便重构 /goal 的硬编码

`interactive.go` 中现有代码：

```go
if result.ShouldQuery {
    m.runPrompt(ctx, "Start working on the goal.")
}
```

改为：

```go
if result.ShouldQuery {
    input := result.QueryInput
    if input == "" {
        input = "Start working on the goal."
    }
    m.runPrompt(ctx, input)
}
```

`/goal` 命令本身也可以改成用 `QueryInput`，消除硬编码。

## 这次不做什么

- 不丰富 frontmatter（model/effort/hooks/paths）— 无使用场景
- 不做 fork 模式 — pi-go 无 sub-agent 机制
- 不做条件激活 (paths 匹配) — 等有实际 skill 库再加
- 不改 `internal/skill/skill.go` 加载逻辑 — 已有实现够用
- 不做 GUI 前端 `/` 自动补全 — 前端独立任务
- 不做 serve 模式 slash 支持 — 后续再做，先验证 interactive 模式

## 技术方案

### 架构影响

```
用户输入 "/research xxx"              LLM 决定用 skill
        │                                    │
        ▼                                    ▼
  slashcmd.Execute()                  SkillTool.Execute()
  → unknown command                   → FindByName
  → fallback FindByName               → FormatInvocation
  → FormatInvocation                  → ToolResult(content)
  → QueryInput                         │
        │                              ▼
        ▼                        agent loop
  runPrompt(QueryInput)           LLM 看到 skill 指令
        │                              │
        └────────────┬─────────────────┘
                     ▼
             Agent 按 skill 指令执行
```

### 新增/修改文件清单

```
internal/tools/
└── skill.go                  [新增] SkillTool 实现

internal/runtime/
├── application.go            [修改] ToolBuildOptions 加 Skills 字段
└── agent_session.go          [修改] toolBuildOptions 填入 skills

internal/agents/coding/
├── application.go            [修改] BuildTools 传递 skills
├── tools/tools.go            [修改] ListOptions 加 Skills，BuildList 追加 SkillTool
└── cli/interactive.go        [修改] fallback skill 查找 + QueryInput 处理

internal/slashcmd/
└── registry.go               [修改] CommandResult 加 QueryInput 字段
```

### 数据流

```
场景 A：LLM 触发
  用户: "帮我调研一下这个项目"
  → LLM 判断匹配 skill "research"
  → LLM 调用 Skill(skill: "research", args: "这个项目")
  → ToolResult: <skill name="research">...skill 内容...</skill>
  → LLM 按 skill 指令执行后续步骤

场景 B：用户触发
  用户: /research 这个项目
  → slashcmd.Execute → unknown command
  → fallback FindByName("research") → 命中
  → FormatInvocation → QueryInput
  → runPrompt(QueryInput)
  → Agent 收到 skill 指令 + 用户参数，按指令执行
```

## 依赖关系

| 依赖 | 状态 | 说明 |
|------|------|------|
| `internal/skill` 包 | ✅ 已存在 | FindByName / FormatInvocation 已实现 |
| `internal/slashcmd` 框架 | ✅ 已存在 | Registry / Command / CommandResult |
| `internal/tools/` 目录 | ✅ 已存在 | bash/read/write/edit/grep/find/ls 在此 |
| `agent.Tool` 接口 | ✅ 已存在 | Name/Description/Parameters/Validate/Execute |

## 风险和取舍

| 风险 | 缓解 |
|------|------|
| SkillTool 让 LLM 调用不存在的 skill 名 | Execute 返回明确错误信息，LLM 可自行修正 |
| Skill 内容过长撑爆上下文 | FormatInvocation 已有完整内容，后续可加截断；当前 skill 文件通常不大 |
| `/name` 与内置命令冲突 | fallback 只在 registry 返回 unknown command 时触发，不会覆盖内置命令 |
| fallback 模式下 `/help` 无法列出 skill | 可单独加 `/skills` 命令列出可用 skill，本次不做 |

## 完成标志

1. `go build ./cmd/pi-agent` 编译通过
2. `go test ./internal/tools/... ./internal/skill/... ./internal/slashcmd/... -race` 全部通过
3. 手动验证：
   - `PI_GO_PROVIDER=mock ./pi-agent -mode chat` → LLM 能调用 `skill` 工具
   - 输入 `/skill-name` → skill 内容注入对话 → agent 自动响应
   - `/goal` 行为不变（向后兼容）
