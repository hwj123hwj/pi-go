---
status: approved
author: plan-agent
created: 2026-05-26
updated: 2026-05-26
depends-on:
  - docs/references/feishu-integration-ref.md
  - docs/PRODUCT_ROADMAP.md
---

# 飞书 Bot Bridge 执行文档

> 目标：新增独立的 `cmd/pi-feishu-bridge` 进程，通过调用 `pi-agent` 现有 HTTP API
> 将飞书 Bot 与 coding-agent 打通，实现"飞书消息 → Agent 处理 → 飞书回复"的完整链路。
>
> 本文档供执行 agent 直接施工使用。

---

## 1. 整体架构

### 1.1 进程拓扑

```
飞书云  │  WebSocket 长连接（飞书官方 SDK）
        ▼
cmd/pi-feishu-bridge（新增，本次要做的）
        │  HTTP POST /chat/stream（SSE，调已有 pi-agent）
        ▼
cmd/pi-agent -mode serve（已有，不改）
        │  PromptStream → agent loop
        ▼
LLM Provider（DeepV / OpenAI 等）
```

### 1.2 对话路由策略

bridge 在内存中维护一张 `chatKey → sessionID` 映射表：

- **chatKey**：群聊用 `chat_id`，单聊用 `open_id`（两者互斥，取消息中非空的那个）
- **sessionID**：由 `POST /sessions` 创建后由 pi-agent 返回的随机 ID

**为什么不能直接把 `chat_id` 当 `session_id` 传给 pi-agent？**
当前 `app.LoadOrCreateSession(ctx, id)` 的行为是：若传入 ID 在磁盘上不存在，
会忽略该 ID 并生成一个全新的随机 session ID（见 `internal/runtime/agent_session.go`）。
因此 bridge 必须先通过 `POST /sessions` 创建 session 并拿到服务端分配的 ID，
再把它和 chatKey 的对应关系存在本地 map 里。

映射表只存内存，进程重启后丢失 → 自动开启新 session，第一版可接受。

---

## 2. 这次不做什么

- 不改 `internal/` 任何已有代码
- 不改 `cmd/pi-agent/`
- 不实现流式逐字回复到飞书（飞书不支持消息内容动态更新，只需发最终完整回复）
- 不实现扫码自动建 App（手动配置 App ID/Secret）
- 不实现 markdown → 飞书 post 富文本转换（第一版纯文本即可）
- 不实现多 Bot 多项目管理
- 不实现权限控制（白名单等）

---

## 3. 新增文件清单

```
pi-go/
├── cmd/
│   └── pi-feishu-bridge/
│       └── main.go                      ← 新增：入口，读配置，启动 gateway
├── internal/
│   └── feishu/
│       ├── client.go                    ← 新增：飞书 SDK 薄封装（发消息 / reaction）
│       ├── client_test.go               ← 新增：reply fallback / 文本截断等单测
│       ├── gateway.go                   ← 新增：WebSocket 长连接，接收事件
│       ├── gateway_test.go              ← 新增：去重逻辑单测
│       └── handler.go                   ← 新增：消息处理，调 pi-agent HTTP API
├── deploy/
│   └── pi-feishu-bridge.service         ← 新增：systemd 服务文件模板
└── .github/workflows/
    └── deploy.yml                       ← 修改：增加 bridge 的 build + deploy 步骤
```

**注意**：`go.mod` 需要新增飞书官方 Go SDK 依赖（见第 4.1 节），第一版优先复用 SDK 自带的 WebSocket 和 REST 能力，不自己重写 token/cache/reply/reaction 全套 HTTP。

---

## 4. 实现细节

### 4.1 依赖

飞书官方 Go SDK：

```bash
go get github.com/larksuite/oapi-sdk-go/v3@latest
```

只使用以下模块：
- `larkcore` — 基础配置
- `larkim` — 消息收发
- `larkws` — WebSocket 长连接

### 4.2 `internal/feishu/client.go`

封装飞书 **REST 调用**，但第一版不自己写裸 HTTP，而是**薄封装飞书 SDK 的 REST 能力**。

职责：

1. **持有 SDK client**
   - 内部持有 `*lark.Client`
   - token 获取、缓存、刷新交给 SDK 自己处理

2. **发消息**
   - `ReplyMessage(messageID, text string) error` — 回复指定消息（带引用）
   - `SendMessage(chatID, text string) error` — 直接发到群/会话（不引用）
   - `ReplyMessage` 失败且错误码为 `10003`（消息超 30 天）时，自动 fallback 到 `SendMessage`

3. **Emoji Reaction**
   - `AddReaction(messageID string) (reactionID string, err error)` — 添加 emoji reaction
   - `RemoveReaction(messageID, reactionID string) error` — 删除 emoji reaction
   - 两个方法失败只记 warn 日志，不阻断主流程

4. **文本长度限制**
   - 飞书单条文本消息有长度上限
   - 第一版统一在发送前截断到一个保守值（例如 25000 字符）并追加 `\n\n（已截断）`

实现要点：

```go
type Client struct {
    sdk *lark.Client
}

func NewClient(appID, appSecret string) *Client
```

**说明**：
- `internal/feishu/client.go` 负责**主动 REST 调用**
- `larkws.NewClient(...)` 负责 **WebSocket 收事件**
- 两者都复用同一套 App ID / App Secret，但各自通过 SDK 管理认证，不共享我们自己写的 token 状态

### 4.3 `internal/feishu/gateway.go`

用飞书 SDK 建立 WebSocket 长连接，接收 `im.message.receive_v1` 事件。

核心职责：

1. **事件过滤**
   - 只处理 `msg_type == "text"` 的消息
   - 忽略 Bot 自己发出的消息（检查 `sender.sender_type != "user"`）

2. **消息去重**（必须实现，防断线重连导致重复处理）
   - 用 `map[string]struct{}` 记录已处理的 `message_id`
   - map 条目数超过 1000 时，随机删除直到降到 900 条（`map` 无顺序，不做"删最旧"）
   - 用 `sync.Mutex` 保护

3. **文本提取**
   - 飞书事件里的 `message.content` 是 JSON 字符串，不是裸文本
   - 必须先 `json.Unmarshal([]byte(rawContent), &struct{ Text string \`json:"text"\` })`
   - 群聊 `@Bot` 文本会带 `<at user_id="...">...</at>` 标签，必须剥掉再交给 agent
   - 建议用正则去掉 `<at[^>]*>.*?</at>`，然后 `strings.TrimSpace`

4. **把消息交给 Handler**
   - 提取 `chat_id`、`message_id`、`open_id`、清洗后的纯文本
   - 计算 `chatKey`：
     - 群聊：`chat_id`
     - 单聊：`open_id`
   - 异步调用 `handler.Handle()`（每条消息启动一个 goroutine）

SDK 形态说明：

- WebSocket 长连接由 `larkws.NewClient(appID, appSecret, larkws.WithEventHandler(dispatcher))` 建立
- 事件处理通过 `dispatcher.OnP2MessageReceiveV1(...)` 注册
- `Gateway` 不需要持有我们自己的 `*feishu.Client`

```go
type Gateway struct {
    appID     string
    appSecret string
    handler *Handler
    seen    map[string]struct{}
    mu      sync.Mutex
}

func NewGateway(appID, appSecret string, handler *Handler) *Gateway
func (g *Gateway) Start(ctx context.Context) error  // 阻塞，直到 ctx 取消
```

### 4.4 `internal/feishu/handler.go`

消息处理核心，完整流程：

```
1. AddReaction(messageID) → 显示 🤔（失败不中断）
2. 判断是否是斜杠命令（以 "/" 开头）
   ├─ /new      → POST /sessions 创建新 session，拿到新 sessionID，
   │              更新本地 chatKey→sessionID 映射，回复"已开启新对话"
   ├─ /help     → 回复帮助文本（本地硬编码，不走 agent）
   └─ 其他 /xxx → 回复"未知命令，输入 /help 查看可用命令"
3. 非斜杠命令 → 调用 pi-agent HTTP API
   a. 用 chatKey 查本地 sessions map
      ├─ 有映射 → 用已有 sessionID
      └─ 无映射 → POST /sessions 创建新 session，把返回的 sessionID 存入 map
   b. POST /chat/stream，body: {"prompt": text, "session_id": sessionID}
   c. 消费 SSE 流，收集所有 text_delta 拼成完整文本
   d. 遇到 event:done → 结束
   e. 遇到 event:error → 记录错误，回复"处理出错，请重试"
4. ReplyMessage(messageID, fullText)（或 fallback SendMessage）
5. RemoveReaction(messageID, reactionID)（失败不中断）
```

SSE 消费实现要点：

```go
// 用 bufio.Scanner 逐行读取 SSE 响应体
// SSE 事件以空行分隔：
//   event: text_delta
//   data: {"type":"text_delta","text_delta":"hello"}
//
//   <empty line>
//
//   event: done
//   data: {...}
//
// 第一版可以假设 pi-agent 的 data 行始终是单行 JSON，不跨多行。
// 解析时维护一个小状态机：
//   currentEvent string
//   currentData  string
// 遇到空行时提交一条完整事件。
```

`session` 映射管理：

```go
type Handler struct {
    piAgentURL string            // 如 "http://127.0.0.1:8080"
    client     *Client
    sessions   map[string]string // chatKey → sessionID（pi-agent 分配的随机 ID）
    mu         sync.Mutex
    httpClient *http.Client
}

func NewHandler(piAgentURL string, client *Client) *Handler
func (h *Handler) Handle(ctx context.Context, chatKey, messageID, text string)
// chatKey = chat_id（群聊）或 open_id（单聊），取消息中非空的那个
```

**映射表说明**：
- `sessions` map 存在内存中，key 是飞书的 chatKey，value 是 `POST /sessions` 返回的 pi-agent session ID
- 进程重启后映射丢失，重启后会自动通过 `POST /sessions` 创建新 session，第一版可接受
- `sessions` map 的所有读写都必须在 `mu` 保护下进行
- 像 `POST /sessions` 这类慢操作应在锁外执行：先调 API 拿到 `sessionID`，再加锁写 map，避免长时间持锁

### 4.5 `cmd/pi-feishu-bridge/main.go`

```go
package main

// 读取环境变量：
//   FEISHU_APP_ID       （必须）
//   FEISHU_APP_SECRET   （必须）
//   PI_AGENT_URL        （可选，默认 "http://127.0.0.1:8080"）
//
// 启动顺序：
// 1. 读环境变量，缺少必需项则 Fatal
// 2. 创建 feishu.Client
// 3. 创建 feishu.Handler
// 4. 创建 feishu.Gateway
// 5. gateway.Start(ctx)  阻塞运行
// 6. 监听 SIGINT/SIGTERM，优雅退出
```

### 4.6 `deploy/pi-feishu-bridge.service`

参考 `deploy/pi-go.service` 的模板格式：

```ini
[Unit]
Description=Pi-Go Feishu Bridge
After=network.target pi-go.service
Wants=pi-go.service

[Service]
Type=simple
User=root
WorkingDirectory=__DEPLOY_PATH__/current
EnvironmentFile=__DEPLOY_PATH__/shared/.env
ExecStart=__DEPLOY_PATH__/current/pi-feishu-bridge
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

注意：`EnvironmentFile` 和 `pi-go.service` 共用同一个 `.env` 文件，
飞书相关变量追加进去即可。

### 4.7 修改 `.github/workflows/deploy.yml`

在现有 workflow 的基础上新增以下内容：

**在 "Build linux binary" 步骤之后增加**：

```yaml
- name: Build feishu-bridge linux binary
  run: |
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
      go build -o "dist/pi-feishu-bridge" ./cmd/pi-feishu-bridge
```

**在 "Package release" 步骤中，`cp README.md` 之后增加**：

```bash
cp "dist/pi-feishu-bridge" "dist/${RELEASE_NAME}/pi-feishu-bridge"
```

**在 "Prepare deployment assets" 步骤中增加**：

```bash
sed "s#__DEPLOY_PATH__#${{ secrets.DEPLOY_PATH }}#g" \
  deploy/pi-feishu-bridge.service > dist/deploy/pi-feishu-bridge.service
```

**在 "Upload release bundle" 步骤的 scp 命令中增加**：

```
"dist/deploy/pi-feishu-bridge.service" \
```

**在 "Install release on server" 步骤的远程命令中追加**（在 `systemctl restart pi-go` 之后）：

```bash
if [ -f /tmp/pi-feishu-bridge.service ]; then
  cp /tmp/pi-feishu-bridge.service /etc/systemd/system/pi-feishu-bridge.service
  systemctl daemon-reload
  systemctl enable pi-feishu-bridge.service
  systemctl restart pi-feishu-bridge.service || true
fi
```

**新增 GitHub Secrets**（在文档中注明，不在 yml 中硬编码）：

| Secret 名 | 说明 |
|---|---|
| `FEISHU_APP_ID` | 飞书应用 App ID |
| `FEISHU_APP_SECRET` | 飞书应用 App Secret |

这两个变量通过 `.env` 文件下发（追加到 `PI_GO_ENV` Secret 内容中）。

---

## 5. 环境变量完整清单

部署时 `.env` 文件（即 GitHub Secret `PI_GO_ENV` 的内容）需包含：

```dotenv
# 已有（pi-agent 相关）
PI_GO_PROVIDER=deepv
DEEPV_ENABLED=true
DEEPV_MODEL=deepseek-v4-flash
PI_GO_ENABLE_BASH=true
PI_GO_HOST=127.0.0.1
PI_GO_PORT=8080

# 新增（pi-feishu-bridge 相关）
FEISHU_APP_ID=cli_xxxxxxxxxx
FEISHU_APP_SECRET=xxxxxxxxxxxxxxxxxx
PI_AGENT_URL=http://127.0.0.1:8080
```

---

## 6. 错误处理规范

| 场景 | 处理方式 |
|---|---|
| `FEISHU_APP_ID` 或 `FEISHU_APP_SECRET` 未配置 | 启动时 `log.Fatal` |
| `PI_AGENT_URL` 对应的服务不可达 | 不影响启动，每次请求失败时回复"服务暂不可用" |
| agent busy（HTTP 409） | 同一 `chatKey` 下若上一条仍在处理，回复"上一条消息还在处理中，请稍后再试" |
| SSE 流中断 | 如果已收集到部分文本则发出；否则回复"处理出错，请重试" |
| AddReaction / RemoveReaction 失败 | 只记 warn 日志，不中断主流程 |
| ReplyMessage 返回 code 10003 | fallback 调 SendMessage，记 info 日志 |
| 飞书回复文本过长 | 截断到 25000 字符并追加 `（已截断）` |

---

## 7. 测试要求

### 7.1 必须有测试的部分

**`client_test.go`**：
- `ReplyMessage` 遇到错误码 `10003` 时 fallback 到 `SendMessage`
- 过长文本会被安全截断
- reaction 失败时返回错误但不 panic

**`gateway_test.go`**：
- 同一 message_id 第二次到达被去重（不调 handler）
- 去重 map 超过 1000 条时触发清理（无序删至 900 条）
- 文本 JSON 能正确解析出 `text`
- `<at ...>...</at>` 标签会被剥掉

### 7.2 不强求的测试

- `handler.go` 的 HTTP 调用逻辑（依赖外部服务，用 mock httptest 过于繁琐）
- 实际飞书 API 调用

### 7.3 运行方式

```bash
go test ./internal/feishu/... -race
go vet ./...
go build ./cmd/pi-feishu-bridge
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/pi-feishu-bridge
```

---

## 8. 验收标准

1. `go build ./cmd/pi-feishu-bridge` 编译通过
2. `go test ./internal/feishu/... -race` 通过
3. `go vet ./...` 通过
4. 在本地手动验证（需要真实飞书 App）：
   - 飞书群 @Bot 发消息 → Bot 显示 🤔 → 收到 agent 回复 → 🤔 消失
   - 发 `/new` → 回复"已开启新对话"
   - 发 `/help` → 回复帮助文本
   - 发 `/unknown` → 回复"未知命令..."
   - **同一群聊**里连续发两条消息，第一条未完成时第二条收到 busy 提示
5. 本地 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/pi-feishu-bridge` 通过
6. deploy.yml 在 CI 中能构建出 `pi-feishu-bridge` 二进制（合并后或通过手动触发验证）

---

## 9. 飞书开放平台配置（人工操作，不在代码里）

执行前需要先在飞书开放平台完成：

1. 创建应用 → 获取 `App ID` 和 `App Secret`
2. **开启 WebSocket 事件订阅**（不是 Webhook）：
   - 事件列表勾选 `im.message.receive_v1`
3. **权限配置**（申请并开通）：
   - `im:message` — 读取消息
   - `im:message:send_as_bot` — 发送消息
   - `im:reaction:write` — emoji 反应（可选，没有则跳过 reaction 功能）
4. 发布应用，开启 Bot 能力
5. 将 Bot 拉入测试群

---

## 10. 执行顺序建议

1. `go get github.com/larksuite/oapi-sdk-go/v3`，确认 `go.mod` 更新
2. 实现 `internal/feishu/client.go`（基于 SDK 的 REST 薄封装）+ `client_test.go`
3. 实现 `internal/feishu/gateway.go`（基于 SDK 的 WebSocket client）+ `gateway_test.go`
4. 实现 `internal/feishu/handler.go`
5. 实现 `cmd/pi-feishu-bridge/main.go`
6. `go build ./cmd/pi-feishu-bridge` 确保编译通过
7. 新增 `deploy/pi-feishu-bridge.service`
8. 修改 `.github/workflows/deploy.yml`
9. 本地手动端到端验证（需要真实飞书 App 和运行中的 pi-agent）
10. `go vet ./...` 全量检查

---

## 11. 一句话目标

新增一个独立的 `pi-feishu-bridge` 进程，通过调用 `pi-agent` 现有的 `/chat/stream` SSE 接口，
把飞书消息路由到 coding-agent，实现"飞书 → Agent → 飞书"的完整闭环，
不改动任何已有代码。
