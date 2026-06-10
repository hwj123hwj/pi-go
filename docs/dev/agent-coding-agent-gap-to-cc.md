# Agent / Coding-Agent 对齐 Claude Code 基础能力差距文档

> 目标：聚焦 `pi-go` 的 agent runtime 与 coding-agent 应用层，评估相对 `cc-haha` / Claude Code 源码基础能力的差距，并给出优先补齐路线。
>
> 明确不纳入本轮对齐范围：
>
> - TUI
> - Desktop 工作台
> - 多模型 provider / provider proxy
> - IM / H5 / 定时任务等入口层能力

## 1. 结论摘要

`pi-go` 当前已经具备一个可运行的 agent 内核：双层 loop、streaming、tool call、session、compaction、slash command、skills、extension、operations(local/ssh) 都已成型。

真正和 Claude Code 基础能力拉开差距的，不是“有没有 agent loop”，而是以下四类能力：

1. **系统提示词工程化不足**
   - 当前是线性拼接 prompt。
   - 缺 section registry、静态/动态分层、prompt cache 边界、可控动态 section。

2. **工具生命周期与工具加载模型不够厚**
   - 当前工具接口偏执行函数。
   - 缺 `isReadOnly` / `isDestructive` / `isEnabled` / `shouldDefer` / permission decision / result budget 等产品级元数据。

3. **Coding tools 的安全与防错不足**
   - bash、read、edit、write 已可用，但缺权限审批、sandbox、bash streaming/background、read-before-write、mtime 校验、结构化 diff、二进制/图片/PDF/notebook 感知等。

4. **Skills runtime 已基本成型（✅ 已实现），待补 eval 和生产加固**
   - ✅ 已实现：skill 列表、skill tool 完整内容注入、`when_to_use` / `allowed-tools` / `paths` / `context: fork` / branch / argument frontmatter、路径条件激活、多来源合并、工具访问后的嵌套 skill 动态发现、runtime tool policy（工具收窄、violation 渐进式收紧、write/edit-only hard-stop）、MCP resource adapter（HTTP Streamable + Stdio JSON-RPC）、cached source provider（内存 + JSON 文件持久化）、session fork 隔离执行。
   - 剩余差距：动态发现 cache invalidation 策略、更大规模真实任务 eval、MCP OAuth/授权策略、provider 官方 tokenizer backend。

优先级判断：

**不要先做 Desktop。也不要先继续堆 provider。下一步应该优先加厚 agent runtime 和 coding-agent app 层。**

## 2. 当前 pi-go 能力边界

### 2.1 Agent Runtime 已具备

关键文件：

- `internal/agent/loop.go`
- `internal/agent/tool.go`
- `internal/agent/tool_lifecycle.go`
- `internal/runtime/agent_session.go`
- `internal/session/`
- `internal/compaction/`
- `internal/operations/`

已有能力：

- 双层 agent loop：外层 follow-up，内层 tool call。
- streaming event 抽象。
- tool call 执行。
- 并发安全分批：只读/安全工具可并行，不安全工具串行。
- tool lifecycle hooks：before / after / prepare arguments。
- session JSONL 持久化。
- compaction：LLM summary + recent messages。
- local / ssh operations 抽象。
- goal-driven loop。

主要短板：

- session storage 打开时全量加载 JSONL 到内存。
- compaction token 估算已从 `4 chars ~= 1 token` 升级为 model-aware counter，并有可注册 backend registry；仍缺具体 provider 官方 tokenizer backend 和精度回归样本。
- tool result budget / large result persistence 不完整。
- permission decision 不是 runtime 一等能力。
- interrupt / cancel / background task 模型薄。

### 2.2 Coding-Agent 已具备

关键文件：

- `internal/agents/coding/application.go`
- `internal/agents/coding/prompt/builder.go`
- `internal/agents/coding/tools/tools.go`
- `internal/tools/`
- `internal/skill/`

已有能力：

- 7 个基础工具：`bash/read/write/edit/grep/find/ls`。
- `edit` 支持 single replace、replace_all、多 edit batch。
- per-file mutation queue。
- `read` 支持 offset/limit。
- workspace path safety。
- `grep/find/ls` 基础代码检索能力。
- skill tool：系统提示列出 skill 摘要，调用后注入完整 skill 内容。
- profile：coding / review。
- project context：加载 `CLAUDE.md` / `AGENTS.md`。

主要短板：

- bash 无 streaming/background/sandbox/危险命令分析。
- read 无图片/PDF/notebook/二进制/设备文件防护。
- edit/write 无 read-before-write、mtime stale check、encoding/line ending 保留、结构化 patch。
- grep/find 未充分利用 ripgrep/gitignore 语义。
- tools 没有统一 permission flow。

**✅ 已补齐**（原"skills 缺条件激活、动态发现、frontmatter 丰富语义"）：

- 条件激活：`paths` 匹配自动激活、多 skill steering。
- 动态发现：工具访问路径祖先目录中的嵌套 `.claude/skills` / `.agents/skills`。
- frontmatter 丰富语义：`when_to_use` / `allowed-tools` / `paths` / `branches` / `arguments` / `argument-hint` / `context: fork` / `disable-model-invocation`。
- runtime policy：工具收窄、bash 命令级约束、violation 渐进式收紧、skill 文件只读保护。
- MCP：HTTP Streamable + Stdio JSON-RPC 两种 transport。
- 多来源：source registry + priority merge（project > user > managed > plugin > bundled > system）。
- 持久化缓存：`CachedSourceProvider` + `JSONFileSourceCache`。

## 3. 与 cc-haha 的核心差距

### 3.1 系统提示词设计

`pi-go` 当前：

- `BuildSystemPrompt()` 线性拼接。
- 内容包括 base prompt、tool summary、guidelines、project context、skills、append prompt、goal、date/cwd/git branch。
- 优点是简单直接，易维护。
- 缺点是动态内容和静态内容混在一起，后续越加越难维护，也不利于 prompt cache。

`cc-haha` 参考设计：

- 静态 prompt 与动态 prompt 分层。
- 使用 `SYSTEM_PROMPT_DYNAMIC_BOUNDARY` 作为 cache boundary。
- 动态 section 通过 registry 管理。
- section 可缓存，也可标记为 volatile。
- system prompt 可按 enabled tools、memory、env、language、output style、MCP instructions、scratchpad、token budget 等组合。

差距判断：

| 能力 | pi-go | 目标 |
|------|-------|------|
| 静态/动态 prompt 分层 | 无 | 需要 |
| prompt section registry | 无 | 需要 |
| section 缓存 | 无 | 需要 |
| cache boundary | 无 | 需要 |
| 动态 section 可控重算 | 无 | 需要 |
| tool-aware prompt | 基础版 | 需要增强 |

优先补齐：

1. 新增 `internal/prompt/sections` 或等价抽象。
2. 将 coding prompt 拆为：
   - static intro
   - static task rules
   - static tool-use rules
   - dynamic env/project context
   - dynamic skills
   - dynamic goal/profile
3. 引入 section name + cache policy。
4. 避免后续所有需求继续塞进一个 builder。

### 3.2 工具加载与 Tool Interface

`pi-go` 当前：

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any
    Validate(params json.RawMessage) (json.RawMessage, error)
    Execute(ctx context.Context, params json.RawMessage, onUpdate func(PartialResult)) (ToolResult, error)
}
```

已有可选接口：

- `ToolWithMode`
- `ToolWithPromptInfo`
- `ConcurrencySafeChecker`
- `ToolWithPrepareArguments`

`cc-haha` 参考设计中，Tool 不只是执行器，还承担：

- 是否只读
- 是否破坏性
- 是否启用
- 是否延迟加载
- 是否 open-world
- 是否需要用户交互
- 权限检查
- 输入校验
- path 提取
- permission matcher
- progress
- result budget
- MCP/LSP 标记

差距判断：

| 能力 | pi-go | 目标 |
|------|-------|------|
| `IsReadOnly` | 部分用 concurrency safe 替代 | 需要一等接口 |
| `IsDestructive` | 无 | 需要 |
| `IsEnabled` | 由 build list 控制 | 需要 |
| `ShouldDefer` | 无 | P1 |
| `CheckPermission` | 无 | P0 |
| `GetPath` | 无 | P0 |
| result budget | 简单截断 | P0 |
| large result persistence | 无 | P1 |
| ToolSearch / deferred tools | 无 | P1 |
| MCP/LSP tool metadata | 无 | P1 |

优先补齐：

1. 扩展 tool optional interfaces，而不是一次重写 Tool 接口。
2. P0 先加：
   - `ToolReadOnly`
   - `ToolDestructive`
   - `ToolPathProvider`
   - `ToolPermissionChecker`
   - `ToolResultLimiter`
3. agent loop 在执行工具前统一走 permission decision。
4. coding tools 逐步实现这些接口。

### 3.3 Coding Tools

#### Bash

`pi-go` 当前：

- 每次执行一个 shell command。
- 支持 timeout。
- 合并 stdout/stderr。
- strip ANSI。
- binary output 简单检测。
- max output 截断。

缺口：

- streaming partial output 未打通。
- background task / detached task 无。
- sandbox 无。
- command semantic parser 无。
- destructive command warning 无。
- permission rule 无。
- sed edit preview 无。
- git operation tracking 无。
- long-running command auto-background 无。

P0 目标：

- bash streaming。
- command timeout / cancel 语义稳定。
- permission checker 接入。
- destructive command 粗分类。
- read/search/list command 分类，用于 UI/日志/结果摘要。

P1 目标：

- background task。
- sandbox。
- sed edit preview。
- large output persistence。

#### Read

`pi-go` 当前：

- text read。
- offset/limit。
- line number。
- workspace 限制。

缺口：

- 二进制扩展识别。
- device file 防护。
- 图片/PDF/notebook 支持。
- 文件大小/token 预算。
- read dedup。
- similar path suggestion。
- read permission。

P0 目标：

- size limit。
- binary/device guard。
- read permission。
- mtime/read state 记录，为 edit stale check 服务。

P1 目标：

- image/PDF/notebook。
- read dedup。
- similar path suggestion。

#### Edit / Write

`pi-go` 当前：

- exact string replace。
- replace_all。
- multi-edit。
- create file。
- mutation queue。
- 简单 diff context。

缺口：

- 未强制 read-before-write。
- 无 mtime stale check。
- 不保留 encoding / line endings。
- 无结构化 patch。
- 无 permission approval。
- 无 secret/settings file special validation。

P0 目标：

- read-before-edit/write。
- readFileState：记录 path、mtime、content hash、range。
- edit/write 前检查文件是否被用户或 formatter 改过。
- structured patch 返回。
- write/edit permission checker。

P1 目标：

- encoding / line ending 保留。
- secret/config 文件特殊校验。
- IDE/LSP change notification。

### 3.4 Skills

> **状态：Phase C（Skills 动态化）和 Phase D 中的 `context: fork` 已基本完成。**
> 详见下方已完成标注。

`pi-go` 当前：

缺口：

| 能力 | pi-go | 目标 |
|------|-------|------|
| `when_to_use` | 已解析并进入 skill index / invocation contract；兼容 `when-to-use` | P0 |
| `allowed-tools` | 已解析并进入 runtime tool policy，兼容 `allowed_tools` / `allowedTools`，支持与当前注册工具取交集、工具收窄、Bash(...) 命令级约束、Bash allowlist 对 shell wrapper、前置 env assignment / `env -C` / `sh -c` 内层命令的多候选真实命令匹配，并按 `&&` / `;` / `|` 等 shell segment 逐段校验，避免允许命令后拼接未授权命令；`skill` tool 作为入口工具不会在 active policy 内暴露，active skill 中尝试嵌套调用 skill 会被视为 workflow escape 并触发 write/edit-only hard-stop，避免覆盖当前 workflow；没有明确 `Bash(...)` 规格时继续拒绝 shell 探索命令，显式 allowlist 的 `Bash(cat:*)` / `Bash(sed:*)` 可执行但仍受 selected branch / skill 文件 / workspace `paths` 约束；skill 探索类重复违规快速 write/edit-only hard-stop，以及 active policy 下禁用并行工具批次 | P0 |
| `paths` / selected branch | 已支持输入级自动激活、多匹配 steering、工具访问后 follow-up，并进入 read/write/edit/search/bash 路径硬约束；路径 policy 和工具访问后的动态 skill discovery 都基于 canonical path tool name 执行，`search -> grep`、`glob -> find`、大小写变体等兼容别名不会绕过 selected branch / workspace `paths` 或 path-matched skill steering；active skill policy 期间会抑制新的 path-matched skill follow-up，避免当前 skill workflow 被其它 skill 打断；bash 路径提取覆盖 `--out=path`、`--out path` / `-o path`、`-C` / `--directory`、路径型 env assignment、前置 env assignment 后的真实路径命令、shell 重定向目标、`sh/bash/zsh -c` 内层命令、`python -c` / `node -e` 文件 API 语境里的字符串路径、`cd` / `pushd` / `env -C` 简单工作目录折算、`<SKILL_ROOT>` 占位符和路径型命令裸参数；bash 中相对 `assets/...` / `references/...` 等 skill 文件引用也会进入 selected branch allowlist，显式 `cd` 到 workspace 后不会误当 skill 文件；skill 文件在执行期只读，`grep/find/ls` 探索 skill root 和 `write/edit` 修改 skill 文件都会被拦截 | P0 |
| `argument-hint` / `arguments` | 已解析并进入 skill index / invocation contract；兼容 `argument_hint` / `argumentHint` | P1 |
| `model` override | 无 | P2 |
| `context: fork` | 已支持子 agent/session 隔离、结果 merge 和审计元数据 | P1 |
| skill hooks | 无 | P2 |
| dynamic skill discovery | 已支持工具访问路径祖先目录中的嵌套 `.claude/skills` / `.agents/skills` 动态加载、merge、相对 `paths` 补全为 workspace-relative pattern、当前 `skill` tool 刷新，以及基于 `.md` 文件签名的新增/修改/删除重载 | P0 |
| MCP skills | resource adapter + stdio JSON-RPC transport + Streamable HTTP/SSE response；缺 OAuth/授权策略 | P1 |
| plugin/bundled/managed/user/project 多来源 | 已有 source registry + priority merge | P1 |

P0 目标：

1. ✅ 已扩展 `Skill` struct：`WhenToUse`、`AllowedTools`、`Paths`、`Arguments`、`ArgumentHint`、`Context`、`Branches` 等；`branches.<name>.paths` 支持精确文件、目录和 glob/`**` 展开。
2. ✅ 系统提示中已展示 `when_to_use` / argument contract / paths 等轻量索引。
3. ✅ 文件工具读/写/编辑/搜索/list 时，已按访问路径发现嵌套 `.claude/skills` / `.agents/skills`。
4. ✅ `paths` 匹配后已支持单一 skill 自动激活、多 skill steering、工具访问后 follow-up。
5. ✅ skill 调用时已把 `allowed-tools` 注入 runtime tool policy，并动态缩小可见工具和工具说明；普通输入中显式点名已加载 skill 时也会自动展开完整 skill 并激活 policy，且显式点名另一个 skill 会覆盖上一轮 skill continuation。

P1 目标：

- ✅ `context: fork`：已支持子 agent/session 隔离、provider fork、结果 merge 和审计元数据。
- ✅ 多来源加载优先级：source registry + `MergeByPriority`。
- ✅ 动态 skill cache invalidation 已覆盖嵌套 project skill source；远端/MCP source 仍依赖 provider/cache 策略。
- ✅ MCP skills：resource adapter + stdio JSON-RPC transport + Streamable HTTP/SSE response；缺 OAuth/授权策略。
- ✅ 持久化缓存：`CachedSourceProvider` + `JSONFileSourceCache`。
- 远端/MCP cache invalidation 策略仍待设计。

## 4. 推荐落地路线

### Phase A：Prompt 与 Tool Metadata 地基

目标：先让 agent runtime 能承载更复杂的 coding-agent 行为。

任务：

- 引入 prompt section registry。
- 拆分 coding system prompt。
- 扩展 tool optional interfaces。
- agent loop 执行工具前统一 permission decision。
- tool result 加统一 budget / truncation 策略。

完成标准：

- 新增 prompt section 不需要改一个巨型 builder。
- 工具是否只读、是否破坏性、路径是什么、是否需要权限，runtime 可以统一询问。
- 现有 7 个工具全部实现基础 metadata。

### Phase B：Coding Tools 防错闭环

目标：把 coding-agent 从“能改文件”提升为“可靠改文件”。

任务：

- readFileState。
- read-before-edit/write。
- stale mtime check。
- structured patch。
- bash streaming。
- bash destructive classifier 初版。
- read binary/device/size guard。

完成标准：

- 未读文件不能直接 edit/write，除非明确 create。
- 文件被外部改动后，edit/write 会阻止并要求重读。
- bash 长输出可以 streaming，失败可见。
- destructive bash 进入 permission decision。

### Phase C：Skills 动态化 ✅ 已完成

> **状态**：全部完成。详见 `internal/skill/`、`internal/agent/tool_policy.go`、`internal/tools/skill.go`。
>
> - frontmatter 增强（`when_to_use` / `allowed-tools` / `paths` / `branches` / `arguments` / `context`）。
> - dynamic discovery（嵌套 `.claude/skills` / `.agents/skills` 自动加载）。
> - conditional activation by paths（单 skill 自动激活、多 skill steering）。
> - allowed-tools 生效（runtime tool policy + violation 渐进式收紧）。
> - MCP skills（HTTP Streamable + Stdio JSON-RPC）。
> - 持久化缓存（`CachedSourceProvider` + `JSONFileSourceCache`）。

### Phase D：Fork / Sub-agent / Deferred Tools（部分完成）

> **已完成**：
> - ✅ skill `context: fork`：子 agent/session 隔离、provider fork、结果 merge 和审计元数据。
> - ✅ session fork：`ForkEmpty()` 创建空隔离 session，不再简单复制 JSONL。
>
> **未完成**：
> - AgentTool / sub-agent 初版。
> - ToolSearch / shouldDefer 初版。
> - MCP tool pool 接入。

完成标准：

- 主 agent 可创建隔离子 agent 执行任务。
- 大量低频工具不需要全部进入首轮 prompt。
- MCP tools 能进入统一 tool pool。

## 5. 不建议现在做的事

当前阶段不建议优先投入：

- Desktop 工作台增强。
- TUI。
- Provider proxy。
- 大型插件市场。
- IM/H5/定时任务。
- 完整 LSP/Notebook/PDF/image 全家桶。

这些都可以做，但应该建立在 agent runtime 和 coding-agent app 足够厚之后。

## 6. 最小 P0 清单

如果只做一轮最小对齐，建议按这个顺序：

1. Prompt section registry。
2. Tool metadata optional interfaces。
3. Permission decision 接入 agent loop。
4. Read state + edit/write stale check。
5. Bash streaming + destructive classifier 初版。
6. ~~Skill frontmatter：`when_to_use` / `allowed-tools` / `paths`。~~ ✅ 已完成。
7. ~~Dynamic skill discovery。~~ ✅ 已完成。

当前进度：7 项中已完成 2 项（#6、#7），剩余 5 项。
