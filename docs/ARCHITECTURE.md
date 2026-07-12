# 架构设计

> 本文档面向开发者和贡献者，描述 pi-go 的四层架构和模块划分。

---

## 四层架构

```
┌─────────────────────────────────────────────────────┐
│  Entrypoints（组装与入口）                           │
│  cmd/pi-agent  cmd/pi-feishu-bridge                  │
├─────────────────────────────────────────────────────┤
│  Application（领域应用层，可插拔）                    │
│  agents/coding/ — 工具集、提示、命令、Profile        │
│  agents/music/  — 音乐助手应用                       │
│  agents/kb/     — 知识库 Agent（第二大脑）            │
├─────────────────────────────────────────────────────┤
│  Platform（运行时平台层，领域无关）                   │
│  runtime/ — AgentSession 生命周期、Application 接口  │
├─────────────────────────────────────────────────────┤
│  Core（核心层，零领域知识）                           │
│  agent/  ai/  session/  compaction/  operations/     │
│  prompt/  skill/  extensions/  slashcmd/             │
└─────────────────────────────────────────────────────┘
```

## 层间依赖规则

| 规则 | 说明 |
|------|------|
| Core → 上层 | ❌ Core 不依赖任何上层模块 |
| Platform → Core | ✅ Platform 只依赖 Core |
| Application → Core | ✅ 通过 `runtime.Application` 接口解耦 |
| Entrypoints → All | ✅ 组装所有依赖，是唯一的 "wire up" 层 |

**核心原则**：Core 层零领域知识，永远不知道 coding/music/kb 的存在。新增一个 Application 不需要改 Core。

## 核心模块

### Core 层

| 模块 | 职责 |
|------|------|
| `agent/` | Agent 循环（双层：外层 follow-up + 内层 tool call） |
| `ai/` | LLM Provider 接口、流式事件、模型定义 |
| `session/` | 会话持久化（JSONL append-only，树状分支） |
| `compaction/` | 上下文压缩（LLM 摘要 + MicroCompact 零成本清理） |
| `operations/` | 执行抽象（本地 / SSH 远程） |
| `prompt/` | 系统提示构建、Context 文件加载 |
| `skill/` | Skills 加载器（`.claude/skills/`） |
| `extensions/` | Extension 接口（工具/命令/钩子注入） |
| `slashcmd/` | Slash 命令框架 |

### Platform 层

| 模块 | 职责 |
|------|------|
| `runtime/` | AgentSession 生命周期管理、模型切换、rebuild |
| `config/` | 配置加载（环境变量 / YAML / .env） |
| `server/` | HTTP 服务器（REST + SSE + WebSocket） |
| `sessionmgr/` | 多会话管理 |
| `tui/` | 终端 UI（Bubble Tea） |
| `app/` | 应用启动与依赖组装 |

### Application 层

| 应用 | 说明 |
|------|------|
| `agents/coding/` | 编程助手：8 工具 + 16 命令 + coding/review profile |
| `agents/music/` | 音乐助手：多源播放 + 代理 + 缓存 |
| `agents/kb/` | 知识库 Agent：语义搜索 + 自动写入 |

## Agent 循环

```
用户输入
  │
  ▼
┌──────────────────────────┐
│ 外层循环 (Follow-up)      │ ← Goal-Driven / LLM 评估器
│  ┌────────────────────┐  │
│  │ 内层循环 (Tool)     │  │ ← 工具调用、结果处理
│  │  LLM → Tool → LLM  │  │
│  └────────────────────┘  │
│  达成目标？ → 是 → 结束   │
│            → 否 → 继续    │
└──────────────────────────┘
```

- **外层循环**：Goal 驱动，LLM 评估器 + 关键词回退判断完成度
- **内层循环**：Tool 调用，支持顺序 / 并行执行
- **循环检测**：SHA256 指纹识别连续相同调用，柔性提醒
- **确认门控**：危险工具执行前用户确认

## 添加新的 Application

1. 在 `internal/agents/` 下创建新目录
2. 实现 `runtime.Application` 接口
3. 在 `cmd/pi-agent/main.go` 中注册
4. Core 层零改动

```go
type Application interface {
    BuildTools(opts ToolBuildOptions) []agent.Tool
    BuildPrompt(opts PromptBuildOptions, profile, goal string) string
    SlashCommands() []slashcmd.Command
}
```

## 相关文档

- [项目上下文快照](PROJECT_CONTEXT.md)
- [贡献指南](CONTRIBUTING.md)
- [产品路线图](PRODUCT_ROADMAP.md)
