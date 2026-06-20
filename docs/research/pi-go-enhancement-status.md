# pi-go 增强进度：P0-P2 落地状态

> 更新日期：2026-06-20
> 背景：基于 DeepVcodeClient 差距分析（参考 `deepv-code-full-analysis.md`、`cc-haha-web-fetch-analysis.md`）得出的 P0-P4 优先级，本文记录已落地的增强项及其 commit/PR，作为当前能力基线。
> 注：原始的 `deepvcode-gap-deep-analysis.md`（P0 前的深度差距分析）在分支切换中丢失，未重建——其"差距"结论已部分过时（多项已不再是差距），本文以"落地状态"替代，更实用。

---

## 已落地（P0-P2）

### P0：工具确认机制 + ToolResult 字段分离 ✅

- **commit**: `9bafe3f`（随 music PR #16/#17 进入 main）
- **内容**:
  - `ToolWithConfirmation` 可选接口（duck-typing，危险工具执行前确认）
  - bash 危险命令检测、write/edit 无条件确认；serve/feishu 默认放行，chat 注入 ConfirmFunc 读 stdin
  - 拒绝语义：阻断 + 回告 LLM（`IsError=false` 避免 Agent 误判重试）
  - `ToolResult` 加 `UserFacing` 字段 + `DisplayText()` 方法（分离"给用户看"和"给模型看"）

### P1：循环检测 + 会话级 Hook + 压缩 bugfix ✅

- **PR**: [#19](https://github.com/hwj123hwj/pi-go/pull/19)（3 个 commit，已合并）

**循环检测**（`fa56beb`/`7dd32be`）:
- 连续相同 tool call（SHA256 指纹 name:args）达阈值 5 → 注入提醒到 followUpQueue，柔性不中断
- per-prompt 重置，参考 DeepV loopDetectionService 第 1 层算法（砍掉 content/LLM 确认）

**会话级观察型 Hook**（`af7774c`/`ed7d35f`）:
- `SessionStartHook`/`SessionEndHook`/`PreCompressHook`，全部观察型（error 仅 log 不阻断）
- 复用 LifecycleHooks 注册链，Extension 贡献走新独立接口
- 砍掉 BeforeModel/AfterModel（无 Proxy 价值打折）等

**SplitMessages bugfix**（`0eef152`/`ca48ce1`）:
- 调试 Hook 时发现的既有 bug：旧版只在 user→assistant 边界切割，工具调用场景无法压缩
- 修复：接受任意完整 turn 边界（tool 结果后、纯文本 assistant 后），不割裂 tool_call 配对

### P2：web_fetch + MicroCompact ✅

**web_fetch 内置工具** — [PR #21](https://github.com/hwj123hwj/pi-go/pull/21)（`47dccdb`，已合并）:
- 参照 cc-haha（=Claude Code 官方），不学 DeepV（DeepV 连 HTML→markdown 都不做）
- 结构化参数 `{url, prompt?}`，html-to-markdown 库转换
- 三层长度控制 + SSRF 双重防护（入口 isPrivateHost + CheckRedirect 每跳校验）
- 默认关闭 `EnableWeb`（和 bash 一致的安全默认）
- **不做 web_search**（编程 agent 低频，用户给链接更准）

**MicroCompact** — [PR #22](https://github.com/hwj123hwj/pi-go/pull/22)（`f2063f6`，已合并）:
- 参照 cc-haha microCompact.ts：清旧 tool result，**不调 LLM**（非分级摘要——调研后纠正了原设想）
- 两级阈值：60% 触发 Micro（清旧 read/bash/grep/find/ls/web_fetch result，保留最近 5 个）→ 90% 才全量 AutoCompact
- ToolCallID→工具名 回溯关联判断可压缩性；防膨胀（占位符更长时跳过）
- 不持久化到 session（幂等重清理）

---

## 暂缓 / 未做（P3-P4）

按差距分析原优先级，以下明确不在当前主线：

| 项 | 原优先级 | 状态 | 说明 |
|---|---|---|---|
| 子 Agent / SubAgent | P3（已降级） | 未做 | pi-go 当前无多 Agent 场景刚需；DeepV 的 4 Agent 类型 + TaskTool 工作量大 |
| MCP 协议支持 | P4 | 未做 | 工作量大，按需。DeepV 有完整 MCP（3 Transport + OAuth） |
| LSP 客户端 | P4 | 未做 | 按需。Go 有 lsp 库可用 |
| 沙箱执行 | P4 | 未做 | 按需。DeepV 继承自 gemini-cli（docker/podman/macOS-seatbelt） |
| PostCompact 恢复 | P2 后续 | 未做 | MicroCompact 已落地，全量摘要后的"恢复文件内容"留后续 |
| web_fetch 精细化 | P2 后续 | 未做 | 跨域重定向精细处理、域名白名单确认机制留后续 |

---

## pi-go 不可丢弃的结构性优势（保持）

这些是 DeepVcodeClient 用 fork 路径换不来的，增强任何能力时都需守住：

1. **`runtime.Application` 解耦点** — Platform↔App 通过 BuildTools/BuildPrompt/NewSessionExt 解耦，同一 Platform 跑多个 Application
2. **Go 单二进制** — 零运行时依赖（DeepV 需 Node.js 20+）
3. **Provider 注册制** — 直连厂商，无代理依赖（DeepV 的 Proxy 是商业模式产物）
4. **`operations.Operations` 抽象** — Local/SSH 无缝切换
5. **JSONL 流式会话** — 树状存储 + MoveTo 分支（DeepV 用 JSON 整体加载）
6. **goal-driven loop** — 已实现的长任务目标驱动

---

## 关联文档

- `docs/research/deepv-code-full-analysis.md` — DeepV 功能清单层分析（2026-05-24，旧）
- `docs/research/cc-haha-web-fetch-analysis.md` — cc-haha web_fetch 源码级分析（web_fetch 实现参照）
- `docs/decisions/deepvcode-essence-absorption.md` — 阶段性吸收建议（web 部分已更新：只 web_fetch、砍 web_search）
