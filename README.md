# Pi-Go

用 Go 实现的通用 Agent 框架，核心目标：**可扩展的 Agent 底座 + 可插拔的应用层**。当前主要应用是 coding-agent（代码编辑助手）、music-agent（音乐助手）和 kb-agent（个人知识库）。

## 架构

![架构图](docs/references/structure.png)

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

**层间依赖规则**：Core 不依赖上层；Platform 只依赖 Core；Application 通过 `runtime.Application` 接口与 Platform 解耦；Entrypoints 组装所有依赖。

## 功能

- **多 Provider 支持**：Anthropic、OpenAI（含本地网关），通过插件注册机制扩展
- **流式输出**：SSE 事件流，支持 text delta / tool call / error 等细粒度事件
- **Agent 循环**：双层循环（外层 follow-up + 内层 tool call），支持顺序/并行工具执行
- **8 个内置工具**：bash、read、write、edit、grep、find、ls、web_fetch
- **会话持久化**：JSONL append-only 存储，支持树状分支
- **上下文压缩**：长对话自动摘要，防止超出上下文窗口
- **MicroCompact**：零 LLM 成本的工具结果清理，保留最近 N 个完整结果
- **技能系统**：加载 `.claude/skills/` 目录下的 SKILL.md
- **HTTP API**：RESTful 接口 + SSE 流式端点 + WebSocket，含 logging/recovery/CORS 中间件
- **CLI 控制面**：Slash Commands 框架 + 16 个内置命令
- **Profile 系统**：coding / review 双 profile，切换即重建 agent
- **SSH 远程执行**：通过 Operations 抽象切换本地 / SSH 执行后端
- **扩展系统**：Extension 接口支持工具、命令、事件钩子注入
- **Tool Lifecycle Hooks**：Before/After hook + PrepareArguments + Confirmation 接口
- **Goal-Driven Loop**：目标驱动模式，LLM 评估器 + 关键词回退自动判断完成度
- **循环检测**：SHA256 指纹识别连续相同工具调用，柔性提醒 Agent 换策略
- **确认门控**：危险工具执行前用户确认（`ToolWithConfirmation` 接口）
- **飞书桥接**：独立服务（`cmd/pi-feishu-bridge`），将 Agent 接入飞书群聊
- **统一用户画像**：跨 Agent 共享的用户记忆层，自动记录用户偏好（频率×时效热度淘汰）
- **KB 向量搜索**：SiliconFlow bge-m3 混合关键词+向量检索，本地 JSON 向量库
- **会话记忆提取**：每轮对话后异步 LLM 抽取用户特征，写入用户画像
- **工具输出概要**：超长工具输出自动生成结构化摘要，保护上下文窗口
- **桌面客户端**：Electron + React GUI，含全局音乐播放器、文件浏览、知识库面板、用户画像面板

## 快速开始

### 安装

**方式一：一键安装（推荐）**

```bash
curl -fsSL https://raw.githubusercontent.com/hwj123hwj/pi-go/main/scripts/install.sh | bash
```

**方式二：go install**

```bash
go install github.com/hwj123hwj/pi-go/cmd/pi-agent@latest
```

**方式三：从源码构建**

```bash
git clone https://github.com/hwj123hwj/pi-go.git
cd pi-go
make build       # 编译到 bin/pi-agent
make install     # 安装到 $GOPATH/bin
```

### 配置

复制 `.env.example` 为 `.env`，填入 API Key：

```bash
cp .env.example .env
# 编辑 .env，设置 PI_GO_PROVIDER 和对应的 API Key
```

### 运行

```bash
# 交互式聊天（Bubble Tea TUI，默认）
pi-agent --mode chat

# 旧的线性 CLI（加 --legacy）
pi-agent --mode chat --legacy

# 单次运行
pi-agent --mode run --prompt "hello"

# HTTP 服务
pi-agent --mode serve --listen 127.0.0.1:8080
```

### 桌面端

pi-go 提供了基于 Electron + React 的桌面客户端：

```bash
cd desktop

# 安装依赖
npm install

# 开发模式（Vite + Electron 热重载）
npm run electron:dev

# 构建桌面应用
npm run electron:build

# 指定架构构建
npm run electron:build:arm64
npm run electron:build:x64
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
| `GET` | `/sessions/{id}/info` | 获取会话信息 |
| `DELETE` | `/sessions/{id}` | 删除会话 |
| `POST` | `/sessions/{id}/model` | 切换会话模型 |
| `POST` | `/sessions/{id}/compact` | 压缩会话上下文 |
| `POST` | `/sessions/{id}/command` | 执行斜杠命令 |
| `GET` | `/sessions/{id}/diff` | 获取会话 Git diff |
| `GET` | `/sessions/{id}/file` | 获取会话文件内容 |
| `GET` | `/models` | 列出可用模型 |
| `GET` | `/tools` | 列出工具 |
| `POST` | `/tools/register` | 注册外部工具 |
| `GET` | `/applications` | 列出可用应用 |
| `GET` | `/profile` | 获取用户画像（所有分类+摘要） |
| `DELETE` | `/profile` | 删除指定用户画像条目 |
| `GET` | `/kb/stats` | 知识库统计 |
| `GET` | `/kb/entries` | 列出知识库条目 |
| `GET` | `/kb/categories` | 知识库分类列表 |
| `GET` | `/kb/tags` | 知识库标签列表 |
| `GET` | `/kb/health` | 知识库健康报告 |
| `GET` | `/kb/read` | 读取知识库条目内容 |
| `GET` | `/workspace/list-dir` | 列出工作目录内容 |
| `GET` | `/workspace/search-files` | 模糊搜索文件 |
| `GET` | `/workspace/read-file` | 读取文件内容 |
| `PUT` | `/workspace/write-file` | 写入文件内容 |
| `GET` | `/ws` | WebSocket 连接 |

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PI_GO_PROVIDER` | _(必填)_ | LLM Provider：`anthropic` / `openai` |
| `PI_GO_API_KEY` | - | Provider API Key（优先） |
| `OPENAI_API_KEY` | - | OpenAI API Key（PI_GO_API_KEY 后备） |
| `PI_GO_MODEL` | - | 模型名称（优先） |
| `OPENAI_MODEL` | - | OpenAI 模型名称（PI_GO_MODEL 后备） |
| `PI_GO_BASE_URL` | - | API 地址（优先） |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI API 地址（后备） |
| `ANTHROPIC_API_KEY` | - | Anthropic API Key |
| `ANTHROPIC_MODEL` | - | Anthropic 模型名称 |
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com` | Anthropic API 地址 |
| `PI_GO_HOST` | `127.0.0.1` | HTTP 监听地址 |
| `PI_GO_PORT` | `8080` | HTTP 监听端口 |
| `PI_GO_DATA_DIR` | `./data` | 数据目录 |
| `PI_GO_SESSION_FILE` | `./data/session.jsonl` | 会话文件路径 |
| `PI_GO_ENABLE_BASH` | `false` | 是否启用 Bash 工具 |
| `PI_GO_BASH_TIMEOUT_SECONDS` | `30` | Bash 命令超时 |
| `PI_GO_ENABLE_WEB` | `false` | 是否启用 Web Fetch 工具 |
| `PI_GO_WEB_TIMEOUT_SECONDS` | `30` | Web Fetch 超时（秒） |
| `PI_GO_MAX_OUTPUT_LEN` | `30000` | 工具输出最大字符数 |
| `PI_GO_WORKSPACE` | 当前目录 | 工作目录 |
| `PI_GO_EXECUTION_MODE` | `local` | 执行后端：`local` 或 `ssh` |
| `PI_GO_SSH_HOST` | - | SSH 模式目标主机 |
| `PI_GO_SSH_PORT` | `22` | SSH 端口 |
| `PI_GO_SSH_WORKDIR` | - | SSH 模式远程工作目录 |
| `PI_GO_ALLOWED_TOOLS` | - | 工具白名单（逗号分隔） |
| `PI_GO_BLOCKED_TOOLS` | - | 工具黑名单（逗号分隔） |
| `PI_GO_KB_REPO_PATH` | `~/agent-lessons` | 知识库仓库路径 |
| `PI_GO_KB_EMBEDDING_API_KEY` | - | KB 向量搜索 API Key（空则仅关键词搜索） |
| `PI_GO_KB_EMBEDDING_BASE_URL` | - | KB Embedding API 地址 |
| `PI_GO_KB_EMBEDDING_MODEL` | `bge-m3` | KB Embedding 模型名称 |
| `PI_GO_HISTORY_FILE` | - | 交互模式历史记录路径 |
| `PI_GO_PROMPT_TEMPLATE` | - | 自定义提示模板路径 |
| `PI_GO_MUSIC_PORT` | - | 音乐服务端口 |

完整环境变量说明见 [CONTRIBUTING.md](docs/CONTRIBUTING.md#八环境变量速查)。

## 参与贡献

详见 [贡献指南](docs/CONTRIBUTING.md)，包含项目结构、开发流程、代码规范等。

## 测试

```bash
go test ./...
```

## 项目统计

- **语言**：Go 1.24+
- **代码量**：~26,000+ 行 Go 代码（144 个源文件 + 67 个测试文件）+ TypeScript 前端
- **应用层**：3 个（coding-agent / music-agent / kb-agent）
- **内置工具**：8 个（read / write / edit / bash / grep / find / ls / web_fetch）
- **斜杠命令**：16 个（help / compact / sessions / session / branch / new / switch / tools / model / models / profiles / profile / goal / context / clear / wiki）
