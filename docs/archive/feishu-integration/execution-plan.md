---
status: draft
author: plan-agent
created: 2026-05-28
updated: 2026-05-28
depends-on:
  - docs/dev/feishu/proposal.md
  - docs/dev/feishu/review.md
---

# 飞书 Bot 完善执行文档

> 目标：在已有骨架实现上补齐消息基础和体验升级的缺失能力。
> 本文档供执行 agent 直接施工使用。

---

## 前置状态

- 骨架实现已完成：`internal/feishu/{client,gateway,handler,markdown}.go`
- review.md 中 B2（/compact 端点）已解决
- review.md 中 B1（ask_user_question 不存在）**仍然开放** — WS 模式不支持卡片回调，
  只能用文本选择作为降级方案，但 agent 侧仍缺少触发交互的工具入口
- 凭证加密 + 授权白名单不做（个人使用场景，环境变量配置即可）

---

## 关键平台限制

> **WS 长连接不支持 `card.action.trigger` 回调。**
>
> 飞书事件分两类：
> - 事件订阅型（`im.message.receive_v1`）→ WS 和 Webhook 都能收到
> - 回调型（`card.action.trigger`，卡片按钮点击）→ **只有 Webhook 能收到**
>
> DeepVcodeClient 自身也确认了这一点（`gateway.ts:1783`）：
> "WebSocket 长连接不支持卡片回调，直接使用文本选择模式"
>
> **影响**：交互式卡片按钮在飞书端可以渲染，但点击事件不会推送到 bridge。
> 所有需要用户选择的场景必须用**文本选择**（回复序号/选项名称）替代。
>
> 本文档的交互设计全部基于此限制。

---

## Phase 1：消息基础

### 1.1 修改 `gateway.go` — 入站图片/post 消息

**做什么**：支持 image 和 post 类型的入站消息

**修改点**（`handleEvent` 方法）：

1. 移除 `if *msg.MessageType != "text" { return }` 过滤
2. 新增 `image` 类型处理：
   - 解析 `content.image_key`
   - 调用新增的 `downloadImageResource` 下载到 `/tmp/`
   - 构造 `![image](localPath)` 作为 text
3. 新增 `post` 类型处理：
   - 解析 `content[locale].content` 二维数组
   - 提取 text / a / at / img 元素，转为 Markdown
   - img 元素同样调 `downloadImageResource`
4. 新增 `downloadImageResource(messageID, imageKey) (string, error)` 方法

**downloadImageResource API 路径**：

```
GET /open-apis/im/v1/messages/{messageId}/resources/{imageKey}?type=image
Authorization: Bearer {tenant_access_token}
```

返回图片二进制流，保存到 `os.TempDir()/feishu-image-{imageKey}.png`。

**参考**：`gateway.ts:385-408`（`downloadImageResource`）+ `gateway.ts:454-530`（消息解析）

**验证**：
- 在飞书端发图片消息 → 日志显示 "downloaded to /tmp/..."
- 发 post 富文本消息 → Agent 收到正确的 Markdown 文本

### 1.2 修改 `gateway.go` — 内容去重

**修改点**：

```go
type Gateway struct {
    // ... existing fields
    recentContents map[string]int64 // "chatID:text" → 首次处理时间戳
    dedupWindowMs  int64            // 5000
}
```

在 `handleEvent` 中，messageID 去重之后增加：
- `contentKey = chatID + ":" + text`
- 5 秒内相同 contentKey → 跳过
- 清理时机：**在每次去重检查时顺带清理**（遍历 map，删除超过 `2 * dedupWindowMs` 的条目），
  不用独立 goroutine

**参考**：`gateway.ts:300-303` + `gateway.ts:576-585`

**验证**：
- 单测：发送相同 contentKey 两次（间隔 < 5 秒）→ 第二次被跳过
- 单测：发送相同 contentKey 两次（间隔 > 5 秒）→ 两次都处理

---

## Phase 2：体验升级

### 前置：`client.go` — getTenantToken 缓存

**问题**：当前 `getTenantToken()`（L311-340）每次调用都请求飞书 API。CardKit 流式场景下
一次 SSE 可能触发几十次 API 调用，会被限流。

**修改**：在 `Client` 结构体中增加 token 缓存字段：

```go
type Client struct {
    sdk             *lark.Client
    appID           string
    appSecret       string
    cachedToken     string    // 新增
    tokenExpiresAt  time.Time // 新增
    tokenMu         sync.RWMutex
}
```

`getTenantToken` 逻辑：先检查缓存是否有效（过期前 60 秒刷新），有效则直接返回。
飞书 token 有效期 2 小时。

**参考**：`gateway.ts:361-380`（带缓存的 `getTenantToken`）

### 2.1 新增 `internal/feishu/cardkit.go`

**做什么**：CardKit 2.0 流式卡片 API

**核心结构**：

```go
// StreamingCardHandle 流式卡片句柄
type StreamingCardHandle struct {
    MessageID string
    CardID    string
    client    *Client
    sequence  int
    mu        sync.Mutex
    lastPush  time.Time   // 节流：上次推送时间
    minInterval time.Duration // 节流：最小推送间隔 1500ms
}

func (h *StreamingCardHandle) PushContent(content string) error
func (h *StreamingCardHandle) PushFooter(metrics FooterMetrics) error
func (h *StreamingCardHandle) Finalize(content string, metrics *FooterMetrics) error

type FooterMetrics struct {
    Status            string
    ElapsedMs         int64
    Model             string
    InputTokens       int
    OutputTokens      int
    CacheReadTokens   int
    CacheHitRate      float64
    ContextPercentage float64
}
```

**节流策略**：`PushContent` 内置 1500ms 最小间隔。如果距上次推送不足 1500ms，
跳过本次推送（内容会在下次推送时包含最新累计值）。`PushFooter` 独立节流。

**卡片分段保护**：单卡片内容超过 8500 字符时，自动截断并追加"（内容过长，已截断）"。
飞书 CardKit 对单卡片 JSON 有大小限制，超长内容会导致 API 返回错误。

**CardKit 底层 API**（直接 fetch `/open-apis/cardkit/v1/...`）：

```go
func (c *Client) CreateCardKitCard(card map[string]any) (cardID string, err error)
func (c *Client) SendCardKitMessage(chatID, cardID, replyTo string) (messageID string, err error)
func (c *Client) StreamCardKitElement(cardID, elementID, content string, sequence int) error
func (c *Client) SetCardKitStreamingMode(cardID string, enabled bool, sequence int) error
func (c *Client) UpdateCardKitCard(cardID string, card map[string]any, sequence int) error
```

**风险**：`SendCardKitMessage` 需要通过 `im.message.create` 发送 `msg_type=interactive`，
content 引用 `card_id`（格式 `{"type":"card","data":{"card_id":"xxx"}}`）。
飞书 Go SDK 的 `larkim` 模块是否支持这种引用模式需要验证。
**降级方案**：如果 Go SDK 不支持，改用 raw HTTP 请求（`doUpload` 同模式）。

**卡片构建函数**：

```go
func BuildStreamingCard(initialContent, initialFooter string) map[string]any
func BuildFinalCard(content string, metrics *FooterMetrics) map[string]any
func RenderFooterMarkdown(metrics FooterMetrics) string
```

**关键常量**：

```go
const (
    CardKitStreamingElementID = "streaming_content"
    CardKitFooterElementID    = "footer_content"
    CardKitLoadingElementID   = "loading_icon"
    CardKitLoadingImgKey      = "img_v3_02vb_496bec09-4b43-4773-ad6b-0cdd103cd2bg"
    CardKitMaxContentChars    = 8500  // 单卡片内容字符上限
)
```

**参考**：`gateway.ts:66-275`（常量+构建）+ `gateway.ts:1463-1584`（CardKit API）

**验证**：
- 单测：`BuildStreamingCard` 输出符合 CardKit 2.0 schema
- 单测：`RenderFooterMarkdown` 各种 metrics 组合
- 单测：超过 8500 字符时自动截断
- 集成：发消息 → 飞书端看到流式打字机效果 → 终态带 footer

### 2.2 新增 `internal/feishu/cardkit_test.go`

测试用例：
- `TestBuildStreamingCard_Schema`：验证 schema 2.0、streaming_mode、element_id
- `TestBuildFinalCard`：验证终态卡片结构
- `TestRenderFooterMarkdown`：状态/耗时/token/cache 各种组合
- `TestRenderFooterMarkdown_Error`：错误状态红色标记
- `TestContentTruncation`：超过 8500 字符截断

### 2.3 修改 `handler.go` — 流式卡片替代 text 更新

**修改点**（`handleAgentMessage` 方法）：

```go
// 之前：
botMsgID, _ := h.client.SendMessage(ctx, chatKey, "⏳ 处理中...", "")
// ... 3 秒 ticker UpdateMessage ...
h.client.SendMarkdown(ctx, chatKey, fullText, messageID)

// 之后：
handle, _ := h.client.SendStreamingCard(ctx, chatKey, "", messageID)
// ... SSE 消费中（每个 text_delta 事件）：
handle.PushContent(buf.String())  // 内置 1500ms 节流
// ... done：
handle.Finalize(fullText, &footerMetrics)
```

**降级方案**：如果 `SendStreamingCard` 返回 `messageID == nil`（CardKit 创建失败），
回退到现有的 PATCH text 方案（SendMessage + ticker UpdateMessage）。

需要从 SSE 事件中提取 token 用量信息（如果 server 提供的话）。

**参考**：`feishuCommand.ts` 中的流式卡片使用模式

### 2.4 完善 `gateway.go` — 文本选择机制（WS 模式下唯一的交互方式）

> **WS 模式不支持卡片按钮回调**，所有用户选择场景通过文本匹配实现。

**修改点**：

完善 `textChoiceCallback` 的实际使用：

```go
func (g *Gateway) WaitForTextChoice(
    chatID string,
    title string,
    content string,     // 选项的详细描述/问题解析
    buttons []string,   // 选项 label 列表
    defaultVal string,  // 超时默认值
    timeout time.Duration,
) (string, error)
```

**实现逻辑**：
1. 发送格式化选项列表（Markdown post 格式）：
   ```
   **请选择**

   {content}

   > **1**. 选项A
   > **2**. 选项B
   > **3**. 选项C

   请回复序号或选项名称进行选择。
   ```
2. 注册 `textChoiceCallback`，拦截下一条来自同一 chatID 的消息
3. 匹配逻辑：数字匹配（"1" → 第一项）+ 文本匹配（大小写不敏感，匹配 button label）
4. 60 秒超时返回 `defaultVal`
5. 并发安全：同一 chatID 只允许一个等待器（新的覆盖旧的，旧的 resolve 为 defaultVal）

**参考**：`gateway.ts:1797-1824`（`waitForTextChoice` 实现）+ `gateway.ts:1783-1785`（WS 降级说明）

**验证**：
- 单测：数字匹配 + 文本匹配 + 超时默认值
- 集成：Agent 需要用户选择 → 飞书显示选项列表 → 用户回复序号 → 选择正确传递

---

## Phase 3：运维完善

### 3.1 新增 `internal/feishu/markdown_style.go`

**做什么**：Markdown 样式优化，适配飞书卡片显示

```go
func OptimizeMarkdownStyle(text string, cardVersion int) string
func StripInvalidImageKeys(text string) string
```

处理逻辑：
1. 提取代码块（占位符保护）
2. 标题降级：H1→H4，H2~H6→H5（仅当原文有 H1~H3 时）
3. 连续标题间增加 `<br>` 段落间距
4. 表格前后 `<br>` + 空行规范化
5. 还原代码块，前后 `<br>`
6. 压缩多余空行（3+ → 2）
7. `StripInvalidImageKeys`：移除非 `img_` 前缀的 `![]()` （防止 CardKit 200570 错误）

**参考**：`DeepVcodeClient/packages/cli/src/services/feishu/markdown-style.ts`（110 行）

**修改 `markdown.go`**：在 `mdToPostContent` 输出后调用 `OptimizeMarkdownStyle`

### 3.2 新增 `internal/feishu/registration.go`

**做什么**：扫码自动建飞书应用

```go
func InitRegistration(domain string) error
func BeginRegistration(domain string) (*BeginResult, error)
func PollRegistration(deviceCode string, interval, expireIn int, domain string) (*PollResult, error)
func ProbeCredentials(appID, appSecret, domain string) (*ProbeResult, error)

type BeginResult struct {
    DeviceCode string
    QRUrl      string
    UserCode   string
    Interval   int
    ExpireIn   int
}

type PollResult struct {
    AppID     string
    AppSecret string
    Domain    string
    OpenID    string
}

type ProbeResult struct {
    BotName       string
    BotOpenID     string
    GrantedScopes []string
}
```

飞书私有端点：`accounts.feishu.cn/oauth/v1/app/registration`

**参考**：`DeepVcodeClient/packages/cli/src/services/feishu/registration.ts`（266 行）

### 3.3 新增 `internal/feishu/scopes.go`

**做什么**：权限 scope 管理

```go
var RequiredAppScopes = []string{
    "im:message.group_at_msg:readonly",
    "im:message.p2p_msg:readonly",
    "im:message:readonly",
    "im:message:send_as_bot",
    "im:message:update",
    "im:message:recall",
    "im:message.reactions:read",
    "im:message.reactions:write_only",
    "im:chat",
    "im:chat:read",
    "im:chat:update",
    "im:resource",
    "cardkit:card:read",
    "cardkit:card:write",
}

func BuildScopeApplyUrl(appID string, scopes []string, brand string) string
func MissingScopes(granted, required []string) []string
```

**参考**：`DeepVcodeClient/packages/cli/src/services/feishu/scopes.ts`（176 行）

---

## 错误处理规范

| 场景 | 行为 |
|------|------|
| CardKit API 不可用 | 降级到 PATCH text 方案（现有逻辑） |
| CardKit 单卡片超 8500 字符 | 自动截断 + 追加截断提示 |
| 图片下载失败 | 跳过该图片，不影响消息处理 |
| 文本选择超时（60 秒） | 返回 defaultValue，更新消息为"已超时" |
| SSE 流超时（server 侧 5 分钟限制，`server.go:159`） | bridge 侧 httpClient 超时 10 分钟，先于 server 超时；超时后 UpdateMessage 错误提示 + 移除 reaction |
| 文件上传失败 | 日志 warn + 跳过，不影响主流程 |
| getTenantToken 失败 | 返回 error，上层根据场景处理（发送/上传等操作失败） |

---

## 执行顺序建议

```
Phase 1（消息基础）— 可并行：
  1.1 gateway 图片/post 消息     ← 独立
  1.2 gateway 内容去重           ← 独立

前置（Phase 2 依赖）：
  getTenantToken 缓存            ← Phase 2 所有任务的前置

Phase 2（体验升级）— 有依赖：
  2.1 cardkit.go + 2.2 cardkit_test.go    ← 依赖 token 缓存
  2.3 handler 流式卡片                     ← 依赖 2.1
  2.4 文本选择机制                         ← 依赖 token 缓存，可与 2.1 并行

Phase 3（运维完善）— 互相独立：
  3.1 markdown_style.go           ← 独立
  3.2 registration.go             ← 独立
  3.3 scopes.go                   ← 独立
```

---

## 验证清单

### Phase 1 验证
- [ ] `go test ./internal/feishu/ -run TestDedup -v` 通过
- [ ] 飞书端发图片 → Agent 收到图片描述
- [ ] 飞书端发 post 富文本 → Agent 收到 Markdown

### Phase 2 验证
- [ ] `go test ./internal/feishu/ -run TestCardKit -v` 通过
- [ ] `go test ./internal/feishu/ -run TestTextChoice -v` 通过
- [ ] 飞书端发消息 → 看到 CardKit 流式打字机效果
- [ ] 流式结束 → 终态卡片带 footer（状态/耗时/token）
- [ ] CardKit 不可用时 → 自动降级到 PATCH text
- [ ] 超长回复 → 卡片自动截断，不报错
- [ ] 文本选择 → 用户回复序号 → 选择结果正确传递

### Phase 3 验证
- [ ] `go test ./internal/feishu/ -run TestMarkdownStyle -v` 通过
- [ ] 长回复 Markdown → 卡片中标题/表格/代码块显示正常
- [ ] `/feishu setup` → 扫码 → 自动创建应用
- [ ] 一键申请权限链接 → 飞书后台预选好 scope
