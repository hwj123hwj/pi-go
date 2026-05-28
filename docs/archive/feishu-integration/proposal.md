---
status: draft
author: plan-agent
created: 2026-05-27
updated: 2026-05-28
---

# 飞书 Bot 完整对接提案（v2）

## 目标

在已有骨架实现（client/gateway/handler/markdown）之上，参考 DeepVcodeClient 的成熟实现，
补齐缺失能力，使飞书用户体验接近 CLI 交互水平。

## 现状

### 已实现（骨架级）

| 功能 | 文件 | 状态 |
|------|------|------|
| WS 长连接 + 消息接收 | `gateway.go` | 完成 |
| 消息发送 / 回复 / 更新 | `client.go` | 完成 |
| SSE 流式消费 + 3 秒 ticker 更新 | `handler.go` | 完成 |
| Markdown → post 转换 | `markdown.go` | 完成 |
| 文件路径检测 + 自动发送 | `handler.go` | 完成 |
| emoji reaction（思考中） | `client.go` + `handler.go` | 完成 |
| 斜杠命令（/new /compact /status /help） | `handler.go` | 完成 |
| textChoiceCallback 预留字段 | `gateway.go` | 仅声明 |

### 对比 DeepVcodeClient 缺失的能力

按优先级排序：

| # | 缺失能力 | 影响 | 优先级 |
|---|---------|------|--------|
| 1 | 入站图片消息处理 | 用户发图片 Agent 收不到 | P0 |
| 2 | 内容+时间窗口去重 | WS 重连可能重复处理 | P1 |
| 3 | CardKit 2.0 流式卡片 | 用户体验差距最大 | P1 |
| 4 | 交互式卡片（按钮选择） | ask_user_question 无法交互 | P1 |
| 5 | Markdown 样式优化 | 卡片显示不理想 | P2 |
| 6 | 扫码自动建应用 | setup 流程繁琐 | P2 |
| 7 | 权限 scope 管理 | 用户可能漏开权限 | P2 |
| 8 | 多项目路由 | 无法按群路由到不同项目 | P3 |
| 9 | 多群并发队列 | 高并发场景瓶颈 | P3 |

> **不做**：凭证加密存储 + 授权白名单 — 个人使用场景，环境变量配置即可。

## 这次做什么

分三个阶段实施，每个阶段独立可交付。

### Phase 1：消息基础（P0）

#### 1.1 入站图片消息处理

在 `gateway.go` 的 `handleEvent` 中：
- 支持 `image` 类型消息：下载图片资源到临时目录，转为 `![image](/tmp/feishu-image-xxx.png)` 文本
- 支持 `post` 类型消息：解析富文本（文本、链接、图片），转为 Markdown
- 新增 `downloadImageResource(messageID, imageKey) (localPath, error)`

参考：`gateway.ts:454-530`（image/post 消息解析）

#### 1.2 内容+时间窗口去重

在 `gateway.go` 中增强去重：
- 保留现有 messageID 去重
- 新增内容+时间窗口去重：`chatID:text` 作为 key，5 秒内相同内容视为重复
- 定期清理过期记录

参考：`gateway.ts:300-303`（`recentContents` Map + `dedupWindowMs`）

### Phase 2：体验升级（P1）

#### 2.1 CardKit 2.0 流式卡片

新增 `internal/feishu/cardkit.go`：
- `BuildStreamingCard(initialContent, initialFooter) CardJSON`
- `BuildFinalCard(content, footerMetrics) CardJSON`
- `CreateCardKitCard(card) (cardID, error)`
- `StreamCardKitElement(cardID, elementID, content, sequence) error`
- `SetCardKitStreamingMode(cardID, enabled, sequence) error`
- `UpdateCardKitCard(cardID, card, sequence) error`

在 `client.go` 中新增：
- `SendStreamingCard(chatID, initialContent, replyTo) (handle, error)`
- 返回句柄包含 `PushContent` / `PushFooter` / `Finalize`

在 `handler.go` 中重构 `handleAgentMessage`：
- 用 CardKit 流式卡片替代 text 消息 + ticker 更新
- footer 显示状态、耗时、token 用量

参考：`gateway.ts:167-275`（card 构建）+ `gateway.ts:1463-1545`（流式卡片 API）

#### 2.2 文本选择机制（WS 模式下唯一的交互方式）

> WS 长连接不支持 `card.action.trigger` 回调，所有用户选择场景通过文本匹配实现。

在 `gateway.go` 中完善：
- `WaitForTextChoice(chatID, title, content, buttons, defaultVal, timeout) (choice, error)`
- 发送格式化选项列表（Markdown），用户回复序号或选项名称
- 匹配逻辑：数字匹配 + 文本匹配（大小写不敏感）
- 60 秒超时返回默认值

在 `client.go` 中新增：
- `SendCard(chatID, title, content, buttons, replyTo) (messageID, error)` — 纯展示用途，不处理点击回调`

参考：`gateway.ts:1230-1339`（sendCard）+ `gateway.ts:1089-1139`（waitForTextChoice）

#### 2.3 内容去重

在 `gateway.go` 中增强去重：
- 保留现有 messageID 去重
- 新增内容+时间窗口去重：`chatID:text` 作为 key，5 秒内相同内容视为重复
- 定期清理过期记录

参考：`gateway.ts:300-303`（`recentContents` Map + `dedupWindowMs`）

### Phase 3：运维完善（P2-P3）

#### 3.1 Markdown 样式优化

新增 `internal/feishu/markdown_style.go`：
- 标题降级：H1→H4，H2~H6→H5
- 表格前后段落间距（`<br>`）
- 代码块前后 `<br>`
- `StripInvalidImageKeys`：清理非 `img_` 前缀的图片引用

在 `markdown.go` 的输出后调用 `OptimizeMarkdownStyle`。

参考：`DeepVcodeClient/packages/cli/src/services/feishu/markdown-style.ts`

#### 3.2 扫码建应用 + scope 管理

新增 `internal/feishu/registration.go`：
- `InitRegistration` / `BeginRegistration` / `PollRegistration`
- `ProbeCredentials`：校验凭证 + 查 bot 信息 + 查已开通 scope

新增 `internal/feishu/scopes.go`：
- `RequiredAppScopes` 定义（14 个必需 scope）
- `BuildScopeApplyUrl`：一键申请权限链接
- `MissingScopes`：scope 比对

参考：`registration.ts` + `scopes.ts`

#### 3.3 多群并发队列

在 `handler.go` 中：
- 每个 chatKey 独立队列 + 独立处理 goroutine
- 每群独立的 context/cancel
- 消息排队而非丢弃

## 这次不做什么

- **不改 `internal/` 已有代码**（与 execution-plan 约束一致）
- **不做多项目路由**（需要独立产品规划）
- **不做飞书 Webhook 回调模式**（WS 模式足够）
- **不做权限审计日志**

## 技术方案

### 架构影响

```
飞书云  │  WebSocket 长连接
        ▼
cmd/pi-feishu-bridge
  ├─ registration.go  [新增] 扫码建应用
  ├─ scopes.go        [新增] 权限 scope 管理
  ├─ cardkit.go       [新增] CardKit 2.0 流式卡片
  ├─ markdown_style.go[新增] Markdown 样式优化
  ├─ gateway.go       [扩展] 图片/post 消息解析 + 内容去重 + card 回调
  ├─ client.go        [扩展] SendCard / CardKit API / downloadImage / token 缓存
  ├─ handler.go       [扩展] 流式卡片 + 图片入站 + 降级逻辑
  └─ markdown.go      [扩展] 调用样式优化
        │  HTTP POST /chat/stream（SSE）
        ▼
cmd/pi-agent -mode serve（不改）
```

### 新增/修改文件清单

```
internal/feishu/
├── registration.go        [新增] 扫码建应用
├── scopes.go              [新增] scope 管理
├── cardkit.go             [新增] CardKit 2.0 API
├── cardkit_test.go        [新增]
├── markdown_style.go      [新增] 样式优化
├── client.go              [修改] 新增 SendCard / CardKit / downloadImage / token 缓存
├── gateway.go             [修改] 图片/post 消息 + 内容去重 + 文本选择
├── handler.go             [修改] 流式卡片 + 降级逻辑
└── markdown.go            [修改] 调用样式优化
```

### 数据流（Phase 2 完成后）

```
用户发消息 → 飞书 WS 推送
  → Gateway 去重（messageID + 内容窗口）
  → 图片消息 → downloadImageResource → 转文本
  → Handler 收到消息
    → AddReaction(THINKING)
    → 斜杠命令检查
    → SendStreamingCard(chatID, "", messageID) → cardHandle（失败则降级 PATCH text）
    → POST /chat/stream → SSE 流
    → text_delta → cardHandle.PushContent(buf)（内置 1500ms 节流）
    → done → cardHandle.Finalize(fullReply, footerMetrics)
    → sendDetectedFiles(chatID, messageID, fullReply)
    → RemoveReaction(THINKING)
```

## 依赖关系

| 依赖 | 状态 | 说明 |
|------|------|------|
| 飞书骨架实现 | ✅ 已完成 | client/gateway/handler/markdown |
| pi-agent `POST /chat/stream` | ✅ 已存在 | server.go:143 |
| pi-agent `POST /sessions` | ✅ 已存在 | server.go:209 |
| pi-agent `POST /sessions/{id}/compact` | ✅ 已存在 | server.go:73（review B2 已解决） |
| 飞书 Go SDK | ✅ 已引入 | go.mod 中 larksuite/oapi-sdk-go/v3 |
| CardKit 2.0 API | 需验证 | 飞书 Go SDK 是否支持 cardkit 模块 |

## 风险和取舍

| 风险 | 缓解 |
|------|------|
| CardKit 2.0 Go SDK 可能不支持 | 降级到 PATCH text 方案（现有实现） |
| 飞书私有注册协议可能变更 | registration 作为可选功能，手动配置始终可用 |
| 凭证加密密钥与密文同位 | 威胁模型明确：防被动暴露，不防主动攻击 |
| `ask_user_question` 工具不存在 | WS 模式不支持卡片回调，用文本选择（回复序号）替代；B1 仍开放 |

## 完成标志

1. `go build ./cmd/pi-feishu-bridge` 编译通过
2. `go test ./internal/feishu/... -race` 全部通过
3. 飞书端验证：
   - Phase 1：发图片 → Agent 能看到；发 post 富文本 → Agent 收到 Markdown
   - Phase 2：发消息 → CardKit 流式卡片实时更新 → 终态带 footer metrics
   - Phase 2：LLM 回复含选项 → 飞书显示选项列表 → 用户回复序号 → 选择正确传递
   - Phase 3：`/feishu setup` 扫码 → 自动创建应用 + 一键申请权限
