# Skill 加载探索过度问题修复记录

> 目标：记录 `guizang-ppt-skill` 测试中出现的“agent 读完 skill 后做大量探索”的根因、修复方式，以及后续继续对齐 Claude Code skill runtime 的方向。

## 1. 现象

测试 `guizang-ppt-skill` 时，agent 在加载 skill 后容易继续做 `ls` / `find` / `grep` 之类的探索动作，甚至把整个 skill 目录当作未知项目理解。

这和 Claude Code 的表现不同。Claude Code 加载 skill 后更倾向于按 skill body 的固定工作流走：只读被明确引用的模板、reference、script，而不是扫描 skill 目录。

## 2. 为什么 guizang 会暴露这个问题

`guizang-ppt-skill` 是重型工作流 skill，不是几行提示词。

它的 `SKILL.md` 内部已经给出了明确加载顺序：

- 根据风格选择 `assets/template.html` 或 `assets/template-swiss.html`。
- 选择主题时读 `references/themes.md` 或 `references/themes-swiss.md`。
- 选择布局时读 `references/layouts.md`，瑞士风先读 `references/swiss-layout-lock.md` 再读 `references/layouts-swiss.md`。
- 需要地图、截图、配图时才读对应 reference。
- 生成后再读 `references/checklist.md` 或运行 `scripts/validate-swiss-deck.mjs`。

所以正确行为不是“完全不读额外文件”，而是“只读 skill 当前分支明确引用的文件”。

## 3. 根因

### 3.1 skill 内容注入缺少执行契约

原来的 `FormatInvocation` 只注入：

```text
<skill name="...">
References are relative to ...

<skill body>
</skill>
```

这只能告诉模型“相对路径从哪里解析”，但没有告诉模型：

- 不要再读 `SKILL.md`。
- 不要探索 skill 目录。
- 只读当前分支显式引用的文件。
- 遇到 A/B 风格分支时只走匹配分支。
- 缺少关键信息时应该问 1-3 个问题，而不是靠探索补信息。

### 3.2 全局 coding-agent prompt 与 skill workflow 有冲突

coding-agent 的通用策略通常鼓励先理解上下文、读文件、搜索定位。

这对修代码是对的，但对 `guizang-ppt-skill` 这类“已经给出完整操作手册”的 skill，会变成多余探索。

### 3.3 slash fallback 没有真正展开 skill

交互式 `/skill-name` fallback 之前只是改写成：

```text
Use the "xxx" skill for this request.
```

这会让模型再自己决定下一步，弱化了 slash command 在 Claude Code 里的“展开为完整 prompt”的语义。

## 4. 本次修复

### 4.1 `FormatInvocation` 增加 skill execution contract

现在 skill tool 和 slash fallback 注入的内容都会包含：

- `Base directory for this skill`
- `SKILL_ROOT`
- `<SKILL_ROOT>` / `${CLAUDE_SKILL_DIR}` / 相对路径解析规则
- 不读当前 `SKILL.md`
- 不用 `ls/find/grep/bash` 探索 skill 目录
- 只读 skill 明确引用、且属于当前分支的文件
- 分支信息缺失时问 1-3 个关键问题
- workspace 外围文件只按用户任务需要检查

这样模型拿到的是“可执行工作流”，不是“一个待探索资料夹”。

### 4.2 slash fallback 改成直接展开 skill

交互式 `/skill-name args` 现在走：

```go
skill.FormatInvocation(*s, args)
```

这更接近 Claude Code slash skill 的语义：用户显式调用 skill 时，当前请求直接变成 skill prompt。

### 4.3 解析更多 frontmatter

`Skill` 现在保留这些字段：

- `when_to_use`
- `when-to-use`
- `allowed-tools`
- `allowed_tools`
- `allowedTools`
- `paths`
- `context: fork`
- `branches`

frontmatter 解析已改为 YAML 解析，支持 `allowed-tools` / `allowed_tools` / `allowedTools` 和 `paths` 使用单行逗号列表或 YAML list，也支持 `when_to_use` / `when-to-use` 这类折叠文本。`context` 解析会忽略大小写和空白，`disable-model-invocation` / `disable_model_invocation` / `disableModelInvocation` 都能识别。`branches` 可声明互斥 workflow 分支、别名和每个分支允许读取的 skill 文件；运行时会优先用 `branches.<name>.aliases` 推断 selected branch，并按 `branches.<name>.paths` 生成 allowed/blocked skill file allowlist。branch paths 支持精确文件、目录和 glob/`**` 展开；多个分支都声明的重叠文件会被视为 shared file，不会误拦。

`argument-hint` / `argument_hint` / `argumentHint` 和 `arguments` 也已进入 loader：前者作为自由文本参数提示，后者支持 YAML list、map 简写和 `{name, description, required}` 结构。它们会出现在 `<available_skills>` skill index、path discovery steering 和 `FormatInvocation` execution contract 中，帮助模型在调用 skill 时传入更贴近作者意图的参数，而不是空参调用后再探索。

`allowed-tools` / `allowed_tools` 现在会进入 runtime policy：工具名用于收窄 LLM 可见 tools，`Bash(...)` 规格会保留为 bash 命令级 allowlist。`context: fork` 已接入运行时隔离语义，但还不是完整独立子 agent。

### 4.4 Skill execution policy

端到端测试证明，单靠 prompt contract 不够；模型仍可能用工具绕过 workflow。因此这次把 skill 激活从“只注入消息”升级为“同时激活运行时策略”。

实现点：

- `ToolResult` 新增 `ActivatePolicy`，skill tool 执行成功后激活 `ToolPolicyActivation`。
- slash fallback 显式调用 skill 时也调用 `AgentSession.ActivateSkillPolicy(...)`，不经过 tool 也能进入同一套 policy。
- 用户在普通输入里显式点名已加载 skill 时，`AgentSession.prepareInputForPrompt(...)` 会直接展开完整 skill 并激活同一套 policy；不会只把 skill 名交给模型自行决定是否调用 tool。若上一轮存在 active skill 记录，显式点名同一个 skill 会保留 continuation 语义和上一轮 branch/args；显式点名另一个 skill 会覆盖上一轮 skill context，避免“继续用 docs-skill ...”被误当成 guizang continuation。
- `Agent.llmRequest(...)` 会按 active policy 动态缩小 `tools` 列表；模型后续请求里看不到不允许的工具。
- active policy 还会动态改写可见 tool definition 的 description：`read`/`write`/`edit`/`grep`/`find`/`ls`/`bash` 等路径型工具会携带当前 skill、selected branch、允许精确 skill 文件、被阻断分支文件、`Bash(...)` 命令模式和 workspace `paths` 约束。模型在调用前就能看到当前工具边界，而不是只在撞 guardrail 后学习。`bash` description 会和命令策略保持一致：没有 `Bash(...)` 规格时提示不要用 shell inspection；存在明确 `Bash(...)` allowlist 时提示只用这些 command patterns，不再同时说“不要用 cat/sed/grep”。
- 工具路径硬校验按 canonical tool name 识别路径字段：`read/write/edit/grep` 使用 `path` / `file_path`，`find/ls` 使用 `path` / `dir`；`search -> grep`、`glob -> find` 这类兼容别名会进入同一套 selected branch / workspace `paths` 约束，不会因为工具名别名绕过 skill 目录探索禁令。不会把 `pattern` 这类非路径字段误当成 workspace 路径，同时避免不同工具实现或兼容层使用 `file_path` 别名时绕过 selected branch / workspace `paths`。
- `skill` tool 是 skill workflow 的入口工具，不会进入 active execution policy。即使某个 skill 的 frontmatter 误写了 `allowed-tools: skill, read`，active policy 也会剔除 `skill`，避免当前 skill 执行中嵌套加载另一个 skill 并覆盖 policy；模型仍尝试调用 `skill` 时会被视为 workflow escape，在违规阈值处和 skill 探索一样进入 write/edit-only recovery。
- 工具执行前再次做硬校验，防止模型 hallucinate 已被移除的 tool call；工具名级 policy 校验会先于 tool lookup 执行，因此模型调用已经被过滤掉或根本未注册的工具名时，也会计入 skill policy violation 并触发 recovery，而不是退化成普通 `tool not found`。
- active policy 装载时会把 frontmatter `allowed-tools` 与当前 agent 实际注册工具取交集。移植自 Claude Code 的 skill 如果声明了 `Task` / `Glob` / 其他当前 runtime 没有的工具，这些工具不会进入 policy snapshot；模型继续调用时会被当作 skill guardrail violation，而不是普通缺工具错误。
- `allowed-tools` / `allowed_tools` 缺失时使用保守默认：`read, write, edit, bash`。
- `allowed-tools` 中的 `Bash(npm run build:*)` / `Bash(node scripts/validate.mjs:*)` 这类规格不再只降级成“允许 bash”，而是进入 active policy 的 bash 命令模式约束；声明了 Bash 规格时，其他 bash 命令会被 guardrail 拒绝并计入 violation。命令模式匹配会生成多个候选：保留外层 wrapper 命令以兼容 `Bash(bash -lc:*)`，也会剥离前置 `env` / `FOO=bar` / `env -C DIR`，并进入 `sh/bash/zsh -c "..."` 内层命令后匹配真实命令；`paths` 仍检查原始命令里的 env 路径值、cwd 参数和内层命令路径。没有明确 `Bash(...)` 规格时，active skill policy 会继续拒绝 `ls/find/grep/cat/head/...` 这类 shell 探索命令；如果 skill 作者显式 allowlist 了 `Bash(cat:*)` / `Bash(sed:*)` 等 inspection 命令，则信任该命令规格，但仍继续执行 selected branch、skill 文件只读和 workspace `paths` 硬约束。
- bash 中的相对 skill 文件引用也会参与 selected branch allowlist：默认 cwd 下明显的 `assets/...` / `references/...` / `scripts/...` 等 skill-relative 引用，以及显式 `cd <SKILL_ROOT>` / `cd $CLAUDE_SKILL_DIR` 后的相对路径，都会按 skill root 解析；如果已经 `cd docs` 这类 workspace 路径，则后续 `assets/...` 会按 workspace path 处理，避免误拦。bash 明确引用 `<SKILL_ROOT>/SKILL.md`、`$CLAUDE_SKILL_DIR/SKILL.md` 或整个 skill root 时，也会像 `read SKILL.md` / `find <SKILL_ROOT>` 一样被当作重复探索 guardrail violation。
- `paths` 会进入 workspace 路径模式约束；有声明时，读写搜改必须落在声明路径内，精确 skill asset 例外。`*.pptx`、`docs/*.md`、`docs/**`、`src/**/*.go` 这类常见模式同时支持相对路径、绝对 workspace 路径后缀和 basename glob 匹配；bash 命令里的文件路径参数也会套同一套 `paths` 约束，避免用 shell 绕过 workspace pattern。bash 路径提取覆盖普通路径、`--out=path` / `VAR=path`、`--out path` / `-o path` / `-C path` / `--directory path` 等分离式 flag value、`OUT_DIR=tmp` 这类路径型 env assignment、`NODE_ENV=prod mkdir tmp` 这类前置 env assignment 后的真实路径命令、`>` / `>>` / `<` / `2>` 这类 shell 重定向目标、`sh/bash/zsh -c "..."` 内层命令字符串、`python -c` / `node -e` 文件 API 语境里的字符串路径、`cd` / `pushd` / `env -C` 简单工作目录折算、`<SKILL_ROOT>` 占位符，以及 `mkdir/cp/mv/touch/tee/install/rsync` 这类路径型命令里的裸相对目录参数。工具访问后的动态 skill 发现 hook 也按 canonical path tool name 提取路径，`search -> grep`、`glob -> find` 和大小写变体都会触发 path-matched skill discovery/steering。
- selected branch 会优先从 frontmatter `branches` 推断：例如 `branches.swiss.aliases` 包含 `风格 B / 瑞士 / Swiss Style` 时，请求命中这些词会走 swiss 分支，并只允许 `branches.swiss.paths` 声明的文件。`branches.<name>.paths` 可写精确文件、目录、`*.md`、`**` 等模式，runtime 会展开到当前存在的 skill 文件。没有声明 `branches` 的旧 skill 继续用启发式兜底：`风格 A / 电子杂志 / magazine` 走 magazine，`风格 B / 瑞士 / Swiss` 走 swiss；asset allowlist 也会识别 `template-a.html` / `template-b.html`、`style-a-*` / `style-b-*`、`*-swiss.*` 等常见文件名分支标记。若用户没有给出分支，但 skill 声明或引用了分支专属文件，这些文件会进入 blocked skill paths；模型必须先问用户选择分支，不能先通过 `read` 或 `bash cp/cat/...` 读取 A/B 模板试探。
- skill asset 只允许读取当前分支明确引用的文件。例如 Swiss 分支允许 `template-swiss.html`、`themes-swiss.md`、`layouts-swiss.md`、`swiss-layout-lock.md`，拒绝 `SKILL.md` 和 A 分支模板。即使 frontmatter 显式允许 `grep/write/edit`，active policy 也会拒绝在 skill root 下 grep/search 或修改 skill 文件；相对 `grep assets/...` 只会在该路径命中当前 policy 已知的 allowed/blocked skill asset 时拦截，普通 workspace `assets/...` 不会被误当成 skill 文件。skill 文件在执行期始终是只读参考。

原来的通用工具层粗 guardrail 已从执行路径移除。`bash ls/find/cat/head/wc` 这类拒绝现在只发生在 active skill policy 内，不影响普通 coding-agent 场景。

### 4.5 重复违规恢复与 hard-stop

active policy 维护 violation 计数：

- 首次违规：返回结构化 tool error，说明 active skill、被拒原因、恢复路径。
- 达到阈值：注入 `<skill_policy_recovery>` user message，强制要求停止探索、只走当前分支精确文件。
- 重复 shell inspection：从 active tool policy 中移除 `bash`，后续 LLM 请求的 `tools` 会从 4 个缩到 3 个。
- skill 探索 / workflow escape 类重复违规在阈值处直接进入 write/edit-only：包括回读 `SKILL.md`、读未选/未允许分支文件、`find/grep/ls` 继续探索 skill 目录、active skill 内尝试调用 `skill` tool 嵌套加载其它 skill 等，立即移除 `read/find/grep/ls/bash`，终止当前“继续探索/继续读源文件/切走 workflow”的错误路径。
- 其他重复违规到 `max+1`：进入 write/edit-only 模式，移除 `read/find/grep/ls/bash`。
- write/edit-only 后的 recovery 文案会明确写入 `Read/search/exploration has been terminated` 和 `Use write or edit now`，不只依赖工具列表变化。
- 如果某个 skill 本来只允许 `bash` / `read` 这类探索工具，重复违规后 allowed tool set 可能被删空；现在 active policy 下空 allowed set 表示“没有工具可见/可执行”，不会退化成 unrestricted。模型即使继续 hallucinate 已被移除的工具名，也会先命中 skill policy violation，而不是执行真实工具；recovery 和 tool result 会明确提示 no-tools hard-stop，要求不要再调用工具，改为提问或返回 blocked 状态。
- active skill policy 期间，工具调度器不再把 read/grep 这类并发安全工具合并成并行批次；所有 tool call 会顺序执行。这样同一个 assistant turn 内如果第一个 call 触发 violation 并移除 `bash` / `read`，后续 call 会立即看到收窄后的 policy，避免同批并发把错误探索继续跑完。

### 4.6 Invoked skill state 与 compact 恢复

skill 调用现在不只是 transient prompt：

- session JSONL 新增 `skill_invocation` entry，结构化记录 skill name、args、file path、base dir、selected branch、allowed tools、allowed skill files、paths、compact context；fork skill 还会记录 `fork_session_path`。
- `skill` tool 激活 policy 时会写入 `skill_invocation`。
- slash skill fallback 激活 policy 时也会写入同样的 session entry。
- `BuildContext` 不把 `skill_invocation` 当普通对话消息注入，避免污染上下文；它作为结构化状态留给 compact/recovery/audit 使用。
- 自动 compact 和手动 `/compact` 时，如果当前有 active skill policy，会把 `<active_skill_context>` 作为额外摘要指令交给 summarizer。
- compact 结果会确定性追加同一段 `<active_skill_context>`，不依赖 summarizer 是否“记得住”。
- 后续 turn 如果用户输入像“继续 / 刚才 / 那份 / 换风格 / restyle / continue”这类 continuation，或明确点名最近一次 skill name，会优先查找最近一次 `skill_invocation` 对应的已加载 skill。
- 如果该 skill 仍可用，会自动重新展开完整 skill body，并重新激活 skill execution policy；新的 branch 会按当前 continuation 请求重新推断，例如“换成瑞士风”会从上一轮 magazine 切到 swiss。
- 这条跨 turn hard reactivation 只对 continuation/显式 skill name 生效；普通新任务不会继承上一轮 active policy，避免旧 skill 错绑后续无关请求。
- continuation 分支推断会优先读取 `Current continuation request`，只有当前请求没有明确分支时才回退到 previous args / previous branch；避免“上一轮风格 A”压过“这轮换成风格 B”。
- 如果该 skill 当前不可用，才退回注入 `<previous_skill_context>`，带上最近一次 `skill_invocation` 的 skill name、branch、args、SKILL_ROOT、允许文件、被阻断的 branch-specific 文件和 compact context。
- session 层新增 `SkillInvocations(ctx)` 查询接口，按当前 path 的时间顺序返回所有 skill 调用记录，并带上 entry id、timestamp、branch、allowed tools、allowed skill paths、path patterns、compact context 和 `fork_session_path`。HTTP server 同步暴露 `GET /sessions/{id}/skills`，给 CLI/桌面端/API 做“本轮 agent 为什么这么走”的审计面板提供稳定数据源。
- agent 层新增 active skill policy snapshot：`Agent.ActiveToolPolicySnapshot()` / `AgentSession.ActiveToolPolicySnapshot()` 会返回当前是否有 active policy、skill name、selected branch、动态收窄后的 allowed tools、allowed/blocked skill files、path patterns、violation 计数、最后一次 violation 原因、write/edit-only 和 no-tools hard-stop 状态。HTTP server 暴露 `GET /sessions/{id}/skill-policy`；如果 session 只存在于磁盘、当前未加载运行时，则返回 `active:false`，避免把历史 invocation 误当作仍在生效的 policy。

这补齐了 CC 类系统里很关键的一点：长上下文压缩后，模型仍能知道当前 skill、分支、允许文件和禁止探索规则。

### 4.7 `paths` 动态发现与自动激活

`paths` 现在同时用于四件事：

- skill 激活后：作为 workspace 路径约束，限制 read/write/edit/search 的目标路径。
- skill 激活前：普通用户输入中出现匹配路径，且只有一个可见 skill 匹配时，`AgentSession` 会自动展开该 skill body，并激活对应 skill execution policy。
- skill 激活前：如果多个 skill 同时匹配同一路径，`AgentSession` 会注入 `<path_matched_skills>`，提示模型选择并调用对应 `skill` tool，避免误激活。
- 工具执行后：如果 read/write/edit/grep/find/ls 实际访问了匹配 `paths` 的文件，tool result 会追加一次 `<path_matched_skills>` follow-up，把模型拉回对应 skill。每个 skill 只提示一次，避免刷屏。若当前已有 active skill policy，则不注入新的 path-matched skill steering，避免打断正在执行的 skill workflow。
- 工具执行后：如果访问路径的祖先目录里存在嵌套 `.claude/skills` 或 `.agents/skills`，runtime 会动态加载这些 skill source、按优先级 merge 到当前 session，并刷新当前 agent 的 `skill` tool 可见列表；新发现且匹配路径的 skill 会通过 `<path_matched_skills>` follow-up 提醒模型先调用 `skill` tool。active skill policy 期间同样会抑制这类新 skill follow-up，避免 guizang 等重型 skill 执行中被其它 path skill 抢走控制流。
- 嵌套在 workspace 内的 `.claude/skills` / `.agents/skills` 会归类为 project source，参与 project > managed/user/plugin/system 的合并优先级。
- 嵌套 project skill 的相对 `paths` 会额外补成 workspace-relative pattern。例如 `packages/app/.claude/skills/x/SKILL.md` 中的 `paths: ["src/**"]` 会同时匹配 `packages/app/src/**`，避免 policy 把包内路径误解成仓库根目录路径。
- 动态发现过的嵌套 skill 目录会记录 `.md` 文件签名；后续工具再次访问相关路径时，如果 `SKILL.md` 或同目录 markdown 文件发生新增、修改、删除，runtime 会重新加载该 source root、替换旧 skill，并同步刷新当前 `skill` tool。删除 skill 文件后，该 skill 会从当前 session 可见列表中移除。

例如某 skill 声明：

```yaml
paths:
  - docs/**
```

用户输入 `update docs/guide.md` 时，请求会被增强为：

```xml
<path_matched_skills>
...
</path_matched_skills>

update docs/guide.md
```

单一明确匹配时这是强制自动调用；多匹配时仍是 discovery steering。这样把 `paths` 从“展示字段”推进到运行时 activation/discovery 信号。

### 4.8 `context: fork` 的当前语义

`context: fork` 已经从“同 loop 裁剪历史”推进到 child agent 执行路径：

- execution policy 会记录 `ExecutionContext=fork`，并写入 `skill_invocation`。
- fork skill 的违规阈值更紧：默认 violation 阈值从 2 收敛到 1，更快注入 recovery 并移除错误路径。
- recovery message 和 compact note 会明确写入 `Execution context: fork`，要求模型把它当隔离 skill workflow 处理。
- `FormatInvocation` 不再说“只是 inline for now”，而是要求保持 isolated workflow boundary，减少中间探索污染主任务。
- active `context: fork` 且 fork session 已创建时，主 agent loop 会把当前 pending 交给 child agent 执行。child agent 复用同一 model/provider/tools/system/lifecycle hooks 和当前 active policy，但不携带主 goal，也不读取主 session history。
- active `context: fork` 期间，主 session JSONL 不再持久化 skill body、assistant/tool 中间过程、tool results 和 recovery follow-up；只在最终 stop 结果返回时把最终 assistant message 写回主线。
- active `context: fork` 期间，发给模型的 in-memory `messages` 会裁剪到最近一次完整 skill invocation 起点之后；直接 slash/path/continuation 展开的 fork skill 不会看到主历史，通过 `skill` tool 激活的 fork 会在 skill body follow-up 后重置到 fork 起点。
- active `context: fork` 期间，如果主会话启用了 goal-driven loop，fork skill 结束时不会用主 goal 做完成度评估，也不会注入 `Reminder: your current goal...` 这类主线 continuation，避免主任务目标污染隔离 skill workflow。
- JSONL storage 已实现 `SessionStorage.Fork(ctx, targetID)`：可以复制 root 到目标 leaf 的路径到新的 JSONL storage，fork 后追加不会污染源 session。这是后续真正独立 sub-agent/session fork 的底座。
- JSONL storage 另有 `ForkEmpty(ctx)`：skill `context: fork` 使用空白 fork session，而不是复制主线历史。这样 fork 审计文件只包含 skill invocation 和该 skill 的 user/assistant/tool 中间过程。
- active `context: fork` policy 激活时会创建空白 fork session；fork 期间的 skill body、assistant/tool 中间过程、tool results、recovery follow-up 会写入 fork session，主 session 只合并最终 assistant result。
- 主 session 的 `skill_invocation` 会记录 `fork_session_path`，因此 UI/API/审计可以从主线定位 fork session 的完整中间过程。
- 主 session 会额外写入 `skill_result` entry，记录 fork result merge：skill name、execution context、fork session path、merge mode（当前为 `final_assistant`）、`artifacts`、`changed_files`、`operations`、结构化 `changes`、merge `summary` 和 final result preview。`write` / `edit` 工具会通过 `ToolResult.Details` 返回结构化 file change details（path/tool/operation/summary/diff/bytes/lines），`skill_result.changes` 优先消费这些 details；没有 details 的第三方写入工具会回退到 tool result 文本摘要。`BuildContext` 不把它注入模型上下文，HTTP API 暴露 `GET /sessions/{id}/skill-results` 供 CLI/桌面端/API 审计。
- 文件 diff 生成已增加输入预算：超大文件或超大 line matrix 不再做 O(n*m) LCS，而是返回摘要型 omitted diff；`skill_result` 层仍会对 diff 文本做最终截断，避免重型 deck/HTML skill 产物撑爆 fork merge 记录。
- fork child 会优先通过 provider 的 `Fork()` 创建独立 provider 实例；OpenAI / Anthropic / DeepV / Mock provider 都已实现基础 fork，真实 provider 会重新创建 HTTP client。没有实现 `Fork()` 的 provider 会回退到复用父 provider，保持兼容。
- fork child 会发出 `skill_fork_start` / `skill_fork_end` stream event，包含 skill name、fork session path、是否创建独立 provider、退出状态（`completed` / `canceled` / `error`）以及基础 artifact/changed-file 列表，为 UI/API 后续做子任务状态面板和取消控制提供事件底座。
- parent agent 会保存 active fork child 的 cancel handle，`Agent.CancelActiveFork()` / `AgentSession.CancelActiveFork()` 可以只取消当前 fork skill child，不取消父 prompt context；REST API 暴露 `POST /sessions/{id}/skill-fork/cancel`，WebSocket `cancel` 也会先取消 active fork child，再取消父 prompt。

这已经具备 child agent loop、主 goal 隔离、主 session 隔离、空白 fork session、fork session 中间过程留痕、独立 provider 实例、基础子任务状态事件、显式 fork child cancel、结构化 merge record、artifact/changed-file/operation/change summary/file diff merge、diff 输入预算/截断和最终结果回写。剩余差距主要是更大真实任务验证。

### 4.9 多来源 skill 合并

skill loader 现在不再只做“多目录扁平加载 + workspace-only 过滤”，而是给每个 skill 标注来源：

- `project`：当前项目 `.claude/skills` / `.agents/skills`
- `user`：用户级 `.claude/skills` / `.agents/skills`
- `managed`
- `plugin`
- `bundled`
- `system`
- `unknown`

同名 skill 会按来源优先级合并：

```text
project > user > managed > plugin > bundled > system > unknown
```

同一来源内仍用更短、更靠近根的路径作为 tie-breaker，避免嵌套副本意外覆盖主 skill。

runtime 默认会同时加载项目级、用户级、Codex managed/system、以及 plugin cache 下发现到的 `skills` source root。plugin cache 不会整棵树作为 skill root 粗暴注入，而是只把递归发现到的 `skills` 目录交给 loader。用户级/global skill 不再因为“不在 workspace 内”被直接过滤掉；`skill` tool 仍会注入完整 `SKILL.md` 正文。为了避免用户级 skill 引用的模板、reference 文件被 `read` 的 workspace 限制挡住，coding agent 的 `read` 工具增加了只读 `SkillRoots` 白名单，只允许读取已加载 skill root 下的文件；写入、编辑、目录搜索不会因此放开到 workspace 外。

skill loader 现在也有统一 `SourceRegistry` 抽象：任何 source provider 都可以实现 `LoadSkills(ctx)` 并输出带 `Source` 的 skill 列表，registry 会复用同一套 `MergeByPriority` 规则。本地目录加载已迁移为 `LocalSourceProvider`，runtime 默认 source root 通过 registry 加载。`HTTPSkillSourceProvider` 已提供具体远端 JSON index 协议实现：支持 `{"skills":[...]}` 或直接数组两种响应、Bearer token 认证、额外 header、context/timeout、响应大小限制、远端 skill name/description 校验和 source/source_root/file_path 补齐。运行时已接入远端 HTTP skill sources：`PI_GO_REMOTE_SKILL_SOURCES` 可配置逗号分隔的 `endpoint|name|token`，`PI_GO_REMOTE_SKILL_TOKEN` 提供默认 token，`PI_GO_SKILL_SOURCE_CACHE_DIR` 可覆盖持久化 cache 目录；未显式设置时默认使用 `DataDir/skill-source-cache`。`MCPSkillSourceProvider` 已提供 MCP resource adapter：通过 `ListSkillResources(ctx)` / `ReadSkillResource(ctx, uri)` 读取远端 `SKILL.md` 内容，复用本地 frontmatter 解析、source/source_root/file_path/base_dir 补齐、诊断和同一套 merge/cache pipeline。`StdioMCPSkillClient` 已提供真实 MCP stdio JSON-RPC transport：启动本地 MCP server，执行 `initialize` / `notifications/initialized` / `resources/list` / `resources/read`，按 Content-Length framing 读写消息，并支持 text/blob resource content。`HTTPMCPSkillClient` 已提供 MCP Streamable HTTP transport：POST JSON-RPC 到远端 endpoint，支持 session id、Bearer token、`application/json` 响应和 `text/event-stream` 响应中的 JSON-RPC payload。运行时可通过 `PI_GO_MCP_SKILL_SOURCES=name|command|args` 接入 stdio MCP skill source，或通过 `PI_GO_MCP_SKILL_SOURCES=name|endpoint|token|streamable-http` 接入 HTTP/SSE MCP skill source，两者都复用同一套远端缓存降级。

`CachedSourceProvider` 已提供远端/MCP provider 的失败降级底座：首次成功加载后缓存非空 skill 列表；后续 provider 返回诊断且没有 skills 时，会返回上一次成功的 skills，并追加 `cache_fallback` 诊断。这样远端 registry 临时不可用时不会把当前可用 skill 索引打空。`SourceCache` / `JSONFileSourceCache` 已提供可选持久化缓存；成功加载会写入 JSON cache，进程重启后也能在远端失败时恢复上一次成功结果，cache 写入失败会通过 `cache_store_failed` 诊断暴露。MCP OAuth/授权策略仍留给后续 MCP client 实现。

这补上了 CC 类 skill runtime 很关键的一层：项目可以覆盖用户 skill，用户 skill 可以覆盖更低优先级托管/plugin/bundled/system skill，同时全局 skill 的资源文件仍可通过精确路径读取。

### 4.10 Skill listing budget

系统提示中的 `<available_skills>` 现在只作为轻量索引，不注入 skill body，并额外加了预算保护：

- 上层未传模型窗口时，默认 skill listing 预算约 12k 字符。
- coding-agent runtime 会把当前模型的 context window 传给 prompt builder；skill listing 预算会按窗口动态计算，并限制在保守上下限内。
- prompt builder 会先统计 skill listing 之前已经构造出的系统提示和工具 schema token，占用量通过统一的 model-aware token counter 估算，再从 context window 中扣除并计算剩余 skill index 预算；中文 skill 描述不会再按 `4 chars ~= 1 token` 被明显低估。
- prompt builder 还会估算 `tools` API 参数里的工具 name、description、JSON parameters schema 占用，并从 skill listing 预算中扣除。
- token counter 已增加可注册 backend registry；默认仍使用 heuristic backend，后续接 provider 官方 tokenizer 时只需注册新的 backend 覆盖对应模型族，`compaction` 和 prompt builder 调用面不需要改。
- 单个 `description` / `when_to_use` / `argument_hint` / `arguments` / `paths` 字段会截断到固定上限，避免某个 skill 的 frontmatter 挤爆系统提示。
- 超出预算的后续 skill 会被省略，并写入 `<omitted count="...">` 说明。
- 被省略的 skill 没有从 runtime 删除；如果用户显式命名 skill，仍可通过 skill tool / slash invocation 加载完整内容。

这让大量 project/user/plugin skills 共存时，系统提示更接近 CC 的“索引 + 按需加载”模型。

## 4.11 端到端验证记录

测试命令形态：

```bash
PI_GO_ENABLE_BASH=true PI_GO_DATA_DIR=tmp/guizang-skill-e2e-*/data \
  /tmp/pi-agent-skill-e2e -mode chat
```

历史测试输入两轮：

1. `/guizang-ppt-skill ... 机器学习 PPT ... 风格 A ...`
2. `继续使用 guizang-ppt-skill ... 切换成风格 B ...`

prompt-only / 工具层粗 guard 阶段观察结果：

- 修复前：模型先正确读取 `assets/template.html`、`references/themes.md`、`references/layouts.md`，随后开始用 `bash head/cat/ls/find` 探索 skill 目录，还扫了 `.claude/skills` 和 repo 上级目录。
- 加 contract 后：仍会探索，说明 prompt-only 不足。
- 加工具层 guard 后：`bash ls/find/cat/head/wc`、`ls .claude/skills/...`、`find .claude/skills/...` 被拒绝；模型能看到恢复说明，但本地默认 provider `openai/mimo-opus` 仍会反复撞墙。

runtime policy 阶段观察结果：

- 风格 A slash skill 首轮请求直接 `tools=4`，只有 `read/write/edit/bash`。
- 模型先正确读取 A 分支：`assets/template.html`、`references/themes.md`、`references/layouts.md`。
- 模型随后尝试 `bash ls/cat/head/wc`，policy 拒绝并注入 `<skill_policy_recovery>`。
- 重复违规后，后续请求降为 `tools=3`，`bash` 被移除；模型再尝试 `bash` 时被硬拒绝。
- 模型最终改用 `write` 直接生成文件：`tmp/guizang-skill-policy-e2e-hardstop/work/ml-deck/ppt/index.html`。
- 风格 B 验证中，policy 正确推断 `Selected branch: swiss`，允许读取 swiss 模板/主题/layout lock/layouts，拒绝 `SKILL.md` 和 `find`。
- 风格 B 本地模型仍有“反复读模板/回头读 SKILL.md”的倾向；hard-stop 已补到超过阈值后进入 write/edit-only，避免继续读源文件。
- `internal/skill/policy_eval.go` 新增轻量 policy eval harness：每个 case 输入 skill、args、期望 branch、execution context、allowed tools、allowed tool specs、path patterns、期望 allowed/blocked/not-allowed/not-blocked 文件，输出 pass/fail、false positive、false negative 计数。当前 guizang-style eval 覆盖风格 A、风格 B、未选分支三类请求，期望误放/误拦计数为 0；docs-maintainer 样本覆盖 `allowed-tools` / `Bash(...)` / `paths` / `context: fork`；另有 intentional mismatch 测试确认 harness 能识别误放和误拦。
- runtime 级新增 guizang continuation 工具调用回归：先记录上一轮 `magazine` skill invocation，再输入“继续把刚才那份换成瑞士风”，确认 session 自动重新展开完整 skill、active policy 切到 `swiss`、首轮可见工具被收窄为 `read/write/edit/bash`，模型若继续读旧 `assets/template.html` 会被 selected branch allowlist 拒绝，而后续读 `assets/template-swiss.html` 可以通过。另有显式点名覆盖回归：上一轮 guizang 后输入“继续用 docs-skill ...”会展开 docs-skill，不会继承 guizang branch。
- runtime 级新增 guizang A->B trace 回归：第一轮显式点名 `guizang-ppt-skill` 生成风格 A，第二轮切瑞士风时脚本化模型故意读取旧 A 模板、用 bash 复制 `SKILL.md`、再尝试读取 Swiss 模板；日志断言旧分支和 shell SKILL.md 探索都被拒绝，第二次探索违规后下一轮可见工具收窄到 `write/edit`，后续 `read` 被 `tool "read" is not allowed` 拦截，最终只能用 `write` 产出。
- `internal/skill/path_activation_eval.go` 新增 path activation eval harness：每个 case 输入 skills、workspace、用户输入，输出 `none` / `auto_invoke` / `steer` 以及 false positive / false negative 计数。当前矩阵覆盖单一 docs path 自动调用、多 deck skill 匹配时 steering、deck HTML path 自动激活 guizang、纯自然语言不触发、`PPTX` 普通词不误触发、workspace 外路径不触发。runtime hook 回归覆盖 `search` / `glob` / 大小写别名访问路径后仍能发现或 steer path-matched skills。
- 本轮新增验证：`go test ./...` 全量通过；`go build ./cmd/pi-agent` 通过；前序 guizang A/B 回归日志和 A 风格产物仍可用；policy eval harness、path activation eval harness、guizang A/B/未选分支零误放误拦矩阵、docs-maintainer allowed-tools/path/context policy eval、guizang path activation 正/负样本、guizang continuation 风格切换覆盖旧分支覆盖新分支的回归、显式点名上一轮 skill 的跨 turn 重新展开回归、无关输入不继承上一轮 skill context 回归、tool path discovery、单一 path 匹配自动激活、path 多匹配 steering、active policy 下动态 tool definition constraint 注入、frontmatter `argument-hint` / `arguments` 解析和 skill index/invocation contract 注入、frontmatter `branches` 别名/路径驱动 selected branch 和 allowlist、filtered/unknown tool policy violation、allowed tool set 被删空后仍保持 hard-stop 的回归、active skill policy snapshot 和 `GET /sessions/{id}/skill-policy` 审计接口、连续违规后 write/edit-only hard-stop 和强 recovery 文案、Bash(...) 命令级 allowlist、bash workspace path pattern enforcement、绝对 workspace path pattern 匹配、basename glob path pattern、目录 glob path pattern、通用 style-a/style-b 分支文件 allowlist、未选分支时阻止 read/bash 读取分支专属文件、fork-mode policy、fork-mode child agent handoff、fork-mode 独立 provider 实例、fork-mode 子任务状态事件、fork-mode 显式 cancel、fork-mode LLM request 历史裁剪、fork-mode 主 session 中间过程隔离、fork-mode 空白 fork session 审计、fork-mode 中间过程写入 fork session、fork-mode skill_result merge record、fork-mode artifact/changed-file/operation/change summary/file diff merge record、fork-mode diff 输入预算/截断、fork-mode 跳过主 goal evaluator/continuation、fork_session_path 审计元数据、skill invocation audit query、skill result audit query、JSONL session fork、continuation 自动重新展开 skill、continuation previous skill context fallback、多来源同名合并、统一 source registry、本地 LocalSourceProvider、HTTPSkillSourceProvider 远端 JSON index 协议/Bearer auth/错误诊断、运行时 remote skill source env 配置与默认持久化 cache、MCPSkillSourceProvider resource adapter/frontmatter 解析/读列表失败诊断/持久化缓存降级、MCP stdio JSON-RPC transport、MCP Streamable HTTP/SSE response transport、运行时 MCP skill source env 配置、CachedSourceProvider 远端失败缓存降级、JSONFileSourceCache 持久化恢复和 cache store 失败诊断、默认多来源 skill 发现、用户级 skill root 只读访问、skill listing 预算裁剪、按 context window 动态调整、系统提示已占用空间和工具 schema 估算扣除、可注册 token counter backend registry 都有单元测试覆盖。
- Anthropic 路径本次未能验证：当前环境的 `ANTHROPIC_BASE_URL` 指向 `127.0.0.1:4319`，服务未启动。
- OpenAI 官方路径本次未能验证：当前网络访问 `api.openai.com` 超时。

结论：

这次修复已经从“工具层粗拒绝”推进到真正的 skill execution policy：skill 激活期间按 `allowed-tools` / selected branch / `paths` 动态缩小工具和路径权限，并对重复违规做 steering 与 hard-stop。与 Claude Code 的差距主要剩更大规模真实任务 eval、MCP OAuth/授权集成、官方 tokenizer backend 实现和更稳定的模型侧遵循。

## 5. 与 Claude Code 的对齐程度

已对齐：

- skill body 只在调用时注入，列表里只放摘要和触发信息。
- 注入时携带 skill base directory。
- slash skill 直接展开为完整 prompt。
- frontmatter 开始承载 `when_to_use` / `argument-hint` / `arguments` / `allowed-tools` / `paths` / `context` / `branches`。
- 对重型 skill 明确约束 progressive disclosure：按分支读文件，不扫目录。
- skill 激活期有 runtime tool policy：动态缩小工具列表，动态改写路径型 tool definition 说明当前约束，拒绝越权 tool call，对 `Bash(...)` 做命令级约束，并把 `paths` 约束覆盖到 read/write/edit/search 和 bash 文件路径参数。
- 重复违规会注入 recovery，并逐步移除 `bash`、再进入 write/edit-only；如果 allowed tool set 被删空，也保持 hard-stop，不会退回 unrestricted。
- invoked skill 会写入 session state，并可通过 `SkillInvocations(ctx)` / `GET /sessions/{id}/skills` 查询当前 path 的完整 skill 调用轨迹。
- 当前 active skill policy 可通过 `ActiveToolPolicySnapshot()` / `GET /sessions/{id}/skill-policy` 查询，能看到动态收窄后的工具、violation 计数、最后一次 violation 原因和 hard-stop 状态。
- compact 会保留 active skill context。
- continuation 输入或显式点名上一轮 skill name 会优先自动重新展开上一轮 skill 并重新激活 policy；普通无关输入不继承上一轮 policy，找不到 skill 时才恢复 previous skill context。
- `paths` 已参与输入级自动激活、多匹配 steering、工具访问后的动态 discovery。
- `context: fork` 已进入 runtime policy、session state、recovery、compact note，并隔离 LLM request 历史、主 session 中的中间过程和主 goal-driven continuation；skill fork session 从空白上下文开始，只保留该 skill 的过程，主 session 可通过 `fork_session_path` 定位。
- skill 多来源合并、默认多来源发现和统一 source registry 已具备基础实现：project > user > managed > plugin > bundled > system > unknown。
- 用户级/global skill 不再被 workspace-only 过滤，且其 skill root 可被 `read` 只读访问。
- skill listing 已有预算裁剪、字段截断、按 context window 的动态预算，并会扣除已构造系统提示和工具 schema 的近似占用，保留“索引 + 按需加载”语义。

仍有差距：

- `context: fork` 已有 child agent loop、空白 fork session、独立 provider 实例、子任务状态事件、显式 fork child cancel、`skill_result` merge record、artifact/changed-file/operation/change summary/file diff merge、diff 输入预算/截断和最终结果回写；仍需扩充更大真实任务验证。
- `paths` 已接入 active policy、单一匹配自动激活、多匹配 steering、工具路径 discovery；policy 层和 path activation 层已有可复用 eval harness，且覆盖了 guizang/docs-maintainer 的正负样本；仍需继续扩充更大规模误触发率评估。
- skill 来源合并已有统一 source registry，本地目录 provider、运行时可配置远端 HTTP JSON provider、MCP resource adapter、stdio JSON-RPC transport、Streamable HTTP/SSE response transport 和内存/持久化缓存降级 wrapper 已具备；仍需实现 MCP OAuth/授权策略。
- skill listing 预算已按模型 context window 动态调整，并扣除当前系统提示和工具 schema 的统一 model-aware token counter 估算；token counter 已有可注册 backend registry，仍需接入具体 provider 官方 tokenizer 实现。
- invoked skill 已记录到 session，并支持 continuation 自动重激活/fallback context；fork skill 已记录 fork session 路径，session 层和 HTTP API 已提供查询接口，但还没有做成桌面端可视化面板。
- active policy 仍是 current turn runtime state；跨 turn 只有明确 continuation 或显式点名上一轮 skill name 会重新 hard-activate policy，普通新任务不会继承上一轮 policy。
- 重复违规已有 hard-stop；skill 探索类违规会更快进入 write/edit-only，但阈值仍需更多真实 skill eval 调优。

## 6. 后续建议

P0：

1. 扩展 skill policy eval：加入正常 completion、风格切换、错误路径恢复、更多真实 skill 误伤率。
2. 把当前 fork 隔离升级成真正 sub-agent/session fork，并定义最终结果合并协议。
3. 扩展 path 单一匹配自动激活 eval 样本，纳入更多真实任务和误触发率统计。

P1：

1. 扩充真实 skill eval 样本，覆盖更多重型技能和长会话 continuation。
2. 为 tokenizer registry 接入具体 provider 官方 tokenizer backend，并补 tokenizer 精度回归样本。
3. 补 MCP OAuth/授权策略，并继续扩展远端 MCP 兼容性样本。

验收重点：

- `guizang-ppt-skill` 风格 A 只读 A 分支模板、主题、layout。
- 风格 B 只读 B 分支模板、主题、layout lock、layout swiss。
- 没有用户要求时，不扫 `.claude/skills/guizang-ppt-skill`。
- 缺少风格/页数/主题信息时先问关键问题，而不是搜索目录。
