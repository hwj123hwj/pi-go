<div align="center">

# π-go

**你的 AI 编程搭档，也是你的个人 AI 助手。**

Go 实现的智能 Agent 框架 — 写代码、搜知识、放音乐，一个终端全搞定。

[快速开始](#快速开始) · [功能一览](#核心功能) · [桌面端](#桌面客户端) · [技术文档](#技术文档)

</div>

---

## 它能做什么

**写代码** — 读文件、改代码、跑命令、搜代码库，一个对话窗口搞定。不用切终端。

**记知识** — 自动学习你的习惯和偏好，越用越懂你。个人知识库帮你沉淀经验。

**放音乐** — 内置音乐助手，支持网易云 / B站 / 本地音乐，边写代码边听歌。

**多端使用** — 终端 TUI、桌面客户端、飞书群聊、HTTP API，随时随地用。

---

## 核心功能

| 🤖 智能 Agent | 📝 编程助手 | 🎵 个人助手 |
|---|---|---|
| 多模型自由切换 | 8 个内置工具 | 音乐播放器 |
| 流式实时输出 | Markdown 渲染 | 网易云 / B站 / 本地 |
| 自动上下文压缩 | Git diff 集成 | 用户画像记忆 |
| 循环检测防卡死 | 文件自动补全 | 个人知识库 |

<details>
<summary>📖 展开看完整功能列表</summary>

### Agent 引擎
- **多 Provider**：Anthropic Claude、OpenAI、本地网关，随时切换
- **流式输出**：实时看到 AI 的思考和回复，不用等
- **自动压缩**：长对话自动总结，不会因为太长而"失忆"
- **循环检测**：AI 卡在重复操作时会自动提醒换思路
- **安全确认**：执行危险操作前会先问你

### 编程能力
- **文件读写**：读取、创建、编辑项目文件
- **代码搜索**：按内容或文件名快速定位
- **Shell 命令**：执行 bash 命令（需开启）
- **网页抓取**：获取网页内容用于分析
- **Slash 命令**：`/help`、`/model`、`/compact` 等 16 个快捷命令
- **自动补全**：`/` 补全命令，`@` 补全文件路径

### 个人助手
- **音乐播放**：搜歌、播放、收藏，支持多源混合
- **用户画像**：自动学习你的偏好和习惯
- **知识库**：个人经验沉淀，支持语义搜索
- **飞书集成**：在飞书群里直接和 AI 对话

### 多端支持
- **终端 TUI**：美观的终端界面，Markdown 渲染、语法高亮
- **桌面客户端**：Electron + React，全局音乐播放器
- **HTTP API**：RESTful + WebSocket，方便二次开发
- **飞书桥接**：独立服务，接入飞书群聊

</details>

---

## 快速开始

### 安装

```bash
curl -fsSL https://raw.githubusercontent.com/hwj123hwj/pi-go/main/scripts/install.sh | bash
```

安装脚本会自动完成一切：下载二进制 → 配置 PATH → 创建配置 → 引导填入 API Key。

### 使用

```bash
pi-go chat              # 💬 交互式聊天（推荐）
pi-go run -p "你好"      # ⚡ 单次提问
pi-go serve             # 🌐 HTTP 服务模式
```

### 终端界面快捷键

| 按键 | 功能 | | 按键 | 功能 |
|------|------|-|------|------|
| `Enter` | 发送消息 | | `Ctrl+C` | 中断 / 退出 |
| `Ctrl+J` | 换行 | | `Ctrl+D` | 退出 |
| `Ctrl+L` | 清屏 | | `Ctrl+P` | 切换模型 |
| `Ctrl+R` | 搜索历史 | | `↑` `↓` | 浏览历史 |
| `Tab` | 自动补全 | | `/` | Slash 命令 |

### 配置

安装时如果没有填 API Key，或者想修改配置：

```bash
nano ~/.pi-go/.env
```

最简配置：
```env
PI_GO_PROVIDER=openai
PI_GO_API_KEY=your-api-key
PI_GO_BASE_URL=http://localhost:4001
PI_GO_MODEL=longcat-opus
```

<details>
<summary>⚙️ 其他安装方式</summary>

**go install**
```bash
go install github.com/hwj123hwj/pi-go/cmd/pi-agent@latest
```

**从源码构建**
```bash
git clone https://github.com/hwj123hwj/pi-go.git
cd pi-go
make build && make install
```

</details>

---

## 桌面客户端

pi-go 提供了基于 Electron + React 的桌面客户端，含全局音乐播放器、文件浏览器、知识库面板：

```bash
cd desktop
npm install
npm run electron:dev      # 开发模式
npm run electron:build    # 打包
```

---

## 飞书集成

通过 `pi-feishu-bridge` 独立服务，可以将 AI Agent 接入飞书群聊，在群里直接和 AI 对话、执行 Slash 命令。

详见 [飞书集成文档](docs/references/feishu-integration-ref.md)。

---

---

## 作为 SDK 嵌入你的 Go 服务

π-go 不只是命令行工具——核心 Agent 能力以 `sdk/` 包对外提供，任何 Go 后端服务都能 import 拿到原子能力（Agent 循环、工具系统、会话持久化、上下文压缩、Provider 注册制）：

```go
import (
    "github.com/hwj123hwj/pi-go/sdk/agent"
    "github.com/hwj123hwj/pi-go/sdk/ai"
    "github.com/hwj123hwj/pi-go/sdk/ai/providers"
)

registry := providers.NewRegistry()
registry.Register(myProvider{}) // 实现 providers.Provider 接口

ag := agent.New(agent.Options{
    Model:    ai.Model{ID: "glm-4.7", Name: "GLM", Provider: "my"},
    Registry: registry,
    System:   "你是嵌入在业务服务里的助手",
    Tools:    []agent.Tool{myTool{}}, // 实现 agent.Tool 接口
})
reply, err := ag.Prompt(ctx, ai.NewTextUserMessage("..."))
```

完整可运行示例见 [`sdk/example_test.go`](sdk/example_test.go)。架构约束：`sdk/` 零领域知识、不依赖 `internal/`（测试强制）；音乐/飞书等领域能力属于应用层，通过实现 `runtime.Application` 接口构建，参考 `internal/agents/`。

## 技术文档

> 以下文档面向开发者和贡献者，普通用户不需要看。

| 文档 | 说明 |
|------|------|
| [架构设计](docs/ARCHITECTURE.md) | 四层架构、模块划分、依赖规则 |
| [环境变量](docs/CONFIG.md) | 完整配置项说明 |
| [HTTP API](docs/API.md) | RESTful + WebSocket 接口文档 |
| [贡献指南](docs/CONTRIBUTING.md) | 项目结构、开发流程、代码规范 |
| [项目上下文](docs/PROJECT_CONTEXT.md) | 高层架构快照 |
| [产品路线图](docs/PRODUCT_ROADMAP.md) | 未来规划 |
| [部署指南](docs/deploy.md) | 自动部署说明 |

---

## 参与贡献

欢迎 Issue 和 PR！开发流程详见 [贡献指南](docs/CONTRIBUTING.md)。

```bash
git clone https://github.com/hwj123hwj/pi-go.git
cd pi-go
make test    # 跑测试
make build   # 编译
```

## License

MIT
