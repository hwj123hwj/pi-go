# pi-go 演进：从编程助手到通用个人助手

> 性质：方向性决策文档（roadmap）
> 背景：音乐 agent 已作为第一个"非编程"个人 agent 落地，验证了 `runtime.Application` 的可插拔性。用户明确方向：pi-go 要做成**通用个人助手**，后面还会集成多个个人 agent（记账、健康、日记……），核心诉求之一是"收集个人习惯"（以音乐偏好为首个场景）。本文定义这条演进路线，回答"记忆层怎么补、多 agent 怎么收口、桌面端要不要改"。
> 关联：[skills-vs-application.md](./skills-vs-application.md)（Application 作为抽象已拍板，本文延伸到"记忆"与"agent 自描述"两个新维度）

---

## 1. 现状定位：架构够不够支撑"多个个人 agent"？

先给结论：**在"加一个新 agent"这件事上，架构是 ready 的；在"让 agent 记住用户"和"让 UI 适配异构 agent"这两件事上，架构还缺两个维度。**

### 已经做对的（不动）

Application 可插拔已验证。加一个新个人 agent，链路是清晰的：

```
cmd/pi-agent/main.go
  └─ app.New(Applications: map{
        "coding": CodingApplication,
        "music":  MusicApplication,   // ← 第一个个人 agent，已落地
        "future": XxxApplication,     // ← 加新 agent，只动这里
     })
```

- `app.go` 的 `Applications map[string]runtime.Application` + `ResolveApplication(name)` 是正确扩展点
- `server.go` `createSession` 接收 `application` 字段，`SessionDepsWithApp(req.Application)` 注入——前端选哪个 agent 就建哪个会话
- 音频代理用 `ServeMode.SetExtraRoutes(extraMux)` 挂额外路由，没污染核心路由表
- 每个 Application 独立演化，互不污染（见 skills-vs-application 的"方案 B"论证）

**这意味着用户"后面会集成很多个人 agent"的底气是有的。** 这是本文往下讨论的前提，不重新论证。

### 缺的三个 gap

| Gap | 现状 | 影响 |
|-----|------|------|
| **没有记忆层** | `MusicSessionExt` 只有 `goal`/`rebuild`；`sessionmgr` 只管单 session JSONL；`runtime.Application` 无 Memory 抽象 | agent 永远是"一次性工具"，不记得用户昨天做了什么。**这是从工具到助手的分水岭** |
| **前端 agent 列表写死** | `Sidebar.tsx` 硬编码 "Coding"/"Music"，但后端已有 `GET /applications` 没用上 | 每加一个 agent 要改前端，不 scalable |
| **面板为 coding 硬编码** | `PaneKind` 只有 chat/diff/plan/tasks/terminal/file；音乐的播放器/歌词是 hack 进 `ChatPane` 的 | 记账要表格、健康要图表、日记要日历，全 hack 进 ChatPane 不可持续 |

三个 gap 里，**记忆层是核心、价值最高、且会被后续所有个人 agent 复用**。其余两个是支撑性改动。本文重点设计记忆层。

---

## 2. 记忆层：为什么参考 OpenViking，而不是自己造

用户点名参考 OpenViking（`volcengine/OpenViking`）。调研结论：**采纳它的数据模型与分层思想，pi-go 侧只定义薄抽象接口，记忆后端用 OpenViking，不自己造向量库/检索引擎。**

### OpenViking 核心设计（为什么值得借鉴）

OpenViking 是面向 AI Agent 的"上下文数据库"，核心创新是把 memory/resource/skill 用**文件系统范式**统一组织，而不是传统 RAG 的扁平向量库。四个关键点：

1. **三种上下文类型**，区分生命周期与写入方：

   | 类型 | 用途 | 谁写入 | 对 pi-go 的映射 |
   |------|------|--------|----------------|
   | **Resource** | 用户加的外部知识（文档、手册） | 用户主动加 | 长期不变的领域知识 |
   | **Memory** | agent 对用户/世界的认知 | agent 自动抽取 | **"个人习惯收集"落在这** |
   | **Skill** | 可调用能力 | agent 调用 | 已有 tool/skill 体系 |

2. **Memory 分 user/agent 两类，再细分 8 子类**——这正好覆盖"个人习惯"的各种形态：

   | 子类 | 位置 | 说明 | 音乐场景举例 |
   |------|------|------|-------------|
   | profile | `user/memories/profile.md` | 用户基本信息 | 单文件合并 |
   | **preferences** | `user/memories/preferences/` | 按主题的偏好 | **"喜欢周杰伦/电子/深夜听"** |
   | entities | `user/memories/entities/` | 实体记忆（人、项目） | 常听歌手、常挖歌单 |
   | events | `user/memories/events/` | 事件记录（决策、里程碑） | "第一次听 X 是因为 Y" |
   | trajectories | `user/memories/trajectories/` | 可复用操作契约 | |
   | experiences | `user/memories/experiences/` | 可复用执行洞察 | |
   | tools | `user/memories/tools/` | 工具使用知识 | |
   | skills | `user/memories/skills/` | 技能执行知识 | |

3. **L0/L1/L2 三级分层**，按需加载省 token：
   - L0（Abstract）：一句话摘要，~100 token，快速判断相关性
   - L1（Overview）：核心信息+使用场景，~2k token，规划阶段决策
   - L2（Details）：完整原文，深读时才加载

   这和我们已有的两级 compaction（MicroCompact 清旧 tool result / AutoCompact LLM 摘要）是**不同维度**的东西——compaction 是"会话内历史整理"，L0/L1/L2 是"跨会话记忆的存储分层"。两者互补。

4. **自动会话抽取**：session commit 后异步分析执行结果和用户反馈，自动更新 user/agent 记忆目录。**这才是"越用越懂你"的机制。**

### 为什么不自己造

- **向量检索/分层抽取是重活**：OpenViking 是 Rust+Python 的大工程（crates + src + C++ 扩展），pi-go 作为 Go agent 框架不该背这个负担
- **接入成本可控**（这是关键判断，先前一度担心"引入重依赖"是误解，实际查了官方部署文档后纠正）：
  - **部署 = 一个 Docker 容器**：`docker compose up -d` 起 `ghcr.io/volcengine/openviking:latest`，监听 `localhost:1933`，数据挂 `~/.openviking` 一个目录。**无外部向量数据库**（没有 pgvector/milvus/postgres 依赖，存储和向量都内置）
  - **模型 = 调云端 API，不强制本地跑**：支持 Volcengine(豆包)/OpenAI/任意 OpenAI 兼容网关。pi-go 现有 OpenAI provider + 网关配置（`OPENAI_BASE_URL`）和 OV 可共用同一套 API key，不是额外负担
  - **有官方 Go SDK**（`sdk/go`），是纯 HTTP 客户端（`net/http` + `X-API-Key`），不依赖 Rust 代码、不需要 cgo。`go get github.com/volcengine/OpenViking/sdk/go` 直接用
- **记忆后端可替换**：pi-go 该定义的是 `runtime.Memory` 抽象接口，OpenViking 是其一实现；将来换别的或自建都不影响上层

> **真实成本再评估**：用户接入 OV 的代价 = 跑一个 docker 容器 + 配 API key（和 pi-go 现在调 LLM 同量级）。对"个人助手要长期记忆"这个诉求，合理。唯一需要注意：pi-agent 从单二进制变成"pi-agent + OV server"，但 OV 是 docker 托管的，对用户基本透明。

> **决策：记忆层 = pi-go 侧薄接口 `runtime.Memory` + 混合后端。** 不自造向量库，借鉴 OV 的文件系统范式（目录组织 + 8 类 memory + L0/L1/L2 分层）作为数据模型。后端分阶段：**P1 先本地 JSON 实现**（零依赖、验证抽象）、**OV 作为可选后端**（语义检索/跨海量记忆时切换，接口不变）。

---

## 3. 记忆层在 pi-go 的落地设计

### 接口定义（薄、领域无关，放 Core 层）

```go
// internal/runtime/memory.go

// Memory 是跨 session、跨 agent 的用户记忆抽象。
// 实现分两档：本地 JSON（默认，零依赖）/ OpenViking（语义检索，需跑 OV 服务）/ no-op（兜底）。
// 接口刻意薄：pi-go 只关心"读偏好 / 写偏好"，检索/分层/向量由后端负责。
type Memory interface {
    // Recall 取回与当前意图相关的记忆片段，注入到 prompt 构建。
    // query 是当前用户意图；本地实现按 namespace 直接读文件，OV 实现做语义检索。
    // 返回的结构化记忆由 Application 决定如何塞进 BuildPrompt。
    Recall(ctx context.Context, userID, namespace, query string) ([]MemoryEntry, error)

    // Record 在会话结束后触发，抽取并沉淀记忆。
    // 本地实现直接追加写文件；OV 实现异步抽取（Record 立即返回）。
    Record(ctx context.Context, userID, namespace string, session SessionTelemetry) error
}

type MemoryEntry struct {
    Category string // profile/preferences/entities/... (OpenViking 8 类)
    Content  string
    L0       string // 摘要（若后端提供）
}
```

**关键设计点**：

- `namespace` 区分 agent（如 `music`/`coding`），但同一个 `userID` 下记忆可跨 agent 检索——**为"通用个人助手"的跨 agent 协同留口子**（比如记账 agent 知道你音乐偏好，不奇怪）
- 接口刻意薄（只 Recall/Record 两个方法），把"怎么存、怎么分层、怎么向量化"全部推给后端实现。pi-go Core 不引入向量库依赖
- **后端分两档，同一接口**：本地 JSON（`data/memory/<userID>/<namespace>/preferences.md` 等，对齐 OV 文件系统范式，零依赖）→ OV（语义检索，需 `docker compose up`）。切换只改实现，上层不动
- 默认实现是 no-op（`NilMemory`），保证不接入任何后端也能跑——**记忆是增强项，不是阻塞项**

### Application 怎么用

`runtime.Application` 增加可选的记忆能力（**用 duck-typing 可选接口，不强加**，对齐已有的 `ToolWithConfirmation` 模式）：

```go
// 可选接口：需要记忆能力的 Application 实现
type ApplicationWithMemory interface {
    // MemoryNamespace 返回该 agent 的记忆命名空间
    MemoryNamespace() string // e.g. "music"
}
```

`AgentSession` 在两个时机调 Memory：
1. **prompt 构建前**：`BuildPrompt` 前调 `Recall`，把相关偏好注入（"该用户偏好周杰伦/电子"）
2. **turn 结束后**：调 `Record`，把本轮交互（听了什么、跳过什么、说了什么）交给后端异步抽取

### MusicApplication 作为首个消费者（PoC 范围）

首个落地场景就是用户说的"音乐习惯收集"：

- `Recall("music", "play something")` → 返回 `preferences: 喜欢电子/深夜`、`entities: 常听周杰伦`
- prompt 注入后，music agent 推荐时偏向这些
- `Record` 把"用户播放了 X、跳过了 Y、说'再来首慢歌'"沉淀成 preference/entity

**PoC 目标**：让 music agent 跨 session 记住"你喜欢什么"。这是验证记忆层抽象是否成立的最小闭环。

---

## 4. 演进路线（Phase 化，每阶段可独立交付）

| Phase | 目标 | 关键交付 | 不做 |
|-------|------|---------|------|
| **P0** | 音乐功能修到能用 | 音频代理支持 Range（seek 不再失败）+ 超时/熔断；删 `GetOrLoad` 死代码；music 文案进 i18n；`music_play` 返回结构化结果替代正则解析 | 记忆、面板重构 |
| **P1** | **记忆层落地（核心）** | `runtime.Memory` 接口 + **本地 JSON 实现**（零依赖，对齐 OV 文件系统范式）+ music 偏好 PoC（Recall/Record 闭环）；前端 sidebar 动态拉 `/applications`（去硬编码） | OV 接入、跨 agent 记忆、面板系统 |
| **P1.5** | 升级到语义记忆（可选） | 接入 OV Go SDK 作为 `Memory` 第二实现（需用户 `docker compose up` OV）；切换不改上层 | — |
| **P2** | agent 自描述 + UI 适配 | Application 声明自己的元信息（icon/category/特化面板声明）；面板系统支持 agent 声明的面板（播放器/歌词/表格/图表成为一等公民，不再 hack 进 ChatPane） | 新 agent |
| **P3** | 扩展个人 agent | 接入第二个个人 agent（记账/日记等），验证 P1/P2 抽象的通用性 | — |

### 节奏建议

- **P0 先做**（1-2 天）：音乐是现有功能，有硬伤（seek 失败）必须先修，否则记忆层做出来也是建在坏掉的地基上
- **P1 是核心价值**（2-3 天）：`runtime.Memory` 接口 + **本地 JSON 实现**先跑通 music 偏好 PoC。用本地实现而非直接接 OV，是为了**先把抽象验证对**——抽象错了，接哪个后端都白搭。本地实现零依赖、能立刻让 music"记住喜好"
- **P1.5 按需升级**：等真正需要"语义联想"或"跨海量记忆检索"时，再切 OV 后端（接口不变）。OV 部署就一个 docker 容器 + 云端 API，不重，但没必要在 P1 就引入
- **P2 等 agent 数量到了再做**（1-2 周）：现在只有 coding+music 两个 agent，面板重构是过度设计。等真正要做第三个异构 agent（需要表格/图表那种）时再启动
- **P3 用真实需求驱动**：不要为"通用"而预先造抽象，等真要做第二个个人 agent 时，P1/P2 的抽象会暴露哪里需要调整

### 触发条件（什么时候启动下一 Phase）

| 信号 | 触发动作 |
|------|---------|
| 音乐 seek 拖动失败 / 网易接口挂拖垮服务 | 启动 P0 |
| 用户第二次抱怨"它不记得我喜欢什么" | 启动 P1 记忆层（本地 JSON 实现） |
| 本地 JSON 够用，但需要"语义联想"或记忆量大到检索慢 | 启动 P1.5，切 OV 后端 |
| 要接第三个 agent，前端 sidebar 又要改代码 | 启动 P2 前端动态化 |
| 新 agent 需要非 chat/diff 的面板 | 启动 P2 面板系统 |

---

## 5. 与现有决策的关系

| 现有决策 | 本文的关系 |
|---------|-----------|
| [skills-vs-application.md](./skills-vs-application.md) | Application 作为抽象已拍板，**本文不动这个结论**。本文延伸 Application 缺的两个维度：记忆（`ApplicationWithMemory`）和自描述（P2）。music 是 skills-vs-application 里"循环相同、身份完全不同、一等公民→拆 Application"的**首个真实案例**，验证了该决策 |
| [deepvcode-essence-absorption.md](./deepvcode-essence-absorption.md) | P0-P2 增强已落地（确认机制/循环检测/web_fetch/MicroCompact），与本文的"个人助手 roadmap"是**正交的两条线**——前者是 coding agent 的工程加固，后者是产品形态扩展 |
| `dev/second-agent-validation/` | music agent 实际上**已经完成了"第二个 agent 验证"**这个开发主题，可考虑归档 |

---

## 6. 不做（明确边界）

- **不自造向量库/检索引擎**——语义检索这种重活交给 OV，pi-go 只定义薄接口。但 P1 的**本地 JSON 实现不算"自造向量库"**——它只做结构化偏好存取（按 namespace 存文件、直接读），无向量、无检索引擎，是验证抽象用的轻量后端，不是要取代 OV
- **不为"通用"预先造抽象**——P2 面板系统等真实需要时再做，不在只有 2 个 agent 时重构
- **不强行让所有 agent 都有记忆**——`ApplicationWithMemory` 是可选接口，coding agent 不实现就没有记忆，不强制
- **不把记忆塞进 session 持久化**——session JSONL 是会话内历史，记忆是跨会话认知，两者分开（对齐 OpenViking：session commit 后才抽取记忆，不混存）
- **不在 P1 阶段做跨 agent 记忆协同**——接口留了 `userID`+`namespace` 的口子，但跨 agent 检索是 P3+ 的事

---

## 7. 一句话总结

**架构在"加 agent"上已经 ready（Application 可插拔），缺的是"让 agent 记住用户"（记忆层）和"让 UI 适配异构 agent"（自描述+面板）。记忆层是核心，借鉴 OpenViking 的文件系统范式（目录组织 + 8 类 memory + L0/L1/L2 分层）作为数据模型，pi-go 侧定义薄 `runtime.Memory` 接口，后端分两档：P1 先本地 JSON（零依赖、验证抽象）、需要语义检索时再切 OV（一个 docker 容器 + 云端 API，不重）。music agent 作为首个消费者验证抽象。节奏上 P0 先修音乐硬伤，P1 落记忆层，P2/P3 按真实需求推进，不为通用性预先造抽象。**
