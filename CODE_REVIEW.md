# Pi-Go 代码评审

日期：2026-05-20

这次按“复看修复结果”的方式重新检查了一遍，之前提到的几项里，`session` 持久化主链路、HTTP 参数误导、SSE 重复 done、`grep.show_context` 伪实现这几处已经修好了。下面只保留当前代码里仍然客观存在的问题。

## Findings

### ~~[High] `/chat` 和 `/chat/stream` 的”agent busy”检查不是原子的，竞态下仍可能返回空成功结果~~ ✅ 已修复

位置：
- [internal/server/server.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/server/server.go:75)
- [internal/server/server.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/server/server.go:102)
- [internal/agent/agent.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/agent/agent.go:104)
- [internal/agent/agent.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/agent/agent.go:131)

`server` 现在会先调用 `s.agent.State()`，如果是 `StateRunning` 就返回 `409`。这个方向是对的，但检查和真正进入 `Prompt()` / `PromptStream()` 之间没有共享同一把锁，所以它不是原子操作：

1. 两个请求几乎同时进入 handler。
2. 两边都先看到 `StateIdle`。
3. 第一个请求进入 `Prompt()`，把状态切到 `StateRunning`。
4. 第二个请求随后进入 `Prompt()`，命中 `StateRunning` 分支，被塞进 `followUpQueue` 后直接返回 `ai.AssistantMessage{}` 和 `nil`。

结果就是：虽然 handler 里已经加了 “busy” 判断，但在竞态窗口里，第二个 `/chat` 请求依然可能返回 `200` 且内容为空；`/chat/stream` 也可能拿到一个立即关闭的空流。

建议：
- 把“检查是否忙”和“开始一次新请求”合并成 agent 里的单个原子入口，而不是在 server 外围先查状态。
- 或者让 `Prompt()` / `PromptStream()` 在忙时返回显式错误，由 server 统一映射成 `409`。
- 补一个并发测试，专门覆盖两个请求同时命中的场景。

### ~~[Medium] `Session.MoveTo()` 没有同步更新内存中的 `leafID`，切分支后继续写会挂到旧链上~~ ✅ 已修复

位置：
- [internal/session/session.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/session/session.go:76)
- [internal/session/session.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/session/session.go:27)

`MoveTo()` 会把 storage 里的 leaf 移到新的 `entryID`，但没有更新 `Session` 结构体里的 `s.leafID`。而后续 `AppendMessage()` 构造新 entry 时，父节点取的是内存中的 `s.leafID`：

```go
entry := Entry{Type: EntryTypeMessage, ParentID: s.leafID}
```

所以如果将来在同一个 `Session` 实例上调用过 `MoveTo()`，后续新增消息仍会以旧 leaf 为父节点，分支切换不会真正生效。

这个问题当前还没在主流程里暴露，是因为现有代码几乎没用到 `MoveTo()`；但从 API 语义上看，这是一个真实的状态不一致。

建议：
- `MoveTo()` 成功后同步更新 `s.leafID = entryID`。
- 如果 `summary` 也要参与链路，确认它是否也应该更新 leaf，避免 branch summary 成为悬空 entry。

## 验证

我重新跑了测试：

```bash
go test ./...
```

当前结果通过。

## Summary

这版相比上次已经收敛很多了，之前几处比较明显的问题确实已经修掉。现在我认为还值得优先处理的，主要就是：

1. server 到 agent 之间的并发竞态还没彻底闭环。
2. `Session.MoveTo()` 的内存状态没有和 storage 状态保持一致。
