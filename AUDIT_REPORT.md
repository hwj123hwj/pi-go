# Pi-Go 审核报告

日期：2026-05-21

本次审核基于当前工作区代码进行，重点检查了新引入的 `app / runtime / sessionmgr / server / tools` 分层是否把原有配置和 CLI/HTTP 入口真正接通，并验证是否存在回归。

## Findings

### [High] `-skill-dir` / `AppOptions.SkillDirs` 在新装配层里被直接丢弃，实际不会影响技能加载

位置：
- [cmd/pi-agent/main.go](/Users/weijian/Desktop/develop/test/pi/pi-go/cmd/pi-agent/main.go:33)
- [internal/app/app.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/app/app.go:28)
- [internal/app/app.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/app/app.go:123)
- [internal/runtime/agent_session.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/runtime/agent_session.go:202)

`main` 仍然把 `-skill-dir` 通过 `AppOptions.SkillDirs` 传给 `app.New()`，但 `App` 结构体没有保存这个字段，`skillDirs()` 还固定返回 `nil`。结果是 `NewSession()` / `LoadSession()` 传给 `AgentSessionOptions` 的始终都是空切片，runtime 最终只能回退到默认的 `.claude/skills` 查找逻辑。

这意味着命令行上的自定义技能目录参数现在是失效的，行为和 CLI/构造参数暴露出来的能力不一致。

### [High] 多个工具相关配置项已经声明并从环境变量读取，但运行时并没有真正生效

位置：
- [internal/config/config.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/config/config.go:32)
- [internal/runtime/agent_session.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/runtime/agent_session.go:245)
- [internal/tools/bash.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/tools/bash.go:24)
- [internal/tools/read.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/tools/read.go:93)
- [internal/tools/path.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/tools/path.go:13)

当前至少有三类配置没有被接通：

- `EnableBash`：配置默认是 `false`，也支持环境变量覆盖，但 `buildToolList()` 仍然无条件加入 `bash` 工具。
- `Workspace`：配置里有单独的工作区字段，但 `buildToolList()` 传给 `NewBashTool()` 的是 `cwd`，`read/write/edit/find` 也没有基于 `Workspace` 做路径解析或边界校验。
- `MaxOutputLen`：配置里可设置输出截断长度，但工具实现里都直接用 `DefaultMaxOutputLen` 常量，而不是读配置值。

`internal/tools/path.go` 新增了路径解析和安全检查辅助函数，但当前只在测试里被使用，尚未接入实际工具执行路径。

这类问题的影响不是“配置还没被消费”这么简单，因为它会让调用方误以为已经启用了工作区约束、bash 开关或输出长度控制，实际运行结果却不是这样。

### [Medium] `/tools` 接口返回硬编码列表，和真实可用工具集合已经可能不一致

位置：
- [internal/server/server.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/server/server.go:282)
- [internal/runtime/agent_session.go](/Users/weijian/Desktop/develop/test/pi/pi-go/internal/runtime/agent_session.go:260)

运行时的真实工具集合现在会受以下因素影响：

- extension tools 动态扩展
- `AllowedTools` / `BlockedTools` 过滤
- 后续可能接入的 `EnableBash` 等配置

但 `/tools` 仍然固定返回：

`bash, read, write, edit, grep, find, ls`

因此这个接口在当前架构下已经不是“真实能力枚举”，而只是一个静态占位实现。只要启用扩展或工具过滤，对外返回就会立刻失真。

## 验证

已执行：

```bash
go test ./...
```

当前测试通过，但现有测试没有覆盖以上几条接线问题，因此它们仍然会漏过。

## Summary

当前这轮重构把会话运行时和 HTTP 路由结构梳理清楚了，已有 bug 修复也基本保住了；不过新分层里还存在几处“参数/配置已经暴露，但实际未接通”的问题。建议优先补齐：

1. `SkillDirs` 从 `AppOptions` 到 `AgentSessionOptions` 的完整传递。
2. `EnableBash / Workspace / MaxOutputLen` 在工具构建和执行阶段的真实接线。
3. `/tools` 改为返回运行时真实可用的工具列表。
