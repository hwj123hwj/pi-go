# 任务规划

> 本文档由主 Agent 在建群时自动生成，描述本群 agent 的工作目标。
> 文件持久保存在工作区根目录，可随时查看或编辑。

---

### 任务：移植增强工具到 pi-go

**目标**：从 hwjcode 移植多个增强工具到 pi-go，补全 Agent 的工具集。

**参考源码（hwjcode TypeScript）**：
- `/home/q/hwjcode/packages/core/src/tools/multiedit.ts` — 批量编辑
- `/home/q/hwjcode/packages/core/src/tools/patch.ts` — Patch 应用工具
- `/home/q/hwjcode/packages/core/src/tools/batch.ts` — 批量工具执行
- `/home/q/hwjcode/packages/core/src/tools/todo-write.ts` — Todo 管理
- `/home/q/hwjcode/packages/core/src/tools/todo-store.ts` — Todo 存储
- `/home/q/hwjcode/packages/core/src/tools/memoryTool.ts` — 长期记忆
- `/home/q/hwjcode/packages/core/src/tools/local-time.ts` — 本地时间
- `/home/q/hwjcode/packages/core/src/tools/delete-file.ts` — 删除文件
- `/home/q/hwjcode/packages/core/src/tools/read-many-files.ts` — 批量读文件
- `/home/q/hwjcode/packages/core/src/tools/ask-user-question.ts` — 用户交互提问

**输出到 pi-go（Go）**：
1. `internal/tools/multiedit.go` — 对同一文件执行多个 edit 操作（事务性，全部成功或全部回滚）
2. `internal/tools/patch.go` — 应用标准 unified diff patch
3. `internal/tools/batch.go` — 批量执行多个工具调用
4. `internal/tools/todo.go` — Todo 列表管理（CRUD + JSON 持久化到 `.pi-go/todo.json`）
5. `internal/tools/memory.go` — 长期记忆存储（追加到 `~/.pi-go/memory.json`）
6. `internal/tools/local_time.go` — 获取本地时间（支持时区参数）
7. `internal/tools/delete_file.go` — 安全删除文件（带备份）
8. `internal/tools/read_many_files.go` — 批量读取多个文件
9. `internal/tools/ask_user.go` — 向用户提问（多选/单选）

**pi-go 架构约定（必须遵守）**：
- 先 `read_file` 看 `internal/agent/tool.go` 了解 Tool 接口
- 先 `read_file` 看 `internal/tools/edit.go` 和 `internal/tools/write.go` 了解现有工具模式
- 先 `read_file` 看 `internal/tools/backup.go` 了解文件备份机制（delete_file 和 multiedit 要复用）
- 每个工具实现 `agent.Tool` 接口
- 用 `slog` 做日志
- multiedit 要复用 `internal/tools/edit.go` 的核心逻辑
- todo 和 memory 的持久化路径用 `os.UserHomeDir()` + `.pi-go/`
- ask_user 需要通过 channel 或 callback 与 UI 层交互（参考 pi-go 的 confirmation 机制）

**验收标准**：
- `go build ./...` 通过
- `go vet ./internal/tools/` 通过
- 每个新工具至少有一个基础测试
- multiedit 要有事务性测试（部分失败时全部回滚）
