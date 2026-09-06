# Pi-Go 项目上下文

> 本文档是 pi-go 项目的高层快照，供调研/分析任务快速了解项目全貌，无需重读源码。
> 架构变更时应同步更新本文档。

---

## 定位

pi-go 是一个用 Go 实现的通用 Agent 框架，核心目标是：**可扩展的 Agent 底座 + 可插拔的应用层**。

当前已落地两个一等公民 Application：
- **coding-agent**（代码编辑助手，主力）
- **music-agent**（音乐助手，第一个"非编程"个人 agent，验证了多 agent 架构）

方向上正从"编程助手"演进为**通用个人助手**（详见 [personal-assistant-roadmap.md](./decisions/personal-assistant-roadmap.md)）：music 是首个个人 agent，后续会集成更多（记账/健康/日记），核心诉求之一是"收集个人习惯"。

## 架构

```
┌─────────────────────────────────────────────────────┐
│  Entrypoints（组装与入口）                           │
│  app/ CLI/ server/                                   │
│  cmd/pi-agent  cmd/pi-feishu-bridge                  │
├─────────────────────────────────────────────────────┤
│  Application（领域应用层，可插拔）                    │
│  agents/coding/ — coding-agent 的工具集、提示、命令   │
│  agents/music/  — music-agent 的工具/提示/SessionExt │
│    └─ 依赖 music/（音乐专属基础设施：多源+代理+缓存） │
├─────────────────────────────────────────────────────┤
│  Platform（运行时平台层，领域无关）                   │
│  runtime/ — AgentSession 生命周期、Application 接口  │
├─────────────────────────────────────────────────────┤
│  Core（核心层，零领域知识）                           │
│  ai/ agent/ session/ compaction/ operations/         │
│  prompt/ skill/ extensions/                          │
└─────────────────────────────────────────────────────┘
```

### 层间依赖规则

- **Core** → 不依赖上层任何包
- **Platform** → 只依赖 Core，通过 `runtime.Application` 接口与上层解耦
- **Application** → 实现 `runtime.Application` 接口，依赖 Core + Platform
- **Entrypoints** → 组装所有依赖，注入 Application 实例
- **领域基础设施**（如 `internal/music/`）→ Application 层的叶子库，只被对应 agent 和入口 import，零上层依赖。**不属于标准四层**，是某 Application 的专属支撑层

### 物理位置：`sdk/` 与 `internal/`

Core + Platform（18 个包）物理位于 `sdk/`，是**可被外部 Go 模块 import 的公共 API 面**——其他 Go 后端服务 `import "github.com/hwj123hwj/pi-go/sdk/..."` 即可拿到 Agent 原子能力（详见 `sdk/doc.go` 与 `sdk/example_test.go`）。

Application 层及以下（agents/music/feishu/tui/server 等）位于 `internal/`，编译器强制私有。

**硬约束**：`sdk/` 不得 import `pi-go/internal/` 的任何包，由 `sdk/arch_test.go` 在测试中强制——想进 SDK 的能力必须零领域知识。当前 v0 阶段，SDK API 不承诺向后兼容。

### 关键接口

| 接口 | 位置 | 作用 |
|------|------|------|
| `runtime.Application` | `sdk/runtime/application.go` | Platform 与 Application 的解耦点：`BuildTools()` + `BuildPrompt()` |
| `agent.Tool` | `sdk/agent/` | 工具系统：泛型 schema + 执行函数 |
| `providers.Provider` | `sdk/ai/providers/` | LLM Provider 抽象：注册制 + 懒加载 |
| `operations.Operations` | `sdk/operations/` | 执行后端：本地 / SSH 切换 |
| `music.MusicSource` | `internal/music/source.go` | 音乐源后端抽象：Search/GetAudioURL/GetLyrics/...（网易/B站实现） |
| `music.SourceRouter` | `internal/music/router.go` | 多源路由：按复合 ID（`netease:123`/`bilibili:BV1xx`）分发到对应源 |
| `slashcmd.SessionContext` | `sdk/slashcmd/context.go` | Slash command 可操作的 session 接口（model/profile/switch） |
| `slashcmd.AppContext` | `sdk/slashcmd/context.go` | Slash command 可操作的 app 接口（session CRUD/profiles） |

## 核心能力

| 能力 | 实现位置 | 状态 |
|------|---------|------|
| 统一 LLM API 抽象 | `sdk/ai/` | ✅ 多 Provider（Anthropic/OpenAI/Mock/DeepV） |
| Agent 双层循环 | `sdk/agent/` | ✅ 外层 follow-up + 内层 tool call |
| 7 个内置工具 | `sdk/tools/` + 组装于 `internal/agents/coding/tools/` | ✅ read/write/edit/bash/grep/find/ls |
| 工具过滤 | config AllowedTools/BlockedTools | ✅ |
| Tool Lifecycle hooks | `sdk/agent/tool_lifecycle.go` | ✅ Before/After hook 接口 + PrepareArguments |
| Operations 抽象 | `sdk/operations/` | ✅ Local + SSH 执行后端 |
| 事件系统 + 流式输出 | `sdk/agent/` + SSE | ✅ |
| 上下文压缩 | `sdk/compaction/` | ✅ LLM 摘要 + 保留最近消息 |
| JSONL 树状会话 | `sdk/session/` | ✅ 分支 + MoveTo |
| 扩展系统 | `sdk/extensions/` | ✅ 工具/命令/事件钩子 |
| 技能系统 | `sdk/skill/` | ✅ Markdown 格式加载 |
| HTTP API Server | `internal/server/` | ✅ REST + SSE |
| 多执行模式 | app 层 | ✅ interactive/print/serve |
| SSH 远程执行 | `sdk/operations/ssh.go` | ✅ |
| Slash Commands 框架 | `sdk/slashcmd/` | ✅ 注册制 + 结构化 CommandResult + Session 交接 |
| CLI 控制面 | `internal/agents/coding/commands/` + `cli/` | ✅ /new /switch /sessions /model /models /tools /profiles /profile /goal /context /clear |
| Profile 机制 | `internal/agents/coding/profile/` | ✅ coding/review 双 profile，切换即重建 agent |
| Session Goal | `runtime.AgentSession` + prompt builder | ✅ session 级目标注入 system prompt |
| music-agent（多源音乐） | `internal/agents/music/` + `internal/music/` | ✅ music_play/search/recommend/lyrics/playlist 工具；多源（网易云+B站）+ SourceRouter；网易 VIP 自动降级 B站；B站搜索质量过滤（黑名单+同名）；音频代理支持 Range seek；结构化 PlayDetails 透传到前端 |
| 多 Application 并存 | `internal/app/app.go` | ✅ Applications map（coding/music），前端选 agent 建对应 session |

## 技术栈

- **语言**：Go 1.24+
- **外部依赖**：极简（标准库为主，少量第三方）
- **存储**：文件系统（JSONL 会话、JSON 配置）
- **构建**：Go modules，无复杂构建链

## 与 TypeScript Agent 项目的典型差异

| 维度 | Go (pi-go) | TypeScript (常见) |
|------|-----------|-------------------|
| 并发 | goroutine + channel | async/await + Promise |
| 错误处理 | 多返回值 error | try/catch + Error 对象 |
| 泛型 | 1.18+ 类型参数，较简洁 | 更灵活（条件类型等） |
| 接口 | 结构化类型，隐式实现 | 同为结构化类型但更灵活 |
| 工具链 | go build/test/vet，单一 | npm/tsc/bun，碎片化 |
| 分发 | 单二进制 | Node.js 运行时依赖 |

## 当前进展与方向

- **已完成**：runtime 从 coding-agent 解耦（Application 接口注入）
- **已完成**：CLI 控制面（session 切换、model 切换、profile 切换、结构化 slash command）
- **已完成**：music-agent 落地——多源音乐（网易云+B站兜底）、音频代理、桌面端播放器
- **已完成**：Desktop App（Electron + React）首版，管理 pi-agent 子进程
- **规划中**：记忆层（`runtime.Memory` 接口，music 偏好收集为首场景，详见 personal-assistant-roadmap）；更多个人 agent（记账/健康/日记）
- **文档**：`docs/` 下有架构提案、产品路线图、编码规范、决策记录

## 关键文件速查

| 文件 | 职责 |
|------|------|
| `sdk/ai/stream.go` | 统一 LLM 流式 API 入口 |
| `sdk/agent/agent.go` | Agent 核心状态机 |
| `sdk/agent/loop.go` | Agent 双层循环实现 |
| `sdk/runtime/agent_session.go` | AgentSession 生命周期 |
| `sdk/runtime/application.go` | Application 接口定义 |
| `internal/agents/coding/application.go` | CodingApplication 实现 |
| `internal/app/app.go` | 依赖组装 |
| `sdk/session/session.go` | 树状会话存储 |
| `sdk/compaction/compaction.go` | 上下文压缩策略 |
| `sdk/config/config.go` | 配置结构定义 |
| `sdk/slashcmd/registry.go` | Slash command 注册与执行框架 |
| `internal/agents/coding/commands/builtins.go` | Coding-agent 内置命令（/new /switch /model /profile 等） |
| `internal/agents/coding/profile/profile.go` | Profile 类型定义与 prompt 片段 |
| `internal/agents/coding/cli/interactive.go` | CLI 交互模式（命令分发 + session 交接） |
| `internal/agents/music/application.go` | MusicApplication 实现（首个非编程 agent） |
| `internal/music/source.go` / `router.go` | MusicSource 接口 + SourceRouter 多源路由 |
| `internal/music/netease_adapter.go` / `bilibili_adapter.go` | 网易/B站源 adapter（实现 MusicSource） |
| `internal/music/handler.go` | 音频代理 HTTP handler（Range 透传 + 多源防盗链 Referer） |
