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
