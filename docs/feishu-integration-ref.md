# Pi-Go 飞书 Bot 接入参考文档

> 基于 DeepVcodeClient 的飞书集成经验总结，供 pi-go 对接飞书时复用。
> 参考实现：`DeepVcodeClient/packages/cli/src/services/feishu/` 和 `easyagent/src/easyagent/gateway/feishu_oapi.py`

---

## 核心架构

飞书 Bot 的核心交互模式非常简单：

```
用户发消息 → 飞书事件推送
  → Bot 解析消息内容
  → 调 LLM Agent（带工具执行能力）
  → 得到回复 → 通过飞书 REST API 回复用户
```

### 两种事件接收方式

| 方式 | 说明 | 适用场景 |
|------|------|----------|
| **WebSocket 长连接** | 通过飞书 SDK 建立 WS，自动接收事件推送 | 服务端长驻进程，推荐 |
| **Webhook 回调** | 配置飞书开放平台事件回调 URL，POST 接收 | 已有 HTTP 服务的场景 |

对 pi-go 来说，**推荐用 WebSocket 方式**，因为 pi-go 本身是长驻进程，和 WS 长连接生命周期匹配。也可以单独启动一个桥接进程通过 pi-go 的 HTTP API 调用。

---

## 飞书 Open API 速查

### tenant_access_token

```bash
# 获取（2 小时有效期，需缓存刷新）
POST https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal
Content-Type: application/json

{"app_id": "cli_xxxxx", "app_secret": "xxxxxxxxxxxx"}
```

### 发消息

```bash
# 回复消息（推荐，会带引用）
POST https://open.feishu.cn/open-apis/im/v1/messages/{message_id}/reply
Authorization: Bearer {token}
Content-Type: application/json

{"msg_type": "text", "content": "{\"text\":\"hello\"}"}

# 直接发消息（不引用）
POST https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id
Authorization: Bearer {token}
Content-Type: application/json

{"receive_id": "oc_xxxxx", "msg_type": "text", "content": "{\"text\":\"hello\"}"}
```

> **注意**：reply 模式如果原消息超过 30 天会返回 `code: 10003`，需要 fallback 到直接发送。

### Emoji Reaction（处理中状态提示）

```bash
# 添加（返回 reaction_id）
POST https://open.feishu.cn/open-apis/im/v1/messages/{message_id}/reactions
Authorization: Bearer {token}
Content-Type: application/json

{"reaction_type": {"emoji_type": "THINKING"}}

# 删除
DELETE https://open.feishu.cn/open-apis/im/v1/messages/{message_id}/reactions/{reaction_id}
Authorization: Bearer {token}
```

常用 `emoji_type`（飞书 emoji 枚举值）：
- `THINKING` — 🤔 思考中
- `OK` — 好的
- `MUSCLE` — 💪 加油
- `FIRE` — 🔥 火热
- `LOVE` — ❤️ 爱
- `GOGOGO` — 冲冲冲

**最佳实践**：消息进入处理时添加 `THINKING`，处理完成后移除。这样飞书用户能看到"Bot 正在处理中"的视觉反馈。

### Markdown 消息（富文本）

飞书不支持直接发 markdown，需要用 `post` 消息类型：

```json
{
  "msg_type": "post",
  "content": "{\"zh_cn\": {\"title\": \"\", \"content\": [[{\"tag\": \"text\", \"text\": \"你好\"}, {\"tag\": \"text\", \"text\": \"世界\", \"style\": [\"bold\"]}]]}}"
}
```

需要实现 markdown → post 格式的转换器（支持粗体、标题、列表即可）。

### Bot 信息查询

```bash
# 验证凭证是否有效（setup 流程结束后调用）
GET https://open.feishu.cn/open-apis/bot/v3/info
Authorization: Bearer {token}
```

---

## 实现关键点（踩坑记录）

### 1. 消息去重

**问题**：飞书 WebSocket 事件可能重复推送（尤其在重连后），导致同一条消息被处理多次。

**解决方案**：两层去重

| 层级 | 方案 | 说明 |
|------|------|------|
| 第一层 | messageId Set | 记录已处理的消息 ID，最多保留 N 条（LRU 淘汰） |
| 第二层 | 内容+时间窗口 | 相同 chatId+text 在 5 秒内再次收到视为重复 |

**Go 实现要点**：可以用 `sync.Map` 或 `map[string]struct{}` + `sync.Mutex`，注意 LRU 淘汰不一定要精确，定期清理即可。

### 2. tenant_token 缓存与刷新

token 2 小时过期，需要：

```
1. 启动时获取一次
2. 缓存到内存，记录过期时间
3. 每次使用前检查：是否在 1 分钟内过期？
   - 是 → 刷新
   - 否 → 直接用
```

**注意并发问题**：多个 goroutine 可能同时发送消息，需要用 `sync.RWMutex` 保护 token 的读写。

### 3. message reply 的 10003 错误

如果回复的消息已超过 30 天，飞书 API 返回 `code: 10003`，reply 不可用。处理方式：

```go
func sendReply(chatId, messageId, text string) {
    err := replyMessage(messageId, text) // reply 模式
    if isCode10003(err) {                // 消息太旧
        err = sendDirectMessage(chatId, text) // 直接发
    }
}
```

### 4. Emoji Reaction 失败处理

**问题**：某些场景下（如群聊 @bot、消息类型非 text），添加 reaction 可能失败。

**解决方案**：
- `addReaction` 失败不中断流程，记一条 warn 日志即可
- `removeReaction` 放在 `finally` / `defer` 中执行
- reaction_id 为空时跳过删除

### 5. 斜杠命令拦截

飞书的 `/new`、`/compress`、`/help` 等命令必须在 **本地** 拦截处理，不能发给 LLM。否则 LLM 可能会产生幻觉回复或执行错误操作。

拦截流程：

```
收到消息
  ├─ 以 / 开头 → 检查是否是已知命令
  │   ├─ 是 → 本地处理，返回结果
  │   └─ 否 → 提示"未知命令，输入 /help 查看可用命令"
  └─ 非 / 开头 → 正常走 LLM Agent
```

### 6. Agent 循环的 turn 限制

必须限制最大对话轮数（推荐 20 轮），防止工具调用死循环。每次循环：

```go
for turn := 0; turn < MAX_TURNS; turn++ {
    // 1. 调 LLM，接收流式响应
    // 2. 如果有 tool call → 执行工具 → 继续循环
    // 3. 如果没有 tool call → 返回最终文本
}
// 达到最大轮数 → 返回"已达到最大处理轮数限制"
```

---

## pi-go 推荐实现方案

### 方案一：内部集成（内置 module）

在 pi-go 内部新增 `internal/feishu/` 包，serve 模式下同时启动飞书网关。

```
pi-go serve 模式
  ├─ HTTP Server (:8080) — 已有
  └─ Feishu Gateway — 新增
       ├─ WSClient 连接飞书
       ├─ onMessage → 调 AgentSession.Prompt()
       └─ 发回复 → 走 REST API
```

**适合**：单 Bot、简单部署。

### 方案二：独立桥接进程（推荐）

新增 `cmd/pi-feishu-bridge/main.go`，作为独立进程通过 pi-go HTTP API 对接。

```
pi-go serve (:8080)        ←→   pi-feishu-bridge    ←→   飞书
  (HTTP SSE /chat/stream)        (feishu gateway)        (WS 长连接)
```

**优势**：
- 职责分离，一个 bridge 可对接多个 Bot（不同 App ID）
- bridge 出问题不影响 pi-go 主进程
- 支持多项目：每个项目启动一个 bridge，连接不同的 Bot

### API 调用方式

```
# 同步（简单场景）
POST /chat
{"prompt": "hello", "session_id": "sess_xxx"}

# 流式（带 tool call 的复杂场景）
POST /chat/stream
{"prompt": "hello", "session_id": "sess_xxx"}
→ SSE: event: text_delta, data: "..."
→ SSE: event: tool_start, data: "..."
→ SSE: event: done, data: "..."

# 创建新 session（对应飞书 /new 命令）
POST /sessions
```

---

## 参考实现文件归档

### DeepVcodeClient（TypeScript，可直接翻译为 Go）

| 文件 | 行数 | 核心功能 |
|------|------|----------|
| `gateway.ts` | 480 | 网关核心：连接管理、消息收发、reaction、去重、token 管理 |
| `credentials.ts` | 100 | AES-256 加密存储凭证，支持项目级目录 |
| `registration.ts` | 230 | 扫码自动建应用（飞书私有协议，可选） |
| `feishuCommand.ts` | 587 | TUI 命令注册 + Agent 模式消息处理循环 |

### easyagent（Python，参考 emoji pool 和 gateway 架构）

| 文件 | 核心功能 |
|------|----------|
| `feishu_oapi.py` | 完整 OAPI Gateway，支持 emoji pool、重连、消息去重 |
| `feishu_im_tool.py` | 飞书 IM 工具（LLM 可直接调用） |

---

## 环境与权限

### 飞书开放平台配置

1. 创建应用 → 获取 App ID / App Secret
2. 开启 **WebSocket 事件订阅**（或配置回调 URL）
3. 权限配置：
   - `im:message` — 收发消息
   - `im:message:send_as_bot` — 以 Bot 身份发消息
   - `im:reaction` — emoji 反应
4. 发布应用 → 开启 Bot 能力

### 扫码自动建应用（开发调试用）

飞书开放平台有一个私有注册协议，可以通过扫码自动创建 PersonalAgent 应用并获取 App ID/Secret，三步流程：

1. `init` — 检测环境是否支持
2. `begin` — 获取 device_code 和二维码 URL
3. `poll` — 轮询等待用户扫码

> ⚠️ 此协议未在公开文档中说明，飞书可能随时更改。建议线上环境直接手动创建应用。

---

## 开发建议

1. **先跑通最简单的消息收发**：用 curl 验证 API 可用，再写代码
2. **先实现 text 消息**，再支持 markdown/post 富文本
3. **Gateway 优先**：连接管理、token 刷新、去重是基础，gateway 稳定了再实现 Agent 模式
4. **emoji reaction 是锦上添花**：可以在 Phase 1 不需要，但用户体验提升明显
5. **斜杠命令在 gateway 层拦截**：不要等发到 LLM 再处理
