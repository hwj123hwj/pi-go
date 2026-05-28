---
status: reviewed
author: review-agent
created: 2026-05-27
updated: 2026-05-27
reviewer: review-agent
review-status: pending
depends-on:
  - docs/dev/feishu/proposal.md
---

# 飞书 Bot 完整对接提案 — Review

## 1. 总体评价：approve（附建议）

提案整体质量高，目标清晰，技术方案可行，与已批准的 feishu-bridge execution-plan 衔接合理。5 个能力模块的优先级和取舍判断得当。"不改已有代码"的约束贯穿全文，scope 控制好。

**建议在实施前处理下文的 🔴 Blocker 和 🟡 Strong suggestions，其余可边做边调整。**

---

## 2. 准确性验证

逐项交叉验证提案中的关键断言：

| # | 提案断言 | 验证结果 | 状态 |
|---|---------|---------|------|
| 1 | `POST /chat/stream` SSE 端点已存在，位于 `internal/server/server.go:143-191` | ✅ 准确。`chatStream` 方法位于 L143-191，输出 `event: <type>\ndata: <json>\n\n` 格式的 SSE | ✅ |
| 2 | `POST /sessions` 端点已存在，位于 `internal/server/server.go:209-221` | ✅ 准确。`createSession` 方法位于 L209-221 | ✅ |
| 3 | SSE 事件包含 `text_delta` 事件 | ✅ 准确。`AgentStreamEvent` 的 `StreamEventTextDelta = "text_delta"` 在 `agent.go:366`。server.go L186 用 `event.Type` 序列化，stream consumer 会输出 `text_delta` | ✅ |
| 4 | SSE 事件包含 `done` 事件 | ✅ 准确。`StreamEventDone = "done"` 在 `agent.go:372`，在 stream 结束时发送 (L230) | ✅ |
| 5 | SSE 事件包含 `error` 事件 | ✅ 准确。`StreamEventError = "error"` 在 `agent.go:373` | ✅ |
| 6 | `internal/feishu/` 包不存在，需新建 | ✅ 准确。`internal/feishu/` 目录不存在 | ✅ |
| 7 | feishu-bridge execution-plan 已 approved | ✅ 准确。`execution-plan.md` frontmatter 中 `status: approved` | ✅ |
| 8 | 飞书 Go SDK (`larksuite/oapi-sdk-go/v3`) 需引入 | ✅ 准确。`go.mod` 中无此依赖 | ✅ |
| 9 | `POST /sessions/{id}/compact` 端点不存在 | ✅ 准确。server.go 的路由中无 compact 相关端点 | ✅ |
| 10 | `gateway.ts:429-456` 包含 `updateMessage` 参考 | ⚠️ 无法验证。`gateway.ts` 文件在 pi-go 仓库中不存在，它属于外部项目 DeepVcodeClient。引用的行号范围无法在源码中确认 | ⚠️ |
| 11 | `feishu-integration-ref.md` §9 / §7 存在相关内容 | ⚠️ 部分验证。ref 文档中存在 `mdToPostContent`、`waitForTextChoice` 关键词，但无显式 §9/§7 章节标记。提案引用的是非结构化参考，不影响实际使用 | ⚠️ |
| 12 | `feishuCommand.ts:223-271` 包含 `sendDetectedFiles` 参考 | ⚠️ 同 #10，无法在仓库中验证 | ⚠️ |
| 13 | `AgentStreamEvent` 输出格式为 JSON SSE | ✅ 准确。server.go L185-189: `json.Marshal(event)` → `event: %s\ndata: %s\n\n` | ✅ |
| 14 | `ask_user_question` 工具存在 | ❌ 未找到。搜索 `internal/` 中无 `ask_user_question` 或 `AskUserQuestion` 相关代码 | ❌ |
| 15 | 飞书 WS 模式不支持卡片回调 | ✅ 准确。`feishu-integration-ref.md` 明确指出 "WS 模式下别碰卡片交互，卡片回调在 WS 下收不到" | ✅ |

---

## 3. 发现的问题

### 🔴 Blockers（必须修复）

#### B1. `ask_user_question` 工具不存在，文本选择交互缺少触发来源

提案 §4（文本选择交互）说"LLM 的 `ask_user_question` 工具在飞书端无法直接交互"，暗示 agent 会通过某个工具触发交互。但搜索整个 `internal/` 代码，**不存在** `ask_user_question` 工具或任何类似的多轮交互工具。

**影响**：文本选择交互功能（`TextChoiceWaiter`）设计了一个完整的等待/匹配机制，但没有触发入口。如果 agent 永远不会产生需要用户选择的请求，这个机制就是死代码。

**建议**：
- 方案 A：明确标注文本选择交互为"预留设计，待 agent 支持交互工具后启用"，从第一版实现中移除
- 方案 B：如果执意要做，需要在 pi-agent 侧新增一个交互工具——但这违反"不改已有代码"约束
- 推荐方案 A

#### B2. `/compact` 通过 prompt 触发的方案存在实际障碍

提案说 `/compact` 通过发送特殊 prompt（如"请总结当前对话"）触发。但查看 agent 循环 (`agent/loop.go`) 和 `AgentSession.Compact()` (agent_session.go:215)，compaction 是一个**独立的方法调用**，不是通过 prompt 触发的。发送 "请总结当前对话" 只会让 LLM 生成一段总结文本，**不会执行 compaction**（截断历史、持久化 compaction entry）。

**影响**：`/compact` 命令无法按提案描述工作。用户执行后只能得到一段 LLM 回复，上下文窗口不会被压缩。

**建议**：
- 在 server.go 新增 `POST /sessions/{id}/compact` 端点（调用 `AgentSession.Compact()`），但这违反"不改已有代码"约束
- 或在第一版中将 `/compact` 移出 scope，标注为"需 pi-agent server 支持新端点后实现"
- 或放开约束，仅新增这一个端点（改动量很小，约 20 行）

### 🟡 Strong Suggestions（强烈建议修复）

#### S1. SSE 消费中缺少对 `session_id` 初始事件的处理

server.go L168 在 SSE 流开始时会先发送 `event: session_id\ndata: <id>` 。提案的 SSE 消费循环伪代码没有提到处理这个初始事件。如果 bridge 需要获取 server 分配的 session ID（特别是新建 session 时），必须消费这个事件。

**建议**：在 `handleStreamWithUpdates` 中增加对 `session_id` 事件的处理逻辑，用 server 返回的 session ID 更新本地映射（而非仅依赖 `POST /sessions` 的返回值）。

#### S2. 流式更新的 text 消息最终替换为 post 消息，旧消息处理策略不明确

提案推荐方案 A：中间更新用 text，最终额外发一条 post。但未说明旧的 text 消息是否删除/更新。

**影响**：
- 如果旧 text 消息保留，用户会看到两条消息（"处理中…"的中间状态 + 最终 post），体验不佳
- 如果更新旧消息为最终 post 内容，需要兼容 msg_type 不可变的限制（提案提到 PATCH 的 msg_type 不可变）

**建议**：明确最终策略。推荐：
1. 最终回复时，先 PATCH 更新 text 消息为最终文本（简短版），再额外发一条完整的 post 消息
2. 或删除旧 text 消息（飞书 API 是否支持删除？需验证），再发新 post

#### S3. 文件路径检测的正则表达式范围过大

提案的 `filePathRegex` 匹配所有以 `/` 或 `./` 开头、带扩展名的路径。但 LLM 回复中常见以下误匹配场景：
- 命令行示例：`$ cat /etc/passwd.txt`
- 文档引用：`见 /path/to/config.json`
- 伪代码中的路径

虽然提案说"加文件存在性检查"缓解，但一个更根本的问题是：**LLM 可能产生大量合法但本机不存在的路径**，导致大量无意义的文件系统 stat 调用。

**建议**：
- 增加路径前缀白名单（只匹配工作目录下的路径），减少无效 stat
- 或只检测特定标记后的路径（如 LLM 输出中的 "文件已保存到: /path/..." 模式）

#### S4. 提案引用的外部文件（`gateway.ts`）不在仓库中

提案多次引用 `gateway.ts` 的具体行号（如 `gateway.ts:429-456`、`gateway.ts:504-704`、`gateway.ts:1089-1139`）和 `feishuCommand.ts:223-271`。这些文件不在 pi-go 仓库中，属于外部项目 DeepVcodeClient。

**影响**：执行 agent 无法在仓库上下文中验证这些引用。

**建议**：
- 将关键参考代码片段直接嵌入提案或附录中，减少执行时的上下文切换
- 或在 `docs/references/` 中放置 `gateway.ts` 的副本（如果许可允许）

### 🟢 Nice-to-haves

#### N1. 提案缺少错误场景的完整处理矩阵

execution-plan 有一个清晰的"错误处理规范"表（第 6 节），但本提案缺少类似的内容。特别是：
- `UpdateMessage` PATCH 失败时的行为
- `SendMarkdown` post 发送失败时的 fallback
- 文件上传失败时是否影响主流程
- `UploadImage`/`UploadFile` API 返回的 file_key 失效的处理

**建议**：补充一个扩展的错误处理表。

#### N2. `TextChoiceWaiter` 的并发模型可以更清晰

提案说"同一 chatID 只允许一个等待器"，但没说新等待器覆盖旧等待器时，旧等待器如何通知其调用方。如果旧等待器正在 SSE 消费中等待结果，突然被覆盖，可能导致 SSE 流 hang 或 produce 未预期的行为。

**建议**：明确覆盖时的取消语义——旧等待器的 channel 应该 close 或发送默认值。

#### N3. 缺少性能预估

- Markdown → post 转换对长回复（>5000 字）的性能如何？
- 3 秒 ticker + PATCH 更新在高并发群聊下的 API 调用量？

这些不是 blocker，但在实施前做一次简单评估有助于确定最终参数。

#### N4. 与 execution-plan 的合并策略不够明确

提案说"建议执行策略是：先按基础方案完成骨架，在骨架上逐个叠加"。但两份文档的设计有冲突：
- execution-plan: "不实现流式逐字回复到飞书" → 本提案要做流式更新
- execution-plan: "不实现 markdown → 飞书 post 富文本转换" → 本提案要做 mdToPost

**建议**：明确说明本提案**替代** execution-plan 的"这次不做什么"部分（即直接按本提案实施），而非"先做 execution-plan 再叠加"。

---

## 4. 遗漏检查

### 4.1 缺少的影响区域

| 遗漏 | 说明 |
|------|------|
| **`go.mod` 依赖变更** | 飞书 SDK 是一个较大的依赖（~10+ 子模块），提案未评估对编译时间、二进制大小、CI 的影响 |
| **飞书 API 权限清单** | `UploadImage`/`UploadFile` 需要的权限（`im:resource`、`im:file`）未在权限列表中提及 |
| **SSE 流中 `tool_start`/`tool_end` 事件** | 提案明确说"不做工具调用中间状态的可视化"，但 SSE 流会包含这些事件。handler 需要显式忽略它们，否则可能导致意外行为 |
| **`deploy/pi-feishu-bridge.service`** | 提案的新增文件清单中没有提到 systemd 服务文件，但 execution-plan 有。两份文档应保持一致 |
| **bridge 的日志规范** | execution-plan 有错误处理规范，本提案缺少日志级别和格式的约定 |
| **飞书消息长度限制与 post 格式** | 飞书 post 消息有 JSON 大小限制（通常 ~30KB），超长回复的 mdToPost 输出可能超限。提案只提到 text 格式截断，未提到 post 格式的截断 |

### 4.2 潜在的架构冲突

| 冲突点 | 说明 |
|--------|------|
| **session 映射内存泄漏** | `chatKey → sessionID` 映射只增不减。如果用户创建大量 `/new` session，map 会无限增长 |
| **SSE 流的 context 超时** | server.go L159 设置了 5 分钟超时。对于复杂 tool call 链（多轮 tool use），5 分钟可能不够。bridge 需要处理超时后的 graceful fallback |

---

## 5. 修改建议汇总

按优先级排列：

| # | 优先级 | 建议 | 对应问题 |
|---|--------|------|---------|
| 1 | 🔴 | 移除或标记"预留"文本选择交互（§4），因为 `ask_user_question` 工具不存在 | B1 |
| 2 | 🔴 | `/compact` 改为"需 pi-agent server 新增端点"或放开约束新增端点 | B2 |
| 3 | 🟡 | SSE 消费循环补充处理 `session_id` 初始事件 | S1 |
| 4 | 🟡 | 明确方案 A 中旧 text 消息的处理策略（删除/更新/保留） | S2 |
| 5 | 🟡 | 文件路径检测增加工作目录前缀约束 | S3 |
| 6 | 🟡 | 将关键参考代码片段嵌入提案或附录 | S4 |
| 7 | 🟢 | 补充错误处理规范表 | N1 |
| 8 | 🟢 | 明确 `TextChoiceWaiter` 覆盖时的取消语义 | N2 |
| 9 | 🟢 | 明确本提案替代 execution-plan 的"不做"部分 | N4 |

---

## 附录：源码验证记录

验证的关键文件和行号：

| 文件 | 验证内容 | 结论 |
|------|---------|------|
| `internal/server/server.go:57-84` | 路由注册 | 存在 `/chat/stream`、`/sessions`，无 `/compact` |
| `internal/server/server.go:143-191` | `chatStream` SSE 端点 | 输出标准 SSE，事件格式正确 |
| `internal/server/server.go:209-221` | `createSession` 端点 | 存在且可用 |
| `internal/agent/agent.go:363-390` | `AgentStreamEvent` 类型定义 | 包含 text_delta/done/error/tool_start/tool_end/compacted |
| `internal/agent/agent.go:144-239` | `PromptStream` 方法 | 事件通过 channel 转发，含 buffer(64) |
| `internal/agent/loop.go:42-68` | `RunLoop` 双层循环 | 消费函数只关注 EventDone/EventError |
| `internal/agent/event.go` | AgentEvent 接口 | 包含完整的工具/压缩/批次事件 |
| `internal/runtime/agent_session.go:128-130` | `PromptStream` | 正确调用 agent.PromptStream |
| `internal/runtime/agent_session.go:215-220` | `Compact` 方法 | 存在但 server 无对应端点 |
| `internal/runtime/application.go` | Application 接口 | 清晰的分层设计 |
| `go.mod` | 依赖列表 | 无飞书 SDK，需新增 |
| `internal/feishu/` | 目录 | 不存在，需新建 |
| `docs/dev/feishu-bridge/execution-plan.md` | 已批准文档 | status: approved，scope 与本提案有重叠但可互补 |
| `docs/references/feishu-integration-ref.md` | 参考文档 | 包含关键概念但非结构化章节 |
