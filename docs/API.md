# HTTP API 参考

> pi-go Server 模式（`pi-go serve`）的完整 API 文档。

---

## 快速开始

```bash
pi-go serve
# Server running on http://127.0.0.1:8080
```

### 认证

如果设置了 `PI_GO_API_KEY` 环境变量，所有请求需要 Bearer token：

```bash
curl -H "Authorization: Bearer $PI_GO_API_KEY" http://127.0.0.1:8080/health
```

---

## 对话

### 同步对话

```
POST /chat
```

```json
{
  "message": "帮我看看这个项目结构",
  "session_id": "sess_123"
}
```

### SSE 流式对话

```
POST /chat/stream
```

Server-Sent Events 流式输出，实时返回 AI 回复的每个 token。

```bash
curl -N -X POST http://127.0.0.1:8080/chat/stream \
  -H "Content-Type: application/json" \
  -d '{"message": "你好"}'
```

### WebSocket

```
GET /ws
```

全双工 WebSocket 连接，支持对话、模型切换、流式回复。

---

## 会话管理

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/sessions` | 列出所有会话 |
| `POST` | `/sessions` | 创建新会话 |
| `GET` | `/sessions/{id}/messages` | 获取会话消息 |
| `GET` | `/sessions/{id}/info` | 获取会话信息 |
| `DELETE` | `/sessions/{id}` | 删除会话 |
| `POST` | `/sessions/{id}/model` | 切换会话模型 |
| `POST` | `/sessions/{id}/compact` | 压缩会话上下文 |
| `POST` | `/sessions/{id}/command` | 执行斜杠命令 |
| `GET` | `/sessions/{id}/diff` | 获取会话 Git diff |
| `GET` | `/sessions/{id}/file` | 获取会话文件内容 |

### 切换模型

```
POST /sessions/{id}/model
```

```json
{
  "model": "gpt-4o",
  "provider": "openai"
}
```

---

## 模型与工具

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/models` | 列出可用模型 |
| `GET` | `/tools` | 列出已注册工具 |
| `POST` | `/tools/register` | 注册外部工具 |
| `GET` | `/applications` | 列出可用应用 |

---

## 知识库

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/kb/stats` | 知识库统计 |
| `GET` | `/kb/entries` | 列出条目 |
| `GET` | `/kb/categories` | 分类列表 |
| `GET` | `/kb/tags` | 标签列表 |
| `GET` | `/kb/health` | 健康报告 |
| `GET` | `/kb/read` | 读取条目内容 |

---

## 用户画像

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/profile` | 获取用户画像（所有分类+摘要） |
| `DELETE` | `/profile` | 删除指定画像条目 |

---

## 文件操作

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/workspace/list-dir` | 列出工作目录内容 |
| `GET` | `/workspace/search-files` | 模糊搜索文件 |
| `GET` | `/workspace/read-file` | 读取文件内容 |
| `PUT` | `/workspace/write-file` | 写入文件内容 |

---

## 健康检查

```
GET /health
```

```json
{
  "status": "ok"
}
```
