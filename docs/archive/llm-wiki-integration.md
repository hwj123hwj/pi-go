# LLM Wiki 集成设计

> 将 pi-go 的 llm-wiki 能力从"手动维护的个人知识库"升级为"内置的 slash 命令系统"，使 pi-go 具备给任意项目生成、查询、维护 wiki 的能力。

## 1. 背景与目标

### 1.1 现状

pi-go 的 `.llm-wiki/` 目录已经存在，包含：
- 5 个 source 页面（`source-project-root-v1~v5`）
- 30+ 个 entity 页面（`agent-core.md`, `desktop-app.md` 等）
- `index.md`（导航页）和 `log.md`（操作日志）

但这些页面**完全靠手动 ingest 生成**，没有命令驱动、没有自动感知、没有查询工具。

### 1.2 竞品参考：DeepVcodeClient

DeepVcodeClient 的 llm-wiki 实现（`packages/cli/src/ui/commands/wikiCommand.ts`）：

| 命令 | 功能 |
|------|------|
| `/wiki init` | 初始化 `.llm-wiki/` 目录结构 |
| `/wiki ingest <path>` | 将源文件 ingest 为 wiki 页面 |
| `/wiki query <q>` | 用自然语言查询 wiki |
| `/wiki lint` | 健康检查（死链、孤立页、过期内容） |
| `/wiki status` | 统计页面数量、最近日志 |
| `/wiki log` | 显示操作历史 |

**核心机制**：全部靠 prompt 驱动 AI 手动读写文件，没有专用工具。系统提示自动检测 `.llm-wiki/index.md` 是否存在并注入上下文。

### 1.3 目标

在 pi-go 中实现 `/wiki` slash 命令系统，让 pi-go 具备**给任意项目生成 wiki 的能力**——这是 pi-go 的内置基础能力，不是某个 agent 的附属功能。

## 2. 架构设计

### 2.1 分层

```
┌─────────────────────────────────────────────┐
│  Slash Command Layer (pi-go built-in)        │
│  /wiki init | ingest | query | lint | status  │
├─────────────────────────────────────────────┤
│  Prompt Templates (Go string constants)      │
│  WikiInitPrompt, WikiIngestPrompt, etc.      │
├─────────────────────────────────────────────┤
│  Agent Loop (existing)                       │
│  AI reads/writes .llm-wiki/ files via tools  │
├─────────────────────────────────────────────┤
│  Tool Layer (existing + new)                 │
│  read/write/edit/find/ls (existing)          │
│  wiki_search (NEW — optional query tool)     │
└─────────────────────────────────────────────┘
```

### 2.2 设计原则

1. **Prompt 驱动，工具赋能** — init/ingest/lint 靠 prompt 驱动 AI（需要创造力），query 额外提供 `wiki_search` 工具（关键词精确匹配 + AI 语义理解）
2. **复用现有工具** — AI 通过已有的 `read`/`write`/`edit`/`find`/`ls` 工具操作 `.llm-wiki/` 文件，不新建 wiki 专用文件工具
3. **跨 agent 通用** — `/wiki` 命令注册在 slash command 框架层，coding agent 和未来的其他 agent 都能用
4. **系统提示自动注入** — 检测到 `.llm-wiki/index.md` 存在时，自动在系统提示中注入 wiki 上下文段落

## 3. 目录结构

```
.llm-wiki/
├── index.md          # 导航页（entity/source/concept 分类 + 一行摘要）
├── log.md            # 操作日志（append-only）
├── raw/              # 不可变的原始源文件（用户自行放入）
│   └── (用户放入的源文件)
└── wiki/             # AI 维护的知识页面
    ├── source-*.md   # 源摘要页面（type: source）
    ├── *.md          # 实体/概念页面（type: entity / type: concept）
    └── overview.md   # 项目概览（自动生成）
```

与现有结构完全兼容——已有的 `wiki/` 页面和 `index.md` 无需迁移。

## 4. 命令设计

### 4.1 `/wiki init`

**功能**：初始化 `.llm-wiki/` 目录结构。

**流程**：
1. 检查 `.llm-wiki/index.md` 是否已存在 → 已存在则提示
2. 发送 `WikiInitPrompt` 给 AI
3. AI 创建目录结构 + `index.md` + `log.md` + `overview.md`
4. AI 自动从项目根目录（README、go.mod 等）推断项目信息生成 overview

**Prompt 要点**：
- 创建 `raw/`、`wiki/` 目录
- 生成 `index.md`（含 Sources / Entities / Concepts / Synthesis 四个空 section）
- 生成 `log.md`（含初始条目）
- 生成 `overview.md`（从 README、go.mod、主入口等推断项目信息）
- 使用 YAML frontmatter（`type`, `date`, `tags`）
- 使用 `[[wikilinks]]` 做交叉引用

### 4.2 `/wiki ingest [path]`

**功能**：将源文件 ingest 为 wiki 知识页面。

**两种模式**：
- **指定路径**：`/wiki ingest path/to/file.go` — ingest 单个文件
- **全量 ingest**：`/wiki ingest`（无参数）— 扫描 `raw/` 下所有未 ingest 的文件

**Prompt 要点**（参考 DeepVcodeClient 的 `getWikiIngestPrompt`）：
1. 读取源文件
2. 识别关键实体、概念、事实、关系
3. 创建 `source-<name>.md` 摘要页面（含 frontmatter）
4. 更新或创建 entity/concept 页面
5. 更新 `index.md`
6. 追加 `log.md`
7. 标记与现有页面的矛盾

**增量检测**：AI 先读 `index.md`，跳过已有对应 source 摘要的文件。

### 4.3 `/wiki query <question>`

**功能**：用自然语言查询 wiki。

**两种实现策略**：

**策略 A：纯 prompt（Phase 1 推荐）**
- 发送 `WikiQueryPrompt` + 用户问题
- AI 读 `index.md` → 选择相关页面 → 读取 → 综合回答
- 优点：简单、不需要新工具
- 缺点：大 wiki 时可能遗漏

**策略 B：工具 + prompt（Phase 2）**
- 新增 `wiki_search` 工具：在 `wiki/` 目录下做关键词搜索（grep）
- AI 先用 `wiki_search` 定位相关页面，再读取
- 优点：更精确，大 wiki 时更可靠
- 缺点：需要实现新工具

**Prompt 要点**：
1. 先读 `index.md` 定位相关页面
2. 读取相关 wiki 页面
3. 综合回答，引用具体页面
4. 如信息不足，建议 ingest 相关源文件
5. 如发现矛盾，高亮标记

### 4.4 `/wiki lint`

**功能**：wiki 健康检查。

**检查项**：
- **孤立页面**：`wiki/` 中存在但 `index.md` 未列出
- **死链**：`[[wikilinks]]` 指向不存在的页面
- **缺失页面**：频繁提及但缺少独立页面的实体
- **过期内容**：frontmatter 日期过旧
- **矛盾**：不同页面的声明冲突
- **缺失交叉引用**：应互引但未互引的页面
- **不完整 frontmatter** 缺少 `type`/`date`/`tags`

**Prompt 要点**：
1. 读取 `index.md` 获取页面目录
2. 扫描所有 `wiki/` 页面
3. 逐项检查
4. 报告发现（含文件路径和行号）
5. 修复简单问题（更新索引、添加交叉引用、修复 frontmatter）
6. 大问题先询问用户
7. 追加 `log.md`

### 4.5 `/wiki status`

**功能**：显示 wiki 统计信息。

**纯 Go 实现**（不需要 prompt），直接：
1. 统计 `wiki/` 下的 .md 文件数量（区分 source / entity / concept）
2. 统计 `raw/` 下未 ingest 的文件数
3. 读取 `log.md` 最近 5 条记录
4. 格式化输出

### 4.6 `/wiki log`

**功能**：显示操作历史。

**纯 Go 实现**，直接读取并输出 `log.md`。

## 5. 系统提示自动注入

当 `.llm-wiki/index.md` 存在时，在系统提示末尾追加 wiki 上下文段落：

```markdown
# LLM Wiki

This project has a curated LLM Wiki knowledge base at `.llm-wiki/`. It contains
distilled, AI-maintained knowledge about this codebase — architecture,
key modules, conventions, and gotchas.

## Consult it proactively
- Before exploring the codebase or answering questions about how something works,
  consult `.llm-wiki/index.md` first to see if a relevant page already exists.
- Prefer reading the matching `.llm-wiki/wiki/*.md` page over re-deriving the same
  knowledge by broad code search. Use it to orient yourself, then dig into source.
- Treat the wiki as a strong hint, not absolute truth: if a page looks stale or
  conflicts with the current code, trust the code and consider updating the wiki.

## Maintain it when asked
When the user asks you to "save to wiki", "learn into wiki", "update wiki", or similar:
1. Read `.llm-wiki/index.md` to understand the current structure.
2. Create or update pages in `.llm-wiki/wiki/` with YAML frontmatter (`type`, `date`, `tags`).
3. Use `[[wikilinks]]` for cross-references between pages.
4. Update `.llm-wiki/index.md` to reflect new/changed pages.
5. Append an entry to `.llm-wiki/log.md`.
6. Never modify files in `.llm-wiki/raw/` — those are immutable sources.

The user can also use `/wiki` slash commands for structured operations.
```

**注入位置**：在 `BuildSystemPrompt` 中，追加在 `AppendSystemPrompt` 之后、Goal 注入之前。

**注入条件**：检测 `$CWD/.llm-wiki/index.md` 是否存在。

## 6. 实现计划

### Phase 1：核心命令（推荐优先实现）

| 步骤 | 任务 | 涉及文件 |
|------|------|---------|
| 1 | 创建 prompt 模板文件 | `internal/agents/coding/commands/wiki_prompts.go`（新建） |
| 2 | 创建 wiki 命令注册 | `internal/agents/coding/commands/wiki.go`（新建） |
| 3 | 注册到 slash command 框架 | `internal/agents/coding/coding.go` — 调用 `RegisterWikiCommands(registry)` |
| 4 | 系统提示自动注入 | `internal/agents/coding/prompt/builder.go` — 检测 `.llm-wiki/index.md` |
| 5 | 更新 `/help` 输出 | `builtins.go` — wiki 命令分组 |

### Phase 2：增强

| 任务 | 说明 |
|------|------|
| `wiki_search` 工具 | 在 `wiki/` 目录下做关键词搜索，辅助 query |
| 自动 ingest 触发 | 当 AI 发现自己在反复探索同一模块时，主动建议 ingest |
| Desktop 集成 | 在桌面端添加 wiki 面板（浏览、搜索 wiki 页面） |

### Phase 3：跨 agent

| 任务 | 说明 |
|------|------|
| 提取到共享层 | 将 wiki 命令从 coding agent 提取到所有 agent 共享 |
| KB agent 联动 | KB agent 的 `kb_search` 同时搜索 `.llm-wiki/` |
| 远程项目 wiki | 支持为远程项目（SSH）生成 wiki |

## 7. 文件变更清单

### 新建文件

```
internal/agents/coding/commands/wiki.go          # /wiki 命令注册 + 子命令分发
internal/agents/coding/commands/wiki_prompts.go  # Prompt 模板常量
```

### 修改文件

```
internal/agents/coding/coding.go                  # 注册 wiki 命令
internal/agents/coding/commands/builtins.go      # /help 输出添加 wiki 分组
internal/agents/coding/prompt/builder.go         # 系统提示自动注入 wiki 上下文
```

### 不改动的文件

- `internal/slashcmd/registry.go` — 框架不变
- `internal/slashcmd/context.go` — 上下文不变（wiki 命令通过 CWD 推断路径）
- 现有 wiki 页面 — 完全兼容

## 8. Prompt 模板设计

### 8.1 WikiInitPrompt

```
You are a knowledge base maintainer following the LLM Wiki pattern.

**Task: Initialize the wiki directory structure.**

1. Create `.llm-wiki/raw/` directory (for immutable source documents)
2. Create `.llm-wiki/wiki/` directory (for AI-maintained knowledge pages)
3. Create `.llm-wiki/index.md` with sections: Sources, Entities, Concepts, Synthesis
4. Create `.llm-wiki/log.md` with initial entry
5. Create `.llm-wiki/wiki/overview.md` — infer project info from README, go.mod, main entry points
   - Include YAML frontmatter: type, date, tags
   - Brief architecture overview, tech stack, key directories

Rules:
- Use YAML frontmatter on all pages
- Use [[wikilinks]] for cross-references
- Never modify files in raw/ — those are immutable
```

### 8.2 WikiIngestPrompt

```
You are a knowledge base maintainer. The wiki lives in `.llm-wiki/`.

**Task: Ingest source document into the wiki.**

Source: {path}

Workflow:
1. Read the source file completely
2. Read `.llm-wiki/index.md` to understand existing pages
3. Identify key entities, concepts, facts, relationships
4. Create source summary: `.llm-wiki/wiki/source-{name}.md`
   - YAML frontmatter: type: source, source_path, date, tags, related
   - Key takeaways, important entities, notable code references
   - Contradictions with existing wiki
5. For each significant entity/concept:
   - Update existing page if exists (note the source)
   - Create new page if not exists (type: entity or type, with [[wikilinks]])
6. Update index.md with new entries
7. Append to log.md: ## [YYYY-MM-DD] ingest | {source name}

Rules:
- Always use YAML frontmatter (type, date, tags)
- Use [[wikilinks]] for cross-references
- Never modify raw/ files
- Flag contradictions explicitly
```

### 8.3 WikiQueryPrompt

```
You are a knowledge base assistant. The wiki lives in `.llm-wiki/`.

**Task: Answer a question using the wiki.**

Question: {question}

Workflow:
1. Read `.llm-wiki/index.md` to find relevant pages
2. Read the most relevant wiki pages
3. Synthesize an answer with citations to specific wiki pages
4. If wiki doesn't have enough info, say so and suggest which raw sources to ingest

Rules:
- ONLY read from `.llm-wiki/wiki/`, NEVER from `.llm-wiki/raw/`
- Always cite which wiki pages informed your answer
- If info is missing, suggest: "Run `/wiki ingest .llm-wiki/raw/xxx.md` to add this knowledge"
- Highlight contradictions between wiki pages
```

### 8.4 WikiLintPrompt

```
You are a knowledge base health checker. The wiki lives in `.llm-wiki/`.

**Task: Perform a health check on the wiki.**

Workflow:
1. Read index.md for the full page catalog
2. Scan all wiki/ pages
3. Check for:
   - Orphan pages (in wiki/ but not in index.md)
   - Dead links (wikilinks pointing to non-existent pages)
   - Stale content (check dates in frontmatter)
   - Contradictions across pages
   - Missing cross-references
   - Incomplete frontmatter
4. Report findings with file paths and line numbers
5. Fix straightforward issues (update index, add cross-references, fix frontmatter)
6. Ask before larger changes
7. Append to log.md: ## [YYYY-MM-DD] lint | Health Check
```

## 9. 与现有系统的兼容性

| 方面 | 兼容性 |
|------|--------|
| 现有 wiki 页面 | ✅ 完全兼容，无需迁移 |
| 现有 slash 命令 | ✅ 只新增，不改旧命令 |
| SessionContext 接口 | ✅ 不需要修改，wiki 命令通过 CWD 推断路径 |
| 系统提示构建 | ✅ 追加注入，不影响现有 prompt 结构 |
| Desktop 端 | ✅ 无影响，Desktop 不跑 slash 命令 |
| 飞书桥接 | ✅ 无影响，飞书不跑 slash 命令 |
| KB agent | ✅ 无影响，后续 Phase 3 再联动 |

## 10. 测试策略

| 测试类型 | 覆盖内容 |
|----------|---------|
| 单元测试 | prompt 模板内容验证（非空、包含关键指令） |
| 集成测试 | `/wiki init` → 检查目录结构创建 |
| 集成测试 | `/wiki ingest` → 检查 source 页面 + entity 页面 + index 更新 |
| 集成测试 | `/wiki query` → 检查 AI 只读 wiki/ 不读 raw/ |
| 集成测试 | `/wiki lint` → 检查死链检测、孤立页面检测 |
| 集成测试 | `/wiki status` → 统计数字正确 |
| 集成测试 | 系统提示注入 → 有 .llm-wiki/ 时注入，无时不注入 |
