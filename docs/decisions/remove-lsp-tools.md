# 决策：移除 LSP 工具集

日期：2026-09-07
状态：已实施（`refactor/remove-lsp`）

## 背景

SDK 抽取（见 [sdk-extraction.md](sdk-extraction.md)）后复查边界时发现：`sdk/lsp`（LSP 客户端）和 `sdk/tools/lsp.go`（6 个 LSP 工具）是搬家依赖闭包拖进 SDK 的——全仓库只有 coding 应用注册它们。由此引出根本问题：**LSP 是不是 agent 的基础能力，甚至是不是 coding agent 的必备能力？**

## 证据（三家头部产品的同一答案）

| 产品 | LSP | 代码理解方式 |
|---|---|---|
| pi（pi.dev） | 无，连扩展生态都没有；官方哲学 "Primitives, not features" | grep + read |
| OpenAI Codex（codex-rs） | 无，Rust 核心全源码树零 LSP 痕迹 | shell + apply_patch + 沙箱 |
| Claude Code | 无，工具集为 Read/Edit/Bash/Grep/Glob 等 | grep + 读文件 |

行业配方收敛为：**shell + 搜索 + 读文件 + 结构化编辑 + 沙箱**。LSP 的工程账算不平：每语言一个 server 进程、冷启动慢、有状态、难沙箱；换来的精确导航，grep + 读整个文件基本可覆盖，且模型读代码能力持续增强——成本确定且持续，收益递减。

## 决策

**整体移除**（不是降级到应用层）：删除 `sdk/lsp/`（8 文件）与 `sdk/tools/lsp.go`（6 工具）及其注册、退出清理逻辑。理由：既然连 coding agent 都不需要，留在应用层也只是无人维护的负担；SDK 边界回归"语言级通用原语"。

若未来出现真实反例场景（如超大型代码库上 LSP 显著省 token），以**数据**重新提案，作为可选扩展设计，而不是默认内置。

## 影响

- coding agent 工具集回到 7+1 件套（read/read_many/write/edit/bash/grep/find/ls + web/ask 等），行为对齐 pi/codex 的原语路线
- `Workspace` 选项保留（read_many_files 仍使用）
- 测试：39 包全绿（原 40 包中的 sdk/lsp 随包删除）
- 与本决策相关的既有文档（sdk-extraction.md 中"lsp 进 SDK"的表述）已同步标注
