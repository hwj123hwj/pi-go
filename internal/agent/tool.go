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
