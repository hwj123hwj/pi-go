# 决策：SDK 抽取——Core+Platform 公共化为 `sdk/`

日期：2026-09-06
状态：已实施（`refactor/sdk-extraction`）

## 背景

pi-go 的定位从"对齐 pi 的 coding agent"演进为"可扩展 Agent 底座 + 个人助手应用层"。痛点：全部能力锁在 `internal/`（编译器强制私有），外部 Go 模块一个原子能力都拿不到——想在自己的后端服务里嵌入 agent，只能复制代码。参照 pi-mono 的做法（一堆可发布包），pi-go 也应有公共 API 面。

## 决策

1. **搬门牌而非重写**：Core + Platform 的 17 个目录、19 个包（ai/agent/session/sessionmgr/compaction/operations/prompt/skill/extensions/slashcmd/config/util/runtime/tools/hooks/policy/lsp，其中 ai 含 models/providers 两个子包）从 `internal/` `git mv` 到 `sdk/`，import 路径全仓重写。四层架构和依赖方向**不变**，只变物理可见性。
2. **应用层一律不进 SDK**：agents/music/feishu/tui/server 等领域包留在 `internal/`，应用层是 SDK 的消费者而非组成部分。（lsp 当时作为 tools 的依赖闭包随迁进 SDK，后经证据评估于 2026-09-07 整体移除，见 [remove-lsp-tools.md](remove-lsp-tools.md)）
3. **kbvector 明确排除**：它依赖 `internal/agents/kb/tools`（应用层），耦合了 kb agent，不满足零领域知识门槛。
4. **架构规则测试化**：新增 `sdk/arch_test.go`，`go list -deps` 断言 sdk 不依赖 internal——把文档里的层间规则变成 CI 可拦截的硬约束。
5. **v0 不承诺兼容**：导出符号仍可能变，第一个正式消费方是 pi-go 自身的 Application 层；用 SDK 写出第一个新个人 agent 后再考虑冻结 v1。

## 备选与放弃原因

- **go.work 多 module（独立 pi-sdk 仓库）**：版本化最干净，但发布仪式感对单人项目过重；单 module 公共路径可逆且够用，将来需要独立版本化时再拆。
- **`pkg/` 命名**：Go 社区对 `pkg/` 的含义分歧大（"公共"或"垃圾场"），`sdk/` 语义直白。

## 影响

- 外部服务 import 路径：`github.com/hwj123hwj/pi-go/sdk/{agent,ai,…}`。
- 全仓 import 重写约 300 处，行为零变化（纯移动），`go build ./...`、`go vet`、全量测试保持绿。
- 防重机制：今后任何"领域逻辑想进内核"的 PR 会被 arch_test 当场拦下——SDK 边界同时是防膨胀纪律。
