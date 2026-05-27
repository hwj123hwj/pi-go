package agent

import (
	"context"
	"encoding/json"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Validate(params json.RawMessage) (json.RawMessage, error)
	Execute(ctx context.Context, params json.RawMessage, onUpdate func(PartialResult)) (ToolResult, error)
}

type ToolResult struct {
	Content string
	Details any
	IsError bool
}

type PartialResult struct {
	Content string
	Done    bool
}

// ExecutionMode 控制并行/顺序执行。
type ExecutionMode int

const (
	ExecutionModeParallel   ExecutionMode = iota
	ExecutionModeSequential
)

// ToolWithMode 可选接口：工具可覆盖默认执行模式。
type ToolWithMode interface {
	Tool
	ExecutionMode() ExecutionMode
}

// ToolWithPromptInfo 可选接口：工具提供系统提示中的 snippet 和 guidelines。
// 工具可以实现此接口，让系统提示构建器自动收集这些信息。
type ToolWithPromptInfo interface {
	Tool
	PromptSnippet() string      // 一句话描述工具用途，用于 Available tools 摘要
	PromptGuidelines() []string // 使用指南条目，追加到 Guidelines 区域
}

// ConcurrencySafeChecker 是一个可选接口（与 ToolWithMode 类似）。
// 工具可实现此接口以声明其是否可安全并发执行。
// 未实现此接口的工具默认视为不安全（保守策略）。
//
// 分区调度器 (partitionToolCalls) 在分批次时查询此接口。
// 若工具同时实现了 ToolWithMode 且 ExecutionMode() == Sequential，
// 即使 IsConcurrencySafe 返回 true，也会被保守地视为不安全。
//
// Extension 工具也可以实现此接口（Go duck typing 的自然结果，无需额外注册代码）。
type ConcurrencySafeChecker interface {
	Tool
	IsConcurrencySafe(params json.RawMessage) bool
}
