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
