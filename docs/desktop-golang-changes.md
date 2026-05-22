# Desktop Client — Go 后端变更说明

> 本文档描述 `feat/desktop-client` 分支对 Go 后端的所有改动，供代码审查参考。

---

## 变更概览

| 文件 | 操作 | 行数 | 说明 |
|------|------|------|------|
| `internal/server/websocket.go` | **新增** | +266 | WebSocket 传输层，双向通信 |
| `internal/server/server.go` | 修改 | +141 / -17 | 新增 3 个 REST 端点 + WebSocket 路由重组 |
| `internal/config/config.go` | 修改 | +7 | `PI_GO_DATA_DIR` 环境变量支持 |
| `cmd/pi-agent/main.go` | 修改 | +7 / -2 | `.env` 路径可通过环境变量指定 |
| `go.mod` / `go.sum` | 修改 | — | 新增 `gorilla/websocket` 依赖 |

**改动原则**：所有改动都是**向后兼容**的——现有 CLI/TUI 模式行为完全不变，新增功能仅在收到对应请求时才激活。

---

## 一、WebSocket 传输层（`internal/server/websocket.go`）

### 为什么需要

原有 `/chat/stream` 端点使用 SSE（Server-Sent Events），是单向通道——服务端推送事件给客户端。但桌面 GUI 需要**双向通信**（发送消息、取消生成、切换模型、心跳），SSE 做不到。

### 协议设计

客户端 → 服务端消息格式：

```json
// 发送 prompt
{ "type": "prompt", "session_id": "xxx", "prompt": "你好" }

// 取消正在进行的生成
{ "type": "cancel", "session_id": "xxx" }

// 切换模型
{ "type": "switch_model", "session_id": "xxx", "model": "glm-5" }

// 心跳
{ "type": "ping" }
```

服务端 → 客户端消息格式：

```json
// 通知 session ID（新创建 session 时）
{ "type": "session_id", "session_id": "xxx" }

// 流式事件（核心，每个 agent 事件一条）
{ "type": "event", "session_id": "xxx", "event": { ... } }

// 状态通知
{ "type": "status", "session_id": "xxx", "streaming": true }

// 模型信息
{ "type": "model_info", "session_id": "xxx", "provider": "deepv", "model": "glm-5" }

// 错误
{ "type": "error", "message": "..." }

// 心跳回复
{ "type": "pong" }
```

### 关键实现细节

1. **线程安全写入**：`wsConn` 封装了 `sync.Mutex`，所有写操作通过 `writeJSON()` 串行化。因为 agent 事件在 goroutine 中推送到 WS，而主循环也可能发消息（如 `session_id`）。

2. **取消机制**：每个 WS 连接维护一个 `context.CancelFunc`，新 prompt 自动取消前一个（防止多个生成并发），`cancel` 消息显式取消。

3. **超时控制**：`context.WithTimeout(context.Background(), 5*time.Minute)` 防止单次生成无限运行。

4. **Clean up**：连接断开时（读循环退出），自动取消正在进行的生成任务。

### 为什么选 gorilla/websocket

对比了 `gorilla/websocket` 和 `nhooyr.io/websocket`：
- `gorilla/websocket` 更成熟、社区更大、文档更全
- `nhooyr.io/websocket` 是 go.mod 里的间接依赖（被其他包引入），但直接使用 `gorilla` 更直观
- 两者 API 水平相当，`gorilla` 的 `Upgrader` 模式更适合 HTTP handler 集成

---

## 二、Server 路由重组（`internal/server/server.go`）

### 改动前

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /health", ...)
// ... 8 个路由
handler = corsMiddleware(handler)
handler = recoveryMiddleware(handler)
handler = loggingMiddleware(handler)
```

所有路由共享同一套 middleware chain。

### 改动后

```go
// REST API — 走完整 middleware chain（cors → recovery → logging）
restMux := http.NewServeMux()
restMux.HandleFunc("GET /health", ...)
// ... 原有路由

var restHandler http.Handler = restMux
restHandler = corsMiddleware(restHandler)
restHandler = recoveryMiddleware(restHandler)
restHandler = loggingMiddleware(restHandler)

// WebSocket — 只走 cors，不走 logging（避免 Hijack 冲突）
wsHandler := corsMiddleware(http.HandlerFunc(s.handleWebSocket))

// 顶层路由分发
topMux := http.NewServeMux()
topMux.Handle("GET /ws", wsHandler)
topMux.Handle("/", restHandler)    // 其余全部走 REST
```

### 为什么分离 middleware

WebSocket 升级后连接从 HTTP 切换到 TCP 长连接，`loggingMiddleware` 内部包装的 `responseWriter` 会导致 `Hijack()` 失败。虽然我们给 `responseWriter` 实现了 `Hijack` 接口作为保底：

```go
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
    return rw.ResponseWriter.(http.Hijacker).Hijack()
}
```

但最干净的方式是让 `/ws` 路由完全不经过 `loggingMiddleware`，只保留 `corsMiddleware`。

### 新增的 3 个 REST 端点

| 端点 | 方法 | 用途 |
|------|------|------|
| `GET /models` | 获取可用模型列表 + 当前模型 | 桌面端模型选择器 |
| `GET /sessions/{id}/info` | 获取 session 的 provider/model 信息 | 恢复会话时显示当前模型 |
| `POST /sessions/{id}/model` | 切换 session 的模型 | REST 方式切换模型（WebSocket 也有同功能） |

这三个端点是给桌面 GUI 的 REST API 补充，WebSocket 里也有对应的 `switch_model` 消息。REST 端点方便调试（curl 直测）和未来非 WebSocket 客户端使用。

---

## 三、Config 可配置数据目录（`internal/config/config.go`）

### 改动

```go
// 新增（在 PI_GO_SESSION_FILE 检查之前）
if v := os.Getenv("PI_GO_DATA_DIR"); v != "" {
    c.DataDir = v
    // 如果没有显式设置 SessionFile，则从 DataDir 派生
    if os.Getenv("PI_GO_SESSION_FILE") == "" {
        c.SessionFile = v + "/session.jsonl"
    }
}
```

### 为什么需要

打包为 macOS `.app` 后，应用包是只读的。Go 后端不能在 `.app/Contents/Resources/` 里写 `data/` 目录。Electron 通过 `PI_GO_DATA_DIR` 指向用户可写目录：

```
~/Library/Application Support/Pi-Go/
├── .env          ← 配置文件
└── data/
    └── session.jsonl  ← 会话数据
```

### 向后兼容

- 不设置 `PI_GO_DATA_DIR` 时行为完全不变（使用相对路径 `./data`）
- `PI_GO_SESSION_FILE` 显式设置时，优先级高于 DataDir 派生值

---

## 四、可配置 .env 路径（`cmd/pi-agent/main.go`）

### 改动

```go
// 改动前
_ = config.LoadDotEnv(".env")
_ = config.LoadDotEnv(".env.local")

// 改动后
envFile := os.Getenv("PI_GO_ENV_FILE")
if envFile == "" {
    envFile = ".env"
}
_ = config.LoadDotEnv(envFile)
_ = config.LoadDotEnv(envFile + ".local")
```

### 为什么需要

同样因为 `.app` 包只读。Electron 在首次运行时创建默认 `.env` 到 `~/Library/Application Support/Pi-Go/.env`，然后通过 `PI_GO_ENV_FILE` 环境变量告诉 Go 去哪读。

### 向后兼容

不设置 `PI_GO_ENV_FILE` 时回退到 `".env"`，行为与之前完全一致。

---

## 五、新增依赖

| 依赖 | 版本 | 用途 |
|------|------|------|
| `github.com/gorilla/websocket` | v1.5.3 | WebSocket 协议实现 |
| `nhooyr.io/websocket` | v1.8.17 | 间接依赖（被其他包引入） |

`gorilla/websocket` 是 Go 生态最主流的 WebSocket 库，维护活跃，API 简洁。

---

## 审查要点

1. **WebSocket 连接泄漏**：`handleWebSocket` 使用 `defer ws.close()` 和 `defer cancel()` 确保资源清理，但需要确认极端情况（如 goroutine 写入时连接已关闭）不会 panic。
2. **并发写入安全**：`wsConn` 的 `writeJSON()` 通过 mutex 保护，但 `close()` 和 `writeJSON()` 之间是否有竞态？当前实现中 `close()` 也持锁发 Close 帧，应该安全。
3. **模型列表硬编码**：`listModels()` 里的模型列表是写死的。当前只有 deepv provider 三个模型，后续应改为从 provider 动态获取。
4. **CheckOrigin 全放行**：`upgrader.CheckOrigin` 返回 `true`，因为是桌面应用（本机通信），但如果未来支持远程访问需要收紧。
5. **5 分钟超时**：`context.WithTimeout(context.Background(), 5*time.Minute)` 对复杂代码任务可能不够，需要根据实际使用调整。

---

## 审查修复记录

### Fix 1: SwitchModel 携带 provider 参数（High）

**问题**：`SwitchModel()` 只接受 `modelID`，根据 session 当前 provider 决定写 `OpenAIModel`/`DeepVModel`/`AnthropicModel`。如果 provider 是 `openai`，前端选了 `glm-5`（属于 deepv），会写进 `cfg.OpenAIModel = "glm-5"`，provider 不变，请求走错端点。

**修复**：
- `SwitchModel(ctx, modelID string, provider string)` — 新增 `provider` 参数，非空时同时切换 provider 和 model
- `SwitchModelRequest` 新增 `provider` 字段（可选）
- `wsClientMessage` 新增 `provider` 字段
- 前端 `ModelSelector` 从选中 model 对象取 `provider` 一起传
- CLI `/model` 命令传空字符串（保持只改 model 不改 provider 的行为）

### Fix 2: 切换 session 时同步模型状态（Medium）

**问题**：`modelStore` 的 `currentModel` 是全局的，`sessionStore.switchSession()` 切换 session 时没有拉取目标 session 的实际模型信息，导致下拉框显示的模型和实际不一致。

**修复**：
- `sessionStore.switchSession()` 改为 async，切换后调 `GET /sessions/{id}/info` 拉取 provider/model
- 更新 `modelStore.setCurrentModel()` 为实际值
