package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
)

type BashTool struct {
	workspace string // 工作目录限制，空字符串表示不限制
}

type BashParams struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// NewBashTool 创建 BashTool。workspace 限制命令执行的工作目录。
func NewBashTool(workspace ...string) *BashTool {
	ws := ""
	if len(workspace) > 0 {
		ws = workspace[0]
	}
	return &BashTool{workspace: ws}
}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) Description() string { return "Execute a shell command on the server." }
func (t *BashTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}, "timeout": map[string]any{"type": "integer"}}, "required": []string{"command"}}
}
func (t *BashTool) Validate(raw json.RawMessage) (json.RawMessage, error) {
	var params BashParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if params.Command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if params.Timeout <= 0 {
		params.Timeout = 30
	}
	return json.Marshal(params)
}
func (t *BashTool) Execute(ctx context.Context, raw json.RawMessage, onUpdate func(agent.PartialResult)) (agent.ToolResult, error) {
	var params BashParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return agent.ToolResult{IsError: true}, err
	}
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "sh", "-c", params.Command)
	// 限制工作目录
	if t.workspace != "" {
		cmd.Dir = t.workspace
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return agent.ToolResult{Content: string(out), IsError: true}, err
	}
	return agent.ToolResult{Content: string(out)}, nil
}
