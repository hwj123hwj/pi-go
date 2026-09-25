package workflow

import "context"

// Runner 是工作流步骤的执行端：一次 Prompt 调用 = 一步 Agent 会话。
type Runner interface {
	Prompt(ctx context.Context, prompt string) (string, error)
	Close() error
}

// RunnerOptions 描述一次步骤执行需要的会话形态。
type RunnerOptions struct {
	Model string // 模型覆盖；空 = 默认模型
	Label string // 人类可读标识（workflow/step），用于日志与会话元数据
}

// RunnerFactory 由应用层实现（如 internal/server 用 app.App 派生子会话），
// sdk/workflow 不依赖任何具体会话实现——这是 sdk 不依赖 internal 的边界线。
//
// 每次调用返回独立会话：步骤之间默认上下文隔离，prompt 需通过模板自包含。
// 实现方需保证并发安全（foreach 会并发调用）。
type RunnerFactory interface {
	NewRunner(ctx context.Context, opts RunnerOptions) (Runner, error)
}
