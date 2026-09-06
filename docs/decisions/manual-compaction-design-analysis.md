---
status: approved
author: codex
created: 2026-05-25
updated: 2026-05-25
depends-on:
  - research/codex-rust-cli-analysis.md
  - research/cc-haha-core-engine-analysis.md
  - ../archive/pi-go-analysis.md
  - ../research/claude-code-plugins-hooks-analysis.md
---

# Manual Compaction Design Analysis

> 目标：基于现有调研与 `pi-go` 当前实现，回答一个具体问题：`pi-go` 是否应该把 `/compact` 做成真正的手动压缩命令，如果做，应该做成什么形态。

---

## 1. 结论摘要

### 结论

`pi-go` **现在不应该直接做一个“立即执行、原地改写历史”的 `/compact now`**。

更合理的路线是：

1. **短期**
   - `/compact` 做成真实的“上下文治理入口”，但不是伪装成已执行的假命令
   - 明确展示当前 compaction 策略、goal、风险与建议动作
   - 支持一种**安全的手动触发方式**，但触发点放在**下一次 prompt 前**，而不是在 slash 命令当下直接改写 agent 内存状态

2. **中期**
   - 增加 `PreCompact` 类钩子或“保留信息”机制
   - 让手动 compact 成为“下一轮前先压缩”的显式控制，而不是黑盒副作用

3. **当前阶段不要做**
   - 不要做“命令一执行就立刻重写当前 agent 历史”的强行 compact
   - 不要继续保留误导性的 `requested` 文案

### 推荐方案

在本文末尾的方案对比里，我建议选择：

- **D. 分阶段路线**

即：

- **Phase 1**：把 `/compact` 改造成真实的“manual compact request”机制
- **Phase 2**：补 `PreCompact` 保留信息与更细粒度压缩策略
- **Phase 3**：如果后续仍然需要，再考虑更强的立即压缩语义

---

## 2. 先回答核心问题

### 2.1 成熟产品是否暴露“手动 compact 命令”？

**已确认事实**

- 从现有本地调研材料看，`Codex` 和 `cc-haha` 的重点都在：
  - 自动压缩
  - 多级压缩策略
  - 压缩前后的生命周期钩子
  - goal / context 管理
- 现有材料**没有给出“用户显式输入 `/compact now` 并立即压缩历史”的直接证据**。

**推断**

- 至少从我们当前掌握的材料看，成熟 coding-agent 更强调：
  - 自动治理
  - 渐进压缩
  - 目标和上下文控制
  - 压缩前保留关键信息
- 而不是把“原地立刻压缩”暴露成一个高频用户命令

**产品判断**

- 这说明 `/compact` 真正有价值的方向不是“像清缓存一样立刻执行”，而是“给用户一个安全、透明、可控的上下文治理入口”。

---

## 3. 各系统对比

## 3.1 `pi-go` 当前实现

### 已确认事实

- `pi-go` 已有自动 compaction 主链：
  - [internal/agent/loop.go](../../sdk/agent/loop.go)
  - `maybeCompact()` 会在上下文接近窗口限制时触发摘要压缩
- `runtime.AgentSession.Compact()` 目前仍是 placeholder：
  - [internal/runtime/agent_session.go](../../sdk/runtime/agent_session.go)
- 当前 `/compact` 命令还没有接入真实 compact 行为：
  - [internal/agents/coding/commands/builtins.go](../../internal/agents/coding/commands/builtins.go)

### 当前问题

1. 自动压缩存在，但用户没有真正的显式治理入口
2. `/compact` 名字很强，但实际能力很弱
3. 一旦把它做成“假装已经触发”，会误导用户

---

## 3.2 Claude Code / `cc-haha`

### 已确认事实

来自 [docs/research/cc-haha-core-engine-analysis.md](../research/cc-haha-core-engine-analysis.md) 与 [claude-code-plugins-hooks-analysis.md](../research/claude-code-plugins-hooks-analysis.md)：

- `cc-haha` 有 **4 级渐进压缩策略**：
  - snip
  - microcompact
  - context collapse
  - autocompact
- Claude Code 系体系里存在 `PreCompact` hook
- `PreCompact` 的意义是：
  - 在压缩前注入“哪些信息必须保留”
  - 降低摘要压缩丢关键上下文的风险

### 推断

- `CC`/`cc-haha` 的设计重心更像是：
  - 把压缩做成运行时内部治理能力
  - 同时开放压缩前的保留控制点
- 而不是把手动 compact 设计成一个粗暴的即时命令

### 对 `pi-go` 的启发

- 真正值得学的不是“有没有 `/compact` 这个命令名”
- 而是：
  - 压缩有生命周期
  - 压缩前允许保留关键目标/约束/摘要
  - 压缩最好是渐进式，而不是一步到位全靠 LLM 总结

---

## 3.3 Codex Rust CLI

### 已确认事实

来自 [docs/research/codex-rust-cli-analysis.md](../research/codex-rust-cli-analysis.md)：

- Codex 有 `goal` 管理能力
- Codex 有 `auto_compact`
- Codex 的 compact 体系有阶段区分：
  - pre-turn
  - remote / API-based compaction
- 调研材料还提到 `PreCompact / PostCompact`

### 推断

- Codex 更像是把 compact 视为 session/turn runtime 的内建能力
- 用户显式治理更偏向：
  - goal
  - turn context
  - tool/runtime policy
- 而不是靠一个立即执行的 slash 命令去重写对话历史

### 对 `pi-go` 的启发

- `goal` 和 `context` 命令先做起来是对的
- `/compact` 也更适合变成：
  - runtime-aware 的治理命令
  - 而不是字符串层面的“伪执行反馈”

---

## 4. 真正的设计分歧在哪里

`/compact` 现在有 3 种可能意义：

1. **状态查看**
   - 看当前会话是否会自动压缩
   - 看当前 goal / 风险 / 建议动作

2. **请求下一轮前压缩**
   - 设置一个“下一次 prompt 前先 compact”的控制位
   - 由 runtime 在安全时机执行

3. **立即原地压缩**
   - 当前 slash 命令一执行，马上改写 agent 历史

我认为这三者里：

- 1 太弱，长期不够
- 3 风险太高，当前阶段不合适
- **2 是现在最平衡的方案**

---

## 5. 为什么不建议现在直接做“立即原地压缩”

### 5.1 当前架构下，手动 compact 不是一个纯展示动作

`pi-go` 现在的 compaction 发生在 agent loop 内，和这些因素耦合：

- 当前 message history
- token window
- summarizeFunc
- session persistence
- stream event

如果 slash 命令直接在 loop 外强行改写历史，会带来几个风险：

1. **状态不同步**
   - 当前 agent 内存里的历史
   - session 持久化里的历史
   - 下一轮 prompt 构造时看到的历史

2. **语义不透明**
   - 用户不知道哪些内容被折叠了
   - 也不知道压缩后是否还能恢复

3. **时机不安全**
   - 如果正在执行工具、正在流式输出、或刚发生多步工具调用，立即压缩容易破坏后续判断

### 5.2 你们现在还缺一个关键保护点

从现有调研看，成熟方案里很重要的一层是：

- `PreCompact`
- 或其他“保留信息”机制

而 `pi-go` 现在还没有这一层。  
这意味着如果现在就开放强行手动 compact，很容易让用户把关键 goal、限制条件、最近工具结论一起压掉。

---

## 6. 推荐设计

## 6.1 推荐的 `/compact` 第一阶段语义

我建议把 `/compact` 设计成：

### `/compact`

显示真实上下文治理状态，包括：

- auto-compaction 是否开启
- 当前 goal
- 当前 profile
- 是否已有待执行的 manual compact request
- 简短说明：manual compact 会在下一次 prompt 前执行，而不是当前命令立即改写历史

### `/compact now`

不是“现在立刻压缩”，而是：

- 在当前 session 上设置一个 `pendingManualCompact` 标志与 reason
- 下一次用户真正发送 prompt 时，runtime 在进入 agent loop 前或第一轮前先执行一次 compact

### `/compact clear`

- 清除待执行的 manual compact request

这个方案的好处是：

1. **是真实行为，不是假文案**
2. **执行时机安全**
3. **用户心智清晰**
4. **和当前 runtime 架构更兼容**

---

## 6.2 这个方案需要哪些代码改动

第一阶段不需要重做整个 compaction 架构，只需要：

1. 在 `AgentSession` 增加类似：
   - `pendingManualCompact bool`
   - `manualCompactReason string`

2. 在 slash command 层新增：
   - `/compact`
   - `/compact now [reason]`
   - `/compact clear`

3. 在 prompt/turn 执行入口加一个安全触发点：
   - 如果有 pending manual compact
   - 则在下一次 prompt 进入 agent loop 前先执行 compact
   - 执行成功后清掉 pending 标记

4. 补充 stream / CLI 提示：
   - 告知用户“manual compact executed”
   - 或“manual compact failed”

---

## 7. Phase 2 应该补什么

如果第一阶段跑顺，第二阶段最值得补的是：

### 7.1 `PreCompact` 保留机制

至少让 runtime 在 compact 前显式保留：

- 当前 goal
- 当前 profile 下的重要约束
- 最近关键工具结论

这能明显降低信息丢失风险。

### 7.2 更细粒度压缩策略

参照 `cc-haha` 的启发，`pi-go` 未来不一定只靠一层 LLM 摘要：

- 轻量裁剪
- tool result 缩写
- collapse/projection
- 最后才是 LLM summarize

---

## 8. 不推荐的方案

### A. 保持现在这种“状态文案型 `/compact`”

不推荐。  
原因：名字太强，能力太弱，长期会持续误导。

### B. 直接实现“slash 一执行就原地 compact”

也不推荐。  
原因：当前 runtime 与 session/history 结构下，风险高于收益。

### C. 彻底删除 `/compact`

也不推荐。  
因为你们已经进入强 CLI 控制面阶段，上下文治理入口本身是有价值的，只是语义要做对。

---

## 9. 最终建议

### 最终建议：选 D，分阶段路线

#### Phase 1

把 `/compact` 做成 **真实的 manual compact request 入口**：

- `/compact` 看状态
- `/compact now [reason]` 请求下一轮前压缩
- `/compact clear` 取消请求

#### Phase 2

补 `PreCompact` 保留信息机制。

#### Phase 3

如果未来还需要，再评估是否值得支持更强的“立即压缩”语义。

---

## 10. 一句话结论

`pi-go` 现在最不该做的是“继续假装 `/compact` 已经有真实动作”，也不该直接跳去做危险的即时压缩。  
最合理的下一步，是把 `/compact` 做成一个**真实但安全的手动压缩请求机制**，并在下一轮 prompt 前执行。
