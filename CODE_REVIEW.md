# Pi-Go 代码评审

日期：2026-05-20

本次评审重点查看了 `agent / server / session / tools` 这几条核心链路，优先关注真实的行为错误、接口语义偏差和测试覆盖缺口。

## Findings

### [High] ~~会话持久化在真实运行中实际上没有形成可恢复链路~~ ✅ 已修复

位置：
- [internal/session/session.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/session/session.go:18)
- [internal/session/jsonl.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/session/jsonl.go:102)
- [internal/session/jsonl_test.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/session/jsonl_test.go:23)

`Session.AppendMessage()` 只把消息包装成 `Entry{Type: EntryTypeMessage}` 后直接 append，但没有设置 `ParentID`，也没有在追加后更新 `leaf`。`BuildContext()` 又是通过 `GetPathToRoot(ctx, "")` 从当前 `leafID` 回溯历史；当 `leafID` 为空时，这里会直接返回空切片。

这意味着运行时虽然会不断往 JSONL 文件写消息，但默认恢复流程拿不到任何历史，上下文持久化基本失效。更麻烦的是，现有测试通过“手工设置 parent 和 leaf”的方式验证 `JSONLStorage`，并没有覆盖 `Session.AppendMessage()` 这一真实写入路径，所以这个问题会被测试完全漏掉。

建议：
- 在 `Session` 层维护当前 leaf，追加消息时自动串起 `ParentID`。
- 每次成功追加消息后更新 leaf。
- 增加一个端到端测试，覆盖 `AppendMessage -> BuildContext` 的真实流程，而不是只测底层 storage。

### [High] ~~HTTP 服务把所有请求复用到单个 Agent，导致并发请求语义错误~~ ✅ 已修复

位置：
- [internal/server/server.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/server/server.go:18)
- [internal/server/server.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/server/server.go:89)
- [internal/server/server.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/server/server.go:122)
- [internal/agent/agent.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/agent/agent.go:104)
- [internal/agent/agent.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/agent/agent.go:131)

`Server` 只持有一个全局 `*agent.Agent`。当一个请求正在执行时，第二个 `POST /chat` 请求会走到 `Agent.Prompt()` 的 `StateRunning` 分支，被塞进 `followUpQueue` 后直接返回 `ai.AssistantMessage{}` 和 `nil`。对 HTTP 层来说，这会表现成一个 200 成功但内容为空的响应。`POST /chat/stream` 在同样场景下也只是把消息入队，然后立刻返回一个被关闭的 channel，客户端拿不到本次请求对应的流式结果。

这个行为更像“同一个交互会话里的后续追问”，不适合作为多客户端 HTTP 服务的默认并发模型。当前实现会把彼此独立的 HTTP 请求串到同一条 agent 对话里，既不隔离，也不返回正确结果。

建议：
- 服务端按请求或按 `session_id` 创建/获取独立 agent，而不是全局复用一个实例。
- 如果暂时不支持并发复用，至少在 agent 忙时返回显式的 `409 Conflict` 或 `503 Service Unavailable`，不要返回空成功结果。
- 为 `/chat` 和 `/chat/stream` 增加并发测试，覆盖“第二个请求在第一个尚未完成时到达”的场景。

### [Medium] ~~`/chat` 和 `/chat/stream` 定义了 `session_id / model / max_turns`，但实际上完全没有生效~~ ✅ 已修复

位置：
- [internal/server/server.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/server/server.go:23)
- [internal/server/server.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/server/server.go:75)
- [internal/server/server.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/server/server.go:102)

`ChatRequest` 明确定义了 `SessionID`、`Model`、`MaxTurns`，但两个 handler 只使用了 `Prompt` 字段，剩余参数完全被忽略。对外暴露了这些字段之后，调用方会自然认为它们可用；但现在无论传什么值，行为都不会变化。

这会带来两个问题：
- API 契约与真实行为不一致，容易误导调用方。
- 也解释了为什么当前 HTTP 层并没有真正做到“按会话恢复上下文”或“按请求切模型/回合数”。

建议：
- 要么真正把这些字段接到 agent 构建/查找逻辑里。
- 要么在接口层先删除这些字段，避免形成伪能力。
- 为这几个参数补行为测试，防止以后再次出现“结构体有字段但 handler 不消费”的回归。

### [Medium] ~~SSE 流结束事件会被发送两次~~ ✅ 已修复

位置：
- [internal/agent/agent.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/agent/agent.go:217)
- [internal/server/server.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/server/server.go:130)
- [internal/server/server.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/server/server.go:143)

`Agent.PromptStream()` 在结束时已经向 channel 写入了一个 `StreamEventDone`。但 `chatStream()` 消费完整个 channel 后，又手动补发了一次 `event: done\ndata: {}\n\n`。因此客户端会先收到一个带最终消息的 `done` 事件，再收到一个空的 `done` 事件。

如果前端或 SDK 以“遇到 `done` 就关闭状态机”为约定，第二个 `done` 很容易触发重复收尾、覆盖最终消息，或者让解析逻辑变复杂。

建议：
- 保留一种 done 信号即可，优先沿用 `AgentStreamEvent{Type: done}` 这一统一格式。
- 给流式接口补一个测试，断言 `done` 事件只出现一次。

### [Low] ~~`grep.show_context` 参数是伪实现~~ ✅ 已修复

位置：
- [internal/tools/grep.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/tools/grep.go:27)
- [internal/tools/grep.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/tools/grep.go:121)

`GrepParams` 暴露了 `show_context`，描述上说会输出匹配上下文，但实际格式化输出时 `if params.ShowContext > 0 { ... } else { ... }` 的两个分支完全相同，既没有缓存上下文行，也没有改变输出结构。

这类“参数存在但没有行为”的问题严重度不高，但很容易让上层 agent 或 API 使用方对工具能力产生错误预期。

建议：
- 要么实现上下文行输出。
- 要么移除该参数，保持工具能力描述和真实行为一致。

## Open Questions

- `server` 这一层的目标到底是“单用户会话网关”还是“多客户端 API 服务”？这会直接决定 agent 生命周期、并发模型和 session 设计。
- session 未来如果要支持树状分支，`Session.AppendMessage()` 这层可能需要显式返回新 entry ID，而不是把链路管理藏在 storage 里。

## Summary

当前项目的主体结构是清楚的，测试也大体稳定，但 `session` 和 `server` 两层还存在几个会直接影响外部行为的关键问题。优先级上我建议先修：

1. session 链路写入与恢复不一致。
2. HTTP 服务复用单个 agent 导致并发语义错误。
3. API 字段与真实能力不一致。
