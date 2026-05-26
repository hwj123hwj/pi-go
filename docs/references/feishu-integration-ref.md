# Pi-Go 飞书 Bot 接入参考文档

> 基于 DeepVcodeClient 的飞书集成经验总结，供 pi-go 对接飞书时复用。
> 参考实现：`DeepVcodeClient/packages/cli/src/services/feishu/` 和 `easyagent/src/easyagent/gateway/feishu_oapi.py`
>
> **最后更新**: 2025-05，补充了卡片回调、用户交互、动态工具、流式消息等实战踩坑经验。

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

> ⚠️ **关键限制：WS 长连接只支持事件订阅，不支持回调订阅。**
>
> 飞书的事件分两类：
> - **事件订阅型**（如 `im.message.receive_v1`）→ WS 和 Webhook 都能收到
> - **回调型**（如 `card.action.trigger`，即卡片按钮点击）→ **只有 Webhook 能收到**
>
> 这意味着如果你用 WS 方式，**交互式卡片的按钮点击事件是收不到的**，无论怎么注册都没用。
> 如果需要卡片交互，要么改用 Webhook 方式，要么用文本选择替代（见下方 §8）。

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

### 更新已发送消息（流式进度）

```bash
# 更新消息内容（只能更新 Bot 自己发的消息）
PATCH https://open.feishu.cn/open-apis/im/v1/messages/{message_id}
Authorization: Bearer {token}
Content-Type: application/json

{"content": "{\"text\":\"更新后的内容\"}"}
```

**使用场景**：LLM 长回复时，先发一条"处理中..."的消息，然后边生成边 PATCH 更新，实现流式效果。

**注意事项**：
- 只能更新 Bot 自己发送的消息
- 更新时需要传入完整的 content JSON（不是增量）
- 飞书 API 有频率限制，建议调用方做 3 秒节流
- 更新后消息的 msg_type 不可变（初始是 text 就一直是 text，post 就一直是 post）
- 更新为 post 格式时需要用 `mdToPostContent` 转换

### 文件上传与发送

飞书发文件分两步：先上传获取 key，再用 key 发消息。图片和文件走不同 API。

```bash
# 1. 上传图片（返回 image_key）
POST https://open.feishu.cn/open-apis/im/v1/images
Authorization: Bearer {token}
Content-Type: multipart/form-data

image_type: message
image: <binary>

# 2. 发送图片消息
POST https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id
{"receive_id": "oc_xxx", "msg_type": "image", "content": "{\"image_key\":\"img_xxx\"}"}

# 3. 上传文件（返回 file_key，需要 parent_node / parent_type）
POST https://open.feishu.cn/open-apis/drive/v1/medias/upload_all
Authorization: Bearer {token}
Content-Type: multipart/form-data

parent_type: ccm_import
parent_node: xxx
file_name: report.pdf
file: <binary>

# 4. 发送文件消息
POST https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id
{"receive_id": "oc_xxx", "msg_type": "file", "content": "{\"file_key\":\"file_xxx\"}"}
```

> **建议**：在 pi-go 中实现为 Agent 工具（如 `send_feishu_file`），让 LLM 可以主动发送生成的文件给用户。

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

### 7. WS 模式下的用户交互（卡片回调不可用）

**问题**：LLM 可能调用 `ask_user_question` 工具向用户提问（例如"选择哪个方案？"）。在 TUI 模式下会弹出交互选项，但飞书模式是非交互的，怎么处理？

**根因**：飞书交互式卡片（Interactive Card）的按钮点击事件是**回调型**，只有 Webhook 方式能收到。WS 长连接无论怎么注册 `card.action.trigger` 都不会触发。

**解决方案：文本选择模式**

替代卡片，发送格式化的选项列表，用户回复序号或选项名称来选择：

```
**请选择项目模板**

> **1**. 单页静态
> **2**. React SPA
> **3**. Next.js SSR

请回复序号或选项名称进行选择。
```

匹配逻辑：
- 数字匹配：用户回复 "1" → 选择第一项
- 文本匹配：用户回复 "单页静态" → 选择对应项（大小写不敏感）
- 超时：默认 60 秒后自动选默认值

**实现要点**：
- 在 gateway 层维护一个 `textChoiceCallback`，在主消息处理器之前拦截
- 用户回复的消息被消费后，不让它进入 Agent 循环（否则 LLM 会当成普通消息处理）
- 匹配成功后 resolve Promise，超时后自动 resolve 默认值

**Go 伪代码**：

```go
// 文本选择等待器
func (g *Gateway) WaitForTextChoice(chatId, title string, buttons []Button, defaultVal string, timeout time.Duration) string {
    // 1. 发送格式化选项列表（用 post 消息类型）
    // 2. 注册 textChoiceCallback
    // 3. 等 channel 或 timeout
    g.textChoiceCallback = func(msg FeishuMessage) bool {
        text := strings.TrimSpace(msg.Text)
        // 数字匹配
        if idx, err := strconv.Atoi(text); err == nil && idx >= 1 && idx <= len(buttons) {
            result <- buttons[idx-1].Value
            return true // 消费掉
        }
        // 文本匹配（大小写不敏感）
        for _, btn := range buttons {
            if strings.EqualFold(text, btn.Label) {
                result <- btn.Value
                return true
            }
        }
        return false // 不消费，走正常流程
    }
    defer func() { g.textChoiceCallback = nil }()
    
    select {
    case val := <-result:
        return val
    case <-time.After(timeout):
        return defaultVal
    }
}
```

> **备选方案**：如果 pi-go 部署时能配 Webhook 回调 URL，可以同时支持真正的卡片交互。但 WS 模式下文本选择是唯一可靠方案。

### 8. 动态工具注册/注销

**问题**：飞书模式需要额外的 Agent 工具（如 `send_feishu_file`），但这些工具在非飞书模式下不应该存在，否则 LLM 会尝试调用一个用不了的工具。

**解决方案**：在飞书连接建立时注册工具，断开时注销：

```go
// 飞书连接建立
registry.Register(&SendFeishuFileTool{gateway: gw, getChatId: activeChatId})

// 飞书断开
registry.Unregister("send_feishu_file")
```

**Go 实现建议**：
- ToolRegistry 需要支持 `Unregister(name string)` 方法
- 飞书相关的工具定义在 `internal/feishu/tools/` 包下
- 工具需要依赖 Gateway 实例（构造时注入）
- 注册/注销后需要通知 Agent 重新加载工具列表

### 9. 流式消息更新（长回复体验优化）

**问题**：LLM 回复可能很长（代码生成等），如果等全部完成再发送，用户等待体验差。

**解决方案**：分段更新已发送的消息

```go
// 1. 收到用户消息后，立即发一条"处理中..."
msgId := gateway.SendMessage(chatId, "⏳ 处理中...")

// 2. LLM 流式响应，每 N 秒 PATCH 更新消息
ticker := time.NewTicker(3 * time.Second)
buffer := ""
for {
    select {
    case delta := <-streamCh:
        buffer += delta
    case <-ticker.C:
        gateway.UpdateMessage(msgId, buffer) // PATCH 更新
    }
}

// 3. 最终完成，用完整内容更新一次
gateway.UpdateMessage(msgId, fullResponse)
```

**注意**：
- 飞书 PATCH API 有频率限制，3 秒节流比较安全
- 消息初始用 `text` 类型发的，更新时也必须用 `text` 格式（msg_type 不可变）
- 如果想支持富文本，初始消息就要用 `post` 类型发送

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
| `gateway.ts` | ~1170 | 网关核心：连接管理、消息收发、reaction、去重、token 管理、卡片回调、文本选择、流式更新、文件上传 |
| `feishu-send-file-tool.ts` | ~136 | Agent 工具：发送本地文件/图片到飞书 |
| `credentials.ts` | ~100 | AES-256 加密存储凭证，支持项目级目录 |
| `registration.ts` | ~230 | 扫码自动建应用（飞书私有协议，可选） |
| `feishuCommand.ts` | ~855 | TUI 命令注册 + Agent 模式消息处理循环 + ask_user_question 文本选择路由 |

> **重点参考**：`gateway.ts` 的 `waitForTextChoice()` 和 `textChoiceCallback` 消息拦截机制，以及 `feishuCommand.ts` 的 `handleAskUserQuestionViaCard()` 函数。

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
6. **WS 模式下别碰卡片交互**：直接用文本选择模式（`waitForTextChoice`），卡片回调在 WS 下收不到
7. **动态工具管理**：飞书专用工具（发文件等）在连接时注册、断开时注销
8. **长回复用流式更新**：先发消息再 PATCH，3 秒节流，用户体验远好于等待全部完成
