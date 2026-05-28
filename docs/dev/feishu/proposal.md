---
status: draft
author: plan-agent
created: 2026-05-27
updated: 2026-05-27
---

# 飞书 Bot 完整对接提案

## 目标

在已批准的 feishu-bridge 基础桥接方案之上，扩展为功能完整的飞书 Bot 集成——包括流式消息更新、Markdown 富文本渲染、文件/图片发送工具、文本选择交互和 `/compact` 命令支持，使飞书用户体验接近 CLI 交互水平。

## 为什么现在做

**基础桥接已就绪但体验粗糙**。已批准的 `docs/dev/feishu-bridge/execution-plan.md` 定义了一个最小可行 bridge：
- 纯文本回复（无格式化）
- 等全部生成完再回复（用户面对长回复需等待数十秒）
- 不支持发送文件/图片
- 不支持 `ask_user_question` 等交互工具
- 斜杠命令只有 `/new` 和 `/help`

**DeepVcodeClient 最新提交（37f4b05）提供了成熟的参考实现**。`gateway.ts`（~1170 行）包含了流式更新、Markdown→post 转换、文件上传、文本选择交互等全部功能的 TypeScript 实现，可直接翻译为 Go。

**产品路线图要求 Phase 4 的完成标志是"飞书里可以稳定完成对话、工具调用、状态反馈、会话延续"**。基础桥接只覆盖了"对话"和"会话延续"，缺失"状态反馈"和"工具调用可视化"。

## 这次做什么

基于 DeepVcodeClient 参考实现，在 `internal/feishu/` 包中扩展以下 5 个能力。所有扩展都在 bridge 侧实现，不修改 `internal/` 已有代码（与 feishu-bridge execution-plan.md 的约束一致）。

### 1. 流式消息更新

**问题**：当前方案等 SSE 流完全结束后才回复，长回复场景用户需等待 30+ 秒无反馈。

**方案**：收到消息后立即发一条"处理中…"消息，然后每 3 秒 PATCH 更新内容，最终用完整回复再更新一次。

- 在 `client.go` 中新增 `UpdateMessage(messageID, text string) error`（对应飞书 PATCH API）
- 在 `handler.go` 的 SSE 消费循环中，用 `time.Ticker` 每 3 秒触发一次 `UpdateMessage`
- 初始消息用 `text` 类型发送，更新也用 `text` 格式（飞书 PATCH 的 msg_type 不可变）
- SSE 消费结束时做一次最终更新

**参考**：`gateway.ts:429-456`（`updateMessage`）和 `feishu-integration-ref.md` §9

### 2. Markdown → 飞书 post 富文本转换

**问题**：LLM 回复含代码块、粗体、列表等 Markdown，纯文本发送后格式丢失。

**方案**：实现 `mdToPostContent` 转换器，支持标题、粗体、行内代码、代码块、链接、列表、表格。

- 在 `client.go` 中新增 `SendMarkdown(chatID, markdown, replyToMsgID string) (string, error)`
- 新增 `internal/feishu/markdown.go`，实现 Markdown → 飞书 post 格式转换
- 最终回复用 `post` 类型发送（支持富文本），流式中间更新用 `text` 类型（兼容 PATCH）
- 交互式选项列表也用 `SendMarkdown` 发送

**参考**：`gateway.ts:504-704`（`sendMarkdown` + `mdToPostContent` + `parseInlineMarkdown`）

**注意**：飞书 post 格式不支持真正的代码块语法，只能缩进模拟。这是飞书平台限制，无法绕过。

### 3. 文件/图片发送工具

**问题**：LLM 生成的文件（报告、图片等）无法从飞书端获取。

**方案**：新增 `send_feishu_file` Agent 工具，在 bridge 启动时注册到 pi-agent。

- 在 `internal/feishu/tools/send_file.go` 中实现 `agent.Tool` 接口
- Schema：`file_path`（必须）、`chat_id`（可选，默认当前活跃聊天）
- 执行流程：判断文件类型 → 图片走 `uploadImage` + `sendImage`，其他走 `uploadFile` + `sendFile`
- 在 `client.go` 中新增 `UploadImage`、`SendImage`、`UploadFile`、`SendFile` 四个方法
- 工具通过 handler 在收到消息时动态注入当前 chatID（闭包方式）

**实现方式**：bridge 作为独立进程通过 HTTP API 与 pi-agent 通信，无法直接注册工具到 agent 的 tool registry。因此需要另一种方案：
- **方案 A（推荐）**：在 `handler.go` 中检测 LLM 回复文本里的文件路径（正则匹配），自动上传并发送。参考 `feishuCommand.ts:223-271`（`sendDetectedFiles`）。无需修改 pi-agent。
- **方案 B**：修改 pi-agent server 暴露 tool 注册 API，bridge 远程注册工具。违反"不改已有代码"约束。

选择方案 A：路径检测 + 自动发送。简单可靠，不需要改 pi-agent。

### 4. 文本选择交互

**问题**：LLM 的 `ask_user_question` 工具在飞书端无法直接交互。飞书 WebSocket 长连接不支持卡片回调（`card.action.trigger` 是回调型事件，只有 Webhook 能收到）。

**方案**：实现文本选择模式——发送格式化选项列表，用户回复序号或选项名称来选择。

- 在 `handler.go` 中实现 `TextChoiceWaiter`：
  - 发送选项列表（Markdown 格式）
  - 注册 `textChoiceCallback`，在 gateway 消息处理之前拦截
  - 用户回复匹配后 resolve，60 秒超时返回默认值
- 匹配逻辑：数字匹配（"1" → 第一项）+ 文本匹配（大小写不敏感）
- 被拦截的消息不进入 agent 处理流程
- 需要并发安全：同一 chatID 同时只允许一个等待器

**参考**：`gateway.ts:1089-1139`（`waitForTextChoice`）和 `feishu-integration-ref.md` §7

### 5. 扩展斜杠命令

在 `handler.go` 中扩展斜杠命令处理：

| 命令 | 功能 |
|------|------|
| `/new` | 已有——创建新 session |
| `/help` | 已有——显示帮助 |
| `/compact` | 新增——调用 pi-agent 的 compaction（通过 `POST /sessions/{id}/compact` 需要新增端点，或直接通过 prompt 触发） |
| `/status` | 新增——显示当前 session 状态 |
| `/model` | 新增——显示/切换当前模型 |

**`/compact` 实现方式**：第一版通过发送特殊 prompt（如"请总结当前对话"）触发 pi-agent 的 compaction，不新增 server 端点。这是最简单的实现，虽然不完美但满足基本需求。

## 这次不做什么

- **不改 `internal/` 已有代码**：所有新功能都在 bridge 侧实现。不修改 server 端点、agent 核心、tool registry
- **不做交互式卡片**：飞书 WS 模式不支持卡片回调，这是平台限制。未来如果部署 Webhook 回调 URL 可以重新评估
- **不做扫码自动建应用**：飞书私有注册协议未公开文档，稳定性无保障。手动配置 App ID/Secret 即可
- **不做 Markdown → post 转换的 100% 覆盖**：只支持标题、粗体、行内代码、代码块、链接、列表、表格。飞书 post 格式本身不支持完整 Markdown（如嵌套列表、HTML），无法做到完美
- **不做工具调用中间状态的可视化**：tool_start/tool_end 事件暂不展示给飞书用户，只做文本输出的流式更新
- **不做并发请求管理**：同一 chatKey 的 busy 检测已在基础方案中，不做更复杂的队列/优先级
- **不做权限控制**：白名单、操作审计等留到后续安全加固阶段

## 技术方案

### 架构影响

```
飞书云  │  WebSocket 长连接
        ▼
cmd/pi-feishu-bridge
  ├─ gateway.go    [扩展] 新增 textChoiceCallback 消息拦截
  ├─ client.go     [扩展] 新增 UpdateMessage / SendMarkdown / Upload* / Send* 方法
  ├─ handler.go    [扩展] 流式更新循环 / 文件路径检测 / 文本选择 / 新命令
  ├─ markdown.go   [新增] Markdown → 飞书 post 转换
  └─ tools/        [新增但用方案A替代] 文件路径检测在 handler 中
        │  HTTP POST /chat/stream（SSE）
        ▼
cmd/pi-agent -mode serve（不改）
```

### 核心设计

#### SSE 消费循环重构（`handler.go`）

基础方案的 SSE 消费是"收集所有 text_delta → 最终回复"。需要重构为流式更新模式：

```go
func (h *Handler) handleStreamWithUpdates(ctx context.Context, chatKey, messageID, prompt, sessionID string) {
    // 1. 立即发一条 text 消息，拿到 botMsgID
    botMsgID, _ := h.client.SendMessage(chatKey, "⏳ 处理中...", "")

    // 2. 调 POST /chat/stream
    // 3. 启动 3 秒 ticker
    // 4. 消费 SSE：
    //    - text_delta → 追加到 buffer
    //    - tick → UpdateMessage(botMsgID, buffer)
    //    - done → 最终更新
    //    - error → UpdateMessage(botMsgID, "处理出错")
    // 5. 最终回复：用 SendMarkdown 发送完整内容（post 格式）
    // 6. 删除中间的 text 消息（如果飞书支持，否则保留）
}
```

**关键决策**：中间更新用 text 格式（因为初始消息是 text），最终回复额外发一条 post 格式消息。这是两种选择：

| 选择 | 方案 | 优点 | 缺点 |
|------|------|------|------|
| A | 初始发 text，流式更新 text，完成后额外发 post | 用户看到流式进度 + 最终格式化 | 多一条消息 |
| B | 初始发 post，流式更新 post（每次都做 mdToPost 转换） | 只有一条消息 | 每次更新都要转换，性能差；post 格式更新不如 text 稳定 |

**推荐方案 A**：多一条消息的代价远小于性能和稳定性收益。

#### 文件路径自动检测（`handler.go`）

```go
// 正则匹配 LLM 回复中的文件路径
var filePathRegex = regexp.MustCompile(
    `(?:^|\s|["'\x60])((?:/[\w\-./]+)|(?:\./[\w\-./]+))\.(png|jpg|jpeg|gif|webp|svg|bmp|pdf|txt|csv|json|zip|py|js|ts|md)`)

func (h *Handler) sendDetectedFiles(chatID, replyToMsgID, text string) {
    // 匹配文件路径 → 检查文件是否存在 → 图片走 uploadImage+sendImage，其他走 uploadFile+sendFile
}
```

在最终回复发送后调用。参考 `feishuCommand.ts:223-271`。

#### 文本选择机制（`handler.go`）

```go
type TextChoiceWaiter struct {
    chatID   string
    buttonMap map[string]string  // label(小写) + 序号 → value
    result   chan string
    timer    *time.Timer
}

func (g *Gateway) WaitForTextChoice(chatID, title string, buttons []Button, defaultVal string, timeout time.Duration) string
```

- `gateway.go` 持有当前活跃的 `textChoiceWaiter`
- 消息到达时，先检查是否有等待器匹配当前 chatID
- 匹配成功：消费消息，返回选择结果
- 超时：返回默认值
- 并发安全：同一 chatID 只允许一个等待器（新的覆盖旧的）

### 新增/修改文件清单

```
internal/feishu/
├── client.go           [修改] 新增 UpdateMessage / SendMarkdown / UploadImage / SendImage / UploadFile / SendFile
├── client_test.go      [修改] 新增对应测试
├── gateway.go          [修改] 新增 textChoiceCallback 字段和消息拦截逻辑
├── gateway_test.go     [修改] 新增文本选择去重/拦截测试
├── handler.go          [修改] 重构 SSE 消费为流式更新 + 文件检测 + 文本选择 + 新命令
├── markdown.go         [新增] Markdown → 飞书 post 格式转换器
└── markdown_test.go    [新增] 转换器测试
```

### 数据流（完整链路）

```
用户发消息 → 飞书 WS 推送
  → Gateway 去重 + textChoiceCallback 检查
  → Handler 收到消息
    → AddReaction(🤔)
    → 斜杠命令检查
    → SendMessage("⏳ 处理中...") → botMsgID
    → POST /chat/stream → SSE 流
    → Ticker(3s): UpdateMessage(botMsgID, buffer)
    → SSE done:
      → SendMarkdown(chatID, fullReply, messageID) → postMsgID
      → sendDetectedFiles(chatID, messageID, fullReply)
    → RemoveReaction(🤔)
```

## 依赖关系

| 依赖 | 状态 | 说明 |
|------|------|------|
| feishu-bridge 基础方案 | approved，未实现 | 本提案在基础方案上扩展，执行时需先完成基础方案或并行实施 |
| pi-agent `POST /chat/stream` SSE 端点 | ✅ 已存在 | `internal/server/server.go:143-191`，输出 `AgentStreamEvent` |
| pi-agent `POST /sessions` 端点 | ✅ 已存在 | `internal/server/server.go:209-221` |
| 飞书 Go SDK (`larksuite/oapi-sdk-go/v3`) | 需引入 | feishu-bridge 基础方案已规划 |
| 飞书开放平台应用配置 | 人工操作 | App ID / App Secret / WS 事件订阅 / 权限 |

**与 feishu-bridge execution-plan 的关系**：本提案是基础方案的扩展集。建议执行策略是：
1. 先按基础方案完成骨架（client/gateway/handler/cmd）
2. 在骨架上逐个叠加本提案的 5 个能力
3. 或者直接按本提案的完整规格实施，跳过基础方案的中间状态

## 风险和取舍

| 风险 | 影响 | 缓解 |
|------|------|------|
| 飞书 PATCH API 频率限制 | 流式更新可能被限流 | 3 秒节流已考虑，若仍被限流可退回到 5 秒 |
| Markdown → post 转换不完美 | 代码块、复杂格式显示不佳 | 这是飞书平台限制，可接受。纯文本 fallback 始终可用 |
| 文件路径正则误匹配 | LLM 回复中的非文件路径被检测 | 加文件存在性检查，不存在则跳过 |
| 文本选择被普通消息误触发 | 用户随意发消息可能匹配到等待中的选项 | 只匹配数字 1-N 或精确文本匹配，误触概率低 |
| 不改 pi-agent 意味着功能受限 | 无法实现真正的动态工具注册、tool 执行中间状态展示 | 第一版可接受。后续可在 pi-agent server 中新增 tool webhook 端点 |
| bridge 进程重启丢失 session 映射 | 用户需重新开始对话 | 第一版可接受。后续可持久化映射到文件 |

**关键取舍**：选择"不改 pi-agent 已有代码"意味着文件发送只能用路径检测而非 Agent 工具调用。这是当前约束下的最优解——功能可用但不够优雅。如果后续放开"不改已有代码"的约束，可以新增 server 端的 tool webhook 机制。

## 完成标志

1. `go build ./cmd/pi-feishu-bridge` 编译通过
2. `go test ./internal/feishu/... -race` 全部通过
3. 飞书端手动验证：
   - 发消息 → 立即看到"处理中…" → 约 3 秒后看到内容更新 → 最终看到格式化的 post 回复
   - LLM 回复中包含文件路径 → 文件自动上传并发送到飞书
   - 发 `/compact` → 收到压缩反馈
   - 发 `/status` → 显示当前 session 状态
   - LLM 触发 `ask_user_question`（如果 agent 支持）→ 飞书端显示选项列表 → 用户回复序号 → 选择被正确传递
4. 长回复（>5000 字）场景下，流式更新正常工作，无消息丢失或重复
5. deploy.yml 能构建 `pi-feishu-bridge` 二进制并部署
