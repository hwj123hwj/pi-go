package agent

import (
	"github.com/earendil-works/pi-go/internal/ai"
)

type AgentEvent interface {
	agentEventMarker()
}

type EventAgentStart struct{}

func (EventAgentStart) agentEventMarker() {}

type EventAgentEnd struct {
	Messages []ai.Message
}

func (EventAgentEnd) agentEventMarker() {}

type EventTurnStart struct{}

func (EventTurnStart) agentEventMarker() {}

type EventTurnEnd struct {
	Message     ai.Message
	ToolResults []ai.Message
}

func (EventTurnEnd) agentEventMarker() {}

type EventToolExecutionStart struct {
	ToolCallID string
	ToolName   string
	Args       any
}

func (EventToolExecutionStart) agentEventMarker() {}

type EventToolExecutionUpdate struct {
	ToolCallID    string
	ToolName      string
	Args          any
	PartialResult any
}

func (EventToolExecutionUpdate) agentEventMarker() {}

type EventToolExecutionEnd struct {
	ToolCallID string
	ToolName   string
	Result     any
	IsError    bool
}

func (EventToolExecutionEnd) agentEventMarker() {}

// EventCompacted 上下文压缩完成事件。
type EventCompacted struct {
	Summary     string
	TrimmedFrom int
	TrimmedTo   int
}

func (EventCompacted) agentEventMarker() {}

// EventCompactionFailed 上下文压缩失败事件。
type EventCompactionFailed struct {
	Error string
}

func (EventCompactionFailed) agentEventMarker() {}

// EventToolBatchStart 在每个工具批次开始执行时发出。
// 用于 UI 展示和调试，帮助区分并行批次和串行批次。
type EventToolBatchStart struct {
	BatchIndex int      // 批次序号（从 0 开始）
	Safe       bool     // true = 并行批次, false = 串行批次
	ToolNames  []string // 批次内工具名称列表
}

func (EventToolBatchStart) agentEventMarker() {}
