# Pi-Go

Pi Agent 框架的 Go 语言重写，基于 [Pi TypeScript 原版](../pi) 学习 Agent 开发。

## 架构

```
┌──────────────────────────────────────┐
│  cmd/pi-agent   (CLI 入口)           │
├──────────────────────────────────────┤
│  internal/agent  (Agent 循环/状态)    │
│  internal/server (HTTP API + SSE)    │
├──────────────────────────────────────┤
│  internal/ai     (统一 LLM 流式 API) │
│  internal/ai/providers               │
│    ├── anthropic.go                  │
│    ├── openai.go                     │
│    └── mock.go                       │
├──────────────────────────────────────┤
│  internal/tools   (内置工具集)        │
│  internal/session (JSONL 会话持久化)  │
│  internal/compaction (上下文压缩)     │
│  internal/skill    (技能加载)         │
│  internal/config   (配置管理)         │
│  internal/prompt   (系统提示构建)     │
└──────────────────────────────────────┘
```

## 功能

- **多 Provider 支持**：Anthropic、OpenAI、Mock，通过插件注册机制扩展
- **流式输出**：SSE 事件流，支持 text delta / tool call / error 等细粒度事件
- **Agent 循环**：双层循环（外层 follow-up + 内层 tool call），支持顺序/并行工具执行
- **7 个内置工具**：bash、read、write、edit、grep、find、ls
- **会话持久化**：JSONL append-only 存储，支持树状分支
- **上下文压缩**：长对话自动摘要，防止超出上下文窗口
- **技能系统**：加载 `.claude/skills/` 目录下的 SKILL.md
- **HTTP API**：RESTful 接口 + SSE 流式端点，含 logging/recovery/CORS 中间件

## 快速开始

### 安装

```bash
go build -o pi-agent ./cmd/pi-agent
```

### 配置

复制 `.env.example` 为 `.env`，填入 API Key：

```bash
cp .env.example .env
# 编辑 .env，设置 PI_GO_PROVIDER 和对应的 API Key
```

### 运行

```bash
# 单次运行
./pi-agent -mode run -prompt "hello"

# 交互式聊天
./pi-agent -mode chat

# HTTP 服务
./pi-agent -mode serve -listen 127.0.0.1:8080
```

### 指定会话

```bash
./pi-agent -mode chat -session sess_1234567890
```

## HTTP API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/health` | 健康检查 |
| `POST` | `/chat` | 同步对话 |
| `POST` | `/chat/stream` | SSE 流式对话 |
| `GET` | `/sessions` | 列出会话 |
| `POST` | `/sessions` | 创建会话 |
| `GET` | `/sessions/{id}/messages` | 获取会话消息 |
| `DELETE` | `/sessions/{id}` | 删除会话 |
| `GET` | `/tools` | 列出工具 |

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PI_GO_PROVIDER` | `mock` | LLM Provider：`mock` / `anthropic` / `openai` |
| `ANTHROPIC_API_KEY` | - | Anthropic API Key |
| `ANTHROPIC_MODEL` | - | Anthropic 模型名称 |
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com` | Anthropic API 地址 |
| `OPENAI_API_KEY` | - | OpenAI API Key |
| `OPENAI_MODEL` | - | OpenAI 模型名称 |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI API 地址 |
| `PI_GO_HOST` | `127.0.0.1` | HTTP 监听地址 |
| `PI_GO_PORT` | `8080` | HTTP 监听端口 |
| `PI_GO_SESSION_FILE` | `./data/session.jsonl` | 会话文件路径 |
| `PI_GO_ENABLE_BASH` | `false` | 是否启用 Bash 工具 |
| `PI_GO_BASH_TIMEOUT_SECONDS` | `30` | Bash 命令超时 |

## 测试

```bash
go test ./...
```

## 项目统计

- **语言**：Go 1.22
- **代码量**：~7400 行（49 个 Go 文件）
- **外部依赖**：仅 `testify`（测试断言）
