---
status: approved
author: plan-agent
created: 2026-05-25
updated: 2026-05-25
depends-on:
  - dev/coding-agent/spec.md
  - dev/coding-agent-cli-control-plane/execution-plan.md
  - decisions/skills-vs-application.md
  - research/codex-rust-cli-analysis.md
  - research/cc-haha-core-engine-analysis.md
---

# Coding Agent Slash Hardening 执行文档

> 目标：在已完成的 CLI control plane 基础上，补齐下一批真正高价值的 slash commands，重点解决“任务不清、上下文不清、压缩不可控”这三个日常使用问题。  
> 本文档供执行 agent 直接施工使用。

---

## 1. 为什么现在做这件事

上一阶段已经完成了 CLI 控制面的第一轮建设：

- slash command 框架成型
- `/new` / `/switch` 可用
- `/model` / `/profiles` / `/profile` 已接通
- CLI 状态行已经能展示 session / model / profile

这意味着 `coding-agent` 已经不再只是“能聊天”的原型，而是具备了基本控制面。

但从真实使用角度看，当前还缺几类非常关键的命令：

1. **任务控制**  
   用户缺少一种显式方式告诉 agent：“这一段时间你围绕什么目标工作。”

2. **上下文可见性**  
   用户看不到当前 session 的完整运行时状态，只能零散地靠 `/session`、`/tools`、`/model` 拼起来。

3. **上下文治理**  
   `/compact` 还是占位，长会话失控时，用户没有可靠的手动收口方式。

对比 Codex / cc-haha，这一层恰恰是成熟 coding-agent 的共同点：

- 命令不一定多，但高价值命令一定能解决“跑偏、看不清、控不住”
- 真正有用的是控制面硬化，不是无差别堆命令

所以这一步的目标不是“扩 slash 数量”，而是：

**补齐下一批真正值得做的 slash commands。**

---

## 2. 这次要做什么

本次主题聚焦 5 件事：

1. 新增 `/goal`
2. 新增 `/context`
3. 让 `/compact` 从占位变成真实能力
4. 新增 `/models`
5. 新增 `/clear`

同时顺手收口两件事：

- `/branch` 如果仍无真实实现，继续明确标注 `planned`
- slash help 分组与描述要反映新的命令面，而不是继续堆在一起

---

## 3. 这次不要做什么

### 不要做

- 不要把 Codex 的整套命令体系硬搬过来
- 不要现在做复杂的 `/goal create/update/list/archive`
- 不要在这一步补 `/pwd`
- 不要引入 `/permissions`、`/plugins`、`/jobs`、`/doctor`
- 不要扩展到多 agent orchestration 命令
- 不要顺手做新的工具域

### 说明

`/pwd` 这类命令现在优先级不高。

原因：

- 已有 `bash`
- `cc` 这类产品也更倾向让用户直接进入命令执行，而不是单独做 `/pwd`
- 当前更值钱的是任务与上下文治理

---

## 4. 当前问题总结

### 4.1 缺少显式任务命令

现在用户没有内建方式设置“当前目标”。

这会带来两个问题：

- 长会话容易跑偏
- 目标只能混在自然语言历史里，不能被显式查看和更新

### 4.2 当前 session 可见性不够完整

已有：

- `/session`
- `/tools`
- `/model`
- `/profiles`

但用户仍然很难一眼看清：

- 当前 goal
- 当前 profile
- 当前工具集
- 当前 session 基本状态
- 当前 compact 状态

这正是 `/context` 应该解决的问题。

### 4.3 `/compact` 还是占位

当前 `/compact` 仍然只是提示语，不是实际能力。

这会直接影响长会话的可控性：

- 用户知道上下文太长了
- 但没有显式方法让系统收口

### 4.4 模型切换可发现性仍然一般

现在有 `/model`，但没有 `/models`。  
这意味着用户知道“能切”，却不知道“能切到什么”。

### 4.5 终端体验还缺一个低成本高收益命令

`/clear` 虽然不是核心运行时能力，但对 CLI 很实用：

- 清理展示
- 不影响 session
- 不污染会话状态

---

## 5. 命令优先级

## 5.1 P0 — 必做

### `/goal`

第一版建议支持：

- `/goal`：查看当前 goal
- `/goal <text>`：设置当前 goal
- `/goal clear`：清空当前 goal

这不是复杂任务管理系统。  
第一版 goal 的定位只是：

- 当前 session 的显式任务说明
- 可展示
- 可注入 prompt

### `/context`

建议作为聚合信息命令，展示至少这些内容：

- session id
- provider/model
- profile
- goal
- tool summary
- cwd / workspace
- message count（如果当前容易拿到）

如果其中一两项当前拿不到，可以先做最小版，但 goal / profile / model / tools 应尽量齐。

### `/compact`

第一版至少要从占位变成真实行为。

推荐两种可接受实现：

1. **真实触发手动 compact**
2. **如果当前 runtime 尚不能安全手动 compact，则至少输出真实 compact 状态，并明确为什么当前不支持强制执行**

但不要继续停留在纯占位文案。

## 5.2 P1 — 值得顺手做

### `/models`

作用：

- 列出当前可切换模型
- 让 `/model` 不再是“知道名字的人才能用”

建议输出：

- provider
- model id
- human-readable name（如果已有）

### `/clear`

作用：

- 清空当前终端展示
- 不影响 session / history / state

如果 interactive CLI 里实现最方便，就放在 interactive CLI；  
但从用户角度仍通过 slash command 触发。

---

## 6. `/goal` 设计要求

## 6.1 第一版 goal 的边界

不要做成 Codex 那种更完整的目标系统。  
第一版 goal 只做：

- 当前 session 的单个 goal 字符串

也就是说：

- 不做多 goal 列表
- 不做状态流转
- 不做 goal 历史
- 不做复杂 CRUD

## 6.2 存储位置建议

建议挂在 `runtime.AgentSession` 上，作为 session 级运行时状态。

推荐接口方向：

```go
Goal() string
SetGoal(goal string)
ClearGoal()
```

如果你觉得 `SetGoal("")` 足够，那也可以不单独做 `ClearGoal()`。

## 6.3 prompt 接入建议

goal 要真正有价值，就不能只是展示字段。  
建议在 `PromptBuildOptions` 增加 `Goal`，并由 coding prompt builder 在 system prompt 中加入简短 goal 区段。

要求：

- 简短
- 明确
- 不与 profile 冲突
- 不要把 system prompt 再做得很长

例如：

```text
Current goal:
- Review the recent changes in the authentication flow and identify regressions.
```

---

## 7. `/context` 设计要求

`/context` 不等于 `/session` 的重复版。  
它应该是一个聚合视图。

第一版建议至少输出：

- Session
- Model
- Profile
- Goal
- Tools
- CWD / Workspace

如果实现成本可控，可选再加：

- Message count
- Last active
- Compact status

### 设计原则

`/context` 的价值在于“一眼看全”，所以输出格式应稳定、分行、易扫读。

---

## 8. `/compact` 设计要求

这一步最需要克制。

如果当前 runtime 还不支持真正安全的“手动压缩当前上下文”，那不要硬做危险实现。

推荐优先级：

1. 优先评估现有 `AgentSession.Compact(...)` 能否安全落地
2. 如果能落地，就让 `/compact` 调真实能力
3. 如果暂时还不能安全落地，就把 `/compact` 改成：
   - 显示当前 compact 能力状态
   - 解释自动 compact 何时触发
   - 明确“手动 compact 暂未启用”的真实原因

### 底线

不要继续保留“未来会支持”的空话式占位文案。  
必须变成一个真实命令，要么执行真实逻辑，要么输出真实状态。

---

## 9. `/models` 设计要求

当前系统已经有模型列表来源，至少 server 侧已有模型清单概念。  
这次 CLI 应直接复用已有能力，而不是另造一套模型枚举逻辑。

第一版只需要做到：

- `/models` 能列出当前可切换模型
- `/model` 仍负责查看当前模型和切换目标模型

如果 CLI 当前还没有直接的 model catalog 访问接口，可以在 app/runtime 层补一个轻量读取接口。  
但不要为了这个命令大改 provider 架构。

---

## 10. `/clear` 设计要求

`/clear` 的本质是展示层命令，不是运行时命令。

建议实现原则：

- 不修改 session
- 不修改 message history
- 不修改 goal / model / profile
- 只清理当前终端显示

interactive CLI 中处理即可，但触发方式仍然是 slash command。

如果 terminal clear 做法存在平台差异，可以先做最小兼容实现。

---

## 11. 推荐接口增量

执行 agent 可以按最小原则补接口，但建议至少考虑这些能力：

### SessionContext

```go
Goal() string
SetGoal(goal string)
ClearGoal()
```

### AppContext 或其他只读能力

如果 `/models` 需要统一入口，建议增加一个轻量模型列表读取接口。  
具体放 `AppContext` 还是 session/app 其他位置，由执行 agent 结合当前结构判断，但要保持职责清晰。

---

## 12. 推荐执行顺序

### 第一步：补 runtime/session 级 goal 状态

先把 goal 的最小状态和 prompt 注入链打通。

### 第二步：实现 `/goal`

这是最值得先完成的命令，因为它直接改善“跑偏”问题。

### 第三步：实现 `/context`

复用已有 session/model/profile/tools 能力，把 goal 聚合进去。

### 第四步：把 `/compact` 从占位变成真实命令

这一步优先收口真实性，不要求一步做到最强。

### 第五步：实现 `/models`

提高模型切换的可发现性。

### 第六步：实现 `/clear`

收尾提升 CLI 使用体验。

---

## 13. 测试要求

至少应补这些测试：

- `/goal` 设置、读取、清除
- goal 进入 prompt 构建链
- `/context` 输出包含 goal / profile / model / tools
- `/models` 输出非空且格式稳定
- `/clear` 不修改 session 运行时状态
- `/compact` 不再只是旧占位字符串

如果 `/compact` 做成真实 compact 行为，还需要补对应单测或集成测试。

---

## 14. 验收标准

本次完成后，至少应满足：

1. 支持 `/goal`
2. goal 能进入 prompt 行为链，而不只是显示字段
3. 支持 `/context`
4. `/context` 至少展示 session / model / profile / goal / tools
5. `/compact` 不再只是旧占位文案
6. 支持 `/models`
7. 支持 `/clear`
8. `/branch` 如果仍未实现，继续明确标注 `planned`
9. `go test ./...` 通过
10. 与本次改动相关的新增测试存在，且不是只靠人工验证

---

## 15. 一句话总结

这一步不是“继续加 slash 数量”，而是：

**补上 `coding-agent` 最缺的那几条命令，让 CLI 在任务约束、上下文可见性和上下文治理上真正可用。**
