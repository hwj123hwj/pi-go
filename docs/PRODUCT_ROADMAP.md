# Pi-Go 产品与技术规划

> 目标：把 `pi-go` 从当前的可运行原型，逐步推进到三个可落地的最终形态：
> 1. 类似 Claude Code 的强 `CLI`
> 2. 类似 OpenHanako 的桌面端应用（不自研编辑器）
> 3. 运行在服务器端、通过飞书使用的 Agent 服务

## 1. 产品判断

### 1.1 最终不是三套系统，而是一个内核、三个入口

`pi-go` 后续不应分别做三套能力，而应坚持：

- 一个共用 Agent Runtime 内核
- 三个交付入口：
  - `CLI`
  - `Desktop`
  - `Server + Feishu`

这意味着后续所有重要能力都应优先沉淀在共用层，而不是写死在某一个入口里：

- session 与 session registry
- model/provider 管理
- tool lifecycle 与 tool execution
- slash command
- compaction
- skill / extension
- 审计、日志、权限控制

### 1.2 三种形态的角色分工

#### A. CLI

定位：面向开发者的主生产力入口。

适合场景：

- VS Code 终端
- iTerm / Warp / Terminal
- SSH 远程开发机
- tmux / screen
- 服务器侧直接操作代码仓库

这是最接近 Claude Code 路线、也最值得优先打磨的产品形态。终端场景直接由 CLI 承担，不再单独规划 TUI。

#### B. Desktop

定位：桌面工作台，不做编辑器，重点承载对话、会话、工具执行展示和模型管理。

适合场景：

- 非终端用户
- 演示和产品化展示
- 多会话浏览
- 长消息阅读
- 工具执行可视化

不建议把 Desktop 做成自研 IDE。它更像是 Agent 控制台，而不是代码编辑器。

#### C. Server + Feishu

定位：让 Agent 作为团队服务运行，通过飞书对话和调用。

适合场景：

- 团队共享 Bot
- 代码问答和自动化任务
- 部署在服务器上长期运行
- 与企业工作流整合

这是最适合业务扩展和团队协作的一条线，但前提是底层 runtime、权限与稳定性先足够扎实。

## 2. 当前状态判断

基于当前仓库实现，现状可以概括为：

- Agent 核心循环、session、provider、基础 tools、compaction 已经成型
- Desktop 已有第一版可用壳子
- HTTP/WebSocket 服务能力已具备
- 飞书方向已有参考文档，但尚未形成正式产品模块
- Layering refactor 已完成：Core → Platform → Application → Entrypoints 四层架构落地
- Runtime decoupling 已完成：`runtime.Application` 接口使 Platform 与 Application 解耦
- **Tool lifecycle hooks** 已落地：`BeforeToolCallHook` / `AfterToolCallHook` + `ToolWithPrepareArguments` 可选接口
- **Operations 抽象** 已落地：`LocalOperations` + `SSHOperations` 双后端，工具通过 `BashOperations` / `FileOperations` 接口与执行后端解耦
- **CLI 控制面** 已落地：结构化 slash command（CommandResult）、session 创建/切换（/new /switch）、model 切换（/model）、profile 机制（coding/review，/profile /profiles）、统一命令分发（消灭 /new 特判）

当前最大的现实问题不是”入口不够多”，而是”内核还不够厚”。

核心薄弱点主要集中在：

- tool streaming / partial result 还未打通
- tools 只有基础 7 件套，增强能力不够
- 模型元数据注册表较薄
- 测试更多集中在基础层，入口级和端到端覆盖还不够

结论：

**接下来最重要的不是继续堆新入口，而是先把“共用运行时内核”加厚。**

## 3. 规划原则

### 3.1 优先级原则

后续需求按以下优先级排序：

1. 先做共用内核
2. 再做 CLI 主入口
3. 再稳住 Server + Feishu
4. Desktop 放在产品化和展示层持续打磨

### 3.2 控制复杂度原则

明确不做或后置的事情：

- 不优先自研编辑器
- 不优先做完整 RPC worker 架构
- 不优先追求原版 Pi 的全部高级能力一次性补齐
- 不优先做重型插件市场
- **不做进程沙箱（seatbelt/landlock 级），永不规划**（2026-09-07 定）：安全策略依赖确认门 + policy + 部署层隔离（服务边界/容器/网络），与 pi 的克制哲学一致；后续任何 agent 不得再提议此项

### 3.3 复刻策略

对于原版 Pi 的能力，采取：

- 抄设计思路
- 不强求 1:1 复刻实现路径
- 先做最小可用版，再逐步逼近

## 4. 核心路线图

## Phase 0：稳定当前主干 ✅ **已完成**

目标：把现有代码从”可运行”提升到”可持续演进”。

优先级：`P0`

关键事项：

- 清理当前已知客观问题，确保主分支语义稳定
- 完善 Desktop PR 合并前后的回归检查
- 补关键测试：
  - 多 session
  - 模型切换
  - WebSocket 流程
  - slash command 基础链路
- 把部署链路跑通：
  - GitHub Actions
  - Ubuntu 部署
  - systemd

完成标志：

- ~~`go test ./...` 持续稳定~~ ✅ **已达到** — 全部 PASS，无 fail
- ~~Desktop 主链路可用~~ ✅ **已达到** — `internal/desktop/` 存在，完整 Electron + Vite 项目结构
- ~~服务端可自动部署~~ ✅ **已达到** — `.github/workflows/deploy.yml` 完整链路

## Phase 1：加厚共用运行时内核

目标：让 `pi-go` 真正具备长期承接三种产品形态的基础。

优先级：`P0`

### 4.1 Tool lifecycle ✅ **已完成**

已在 `sdk/agent/tool_lifecycle.go` 落地：
- `BeforeToolCallHook` — 在工具执行前调用，可修改参数或阻止执行
- `AfterToolCallHook` — 在工具执行后调用，可修改结果或标记失败
- `ToolWithPrepareArguments` 可选接口 — 参数规范化和扩充
- `LifecycleHooks` 聚合类型 — 管理 before/after hook 切片
- `AfterHookError` — 保留原始结果的错误包装

后续可补：
- tool partial result 回调链路

### 4.2 Operations 抽象 ✅ **已完成**

已在 `sdk/operations/` 落地：
- `BashOperations` 接口 — 命令执行
- `FileOperations` 接口 — 文件读写
- `LocalOperations` — 本地实现
- `SSHOperations` — 远程实现

已完成”工具对执行后端解耦”的目标。

### 4.3 模型注册表

建议从硬编码映射升级到集中 registry：

- context window
- max tokens
- cost profile
- capability flags

价值：

- 更容易维护 provider/model 扩展
- 为路由、成本统计、能力开关提供依据

### 4.4 工具增强

第一优先补强：

- `bash` partial streaming
- `bash` detached/background 任务能力
- `read` 图片/二进制感知
- `edit` 多 edit batch
- `edit` 更稳的定位与替换能力
- 文件变更串行保护或 mutation queue

完成标志：

- runtime 不再是“只有基础回路”，而是具备可扩展、可远程化、可流式化的 agent 内核

## Phase 2：把 CLI 做成主入口

目标：先把命令行体验做到足够强，让终端场景完全由 CLI 承担。

优先级：`P0`

关键事项：

- ~~强化交互模式输出体验~~ ✅ 基础版已落地
- ~~完善 slash commands~~ ✅ 已落地：
  - `/new` `/sessions` `/session` `/switch` `/model` `/tools` `/compact` `/help` `/profiles` `/profile` `/branch`
  - 结构化 CommandResult、session 交接、统一命令分发
- ~~会话恢复和切换体验~~ ✅ 已落地：`/new` 创建、`/switch` 切换、`SessionRegistry` 缓存复用
- 更好的工具展示：
  - tool start/end
  - 错误提示
  - 截断策略
  - 可选详细模式
- 更清晰的中断、取消、超时反馈

完成标志：

- 不依赖 GUI，开发者已能顺手完成大部分使用场景

## Phase 3：稳定 Server 能力

目标：让服务端形态成为真正可靠的产品底座。

优先级：`P1`

关键事项：

- 把 session-aware server 进一步做扎实
- 完善 WebSocket 协议和错误语义
- 增加服务端配置管理和监控信息
- 补服务端级测试：
  - `/sessions`
  - `/models`
  - `/ws`
  - 并发请求
  - cancel

可选增强：

- 鉴权
- 审计日志
- 请求级限流

完成标志：

- 服务端不仅能给 Desktop 用，也能给 Feishu bridge 或其他客户端稳定复用

## Phase 4：正式接入飞书

目标：把 Agent 变成团队可用服务。

优先级：`P1`

建议路线：

- 优先做独立 `bridge` 进程
- 通过 `pi-go serve` 的 API / WebSocket 与内核通信
- 飞书侧优先使用长连接事件模式

关键事项：

- token 缓存与刷新
- 消息去重
- slash command 本地拦截
- reaction / thinking 状态
- 会话路由：
  - 按用户
  - 按 chat
  - 按 thread
- 权限和安全控制：
  - 能不能开 bash
  - 能不能操作真实仓库
  - 哪些命令允许执行

完成标志：

- 飞书里可以稳定完成对话、工具调用、状态反馈、会话延续

## Phase 5：持续打磨 Desktop

目标：把桌面端做成稳定、耐看、可展示的 Agent 工作台。

优先级：`P2`

定位边界：

- 做 Agent 工作台
- 不做 IDE
- 不和 VS Code 正面竞争

重点方向：

- 设计 token 体系
- 主题系统
- 更好的 tool call 可视化
- 更好的消息阅读体验
- 会话和模型管理体验
- 与本地后端的稳定进程管理

建议风格：

- 保持 `CSS Modules`
- 建轻量设计系统
- 不急于引入 Tailwind/shadcn

完成标志：

- Desktop 成为对外演示、轻度使用、非终端用户的优雅入口

## 5. 优先级矩阵

### P0：现在就该做

- 稳定当前主干
- 加厚 runtime 内核
- 强化 CLI
- 做好测试和部署

### P1：内核稳后立即做

- 稳定服务端协议
- 飞书 bridge
- SSH 远程执行基础版

### P2：作为产品化增强持续推进

- Desktop 体验升级
- 更完整的模型注册表
- 工具增强的高级能力
- 更完整的审计与权限体系

### P3：暂不优先

- 完整 RPC worker 模式
- 自研编辑器
- 重型插件生态
- 全量复刻原版 Pi 的全部高级设施

## 6. 推荐实施顺序

建议按下面顺序推进，不要并行开太多战线：

~~1. 合并并稳定当前 Desktop 分支~~ ✅ **已完成**
~~2. 补关键测试和部署链路~~ ✅ **已完成**
~~3. 做 tool lifecycle~~ ✅ **已完成**
~~4. 做 `Operations + LocalOperations`~~ ✅ **已完成**
~~5. 强化 CLI（控制面：slash commands / session switch / profile）~~ ✅ **已完成**
6. 增强 tools
7. 做 `SSHOperations`
8. 稳定 server
9. 接飞书 bridge
10. 持续打磨 Desktop

## 7. 里程碑定义

### M1：稳定开发者可用版

包含：

- CLI 可稳定使用
- session / model / tool 主链路可靠
- 自动部署可用

### M2：Claude Code 方向初版

包含：

- CLI 成熟
- 基础 SSH 远程执行可用

### M3：团队服务版

包含：

- server 稳定
- Feishu bridge 可用
- 多会话和权限控制可用

### M4：完整产品矩阵初版

包含：

- CLI 主生产力入口
- Desktop 展示与轻量入口
- Server + Feishu 团队服务入口

## 8. 一句话结论

`pi-go` 后续最重要的不是继续分散做入口，而是先把共用内核做厚，再以 `CLI` 为主线，`Server + Feishu` 为服务化方向，`Desktop` 为产品化展示和轻量工作台方向逐步推进。
