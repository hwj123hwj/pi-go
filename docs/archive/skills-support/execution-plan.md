---
status: draft
author: plan-agent
created: 2026-05-28
updated: 2026-05-28
depends-on:
  - docs/dev/skills-support/proposal.md
---

# Skills 支持执行文档

> Goal: 实现 SkillTool + Slash 命令，让 skills 可被 LLM 和用户实际调用
> 本文档供执行 agent 直接施工使用

---

## 前置状态

- `internal/skill/skill.go` 已实现 `LoadFromDirs` / `FindByName` / `FormatInvocation` / `FormatForSystemPrompt`，但 `FindByName` 和 `FormatInvocation` 从未被使用
- `internal/runtime/agent_session.go:337-357` 已在 `rebuildAgent` 中加载 skills 并传入 prompt
- `ToolBuildOptions` 不含 skills，tools 构建链无法访问 skills
- `CommandResult` 不支持自定义 query 输入

---

## Phase 1：SkillTool（LLM 触发）

### 1.1 新增 `internal/tools/skill.go`

**做什么**: 实现 `agent.Tool` 接口的 SkillTool，让 LLM 能通过工具调用加载 skill 内容。

**修改点**: 新建文件。

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/earendil-works/pi-go/internal/agent"
    "github.com/earendil-works/pi-go/internal/skill"
)

type SkillTool struct {
    skills []skill.Skill
}

type SkillParams struct {
    Skill string `json:"skill"`
    Args  string `json:"args,omitempty"`
}

func NewSkillTool(skills []skill.Skill) *SkillTool {
    return &SkillTool{skills: skills}
}

func (t *SkillTool) Name() string { return "skill" }
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
                "description": "Optional arguments to pass to the skill.",
            },
        },
        "required": []string{"skill"},
    }
}

func (t *SkillTool) Validate(params json.RawMessage) (json.RawMessage, error) {
    var p SkillParams
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, fmt.Errorf("invalid parameters: %w", err)
    }
    if p.Skill == "" {
        return nil, fmt.Errorf("skill name is required")
    }
    return json.Marshal(p)
}

func (t *SkillTool) Execute(_ context.Context, params json.RawMessage, _ func(agent.PartialResult)) (agent.ToolResult, error) {
    var p SkillParams
    if err := json.Unmarshal(params, &p); err != nil {
        return agent.ToolResult{}, err
    }

    s := skill.FindByName(t.skills, p.Skill)
    if s == nil {
        return agent.ToolResult{
            Content: fmt.Sprintf("Skill %q not found. Available skills are listed in the system prompt.", p.Skill),
            IsError: true,
        }, nil
    }

    content := skill.FormatInvocation(*s, p.Args)
    return agent.ToolResult{Content: content}, nil
}
```

**验证**:
- 编译通过：`go build ./internal/tools/...`
- 单元测试（见 1.6）

### 1.2 修改 `internal/runtime/application.go`

**做什么**: `ToolBuildOptions` 新增 `Skills` 字段。

**修改点**:
- `ToolBuildOptions` struct 新增 `Skills []skill.Skill`
- 文件顶部 import 已有 `"github.com/earendil-works/pi-go/internal/skill"`（`PromptBuildOptions` 用到），无需新增 import

```go
type ToolBuildOptions struct {
    Workspace      string
    MaxOutputLen   int
    BashOps        operations.BashOperations
    FileOps        operations.FileOperations
    ExtensionTools []agent.Tool
    AllowedTools   []string
    BlockedTools   []string
    Skills         []skill.Skill // 新增
}
```

**验证**: `go build ./internal/runtime/...`

### 1.3 修改 `internal/agents/coding/tools/tools.go`

**做什么**: `ListOptions` 新增 `Skills` 字段，`BuildList` 中创建 SkillTool 并追加到工具列表。

**修改点**:
- `ListOptions` 新增 `Skills []skill.Skill`
- import 新增 `basetools "github.com/earendil-works/pi-go/internal/tools"` 已有，无需改
- `BuildList` 中，在 `toolList = append(toolList, opts.ExtensionTools...)` 之前，追加：

```go
if len(opts.Skills) > 0 {
    toolList = append(toolList, basetools.NewSkillTool(opts.Skills))
}
```

**验证**: `go build ./internal/agents/coding/...`

### 1.4 修改 `internal/agents/coding/application.go`

**做什么**: `BuildTools` 把 `opts.Skills` 传入 `ListOptions`。

**修改点**: `codingtools.ListOptions` 初始化中新增 `Skills: opts.Skills`。

**验证**: `go build ./cmd/pi-agent`

### 1.5 修改 `internal/runtime/agent_session.go`

**做什么**: `toolBuildOptions` 方法中把已加载的 skills 填入 `ToolBuildOptions.Skills`。

**修改点**: 在 `toolBuildOptions` 返回的 `ToolBuildOptions` 中新增 `Skills` 字段。需要把 `rebuildAgent` 中加载的 skills 传递到 `toolBuildOptions`。

当前 `toolBuildOptions` 签名是 `func (s *AgentSession) toolBuildOptions(cwd string) ToolBuildOptions`，不接收 skills。两种处理方式：
- 方案 A：给 `toolBuildOptions` 加参数 `skills []skill.Skill`
- 方案 B：把 skills 存为 `AgentSession` 字段

建议方案 A，改动最小：

```go
func (s *AgentSession) toolBuildOptions(cwd string, skills []skill.Skill) ToolBuildOptions {
    // ... 现有逻辑 ...
    return ToolBuildOptions{
        // ... 现有字段 ...
        Skills: skills,
    }
}
```

`rebuildAgent` 中调用处同步修改：`s.toolBuildOptions(cwd, skills)`。

**验证**: `go build ./cmd/pi-agent`

### 1.6 新增 `internal/tools/skill_test.go`

**做什么**: SkillTool 单元测试。

**测试用例**:
1. `TestSkillTool_InvokeExisting` — 调用存在的 skill，返回 FormatInvocation 内容
2. `TestSkillTool_InvokeNotFound` — 调用不存在的 skill，返回 IsError
3. `TestSkillTool_EmptyName` — skill 参数为空，Validate 报错
4. `TestSkillTool_WithArgs` — 带 args 参数，内容中包含 args

**验证**: `go test ./internal/tools/ -run TestSkillTool -v`

---

## Phase 2：Slash 命令触发（interactive 模式）

### 2.1 修改 `internal/slashcmd/registry.go`

**做什么**: `CommandResult` 新增 `QueryInput` 字段。

**修改点**:

```go
type CommandResult struct {
    Output          string
    SessionSwitchTo SessionContext
    ClearScreen     bool
    ShouldQuery     bool
    QueryInput      string // 新增：ShouldQuery=true 时发给 agent 的 prompt；空则用默认值
}
```

**验证**: `go build ./internal/slashcmd/...`

### 2.2 修改 `internal/agents/coding/cli/interactive.go`

**做什么**: 两处改动——(a) slash 命令 fallback 查找 skill，(b) 用 QueryInput 替代硬编码。

**修改点**:

(a) 在 slash 命令处理中，`Execute` 返回错误时 fallback：

```go
result, err := m.slashCmds.Execute(cmdCtx, input)
if err != nil {
    name, args := slashcmd.ParseSlashCommand(input)
    if s := skill.FindByName(m.skills, name); s != nil {
        result = slashcmd.CommandResult{
            Output:     fmt.Sprintf("Loaded skill: %s", s.Name),
            ShouldQuery: true,
            QueryInput:  skill.FormatInvocation(*s, args),
        }
    } else {
        fmt.Fprintf(m.output, "Unknown command: %s\n", name)
        continue
    }
}
```

(b) `ShouldQuery` 处理中使用 `QueryInput`：

```go
if result.ShouldQuery {
    input := result.QueryInput
    if input == "" {
        input = "Start working on the goal."
    }
    m.runPrompt(ctx, input)
}
```

**验证**:
- `go build ./cmd/pi-agent`
- 手动验证：`PI_GO_PROVIDER=mock ./pi-agent -mode chat`，输入 `/nonexistent` 报错，输入 `/skill-name` 触发 skill

### 2.3 传递 skills 到 InteractiveMode

**做什么**: `InteractiveMode` 需要访问 skills 列表用于 fallback 查找。

**修改点**:

`internal/agents/coding/cli/interactive.go` 的 `InteractiveMode` struct 新增 `skills []skill.Skill` 字段，`NewInteractiveMode` 构造函数新增参数。

调用方（`cmd/pi-agent/main.go` 或 `internal/mode/`）需要把 skills 传入。skills 在 `AgentSession.rebuildAgent` 中加载，需要通过某种方式暴露出来：
- 方案 A：`AgentSession` 新增 `Skills() []skill.Skill` 方法
- 方案 B：在 `AgentSession.BuildAgent` 返回后，从 session 获取 skills

建议方案 A：在 `AgentSession` 中把 skills 缓存为字段，新增 getter。

**验证**: `go build ./cmd/pi-agent`

### 2.4 重构 `/goal` 使用 QueryInput（顺手做）

**做什么**: `/goal` 命令的 handler 返回 `QueryInput`，消除 interactive.go 中的硬编码。

**修改点**: `internal/agents/coding/commands/builtins.go` 中 `/goal` handler 的 `ShouldQuery` 返回加上 `QueryInput`：

```go
return slashcmd.CommandResult{
    Output:      output,
    ShouldQuery: true,
    QueryInput:  "Start working on the goal.",
}, nil
```

**验证**: `go build ./cmd/pi-agent`，`/goal` 行为不变

---

## 错误处理规范

| 场景 | 行为 |
|------|------|
| SkillTool 调用不存在的 skill | ToolResult.IsError=true，提示可用 skill 列表 |
| SkillTool 参数缺少 skill 名 | Validate 返回错误 |
| Slash 命令 fallback 找不到 skill | 打印 "Unknown command: xxx" |
| Skills 目录为空或不存在 | SkillTool 正常注册，调用时返回"not found" |

---

## 执行顺序建议

```
Phase 1（无外部依赖，可连续执行）:
  1.1 skill.go (新增)
  1.2 application.go (ToolBuildOptions)
  1.3 tools.go (ListOptions + BuildList)
  1.4 application.go (BuildTools)
  1.5 agent_session.go (toolBuildOptions)
  1.6 skill_test.go (新增)
  → 验证：go build + go test

Phase 2（依赖 Phase 1 的 skills 传递链路）:
  2.1 registry.go (QueryInput)
  2.4 builtins.go (goal 重构)
  2.3 agent_session.go + interactive.go (skills 传递)
  2.2 interactive.go (fallback + QueryInput)
  → 验证：go build + 手动测试
```

---

## 验证清单

### Phase 1 验证
- [ ] `go build ./cmd/pi-agent` 编译通过
- [ ] `go test ./internal/tools/ -run TestSkillTool -v` 全部通过
- [ ] `go test ./internal/skill/... -v` 已有测试不回归
- [ ] `go vet ./...` 无警告

### Phase 2 验证
- [ ] `go build ./cmd/pi-agent` 编译通过
- [ ] `go test ./internal/slashcmd/... -v` 已有测试不回归
- [ ] 手动：`PI_GO_PROVIDER=mock ./pi-agent -mode chat`
  - [ ] `/help` 正常输出
  - [ ] `/goal test` 行为不变
  - [ ] `/nonexistent` 输出 "Unknown command"
  - [ ] `/skill-name` 触发 skill，agent 收到 skill 内容
- [ ] `go test ./... -race` 全量通过
