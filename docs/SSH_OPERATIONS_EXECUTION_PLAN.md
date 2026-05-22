# Pi-Go SSH / Operations 执行文档

> 用途：指导执行 agent 为 `pi-go` 实现最小可用的 `Operations` 抽象，并在此基础上接入第一版 `SSH` 远程执行能力。  
> 目标不是一次性复刻原版 Pi 的全部远程执行体系，而是先完成可持续演进的底座。

## 1. 本次任务目标

本次实现分两层目标：

### 第一层：必须完成

- 从当前“工具直接绑定本地实现”的结构，抽出统一的 `Operations` 抽象
- 为现有 tools 接入 `LocalOperations`
- 保持当前本地行为不回归

### 第二层：在第一层完成后继续做

- 增加 `SSHOperations`
- 让工具能够在 `local` 和 `ssh` 两种执行后端之间切换
- 第一版只覆盖最关键的远程能力，不追求完整

## 2. 本次明确边界

### 本次要做

- `Operations` 接口设计与落地
- `LocalOperations`
- `SSHOperations v1`
- 配置接线
- 针对 `bash/read/write/ls/find` 的远程支持
- 针对 `edit` 的“远程读回 + 本地替换 + 远程写回”兼容实现
- 单元测试和最小集成测试

### 本次不要做

- RPC worker 模式
- 后台常驻进程管理
- 复杂流式工具输出
- 远程 mutation queue
- 多主机连接池
- Windows 远程路径支持
- GUI / Desktop 配置入口

## 3. 当前现状

当前 `pi-go` 的工具层是直接绑定本地实现的：

- [internal/tools/bash.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/tools/bash.go) — `Execute()` 方法直接调用 `exec.CommandContext`
- [internal/tools/read.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/tools/read.go) — `Execute()` 方法直接调用 `os.ReadFile`
- [internal/tools/write.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/tools/write.go) — `Execute()` 方法直接调用 `os.WriteFile`
- [internal/tools/edit.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/tools/edit.go) — `Execute()` 方法直接调用 `os.ReadFile` + `os.WriteFile`

`runtime` 当前在 [internal/runtime/agent_session.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/runtime/agent_session.go) 的 `buildToolList()` 方法（约 line 291）中通过 functional options 模式实例化各个工具，没有执行后端抽象层。

这会导致：

- 所有 tool 天生只能操作本地
- 后续接入 SSH 时必须逐个工具硬写分支
- server / Feishu / 未来 TUI 的远程工作区能力都会受限

## 4. 设计原则

### 4.1 先抽象，后远程

绝对不要直接在每个 tool 里加：

```go
if cfg.SSHHost != "" {
    // ssh 执行
} else {
    // 本地执行
}
```

这样短期能跑，后续会快速失控。

### 4.2 工具层不关心执行位置

tools 应只表达“我要读文件 / 写文件 / 跑命令”，不关心目标是：

- 本地
- SSH 远端
- 未来 RPC worker

### 4.3 优先保证兼容性

本次改造后：

- 本地模式的用户体验不能明显回退
- 已有 CLI / server / desktop 的调用方式尽量不变
- 配置新增应向后兼容

## 5. 推荐目录结构

建议新增：

```text
internal/operations/
  interface.go
  local.go
  ssh.go
  ssh_test.go
```

说明：

- `operations` 包负责执行后端
- `tools` 包继续负责 tool 参数校验、输出格式化、错误包装
- 不要把 SSH 逻辑直接塞回 `tools`

## 6. 接口设计

建议先做最小接口，不要一开始拆太细。

### 6.1 BashOperations

```go
type BashOperations interface {
    Run(ctx context.Context, req RunRequest) (RunResult, error)
}
```

建议结构：

```go
type RunRequest struct {
    Command string
    Timeout time.Duration
    WorkDir string
}

type RunResult struct {
    Output   []byte
    ExitCode int
}
```

### 6.2 FileOperations

```go
type FileOperations interface {
    ReadFile(ctx context.Context, path string) ([]byte, error)
    WriteFile(ctx context.Context, path string, data []byte, perm os.FileMode) error
    MkdirAll(ctx context.Context, dir string, perm os.FileMode) error
    Stat(ctx context.Context, path string) (FileInfo, error)
    ReadDir(ctx context.Context, path string) ([]DirEntry, error)
    Walk(ctx context.Context, root string, fn WalkFunc) error
}
```

说明：

- 第一版不必完全对齐 `os` 标准库的全部语义
- 只要能支撑 `read/write/edit/ls/find`
- `grep` 本次可以先维持本地 Go 扫描，或延后再接 operations

注意：路径安全检查（`ResolvePath()` / `IsPathSafe()`）应保留在 tools 层，不随 Operations 迁移。Operations 接收到的路径应该已经是经过安全校验的。

### 6.3 CombinedOperations

可选做一个组合容器：

```go
type Operations struct {
    Bash  BashOperations
    Files FileOperations
}
```

## 7. LocalOperations 实现要求

`LocalOperations` 是本次第一优先级，必须先完成。

要求：

- 行为与当前本地工具语义保持一致
- 继续复用 workspace 路径安全检查
- 继续支持当前 timeout、max output truncation 等外围逻辑

建议：

- 路径解析与 `workspace` 校验仍保留在 tools 层
- `LocalOperations` 只负责“已经被允许执行的本地操作”

## 8. SSHOperations v1 设计

### 8.1 支持能力

第一版只支持：

- `bash`
- `read`
- `write`
- `ls`
- `find`
- `edit` 通过 `read + write` 兼容实现

### 8.2 不支持能力

第一版不支持：

- detached/background command
- 流式 stdout/stderr
- 多段 patch 原子提交
- 文件锁
- 远程图片读取

### 8.3 推荐实现方式

第一版可以直接基于本机 `ssh` 命令实现，不必引入额外 Go SSH 客户端库。

原因：

- 依赖更少
- 调试成本低
- 与用户已有 SSH 配置兼容

建议思路：

- `bash`: `ssh user@host -- 'cd <workdir> && <command>'`
- `read`: `ssh user@host -- 'cat <path>'`
- `write`: 通过 `ssh + cat > file` 或 `scp` 写入
- `ls/find`: 优先通过远程 shell 命令实现

注意：

- 所有 shell 参数必须小心转义
- 不允许简单字符串拼接造成命令注入

## 9. 配置设计

建议在 [internal/config/config.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/config/config.go) 的 `Config` 结构体中新增最小配置：

```go
ExecutionMode string // local / ssh
SSHHost       string // user@host
SSHPort       int
SSHWorkDir    string
```

推荐环境变量：

- `PI_GO_EXECUTION_MODE=local|ssh`
- `PI_GO_SSH_HOST=user@host`
- `PI_GO_SSH_PORT=22`
- `PI_GO_SSH_WORKDIR=/path/to/workspace`

兼容策略：

- 默认 `ExecutionMode=local`
- 未配置 SSH 时保持当前行为不变

## 10. runtime 接线要求

在 [internal/runtime/agent_session.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/runtime/agent_session.go) 中：

注意：当前 `buildToolList()` 中还有以下逻辑，接入 Operations 时必须保留：

- **Tool 过滤**：`AllowedTools` / `BlockedTools` 过滤（约 line 341-342）
- **Extension tools**：从 registry 加载扩展工具（约 line 336-339），这些工具也需要 Operations 支持
- **Functional options 模式**：工具构造使用 `WithXxxWorkspace()` 等 option，新增 Operations 注入应保持一致风格

- 先根据配置构建 `Operations`
- 再把 `Operations` 注入各个工具

建议新增：

```go
func (s *AgentSession) buildOperations(cwd string) (*operations.Operations, error)
```

然后 `buildToolList()` 改成：

- 构建 operations
- 传入 tools 构造函数

目标是让 `runtime` 决定执行后端，而不是让 `tool` 自己读全局配置。

## 11. tools 迁移顺序

执行 agent 必须按以下顺序迁移，避免一次面太大：

### Step 1

迁移 `bash` 到 `BashOperations`

完成标志：

- 本地 `bash` 行为不变
- 单元测试通过

### Step 2

迁移 `read` 和 `write` 到 `FileOperations`

完成标志：

- 本地读写行为不变
- workspace 限制仍生效

### Step 3

迁移 `edit`

要求：

- `edit` 仍然保留当前精确匹配语义
- 不要在本次顺手扩展 edit 能力

### Step 4

迁移 `ls` 和 `find`

要求：

- 第一版只保证核心功能正确
- 输出格式尽量保持现有兼容

### Step 5

接入 `SSHOperations`

先覆盖：

- `bash`
- `read`
- `write`
- `edit`
- `ls`
- `find`

### Step 6

评估 `grep` 是否一起迁移

建议：

- 如果迁移复杂，允许本次先不做
- 但要在文档中明确记录为后续事项

原因说明：`grep` 是纯 Go 实现（`regexp` + `bufio.Scanner` 扫描文件），不像 `bash/ls/find` 能简单换成远程 shell 命令。远程化需要改成本地构造 grep 命令 → 远程执行 → 本地解析结果的模式，迁移成本比其他工具高。

## 12. 测试要求

### 12.1 必须新增的单元测试

- `LocalOperations.Run`
- `LocalOperations.ReadFile`
- `LocalOperations.WriteFile`
- `bash/read/write/edit/ls/find` 在注入 operations 后的行为测试

### 12.2 SSH 测试策略

本次不要求真实远程机集成测试作为强制门槛。

建议两层测试：

- 纯单元测试：验证命令构造、转义、错误处理
- 可选集成测试：当本机有可用 SSH target 时再跑

### 12.3 回归测试

执行 agent 在每个主要阶段后都要跑：

```bash
go test ./...
```

如果引入新的配置和 runtime 逻辑，至少补：

- `config` 测试
- `runtime` 测试

## 13. 验收标准

本次任务完成后，必须满足：

### A. 本地模式

- 默认配置下行为与现在兼容
- 不配置 SSH 时完全不影响现有使用者

### B. 代码结构

- tools 不再直接耦合本地 `os/exec` 作为唯一后端
- runtime 能明确选择 `local` 或 `ssh`
- 未来新增 `RPCOperations` 时，无需再改每个 tool 的核心逻辑

### C. SSH 基础能力

- 能通过配置启用 SSH 模式
- 能在远端执行：
  - `bash`
  - `read`
  - `write`
  - `edit`
  - `ls`
  - `find`

## 14. 明确禁止事项

执行 agent 在本次实现中不要做这些事：

- 不要顺手重构全部 tools 输出格式
- 不要顺手引入大型第三方远程执行框架
- 不要为了 SSH 先做 RPC
- 不要把 Desktop 配置面板一起做掉
- 不要把“远程能力”写成 extension demo 而不接入 runtime 主链路

## 15. 推荐提交策略

建议至少拆成以下几个提交或 PR 片段：

1. `operations` 抽象 + `LocalOperations`
2. `bash/read/write/edit/ls/find` 接入 operations
3. 配置与 runtime 接线
4. `SSHOperations v1`
5. 测试与文档补充

## 16. 给执行 agent 的一句话指令

先把 `Operations` 抽出来并让本地模式稳定运行，再在不破坏现有行为的前提下补 `SSHOperations v1`；不要跳过抽象层直接在各个工具里硬写 SSH 分支。
