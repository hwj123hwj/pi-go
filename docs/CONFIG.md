# 配置参考

> pi-go 的完整配置项说明。配置优先级：**环境变量 > .env 文件 > YAML > 默认值**。

---

## 快速配置

安装后编辑配置文件：

```bash
nano ~/.pi-go/.env
```

最简配置（OpenAI 兼容）：
```env
PI_GO_PROVIDER=openai
PI_GO_API_KEY=your-api-key
PI_GO_BASE_URL=http://localhost:4001
PI_GO_MODEL=longcat-opus
```

使用 Anthropic Claude：
```env
PI_GO_PROVIDER=anthropic
ANTHROPIC_API_KEY=your-key
ANTHROPIC_MODEL=claude-sonnet-4-20250514
```

---

## 环境变量

### Provider 与模型

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PI_GO_PROVIDER` | _(必填)_ | LLM Provider：`anthropic` / `openai` |
| `PI_GO_API_KEY` | - | Provider API Key（优先） |
| `OPENAI_API_KEY` | - | OpenAI API Key（后备） |
| `PI_GO_MODEL` | - | 模型名称（优先） |
| `OPENAI_MODEL` | - | OpenAI 模型名称（后备） |
| `PI_GO_BASE_URL` | - | API 地址（优先） |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI API 地址（后备） |
| `ANTHROPIC_API_KEY` | - | Anthropic API Key |
| `ANTHROPIC_MODEL` | - | Anthropic 模型名称 |
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com` | Anthropic API 地址 |

### 服务器

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PI_GO_HOST` | `127.0.0.1` | HTTP 监听地址 |
| `PI_GO_PORT` | `8080` | HTTP 监听端口 |
| `PI_GO_DATA_DIR` | `./data` | 数据目录 |
| `PI_GO_SESSION_FILE` | `./data/session.jsonl` | 会话文件路径 |

### 工具

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PI_GO_ENABLE_BASH` | `false` | 是否启用 Bash 工具 |
| `PI_GO_BASH_TIMEOUT_SECONDS` | `30` | Bash 命令超时 |
| `PI_GO_ENABLE_WEB` | `false` | 是否启用 Web Fetch 工具 |
| `PI_GO_WEB_TIMEOUT_SECONDS` | `30` | Web Fetch 超时（秒） |
| `PI_GO_MAX_OUTPUT_LEN` | `30000` | 工具输出最大字符数 |
| `PI_GO_WORKSPACE` | 当前目录 | 工作目录 |
| `PI_GO_ALLOWED_TOOLS` | - | 工具白名单（逗号分隔） |
| `PI_GO_BLOCKED_TOOLS` | - | 工具黑名单（逗号分隔） |

### 执行后端

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PI_GO_EXECUTION_MODE` | `local` | 执行后端：`local` 或 `ssh` |
| `PI_GO_SSH_HOST` | - | SSH 模式目标主机 |
| `PI_GO_SSH_PORT` | `22` | SSH 端口 |
| `PI_GO_SSH_WORKDIR` | - | SSH 模式远程工作目录 |

### 个人助手

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PI_GO_MUSIC_PORT` | - | 音乐服务端口 |
| `PI_GO_KB_REPO_PATH` | `~/agent-lessons` | 知识库仓库路径 |
| `SILICONFLOW_API_KEY` | - | KB 向量搜索 API Key |
| `SILICONFLOW_EMBEDDING_MODEL` | `bge-m3` | KB Embedding 模型 |
| `SILICONFLOW_BASE_URL` | - | Embedding API 地址 |

### 提示与历史

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PI_GO_HISTORY_FILE` | - | 交互模式历史记录路径 |
| `PI_GO_PROMPT_TEMPLATE` | - | 自定义提示模板路径 |

---

## YAML 配置

除了环境变量和 .env 文件，你也可以用 YAML 配置文件（优先级低于环境变量）：

```yaml
provider: openai
openai_api_key: your-key
openai_base_url: http://localhost:4001
openai_model: longcat-opus
host: 127.0.0.1
port: 8080
enable_bash: true
workspace: /home/user/project
max_turns: 200
```

---

## 安全提示

- ✅ v0.10.3+ 会自动清洗配置值中的 ANSI 转义码
- ✅ `.env` 文件应设置权限：`chmod 600 ~/.pi-go/.env`
- ✅ 不要把 API Key 提交到 Git
- ✅ 生产环境使用 `PI_GO_API_KEY` 环境变量做 HTTP API 认证
