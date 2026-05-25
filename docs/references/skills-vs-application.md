# 什么时候用 Skills，什么时候拆成独立 Application

> 性质：决策参考文档
> 背景：pi-go 已有 `runtime.Application` 接口（`BuildTools` + `BuildPrompt`），也有 `skill` 系统。当需要扩展新的 agent 角色时，走哪条路？本文给出判断框架。

---

## 1. 先搞清楚两条路分别是什么

### 垂直扩展：Skills / Modes

在同一个 `Application`（当前就是 `CodingApplication`）内，通过配置差异实现不同角色：

| 机制 | 做什么 | 举例 |
|------|--------|------|
| **Skill 注入** | 注入领域知识、行为规范、专属 prompt 片段 | `docs-maintainer` skill 定义文档维护流程 |
| **Prompt profile** | 替换或叠加系统提示 | review 模式的 system prompt 强调"只分析不动手" |
| **Tool filtering** | 用 `AllowedTools` / `BlockedTools` 控制可用工具 | 知识库管理员禁用 write/edit/bash |
| **Mode 切换** | 在同一 Application 内切换角色 profile | review 模式 vs 默认编程模式（不同 prompt + 不同工具白名单） |

**本质**：共用同一个执行循环、同一个 Application 实现，差异全部在配置层。

### 水平扩展：独立 Application

实现一个新的 `runtime.Application`，有自己的 `BuildTools()` 和 `BuildPrompt()`：

```go
type BrowserApplication struct{}

func (BrowserApplication) BuildTools(opts ToolBuildOptions) []agent.Tool { ... }
func (BrowserApplication) BuildPrompt(opts PromptBuildOptions) string { ... }
```

**本质**：不同的 Application 实现，但共用 Platform 层（循环、session、compaction 等）。

---

## 2. 垂直扩展的隐藏成本

垂直扩展不是免费的。当 skills/modes 堆积时，会产生膨胀问题：

| 成本 | 表现 |
|------|------|
| **Prompt 膨胀** | system prompt 越来越长，LLM 注意力被稀释，每个领域都做不精 |
| **工具冲突** | 同名工具在不同领域的语义不同（`read` 是读文件还是读网页？） |
| **上下文污染** | 不同领域的中间结果混在一起（DOM 片段和代码上下文） |
| **身份模糊** | "你是编程助手……也可以操控浏览器……也可以查数据库" → LLM 不知道自己该优先做什么 |

**判断标准**：如果 CodingApplication 的 `BuildTools()` 变成了一个大型 switch/case 路由器，说明该拆了。

---

## 3. 判断框架

不是简单的"循环变了就拆，没变就不拆"。实际决策有三层判断：

### 第一层：执行循环变了吗？

pi-go 当前的执行循环是：

```
用户发起 → LLM 推理 → 调工具 → 观察结果 → 回复用户 → 等待下一轮
```

| 如果答案是 | 走这条路 |
|-----------|---------|
| 是，只是角色/工具/策略不同 | **进入第二层** |
| 不是，循环本身有本质差异 | **不能用 skills 解决，但需要的可能不只是新 Application——详见下方** |

什么算"循环变了"：

| 执行模型差异 | 举例 | 为什么 skills 解决不了 |
|-------------|------|---------------------|
| **谁发起？** 从用户发起变成事件触发 | 飞书 Bot 收到 webhook 后自主启动 | Skill 系统没有"被动触发"的入口 |
| **同时服务几个用户？** 从单会话变成多租户并发 | 一个 Bot 实例同时处理 50 个群消息 | 当前架构偏显式 session 路由，还没长成事件驱动的多租户运行时 |
| **是一个循环还是多个循环协作？** 从单个 LLM 循环变成 DAG 管线 | 4-agent 流水线（调研→计划→审核→执行） | 当前循环里没有"调用另一个 agent"的能力 |
| **谁做决策？** 从人审批变成完全自治 | 后台 daemon 自主判断并执行 | 当前循环假设人在回路中 |

**重要澄清**：`runtime.Application` 只负责 `BuildTools()` 和 `BuildPrompt()`，它**不承载执行循环本身**。循环变了意味着你需要的可能不只是新 Application，还需要新的 entrypoint（webhook listener / daemon runner）、新的调度逻辑（DAG 编排器）、甚至新的 runtime orchestration。新 Application 只是其中一环。

所以更准确的说法是：**循环变了 = 至少不能只靠 skills 解决的强信号。具体需要改哪些层，要看具体场景。**

### 第二层：默认行为契约冲突严重吗？

循环没变，但两个角色的默认行为差异可能大到不适合共用同一个 Application：

| 维度 | 冲突举例 |
|------|---------|
| **默认 workspace** | 本地文件系统 vs 互联网/Web 页面 |
| **核心工具集** | 完全不同的工具集合，没有多少重叠 |
| **默认行为策略** | 写代码改文件 vs 操作浏览器点页面 |
| **领域知识** | 代码结构/项目约定 vs 网页结构/反爬策略 |

典型场景对比：

| 角色 | prompt | tools | 默认行为 | 冲突程度 |
|------|--------|-------|---------|---------|
| Code Review 模式 | 换一套提示 | 子集（去掉 write/edit） | 只读分析 | **低** — 不改 coding 的基本假设 |
| 文档整理模式 | 注入文档知识 | 子集 | 只整理文档 | **低** — 还是在操作文件 |
| 浏览器 Agent | 完全不同的提示 | 完全不同的工具集 | 操作网页而非文件 | **高** — workspace、工具、行为策略全变了 |
| 数据分析 Agent | 完全不同的提示 | SQL/query/chart 工具 | 查数据库做图表 | **高** — 和 coding 基本无关 |

**判断标准**：如果你发现自己在 `BuildTools()` 里写 `if mode == "browser" { return completelyDifferentTools }`，而且 prompt 也在完全切换而不是叠加，那就是拆的信号。

冲突严重 → **进入第三层**。
冲突不严重 → **垂直扩展（Skills / Modes）**。

### 第三层：这个能力是一等公民还是附属功能？

即使默认行为契约冲突严重，也不一定要立刻拆。取决于产品定位：

| 如果这个能力是 | 走这条路 | 举例 |
|--------------|---------|------|
| **附属功能**：偶尔用用，不需要独立入口 | skill/extension 挂载 | coding agent 偶尔查个网页 |
| **一等公民**：独立的用户选择，和 coding 是平级的产品形态 | 独立 Application（共用 Platform） | 浏览器模式是一个独立命令，有自己的 prompt 和工具 |

**为什么"一等公民"值得拆？**

对比两种实现：

```go
// 方案 A：mode 切换（塞在 CodingApplication 里）
func (CodingApplication) BuildTools(opts) []agent.Tool {
    switch opts.Mode {
    case "coding":
        return codingTools
    case "browser":
        return browserTools     // browser 知识泄漏进 coding
    case "data":
        return dataTools        // data 也来了
    case "future":
        return futureTools      // 无限膨胀...
    }
}

// 方案 B：独立 Application（各管各的）
func (CodingApplication) BuildTools(opts) []agent.Tool {
    return codingTools          // 永远干净
}

func (BrowserApplication) BuildTools(opts) []agent.Tool {
    return browserTools         // 独立演化
}
```

方案 B 的好处：
- **CodingApplication 永远干净**，不会被非 coding 领域的工具和逻辑污染
- **新增领域不改已有 Application**，新 Application 独立开发
- **产品语义清晰**，"编程模式"和"浏览器模式"是明确的选择，不是一个大杂烩里的 toggle
- **成本不高**，因为共用 Platform 层（循环、session、compaction），只是多了一个 `BuildTools` + `BuildPrompt` 的实现

---

## 4. 完整决策流程图

```
循环变了吗？（事件驱动 / 多租户 / DAG / 自治）
  │
  ├─ 是 → 至少不能只靠 skills，可能需要新 Application + 新 entrypoint + 新调度
  │        具体改哪些层看场景（飞书 Bot = 新 Application + webhook entrypoint）
  │
  └─ 否 → 默认行为契约冲突严重吗？
            │
            ├─ 不严重（工具子集 / prompt 变化 / 角色切换）
            │    → 垂直扩展：Skills / Modes / Tool Filtering
            │    例：review 模式、文档整理、测试专家
            │
            └─ 严重（完全不同的工具集 / workspace / 行为策略）
                 │
                 ├─ 这是附属功能吗？
                 │    ├─ 是 → skill / extension 挂载
                 │    │    例：coding agent 偶尔查个网页
                 │    │
                 │    └─ 否（一等公民产品形态）
                 │         → 独立 Application（共用 Platform）
                 │         例：浏览器 Agent、数据分析 Agent
                 │
                 └─ 拿不准？
                      → 先用 skill/mode 做原型
                      → 如果发现 CodingApplication 开始膨胀，再拆
```

---

## 5. 对 pi-go 的具体建议

### 短期（现在 ~ 近 3 个月）

**主线继续做一个强的 coding-agent。**

遇到新角色需求时，优先走垂直扩展：

| 想做的角色 | 怎么做 |
|-----------|--------|
| Code Review 模式 | skill 注入 review 规范 + `BlockedTools: [write, edit, bash]` |
| 文档整理模式 | skill 注入文档结构知识 + tool filtering |
| 测试专家模式 | skill 注入测试策略 + prompt profile |
| 知识库问答模式 | skill 注入项目上下文 + 只开放 read/grep/find/ls |

这些角色共享 coding 的默认行为（本地文件系统、代码上下文），用 skills 就够了。

### 工具归属原则

不管走哪条路，工具本身不应该膨胀进 `agents/coding/tools/`：

```
internal/
  agents/coding/tools/     ← coding 原生工具：read, write, edit, bash, grep, find, ls
  tools/browser/           ← 通用浏览器工具（不属于任何 agent，谁需要谁挂）
  tools/database/          ← 通用数据库工具
  tools/...                ← 其他通用工具
```

通用工具放在 `internal/tools/` 下，Application 通过 `BuildTools()` 决定挂哪些。这样即使不做新 Application，CodingApplication 也不会因为挂载浏览器工具而变脏。

### Application 边界现在仍然有价值

哪怕短期只有一个 `CodingApplication`，`runtime.Application` 接口的存在仍然有意义：

**它是防止 Platform 层无意识吸收 coding 语义的隔离带。**

没有这条边界时：
- `runtime` 会慢慢长出 `BashEnabled` 之类的字段（coding 特有，不是所有 agent 都需要）
- `AgentSession` 会假设"一定有 write 工具"
- 新功能会不自觉地从 coding 视角设计

有这条边界时：
- 每次往 `runtime` 加东西，都要问：这是通用的还是 coding 特有的？
- coding 特有的逻辑被推到 `agents/coding/` 里
- Platform 层保持领域无关

**所以保留接口，但不急着经营第二个实现。**

### 中期触发条件：什么时候该做第二个 Application

| 触发场景 | 循环变了？ | 契约冲突？ | 一等公民？ | 结论 |
|---------|-----------|-----------|-----------|------|
| **飞书 Bot Agent** | 是（事件驱动 + 多租户） | 是 | 是 | 新 Application + 新 entrypoint（webhook listener）+ 多租户调度 |
| **CI/CD Review Daemon** | 是（事件驱动 + 自治） | 是 | 是 | 新 Application + 新 entrypoint（daemon runner） |
| **浏览器 Agent（一等公民）** | 否 | 是（完全不同的工具/workspace/行为） | 是 | 拆 Application（共用 Platform） |
| **浏览器（附属功能）** | 否 | 中等 | 否 | skill/extension 挂载 |
| **数据分析 Agent** | 否 | 是 | 取决于产品定位 | 一等公民则拆 Application |
| **Multi-Agent 编排器** | 是（DAG） | 是 | 是 | 新 Application + DAG 调度器（不只是新 Application） |

---

## 6. 决策检查清单

当你想做一个新 agent 角色时，按顺序问自己：

```
1. 它的执行循环变了吗？（事件驱动 / 多租户 / DAG / 自治）
   └─ 是 → 至少不能只靠 skills，需要评估改哪些层（Application / entrypoint / 调度器）

2. 它的默认行为和 coding-agent 共享基本假设吗？
   └─ 共享（都是操作本地文件、写代码）→ 用 Skills / Modes
   └─ 不共享（完全不同的 workspace / 工具 / 行为策略）→ ↓

3. 这个能力是一等公民还是附属功能？
   └─ 附属（偶尔用用）→ skill / extension 挂载
   └─ 一等公民（独立产品形态）→ 独立 Application

4. 拿不准？
   └─ 先用 skill/mode 做原型
   └─ 如果 CodingApplication 开始膨胀 → 拆
```

---

## 7. 一句话总结

**Skills 改变"agent 是谁"，Application 改变"agent 怎么跑"。**

但如果"agent 是谁"已经变得面目全非（完全不同的工具集、workspace、行为策略），而它又是一等公民产品，那也应该拆 Application——不是为了循环不同，而是为了保持干净。

**循环不同 → 一定不能只靠 skills，需要评估架构层面的变化（可能是新 Application + 新 entrypoint + 新调度）。循环相同但身份完全不同且是一等公民 → 拆 Application（成本低，共用 Platform）。循环相同、身份微调 → Skills。**
